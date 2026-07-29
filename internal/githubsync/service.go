package githubsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"catenar/internal/contracts"
)

const (
	defaultBranch       = "main"
	maxWorkspaceBytes   = int64(128 * 1024 * 1024)
	linkStateFileName   = "github-workspace-links.json"
	gitCommitAuthorName = "Catenar"
	gitCommitAuthorMail = "catenar@users.noreply.github.com"
)

var safeRefPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

type CommandRunner interface {
	Run(ctx context.Context, dir string, env []string, args ...string) (string, error)
}

type Manager interface {
	Active() (contracts.WorkspaceSnapshot, bool)
	Open(ctx context.Context, path string) contracts.WorkspaceResponse
}

type Options struct {
	Manager         Manager
	ConfigDir       string
	Runner          CommandRunner
	CredentialStore CredentialStore
	Now             func() time.Time
}

type Service struct {
	mu          sync.Mutex
	manager     Manager
	configDir   string
	runner      CommandRunner
	now         func() time.Time
	credentials CredentialStore
}

type linkState struct {
	Link             contracts.GitHubWorkspaceLink `json:"link"`
	LastLocalDigest  string                        `json:"lastLocalDigest,omitempty"`
	LastRemoteDigest string                        `json:"lastRemoteDigest,omitempty"`
}

type persistedLinks struct {
	Links map[string]linkState `json:"links"`
}

type syncSnapshot struct {
	state         linkState
	workspace     contracts.WorkspaceSnapshot
	cachePath     string
	localDigest   string
	remoteDigest  string
	remoteCommit  string
	localChanged  bool
	remoteChanged bool
	initial       bool
}

func NewService(options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	configDir := strings.TrimSpace(options.ConfigDir)
	if configDir == "" {
		if userConfig, err := os.UserConfigDir(); err == nil {
			configDir = filepath.Join(userConfig, "catenar")
		} else {
			configDir = filepath.Join(os.TempDir(), "catenar")
		}
	}
	runner := options.Runner
	if runner == nil {
		runner = execRunner{}
	}
	credentials := options.CredentialStore
	if credentials == nil {
		credentials = defaultCredentialStore()
	}
	return &Service{manager: options.Manager, configDir: configDir, runner: runner, now: now, credentials: credentials}
}

func (s *Service) Link(ctx context.Context, input contracts.GitHubWorkspaceLinkInput) contracts.GitHubSyncResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceSnapshot, ok := s.activeWorkspace()
	if !ok {
		return failure("github_sync.workspace_required", "Open or create a workspace before linking GitHub sync.")
	}
	link, err := normalizeLink(input)
	if err != nil {
		return failure("github_sync.invalid_link", err.Error())
	}

	states, err := s.loadStates()
	if err != nil {
		return failure("github_sync.state_unreadable", "GitHub sync settings could not be read.")
	}
	key := workspaceSnapshot.ID
	if err := s.saveLinkToken(ctx, key, link.RepositoryURL, input.AccessToken); err != nil {
		return failure("github_sync.credential_store_failed", "The GitHub token could not be stored securely in the system credential store.")
	}
	if existing, exists := states.Links[key]; exists {
		if existing.Link.RepositoryURL != link.RepositoryURL || existing.Link.Branch != link.Branch {
			return failure("github_sync.already_linked", "Unlink the current repository before linking a different one.")
		}
		if existing.Link.WorkspacePath == link.WorkspacePath {
			return s.statusLocked(ctx, workspaceSnapshot, states)
		}

		// The repository cache belongs to the workspace and can safely be reused
		// when only the managed Catenar folder changes. Resetting the sync revision
		// makes the next status explicit instead of comparing unrelated folders.
		states.Links[key] = linkState{Link: link}
		if err := s.saveStates(states); err != nil {
			return failure("github_sync.state_write_failed", "The GitHub sync settings could not be saved.")
		}
		snapshot, response := s.snapshotLocked(ctx, workspaceSnapshot, states)
		if response != nil {
			return *response
		}
		return successStatus(s.statusFromSnapshot(snapshot))
	}

	cachePath := s.cachePath(key)
	if _, err := os.Stat(cachePath); err == nil {
		return failure("github_sync.cache_exists", "The local GitHub sync cache already exists. Unlink the old connection first.")
	} else if !errors.Is(err, os.ErrNotExist) {
		return failure("github_sync.cache_unreadable", "The local GitHub sync cache could not be checked.")
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		return failure("github_sync.cache_create_failed", "The GitHub sync cache could not be created.")
	}
	tempCache := cachePath + ".partial"
	_ = os.RemoveAll(tempCache)
	if _, err := s.gitRemote(ctx, key, link, "", "clone", "--single-branch", "--branch", link.Branch, "--", link.RepositoryURL, tempCache); err != nil {
		_ = os.RemoveAll(tempCache)
		return failure("github_sync.clone_failed", "The repository could not be cloned. Check the URL, branch and GitHub credentials configured for Git.")
	}
	if err := os.Rename(tempCache, cachePath); err != nil {
		_ = os.RemoveAll(tempCache)
		return failure("github_sync.cache_create_failed", "The cloned repository could not be stored in the Catenar cache.")
	}

	state := linkState{Link: link}
	states.Links[key] = state
	if err := s.saveStates(states); err != nil {
		_ = os.RemoveAll(cachePath)
		return failure("github_sync.state_write_failed", "The GitHub sync settings could not be saved.")
	}
	snapshot, response := s.snapshotLocked(ctx, workspaceSnapshot, states)
	if response != nil {
		return *response
	}
	if snapshot.localDigest == snapshot.remoteDigest {
		updated, err := s.refreshState(snapshot, true)
		if err != nil {
			return failure("github_sync.state_write_failed", "The initial GitHub sync revision could not be recorded.")
		}
		states.Links[key] = updated.state
		if err := s.saveStates(states); err != nil {
			return failure("github_sync.state_write_failed", "The initial GitHub sync revision could not be recorded.")
		}
		return successStatus(s.statusFromSnapshot(updated))
	}
	return successStatus(s.statusFromSnapshot(snapshot))
}

// SetToken updates the HTTPS Personal Access Token for the active workspace.
// It deliberately does not touch the workspace manifest or the Git remote URL.
func (s *Service) SetToken(ctx context.Context, input contracts.GitHubWorkspaceCredentialInput) contracts.GitHubSyncResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceSnapshot, ok := s.activeWorkspace()
	if !ok {
		return failure("github_sync.workspace_required", "Open or create a workspace before configuring GitHub authentication.")
	}
	states, err := s.loadStates()
	if err != nil {
		return failure("github_sync.state_unreadable", "GitHub sync settings could not be read.")
	}
	state, linked := states.Links[workspaceSnapshot.ID]
	if !linked {
		return failure("github_sync.not_linked", "Link this workspace to a GitHub repository before configuring authentication.")
	}
	if !isHTTPSRepositoryURL(state.Link.RepositoryURL) {
		return failure("github_sync.ssh_credential_not_needed", "This workspace uses SSH. Configure its SSH key instead of an HTTPS token.")
	}
	if err := s.saveLinkToken(ctx, workspaceSnapshot.ID, state.Link.RepositoryURL, input.AccessToken); err != nil {
		return failure("github_sync.credential_store_failed", "The GitHub token could not be stored securely in the system credential store.")
	}
	link := state.Link
	return successStatus(contracts.GitHubSyncStatus{
		Linked:          true,
		TokenConfigured: true,
		Link:            &link,
		Workspace:       &workspaceSnapshot,
	})
}

func (s *Service) Status(ctx context.Context) contracts.GitHubSyncResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceSnapshot, ok := s.activeWorkspace()
	if !ok {
		return successStatus(contracts.GitHubSyncStatus{Linked: false})
	}
	states, err := s.loadStates()
	if err != nil {
		return failure("github_sync.state_unreadable", "GitHub sync settings could not be read.")
	}
	return s.statusLocked(ctx, workspaceSnapshot, states)
}

func (s *Service) Pull(ctx context.Context, input contracts.GitHubSyncActionInput) contracts.GitHubSyncResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceSnapshot, ok := s.activeWorkspace()
	if !ok {
		return failure("github_sync.workspace_required", "Open or create a workspace before pulling GitHub sync.")
	}
	states, err := s.loadStates()
	if err != nil {
		return failure("github_sync.state_unreadable", "GitHub sync settings could not be read.")
	}
	snapshot, response := s.snapshotLocked(ctx, workspaceSnapshot, states)
	if response != nil {
		return *response
	}
	if !snapshot.state.LinkIsPresent() {
		return failure("github_sync.not_linked", "Link this workspace to a GitHub repository first.")
	}
	if (snapshot.initial && snapshot.localDigest != "" && snapshot.remoteDigest != "" || snapshot.localChanged) && !input.Overwrite {
		return failure("github_sync.conflict", "Local workspace changes must be saved or resolved before pulling remote files.")
	}
	if !snapshot.remoteChanged {
		return successStatus(s.statusFromSnapshot(snapshot))
	}
	if _, err := s.gitRemote(ctx, workspaceSnapshot.ID, snapshot.state.Link, snapshot.cachePath, "pull", "--ff-only", "origin", snapshot.state.Link.Branch); err != nil {
		return failure("github_sync.pull_failed", "Remote changes could not be pulled safely. Resolve the Git history outside Catenar, then retry.")
	}
	if head, err := s.git(ctx, snapshot.cachePath, "rev-parse", "HEAD"); err == nil {
		snapshot.remoteCommit = strings.TrimSpace(head)
	}
	remoteWorkspacePath := filepath.Join(snapshot.cachePath, snapshot.state.Link.WorkspacePath)
	if info, err := os.Stat(remoteWorkspacePath); err != nil || !info.IsDir() {
		return failure("github_sync.remote_workspace_missing", "The linked workspace folder is missing from the remote repository.")
	}
	if err := copyWorkspaceContents(remoteWorkspacePath, snapshot.workspace.Path, filepath.Base(snapshot.workspace.ManifestPath)); err != nil {
		return failure("github_sync.workspace_copy_failed", "Remote workspace files could not be applied locally.")
	}
	opened := s.manager.Open(ctx, snapshot.workspace.ManifestPath)
	if !opened.Ok || opened.Data == nil {
		return failure("github_sync.workspace_invalid", "The remote workspace was not valid and was not opened.")
	}
	snapshot.workspace = opened.Data.Workspace
	updated, err := s.refreshState(snapshot, true)
	if err != nil {
		return failure("github_sync.state_write_failed", "The GitHub sync revision could not be recorded.")
	}
	states.Links[snapshot.workspace.ID] = updated.state
	if err := s.saveStates(states); err != nil {
		return failure("github_sync.state_write_failed", "The GitHub sync revision could not be recorded.")
	}
	return successStatus(s.statusFromSnapshot(updated))
}

func (s *Service) Push(ctx context.Context, input contracts.GitHubSyncActionInput) contracts.GitHubSyncResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceSnapshot, ok := s.activeWorkspace()
	if !ok {
		return failure("github_sync.workspace_required", "Open or create a workspace before pushing GitHub sync.")
	}
	states, err := s.loadStates()
	if err != nil {
		return failure("github_sync.state_unreadable", "GitHub sync settings could not be read.")
	}
	snapshot, response := s.snapshotLocked(ctx, workspaceSnapshot, states)
	if response != nil {
		return *response
	}
	if !snapshot.state.LinkIsPresent() {
		return failure("github_sync.not_linked", "Link this workspace to a GitHub repository first.")
	}
	if (snapshot.initial && snapshot.localDigest != "" && snapshot.remoteDigest != "" || snapshot.remoteChanged) && !input.Overwrite {
		return failure("github_sync.conflict", "Remote changes are available. Pull them before pushing local changes.")
	}
	if !snapshot.localChanged {
		return successStatus(s.statusFromSnapshot(snapshot))
	}
	if input.Overwrite && snapshot.remoteChanged {
		if _, err := s.gitRemote(ctx, workspaceSnapshot.ID, snapshot.state.Link, snapshot.cachePath, "pull", "--ff-only", "origin", snapshot.state.Link.Branch); err != nil {
			return failure("github_sync.pull_failed", "Remote Git history could not be updated safely before applying the local workspace.")
		}
		if head, err := s.git(ctx, snapshot.cachePath, "rev-parse", "HEAD"); err == nil {
			snapshot.remoteCommit = strings.TrimSpace(head)
		}
		remoteDigest, err := s.remoteWorkspaceTreeDigest(
			ctx,
			snapshot.cachePath,
			"refs/remotes/origin/"+snapshot.state.Link.Branch,
			snapshot.state.Link.WorkspacePath,
			filepath.Base(workspaceSnapshot.ManifestPath),
		)
		if err != nil {
			return failure("github_sync.remote_workspace_read_failed", "The updated remote workspace could not be inspected.")
		}
		snapshot.remoteDigest = remoteDigest
		if snapshot.localDigest == remoteDigest {
			updated, err := s.refreshState(snapshot, true)
			if err != nil {
				return failure("github_sync.state_write_failed", "The GitHub sync revision could not be recorded.")
			}
			states.Links[workspaceSnapshot.ID] = updated.state
			if err := s.saveStates(states); err != nil {
				return failure("github_sync.state_write_failed", "The GitHub sync revision could not be recorded.")
			}
			return successStatus(s.statusFromSnapshot(updated))
		}
	}
	if err := copyWorkspaceContents(snapshot.workspace.Path, filepath.Join(snapshot.cachePath, snapshot.state.Link.WorkspacePath), filepath.Base(snapshot.workspace.ManifestPath)); err != nil {
		return failure("github_sync.workspace_copy_failed", "Local workspace files could not be staged for GitHub sync.")
	}
	if _, err := s.git(ctx, snapshot.cachePath, "add", "-A", "--", snapshot.state.Link.WorkspacePath); err != nil {
		return failure("github_sync.stage_failed", "The workspace changes could not be staged.")
	}
	if output, err := s.git(ctx, snapshot.cachePath, "commit", "-m", "Sync Catenar workspace", "--", snapshot.state.Link.WorkspacePath); err != nil && !isNothingToCommit(output) {
		return failureWithCause("github_sync.commit_failed", "The workspace changes could not be committed. Check the repository state and Git configuration.", outputError(output, err))
	}
	if output, err := s.gitRemote(ctx, workspaceSnapshot.ID, snapshot.state.Link, snapshot.cachePath, "push", "-u", "origin", "HEAD:"+snapshot.state.Link.Branch); err != nil {
		return failureWithCause("github_sync.push_failed", "The commit was kept locally, but GitHub rejected the push. Pull and resolve the remote changes before retrying.", outputError(output, err))
	}
	if head, err := s.git(ctx, snapshot.cachePath, "rev-parse", "HEAD"); err == nil {
		snapshot.remoteCommit = strings.TrimSpace(head)
	}
	updated, err := s.refreshState(snapshot, true)
	if err != nil {
		return failure("github_sync.state_write_failed", "The GitHub sync revision could not be recorded.")
	}
	states.Links[workspaceSnapshot.ID] = updated.state
	if err := s.saveStates(states); err != nil {
		return failure("github_sync.state_write_failed", "The GitHub sync revision could not be recorded.")
	}
	return successStatus(s.statusFromSnapshot(updated))
}

func isNothingToCommit(output string) bool {
	return strings.Contains(strings.ToLower(output), "nothing to commit")
}

func (s *Service) Unlink(_ context.Context) contracts.GitHubSyncResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceSnapshot, ok := s.activeWorkspace()
	if !ok {
		return successStatus(contracts.GitHubSyncStatus{Linked: false})
	}
	states, err := s.loadStates()
	if err != nil {
		return failure("github_sync.state_unreadable", "GitHub sync settings could not be read.")
	}
	if state, exists := states.Links[workspaceSnapshot.ID]; exists {
		cachePath := s.cachePath(workspaceSnapshot.ID)
		if err := ensureWithin(s.configDir, cachePath); err == nil {
			_ = os.RemoveAll(cachePath)
		}
		_ = state
	}
	if s.credentials != nil {
		_ = s.credentials.Delete(context.Background(), s.credentialAccount(workspaceSnapshot.ID))
	}
	delete(states.Links, workspaceSnapshot.ID)
	if err := s.saveStates(states); err != nil {
		return failure("github_sync.state_write_failed", "The GitHub sync settings could not be removed.")
	}
	return successStatus(contracts.GitHubSyncStatus{Linked: false})
}

func (s *Service) activeWorkspace() (contracts.WorkspaceSnapshot, bool) {
	if s.manager == nil {
		return contracts.WorkspaceSnapshot{}, false
	}
	return s.manager.Active()
}

func (s *Service) statusLocked(ctx context.Context, workspaceSnapshot contracts.WorkspaceSnapshot, states persistedLinks) contracts.GitHubSyncResponse {
	snapshot, response := s.snapshotLocked(ctx, workspaceSnapshot, states)
	if response != nil {
		return *response
	}
	return successStatus(s.statusFromSnapshot(snapshot))
}

func (s *Service) snapshotLocked(ctx context.Context, workspaceSnapshot contracts.WorkspaceSnapshot, states persistedLinks) (syncSnapshot, *contracts.GitHubSyncResponse) {
	state, exists := states.Links[workspaceSnapshot.ID]
	if !exists {
		return syncSnapshot{workspace: workspaceSnapshot}, nil
	}
	cachePath := s.cachePath(workspaceSnapshot.ID)
	manifestName := filepath.Base(workspaceSnapshot.ManifestPath)
	localDigest, err := workspaceTreeDigest(workspaceSnapshot.Path, manifestName)
	if err != nil {
		response := failureWithCause("github_sync.workspace_read_failed", "The local workspace could not be inspected for sync.", err)
		return syncSnapshot{}, &response
	}
	if _, err := os.Stat(cachePath); err != nil {
		response := failure("github_sync.cache_missing", "The GitHub sync cache is missing. Unlink and link the repository again.")
		return syncSnapshot{}, &response
	}
	if _, err := s.gitRemote(ctx, workspaceSnapshot.ID, state.Link, cachePath, "fetch", "origin", state.Link.Branch); err != nil {
		response := failure("github_sync.fetch_failed", "GitHub could not be reached. Local workspace editing and saving are still available.")
		return syncSnapshot{}, &response
	}
	remoteDigest, err := s.remoteWorkspaceTreeDigest(
		ctx,
		cachePath,
		"refs/remotes/origin/"+state.Link.Branch,
		state.Link.WorkspacePath,
		manifestName,
	)
	if err != nil {
		response := failureWithCause("github_sync.remote_workspace_read_failed", "The remote workspace could not be inspected.", err)
		return syncSnapshot{}, &response
	}
	remoteCommit, _ := s.git(ctx, cachePath, "rev-parse", "refs/remotes/origin/"+state.Link.Branch)
	remoteCommit = strings.TrimSpace(remoteCommit)
	// The cache can contain a locally committed workspace after a rejected
	// push. Always compare the remote digest against refs/remotes/origin rather
	// than the cache working tree, otherwise a local-only commit is presented as
	// a remote change in the UI.
	localChanged := state.LastLocalDigest == "" && localDigest != remoteDigest || state.LastLocalDigest != "" && localDigest != state.LastLocalDigest
	remoteChanged := state.LastRemoteDigest != "" && remoteDigest != state.LastRemoteDigest
	initial := state.Link.LastSyncedCommit == "" && localDigest != remoteDigest
	return syncSnapshot{
		state: state, workspace: workspaceSnapshot, cachePath: cachePath,
		localDigest: localDigest, remoteDigest: remoteDigest, remoteCommit: remoteCommit,
		localChanged: localChanged, remoteChanged: remoteChanged, initial: initial,
	}, nil
}

func (s *Service) refreshState(snapshot syncSnapshot, updateCommit bool) (syncSnapshot, error) {
	manifestName := filepath.Base(snapshot.workspace.ManifestPath)
	localDigest, err := workspaceTreeDigest(snapshot.workspace.Path, manifestName)
	if err != nil {
		return syncSnapshot{}, err
	}
	remoteDigest, err := workspaceTreeDigest(filepath.Join(snapshot.cachePath, snapshot.state.Link.WorkspacePath), manifestName)
	if err != nil {
		return syncSnapshot{}, err
	}
	commit := snapshot.remoteCommit
	if updateCommit {
		commit = strings.TrimSpace(commit)
	}
	snapshot.state.LastLocalDigest = localDigest
	snapshot.state.LastRemoteDigest = remoteDigest
	syncedAt := s.now().UTC().Format(time.RFC3339)
	if updateCommit {
		snapshot.state.Link.LastSyncedCommit = commit
		snapshot.state.Link.LastSyncedAt = syncedAt
	}
	snapshot.localDigest, snapshot.remoteDigest = localDigest, remoteDigest
	snapshot.remoteCommit = commit
	snapshot.localChanged, snapshot.remoteChanged, snapshot.initial = false, false, false
	return snapshot, nil
}

func (s *Service) statusFromSnapshot(snapshot syncSnapshot) contracts.GitHubSyncStatus {
	status := contracts.GitHubSyncStatus{
		Linked:              snapshot.state.LinkIsPresent(),
		LocalChanges:        snapshot.localChanged,
		RemoteChanges:       snapshot.remoteChanged,
		Conflict:            snapshot.initial && snapshot.localDigest != "" && snapshot.remoteDigest != "" || snapshot.localChanged && snapshot.remoteChanged,
		InitialSyncRequired: snapshot.initial,
		RemoteCommit:        snapshot.remoteCommit,
		Link:                nil,
		Workspace:           &snapshot.workspace,
	}
	if status.Linked {
		link := snapshot.state.Link
		status.Link = &link
		if isHTTPSRepositoryURL(link.RepositoryURL) && s.credentials != nil {
			_, status.TokenConfigured, _ = s.credentials.Get(context.Background(), s.credentialAccount(snapshot.workspace.ID))
		}
	}
	return status
}

func (s *Service) cachePath(workspaceID string) string {
	digest := sha256.Sum256([]byte(workspaceID))
	return filepath.Join(s.configDir, "github-sync", hex.EncodeToString(digest[:])[:24])
}

func (s *Service) statePath() string { return filepath.Join(s.configDir, linkStateFileName) }

func (s *Service) loadStates() (persistedLinks, error) {
	content, err := os.ReadFile(s.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return persistedLinks{Links: make(map[string]linkState)}, nil
	}
	if err != nil {
		return persistedLinks{}, err
	}
	var states persistedLinks
	if err := json.Unmarshal(content, &states); err != nil {
		return persistedLinks{}, err
	}
	if states.Links == nil {
		states.Links = make(map[string]linkState)
	}
	return states, nil
}

func (s *Service) saveStates(states persistedLinks) error {
	if err := os.MkdirAll(s.configDir, 0o700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return err
	}
	temporary := s.statePath() + ".tmp"
	if err := os.WriteFile(temporary, append(content, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, s.statePath())
}

func (s *Service) git(ctx context.Context, dir string, args ...string) (string, error) {
	return s.runner.Run(ctx, dir, []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_AUTHOR_NAME=" + gitCommitAuthorName,
		"GIT_AUTHOR_EMAIL=" + gitCommitAuthorMail,
		"GIT_COMMITTER_NAME=" + gitCommitAuthorName,
		"GIT_COMMITTER_EMAIL=" + gitCommitAuthorMail,
	}, args...)
}

func (s *Service) gitRemote(ctx context.Context, workspaceID string, link contracts.GitHubWorkspaceLink, dir string, args ...string) (string, error) {
	env := s.gitEnvironment()
	if !isHTTPSRepositoryURL(link.RepositoryURL) || s.credentials == nil {
		return s.runner.Run(ctx, dir, env, args...)
	}
	token, configured, err := s.credentials.Get(ctx, s.credentialAccount(workspaceID))
	if err != nil {
		// Credential storage is an optional authentication source. Preserve the
		// existing SSH/system-Git path if Keychain access is temporarily denied.
		return s.runner.Run(ctx, dir, env, args...)
	}
	if !configured {
		return s.runner.Run(ctx, dir, env, args...)
	}
	credentialEnv, cleanup, err := gitAskPassEnvironment(token)
	if err != nil {
		return "", err
	}
	defer cleanup()
	return s.runner.Run(ctx, dir, append(env, credentialEnv...), args...)
}

func (s *Service) gitEnvironment() []string {
	return []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_AUTHOR_NAME=" + gitCommitAuthorName,
		"GIT_AUTHOR_EMAIL=" + gitCommitAuthorMail,
		"GIT_COMMITTER_NAME=" + gitCommitAuthorName,
		"GIT_COMMITTER_EMAIL=" + gitCommitAuthorMail,
	}
}

func (s *Service) credentialAccount(workspaceID string) string {
	digest := sha256.Sum256([]byte(workspaceID))
	return "workspace-" + hex.EncodeToString(digest[:])
}

func (s *Service) saveLinkToken(ctx context.Context, workspaceID, repositoryURL, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !isHTTPSRepositoryURL(repositoryURL) {
		return errors.New("an HTTPS token can only be used with an HTTPS repository URL")
	}
	if !validAccessToken(value) {
		return errors.New("GitHub token is malformed")
	}
	if s.credentials == nil {
		return errCredentialStoreUnavailable
	}
	return s.credentials.Set(ctx, s.credentialAccount(workspaceID), value)
}

func isHTTPSRepositoryURL(repositoryURL string) bool {
	parsed, err := url.Parse(repositoryURL)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() == "github.com"
}

func validAccessToken(value string) bool {
	if len(value) < 20 || len(value) > 512 {
		return false
	}
	return !strings.ContainsAny(value, " \t\r\n")
}

func gitAskPassEnvironment(token string) ([]string, func(), error) {
	directory, err := os.MkdirTemp("", "catenar-git-askpass-")
	if err != nil {
		return nil, nil, err
	}
	scriptPath := filepath.Join(directory, "askpass")
	const script = "#!/bin/sh\ncase \"$1\" in\n  *Username*) printf '%s\\n' x-access-token ;;\n  *) printf '%s\\n' \"$CATENAR_GITHUB_TOKEN\" ;;\nesac\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		_ = os.RemoveAll(directory)
		return nil, nil, err
	}
	return []string{
		"GIT_ASKPASS=" + scriptPath,
		"GIT_ASKPASS_REQUIRE=force",
		"CATENAR_GITHUB_TOKEN=" + token,
	}, func() { _ = os.RemoveAll(directory) }, nil
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), env...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return output.String(), err
	}
	return output.String(), nil
}

func normalizeLink(input contracts.GitHubWorkspaceLinkInput) (contracts.GitHubWorkspaceLink, error) {
	repositoryURL := strings.TrimSpace(input.RepositoryURL)
	if !validGitHubURL(repositoryURL) {
		return contracts.GitHubWorkspaceLink{}, errors.New("use a GitHub HTTPS URL or git@github.com SSH URL")
	}
	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		branch = defaultBranch
	}
	if !safeGitRef(branch) {
		return contracts.GitHubWorkspaceLink{}, errors.New("branch contains unsupported path characters")
	}
	workspacePath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(input.WorkspacePath)))
	if workspacePath == "." || !safeRelativePath(workspacePath) {
		return contracts.GitHubWorkspaceLink{}, errors.New("workspace path must be a safe relative folder")
	}
	return contracts.GitHubWorkspaceLink{RepositoryURL: repositoryURL, Branch: branch, WorkspacePath: workspacePath}, nil
}

func validGitHubURL(value string) bool {
	if strings.HasPrefix(value, "git@github.com:") {
		return strings.Trim(strings.TrimPrefix(value, "git@github.com:"), "/") != ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() != "github.com" || parsed.User != nil || parsed.Path == "" {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "ssh"
}

func safeGitRef(value string) bool {
	return safeRefPattern.MatchString(value) && !strings.HasPrefix(value, "/") && !strings.HasSuffix(value, "/") && !strings.Contains(value, "..") && !strings.Contains(value, "//") && !strings.Contains(value, "@{")
}

func safeRelativePath(value string) bool {
	return value != "" && !filepath.IsAbs(value) && !strings.HasPrefix(value, "../") && value != ".." && !strings.Contains(value, "/../") && !strings.Contains(value, "\\") && !strings.HasPrefix(value, ".git")
}

func ensureWithin(base, candidate string) error {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("path escapes base directory")
	}
	return nil
}

// copyWorkspaceContents syncs only the workspace manifest and saved requests.
// A workspace can live inside an existing project, so copying or deleting the
// entire root would accidentally include node_modules, build artifacts, or
// user-owned project files.
func copyWorkspaceContents(source, destination, manifestName string) error {
	if err := ensureWithin(filepath.Dir(destination), destination); err != nil {
		return err
	}
	if sourceInfo, err := os.Stat(source); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.MkdirAll(destination, 0o755)
		}
		return err
	} else if !sourceInfo.IsDir() {
		return errors.New("workspace sync source is not a directory")
	}
	manifestName = safeWorkspaceManifestName(manifestName)
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(destination, manifestName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.RemoveAll(filepath.Join(destination, "requests")); err != nil {
		return err
	}
	var total int64
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == source {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("workspace sync path escapes source directory")
		}
		if entry.IsDir() {
			if filepath.ToSlash(relative) != "requests" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(destination, relative), 0o755)
		}
		if !isManagedWorkspaceRelative(filepath.ToSlash(relative), manifestName) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > maxWorkspaceBytes {
			return errors.New("workspace exceeds the sync size limit")
		}
		if err := ensureWithin(destination, filepath.Join(destination, relative)); err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destination, relative), content, info.Mode().Perm())
	})
}

func treeDigest(root string) (string, error) {
	return workspaceTreeDigest(root, "workspace.yaml")
}

func workspaceTreeDigest(root, manifestName string) (string, error) {
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("workspace sync root is not a directory")
	}
	manifestName = safeWorkspaceManifestName(manifestName)
	entries := make([]workspaceDigestEntry, 0)
	var total int64
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if path == root || filepath.ToSlash(relativeWorkspacePath(root, path)) == "requests" {
				return nil
			}
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !isManagedWorkspaceRelative(filepath.ToSlash(relative), manifestName) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		total += int64(len(content))
		if total > maxWorkspaceBytes {
			return errors.New("workspace exceeds the sync size limit")
		}
		entries = append(entries, workspaceDigestEntry{path: filepath.ToSlash(relative), mode: entry.Type(), data: content})
		return nil
	})
	if err != nil {
		return "", err
	}
	return digestWorkspaceEntries(entries), nil
}

type workspaceDigestEntry struct {
	path string
	mode fs.FileMode
	data []byte
}

func digestWorkspaceEntries(entries []workspaceDigestEntry) string {
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	hash := sha256.New()
	for _, entry := range entries {
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", entry.path, entry.mode.String())
		_, _ = hash.Write(entry.data)
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// remoteWorkspaceTreeDigest reads only managed workspace files from the
// fetched origin ref. The cache worktree itself is intentionally not used: it
// can include commits that failed to push because the user's Git credentials
// were rejected.
func (s *Service) remoteWorkspaceTreeDigest(ctx context.Context, cachePath, remoteRef, workspacePath, manifestName string) (string, error) {
	listing, err := s.git(ctx, cachePath, "ls-tree", "-r", "-z", "--full-tree", remoteRef, "--", workspacePath)
	if err != nil {
		return "", err
	}
	entries := make([]workspaceDigestEntry, 0)
	workspacePrefix := strings.TrimSuffix(filepath.ToSlash(workspacePath), "/") + "/"
	for _, record := range strings.Split(listing, "\x00") {
		if record == "" {
			continue
		}
		metadata, path, found := strings.Cut(record, "\t")
		if !found {
			return "", errors.New("remote workspace tree entry is malformed")
		}
		parts := strings.Fields(metadata)
		if len(parts) != 3 || parts[1] != "blob" || !strings.HasPrefix(path, workspacePrefix) {
			continue
		}
		relative := strings.TrimPrefix(path, workspacePrefix)
		if !isManagedWorkspaceRelative(relative, manifestName) {
			continue
		}
		content, err := s.git(ctx, cachePath, "show", remoteRef+":"+path)
		if err != nil {
			return "", err
		}
		entries = append(entries, workspaceDigestEntry{path: relative, data: []byte(content)})
	}
	if len(entries) == 0 {
		return "", nil
	}
	return digestWorkspaceEntries(entries), nil
}

func safeWorkspaceManifestName(value string) string {
	name := filepath.Base(strings.TrimSpace(value))
	if name == "." || name == "" {
		return "workspace.yaml"
	}
	return name
}

func isManagedWorkspaceRelative(relative, manifestName string) bool {
	return relative == manifestName || relative == "requests" || strings.HasPrefix(relative, "requests/")
}

func relativeWorkspacePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	return relative
}

func failure(code, message string) contracts.GitHubSyncResponse {
	return contracts.GitHubSyncResponse{Ok: false, Error: ptr(contracts.NewErrorEnvelope(code, contracts.ErrorCategoryWorkspace, message, nil))}
}

func failureWithCause(code, message string, cause error) contracts.GitHubSyncResponse {
	details := map[string]string{}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return contracts.GitHubSyncResponse{Ok: false, Error: ptr(contracts.NewErrorEnvelope(code, contracts.ErrorCategoryWorkspace, message, details))}
}

func outputError(output string, fallback error) error {
	if message := strings.TrimSpace(output); message != "" {
		return errors.New(message)
	}
	return fallback
}

func successStatus(status contracts.GitHubSyncStatus) contracts.GitHubSyncResponse {
	return contracts.GitHubSyncResponse{Ok: true, Data: &status}
}

func ptr[T any](value T) *T { return &value }

func (state linkState) LinkIsPresent() bool { return state.Link.RepositoryURL != "" }
