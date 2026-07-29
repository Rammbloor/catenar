package workspace

// WorkspaceAccessGrant persists access to a user-selected workspace across
// desktop restarts. On macOS this is backed by a security-scoped bookmark;
// other platforms intentionally use a no-op implementation.
type WorkspaceAccessGrant interface {
	Remember(path string) (string, error)
	Restore(bookmark string) (string, error)
}
