package contracts

// GitHubWorkspaceLinkInput describes an existing GitHub repository that should
// be used as the remote store for the active workspace.
type GitHubWorkspaceLinkInput struct {
	RepositoryURL string `json:"repositoryUrl"`
	Branch        string `json:"branch"`
	WorkspacePath string `json:"workspacePath"`
	// AccessToken is transient: it is accepted from the desktop UI and written
	// only to the operating system credential store. It is never persisted in a
	// workspace file or in GitHub link metadata.
	AccessToken string `json:"accessToken,omitempty"`
}

// GitHubWorkspaceCredentialInput updates the HTTPS token of the active
// workspace without changing its linked repository settings.
type GitHubWorkspaceCredentialInput struct {
	AccessToken string `json:"accessToken"`
}

type GitHubWorkspaceLink struct {
	RepositoryURL    string `json:"repositoryUrl"`
	Branch           string `json:"branch"`
	WorkspacePath    string `json:"workspacePath"`
	LastSyncedCommit string `json:"lastSyncedCommit,omitempty"`
	LastSyncedAt     string `json:"lastSyncedAt,omitempty"`
}

type GitHubSyncActionInput struct {
	Overwrite bool `json:"overwrite"`
}

type GitHubSyncStatus struct {
	Linked              bool                 `json:"linked"`
	LocalChanges        bool                 `json:"localChanges"`
	RemoteChanges       bool                 `json:"remoteChanges"`
	Conflict            bool                 `json:"conflict"`
	InitialSyncRequired bool                 `json:"initialSyncRequired"`
	RemoteCommit        string               `json:"remoteCommit,omitempty"`
	TokenConfigured     bool                 `json:"tokenConfigured"`
	Link                *GitHubWorkspaceLink `json:"link,omitempty"`
	Workspace           *WorkspaceSnapshot   `json:"workspace,omitempty"`
}

type GitHubSyncResponse struct {
	Ok    bool              `json:"ok"`
	Data  *GitHubSyncStatus `json:"data,omitempty"`
	Error *ErrorEnvelope    `json:"error,omitempty"`
}
