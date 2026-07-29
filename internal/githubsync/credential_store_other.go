//go:build !darwin && !windows

package githubsync

import "context"

type unavailableCredentialStore struct{}

func defaultCredentialStore() CredentialStore { return unavailableCredentialStore{} }

func (unavailableCredentialStore) Get(context.Context, string) (string, bool, error) {
	return "", false, errCredentialStoreUnavailable
}

func (unavailableCredentialStore) Set(context.Context, string, string) error {
	return errCredentialStoreUnavailable
}

func (unavailableCredentialStore) Delete(context.Context, string) error {
	return errCredentialStoreUnavailable
}
