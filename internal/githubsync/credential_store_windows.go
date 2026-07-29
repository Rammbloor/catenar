//go:build windows

package githubsync

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credentialTypeGeneric         = 1
	credentialPersistLocalMachine = 2
)

var (
	advapi32         = windows.NewLazySystemDLL("advapi32.dll")
	credentialRead   = advapi32.NewProc("CredReadW")
	credentialWrite  = advapi32.NewProc("CredWriteW")
	credentialDelete = advapi32.NewProc("CredDeleteW")
	credentialFree   = advapi32.NewProc("CredFree")
)

// nativeCredential matches Windows CREDENTIALW. CredentialBlob is deliberately
// a byte pointer because the GitHub token is UTF-8, not UTF-16.
type nativeCredential struct {
	Flags                   uint32
	Type                    uint32
	TargetName              *uint16
	Comment                 *uint16
	LastWrittenLowDateTime  uint32
	LastWrittenHighDateTime uint32
	CredentialBlobSize      uint32
	CredentialBlob          *byte
	Persist                 uint32
	AttributeCount          uint32
	Attributes              uintptr
	TargetAlias             *uint16
	UserName                *uint16
}

type windowsCredentialStore struct{}

func defaultCredentialStore() CredentialStore { return windowsCredentialStore{} }

func (windowsCredentialStore) Get(ctx context.Context, account string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	target, err := windows.UTF16PtrFromString(windowsCredentialTarget(account))
	if err != nil {
		return "", false, fmt.Errorf("encode GitHub credential target: %w", err)
	}
	var credential *nativeCredential
	result, _, callErr := credentialRead.Call(
		uintptr(unsafe.Pointer(target)),
		credentialTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&credential)),
	)
	if result == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read GitHub credential from Windows Credential Manager: %w", callErr)
	}
	defer credentialFree.Call(uintptr(unsafe.Pointer(credential)))
	if credential.CredentialBlobSize == 0 || credential.CredentialBlob == nil {
		return "", false, errors.New("Windows Credential Manager returned an empty credential")
	}
	value := unsafe.Slice(credential.CredentialBlob, credential.CredentialBlobSize)
	return string(value), true, nil
}

func (windowsCredentialStore) Set(ctx context.Context, account, secret string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(windowsCredentialTarget(account))
	if err != nil {
		return fmt.Errorf("encode GitHub credential target: %w", err)
	}
	value := []byte(secret)
	credential := nativeCredential{
		Type:               credentialTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(value)),
		Persist:            credentialPersistLocalMachine,
		UserName:           target,
	}
	if len(value) > 0 {
		credential.CredentialBlob = &value[0]
	}
	result, _, callErr := credentialWrite.Call(uintptr(unsafe.Pointer(&credential)), 0)
	if result == 0 {
		return fmt.Errorf("write GitHub credential to Windows Credential Manager: %w", callErr)
	}
	return nil
}

func (windowsCredentialStore) Delete(ctx context.Context, account string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(windowsCredentialTarget(account))
	if err != nil {
		return fmt.Errorf("encode GitHub credential target: %w", err)
	}
	result, _, callErr := credentialDelete.Call(uintptr(unsafe.Pointer(target)), credentialTypeGeneric, 0)
	if result == 0 && !errors.Is(callErr, windows.ERROR_NOT_FOUND) {
		return fmt.Errorf("delete GitHub credential from Windows Credential Manager: %w", callErr)
	}
	return nil
}

func windowsCredentialTarget(account string) string {
	return githubCredentialService + ":" + account
}
