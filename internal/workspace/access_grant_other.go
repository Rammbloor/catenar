//go:build !darwin

package workspace

import "errors"

type noopWorkspaceAccessGrant struct{}

func defaultWorkspaceAccessGrant() WorkspaceAccessGrant {
	return noopWorkspaceAccessGrant{}
}

func (noopWorkspaceAccessGrant) Remember(string) (string, error) {
	return "", errors.New("persistent workspace access is not required on this platform")
}

func (noopWorkspaceAccessGrant) Restore(string) (string, error) {
	return "", errors.New("persistent workspace access is not required on this platform")
}
