package githubsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catenar/internal/contracts"
)

type fakeManager struct {
	workspace contracts.WorkspaceSnapshot
}

func (manager fakeManager) Active() (contracts.WorkspaceSnapshot, bool) {
	return manager.workspace, manager.workspace.ID != ""
}

func (manager fakeManager) Open(_ context.Context, _ string) contracts.WorkspaceResponse {
	workspace := manager.workspace
	return contracts.WorkspaceResponse{
		Ok:   true,
		Data: &contracts.WorkspaceResult{Workspace: workspace},
	}
}

type fakeGitRunner struct {
	remoteWorkspacePath string
	remoteContent       string
	commitOutput        string
	commitError         error
	pushError           error
	calls               [][]string
	environments        [][]string
}

func (runner *fakeGitRunner) Run(_ context.Context, _ string, env []string, args ...string) (string, error) {
	runner.calls = append(runner.calls, append([]string(nil), args...))
	runner.environments = append(runner.environments, append([]string(nil), env...))
	if len(args) == 0 {
		return "", nil
	}
	switch args[0] {
	case "clone":
		cachePath := args[len(args)-1]
		if err := os.MkdirAll(filepath.Join(cachePath, ".git"), 0o700); err != nil {
			return "", err
		}
		workspacePath := filepath.Join(cachePath, filepath.FromSlash(runner.remoteWorkspacePath))
		if err := os.MkdirAll(workspacePath, 0o700); err != nil {
			return "", err
		}
		if runner.remoteContent != "" {
			if err := os.WriteFile(filepath.Join(workspacePath, "workspace.yaml"), []byte(runner.remoteContent), 0o600); err != nil {
				return "", err
			}
		}
	case "rev-parse":
		return "0123456789abcdef\n", nil
	case "ls-tree":
		if runner.remoteContent == "" {
			return "", nil
		}
		return "100644 blob 0123456789abcdef\t" + runner.remoteWorkspacePath + "/workspace.yaml\x00", nil
	case "show":
		return runner.remoteContent, nil
	case "push":
		if runner.pushError != nil {
			return "remote: Invalid username or token", runner.pushError
		}
	case "commit":
		if runner.commitError != nil {
			return runner.commitOutput, runner.commitError
		}
	}
	return "", nil
}

type fakeCredentialStore struct {
	values map[string]string
}

func (store *fakeCredentialStore) Get(_ context.Context, account string) (string, bool, error) {
	value, ok := store.values[account]
	return value, ok, nil
}

func (store *fakeCredentialStore) Set(_ context.Context, account, secret string) error {
	if store.values == nil {
		store.values = make(map[string]string)
	}
	store.values[account] = secret
	return nil
}

func (store *fakeCredentialStore) Delete(_ context.Context, account string) error {
	delete(store.values, account)
	return nil
}

func TestLinkStoresHTTPSCredentialOnlyInSystemStore(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspacePath, "workspace.yaml"), []byte("name: local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	credentials := &fakeCredentialStore{}
	runner := &fakeGitRunner{remoteWorkspacePath: "catenar/demo", remoteContent: "name: remote\n"}
	service := NewService(Options{
		Manager: fakeManager{workspace: contracts.WorkspaceSnapshot{
			ID: "workspace-id", Name: "Demo", Path: workspacePath, ManifestPath: filepath.Join(workspacePath, "workspace.yaml"),
		}},
		ConfigDir:       t.TempDir(),
		Runner:          runner,
		CredentialStore: credentials,
	})

	const token = "github_pat_012345678901234567890123456789"
	response := service.Link(context.Background(), contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "https://github.com/owner/private-workspaces.git",
		Branch:        "main",
		WorkspacePath: "catenar/demo",
		AccessToken:   token,
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected link to succeed, got %#v", response.Error)
	}
	if !response.Data.TokenConfigured {
		t.Fatalf("expected response to report configured token, got %#v", response.Data)
	}
	if len(credentials.values) != 1 {
		t.Fatalf("expected exactly one system credential, got %#v", credentials.values)
	}
	for _, value := range credentials.values {
		if value != token {
			t.Fatalf("expected stored token to match input")
		}
	}
	stateContent, err := os.ReadFile(filepath.Join(service.configDir, linkStateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateContent), token) {
		t.Fatalf("GitHub token must never be written to link state: %s", stateContent)
	}
	if len(runner.environments) == 0 || !containsEnvironment(runner.environments[0], "CATENAR_GITHUB_TOKEN="+token) {
		t.Fatalf("HTTPS clone must receive the transient token only through askpass env: %#v", runner.environments)
	}
}

func containsEnvironment(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestPushPublishesExistingCommitWhenWorkspaceTreeIsAlreadyClean(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspacePath, "workspace.yaml"), []byte("name: local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeGitRunner{
		remoteWorkspacePath: "catenar/demo",
		commitOutput:        "On branch main\nYour branch is ahead of 'origin/main' by 2 commits.\nnothing to commit, working tree clean\n",
		commitError:         errors.New("exit status 1"),
	}
	service := NewService(Options{
		Manager: fakeManager{workspace: contracts.WorkspaceSnapshot{
			ID: "workspace-id", Name: "Demo", Path: workspacePath,
			ManifestPath: filepath.Join(workspacePath, "workspace.yaml"),
		}},
		ConfigDir: t.TempDir(),
		Runner:    runner,
	})

	if response := service.Link(context.Background(), contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "https://github.com/owner/private-workspaces.git",
		Branch:        "main",
		WorkspacePath: "catenar/demo",
	}); !response.Ok {
		t.Fatalf("link failed: %#v", response.Error)
	}

	if response := service.Push(context.Background(), contracts.GitHubSyncActionInput{}); !response.Ok {
		t.Fatalf("existing local commit must still be pushed: %#v", response.Error)
	}

	pushed := false
	for _, call := range runner.calls {
		if len(call) > 0 && call[0] == "push" {
			pushed = true
			break
		}
	}
	if !pushed {
		t.Fatal("expected push after the no-op commit")
	}
}

func TestStatusAfterRejectedInitialPushReportsOnlyLocalChanges(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspacePath, "workspace.yaml"), []byte("name: local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(Options{
		Manager: fakeManager{workspace: contracts.WorkspaceSnapshot{
			ID: "workspace-id", Name: "Demo", Path: workspacePath,
			ManifestPath: filepath.Join(workspacePath, "workspace.yaml"),
		}},
		ConfigDir: t.TempDir(),
		Runner: &fakeGitRunner{
			remoteWorkspacePath: "catenar/demo",
			pushError:           errors.New("authentication failed"),
		},
	})

	if response := service.Link(context.Background(), contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "https://github.com/owner/private-workspaces.git",
		Branch:        "main",
		WorkspacePath: "catenar/demo",
	}); !response.Ok {
		t.Fatalf("link failed: %#v", response.Error)
	}
	if response := service.Push(context.Background(), contracts.GitHubSyncActionInput{}); response.Ok || response.Error == nil || response.Error.Code != "github_sync.push_failed" {
		t.Fatalf("expected rejected push, got %#v", response)
	}

	status := service.Status(context.Background())
	if !status.Ok || status.Data == nil {
		t.Fatalf("status failed: %#v", status)
	}
	if !status.Data.LocalChanges || status.Data.RemoteChanges {
		t.Fatalf("rejected local push must not be reported as a remote change: %#v", status.Data)
	}
	if !status.Data.InitialSyncRequired {
		t.Fatalf("first successful push is still required: %#v", status.Data)
	}
}

func TestLinkRecognizesMatchingManagedRemoteWorkspace(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	const manifest = "name: demo\n"
	if err := os.WriteFile(filepath.Join(workspacePath, "workspace.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(Options{
		Manager: fakeManager{workspace: contracts.WorkspaceSnapshot{
			ID: "workspace-id", Name: "Demo", Path: workspacePath,
			ManifestPath: filepath.Join(workspacePath, "workspace.yaml"),
		}},
		ConfigDir: t.TempDir(),
		Runner: &fakeGitRunner{
			remoteWorkspacePath: "catenar/demo",
			remoteContent:       manifest,
		},
	})

	response := service.Link(context.Background(), contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "https://github.com/owner/private-workspaces.git",
		Branch:        "main",
		WorkspacePath: "catenar/demo",
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("link failed: %#v", response)
	}
	if response.Data.InitialSyncRequired || response.Data.LocalChanges || response.Data.RemoteChanges {
		t.Fatalf("matching managed workspace must start synchronized: %#v", response.Data)
	}
}

func TestNormalizeLinkValidatesGitHubAndRepositoryPaths(t *testing.T) {
	t.Parallel()

	valid, err := normalizeLink(contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "git@github.com:owner/private-workspaces.git",
		Branch:        "feature/workspaces",
		WorkspacePath: "teams/backend",
	})
	if err != nil {
		t.Fatalf("expected valid link: %v", err)
	}
	if valid.Branch != "feature/workspaces" || valid.WorkspacePath != "teams/backend" {
		t.Fatalf("unexpected normalized link: %#v", valid)
	}

	tests := []contracts.GitHubWorkspaceLinkInput{
		{RepositoryURL: "https://example.com/owner/repo.git", Branch: "main", WorkspacePath: "workspace"},
		{RepositoryURL: "https://user:token@github.com/owner/repo.git", Branch: "main", WorkspacePath: "workspace"},
		{RepositoryURL: "https://github.com/owner/repo.git", Branch: "../main", WorkspacePath: "workspace"},
		{RepositoryURL: "https://github.com/owner/repo.git", Branch: "main", WorkspacePath: "../workspace"},
		{RepositoryURL: "https://github.com/owner/repo.git", Branch: "main", WorkspacePath: ".git/hooks"},
	}
	for _, input := range tests {
		if _, err := normalizeLink(input); err == nil {
			t.Fatalf("expected unsafe link to fail: %#v", input)
		}
	}
}

func TestLinkPersistsOnlyRepositoryMetadata(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspacePath, "workspace.yaml"), []byte("name: local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := t.TempDir()
	runner := &fakeGitRunner{remoteWorkspacePath: "workspaces/demo", remoteContent: "name: remote\n"}
	service := NewService(Options{
		Manager: fakeManager{workspace: contracts.WorkspaceSnapshot{
			ID: "workspace-id", Name: "Demo", Path: workspacePath,
			ManifestPath: filepath.Join(workspacePath, "workspace.yaml"),
		}},
		ConfigDir: configPath,
		Runner:    runner,
	})

	response := service.Link(context.Background(), contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "git@github.com:owner/private-workspaces.git",
		Branch:        "main",
		WorkspacePath: "workspaces/demo",
	})
	if !response.Ok || response.Data == nil || !response.Data.Linked {
		t.Fatalf("expected linked status, got %#v", response)
	}
	if !response.Data.Conflict || !response.Data.InitialSyncRequired {
		t.Fatalf("expected explicit initial conflict, got %#v", response.Data)
	}

	content, err := os.ReadFile(filepath.Join(configPath, linkStateFileName))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(content))
	for _, forbidden := range []string{"token", "password", "credential"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("persisted sync state contains forbidden secret field %q: %s", forbidden, content)
		}
	}
	info, err := os.Stat(filepath.Join(configPath, linkStateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("sync state is accessible outside the current user: %o", info.Mode().Perm())
	}
}

func TestLinkUpdatesWorkspaceFolderForTheSameRepository(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspacePath, "workspace.yaml"), []byte("name: local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(Options{
		Manager: fakeManager{workspace: contracts.WorkspaceSnapshot{
			ID: "workspace-id", Name: "Demo", Path: workspacePath,
			ManifestPath: filepath.Join(workspacePath, "workspace.yaml"),
		}},
		ConfigDir: t.TempDir(),
		Runner:    &fakeGitRunner{remoteWorkspacePath: "workspaces/demo"},
	})

	initial := contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "https://github.com/owner/private-workspaces.git",
		Branch:        "main",
		WorkspacePath: "workspaces/demo",
	}
	if response := service.Link(context.Background(), initial); !response.Ok {
		t.Fatalf("initial link failed: %#v", response.Error)
	}

	updated := initial
	updated.WorkspacePath = "catenar-workspaces/demo"
	response := service.Link(context.Background(), updated)
	if !response.Ok || response.Data == nil || response.Data.Link == nil {
		t.Fatalf("expected link update, got %#v", response)
	}
	if response.Data.Link.WorkspacePath != updated.WorkspacePath {
		t.Fatalf("expected updated folder %q, got %#v", updated.WorkspacePath, response.Data.Link)
	}
}

func TestLinkIncludesLocalWorkspaceInspectionCause(t *testing.T) {
	t.Parallel()

	workspaceFile := filepath.Join(t.TempDir(), "not-a-workspace-directory")
	if err := os.WriteFile(workspaceFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(Options{
		Manager: fakeManager{workspace: contracts.WorkspaceSnapshot{
			ID: "workspace-id", Name: "Demo", Path: workspaceFile,
			ManifestPath: workspaceFile,
		}},
		ConfigDir: t.TempDir(),
		Runner:    &fakeGitRunner{remoteWorkspacePath: "workspaces/demo"},
	})

	response := service.Link(context.Background(), contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "https://github.com/owner/private-workspaces.git",
		Branch:        "main",
		WorkspacePath: "workspaces/demo",
	})
	if response.Ok || response.Error == nil {
		t.Fatalf("expected workspace inspection error, got %#v", response)
	}
	if response.Error.Code != "github_sync.workspace_read_failed" {
		t.Fatalf("expected workspace inspection code, got %#v", response.Error)
	}
	if !strings.Contains(response.Error.Details["cause"], "not a directory") {
		t.Fatalf("expected actionable inspection cause, got %#v", response.Error.Details)
	}
}

func TestInitialConflictRequiresExplicitDirection(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspacePath, "workspace.yaml"), []byte("name: local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeGitRunner{remoteWorkspacePath: "workspaces/demo", remoteContent: "name: remote\n"}
	service := NewService(Options{
		Manager: fakeManager{workspace: contracts.WorkspaceSnapshot{
			ID: "workspace-id", Name: "Demo", Path: workspacePath,
			ManifestPath: filepath.Join(workspacePath, "workspace.yaml"),
		}},
		ConfigDir: t.TempDir(),
		Runner:    runner,
	})

	linked := service.Link(context.Background(), contracts.GitHubWorkspaceLinkInput{
		RepositoryURL: "https://github.com/owner/private-workspaces.git",
		Branch:        "main",
		WorkspacePath: "workspaces/demo",
	})
	if !linked.Ok {
		t.Fatalf("link failed: %#v", linked.Error)
	}
	pushed := service.Push(context.Background(), contracts.GitHubSyncActionInput{})
	if pushed.Ok || pushed.Error == nil || pushed.Error.Code != "github_sync.conflict" {
		t.Fatalf("expected push conflict without overwrite, got %#v", pushed)
	}
	pulled := service.Pull(context.Background(), contracts.GitHubSyncActionInput{})
	if pulled.Ok || pulled.Error == nil || pulled.Error.Code != "github_sync.conflict" {
		t.Fatalf("expected pull conflict without overwrite, got %#v", pulled)
	}
}

func TestTreeDigestTracksContentAndIgnoresGitMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "workspace.yaml"), []byte("name: demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := treeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := treeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal(".git metadata must not affect workspace digest")
	}
	if err := os.WriteFile(filepath.Join(root, "workspace.yaml"), []byte("name: changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	third, err := treeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("workspace content change must affect digest")
	}
}

func TestTreeDigestIgnoresUnmanagedProjectFilesEvenWhenTheyExceedSyncLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "workspace.yaml"), []byte("name: demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	largeCachePath := filepath.Join(root, "node_modules", "cache.bin")
	if err := os.MkdirAll(filepath.Dir(largeCachePath), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(largeCachePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxWorkspaceBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := treeDigest(root); err != nil {
		t.Fatalf("expected unmanaged project files to be ignored, got %v", err)
	}
}
