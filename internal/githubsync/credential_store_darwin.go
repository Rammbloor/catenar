//go:build darwin

package githubsync

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Security
#import <Foundation/Foundation.h>
#import <Security/Security.h>
#include <stdlib.h>

static OSStatus catenar_keychain_set(const char* serviceValue, const char* accountValue, const char* secretValue) {
    NSString *service = [NSString stringWithUTF8String:serviceValue];
    NSString *account = [NSString stringWithUTF8String:accountValue];
    NSData *secret = [[NSString stringWithUTF8String:secretValue] dataUsingEncoding:NSUTF8StringEncoding];
    NSDictionary *query = @{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: service,
        (__bridge id)kSecAttrAccount: account,
    };
    SecItemDelete((__bridge CFDictionaryRef)query);
    NSMutableDictionary *item = [query mutableCopy];
    item[(__bridge id)kSecValueData] = secret;
    return SecItemAdd((__bridge CFDictionaryRef)item, NULL);
}

static char* catenar_keychain_get(const char* serviceValue, const char* accountValue, OSStatus* status) {
    NSString *service = [NSString stringWithUTF8String:serviceValue];
    NSString *account = [NSString stringWithUTF8String:accountValue];
    NSDictionary *query = @{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: service,
        (__bridge id)kSecAttrAccount: account,
        (__bridge id)kSecReturnData: @YES,
        (__bridge id)kSecMatchLimit: (__bridge id)kSecMatchLimitOne,
    };
    CFTypeRef result = NULL;
    *status = SecItemCopyMatching((__bridge CFDictionaryRef)query, &result);
    if (*status != errSecSuccess || result == NULL) {
        return NULL;
    }
    NSData *data = (NSData *)result;
    NSString *secret = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
    CFRelease(result);
    if (secret == nil) {
        *status = errSecDecode;
        return NULL;
    }
    char *copy = strdup([secret UTF8String]);
    [secret release];
    return copy;
}

static OSStatus catenar_keychain_delete(const char* serviceValue, const char* accountValue) {
    NSString *service = [NSString stringWithUTF8String:serviceValue];
    NSString *account = [NSString stringWithUTF8String:accountValue];
    NSDictionary *query = @{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: service,
        (__bridge id)kSecAttrAccount: account,
    };
    return SecItemDelete((__bridge CFDictionaryRef)query);
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"unsafe"
)

type keychainCredentialStore struct{}

func defaultCredentialStore() CredentialStore { return keychainCredentialStore{} }

func (keychainCredentialStore) Get(ctx context.Context, account string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	service := C.CString(githubCredentialService)
	defer C.free(unsafe.Pointer(service))
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cAccount))
	var status C.OSStatus
	value := C.catenar_keychain_get(service, cAccount, &status)
	if status == C.errSecItemNotFound {
		return "", false, nil
	}
	if status != C.errSecSuccess {
		return "", false, fmt.Errorf("read GitHub credential from Keychain: status %d", int(status))
	}
	if value == nil {
		return "", false, errors.New("Keychain returned an empty credential")
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value), true, nil
}

func (keychainCredentialStore) Set(ctx context.Context, account, secret string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	service := C.CString(githubCredentialService)
	defer C.free(unsafe.Pointer(service))
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cAccount))
	cSecret := C.CString(secret)
	defer C.free(unsafe.Pointer(cSecret))
	if status := C.catenar_keychain_set(service, cAccount, cSecret); status != C.errSecSuccess {
		return fmt.Errorf("write GitHub credential to Keychain: status %d", int(status))
	}
	return nil
}

func (keychainCredentialStore) Delete(ctx context.Context, account string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	service := C.CString(githubCredentialService)
	defer C.free(unsafe.Pointer(service))
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cAccount))
	status := C.catenar_keychain_delete(service, cAccount)
	if status == C.errSecItemNotFound || status == C.errSecSuccess {
		return nil
	}
	return fmt.Errorf("delete GitHub credential from Keychain: status %d", int(status))
}
