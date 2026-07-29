package githubsync

import (
	"context"
	"errors"
)

// CredentialStore keeps GitHub credentials outside workspace files and the
// repository cache. The native implementation uses the OS credential store.
type CredentialStore interface {
	Get(ctx context.Context, account string) (string, bool, error)
	Set(ctx context.Context, account, secret string) error
	Delete(ctx context.Context, account string) error
}

var errCredentialStoreUnavailable = errors.New("system credential storage is unavailable")

const githubCredentialService = "Catenar.GitHub"
