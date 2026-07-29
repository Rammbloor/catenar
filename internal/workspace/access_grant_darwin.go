//go:build darwin

package workspace

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#include <stdlib.h>

static NSMutableArray<NSURL *> *catenar_active_workspace_urls;

static void catenar_retain_workspace_url(NSURL *url) {
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        catenar_active_workspace_urls = [NSMutableArray array];
    });
    @synchronized (catenar_active_workspace_urls) {
        [catenar_active_workspace_urls addObject:url];
    }
}

static char *catenar_create_workspace_bookmark(const char *path) {
    if (path == NULL) {
        return NULL;
    }
    @autoreleasepool {
        NSString *pathString = [NSString stringWithUTF8String:path];
        if (pathString == nil) {
            return NULL;
        }
        NSURL *url = [NSURL fileURLWithPath:pathString];
        NSError *error = nil;
        NSData *data = [url bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
                      includingResourceValuesForKeys:nil
                                       relativeToURL:nil
                                               error:&error];
        if (data == nil || error != nil) {
            return NULL;
        }
        NSString *encoded = [data base64EncodedStringWithOptions:0];
        return encoded == nil ? NULL : strdup(encoded.UTF8String);
    }
}

static char *catenar_restore_workspace_bookmark(const char *encoded) {
    if (encoded == NULL) {
        return NULL;
    }
    @autoreleasepool {
        NSString *encodedString = [NSString stringWithUTF8String:encoded];
        NSData *data = [[NSData alloc] initWithBase64EncodedString:encodedString options:0];
        if (data == nil) {
            return NULL;
        }
        BOOL stale = NO;
        NSError *error = nil;
        NSURL *url = [NSURL URLByResolvingBookmarkData:data
                                               options:NSURLBookmarkResolutionWithSecurityScope
                                         relativeToURL:nil
                                   bookmarkDataIsStale:&stale
                                                 error:&error];
        if (url == nil || error != nil || ![url startAccessingSecurityScopedResource]) {
            return NULL;
        }
        catenar_retain_workspace_url(url);
        return strdup(url.path.UTF8String);
    }
}
*/
import "C"

import (
	"errors"
	"strings"
	"unsafe"
)

type darwinWorkspaceAccessGrant struct{}

func defaultWorkspaceAccessGrant() WorkspaceAccessGrant {
	return darwinWorkspaceAccessGrant{}
}

func (darwinWorkspaceAccessGrant) Remember(path string) (string, error) {
	workspacePath := C.CString(path)
	defer C.free(unsafe.Pointer(workspacePath))
	encoded := C.catenar_create_workspace_bookmark(workspacePath)
	if encoded == nil {
		return "", errors.New("macOS could not create a persistent workspace access bookmark")
	}
	defer C.free(unsafe.Pointer(encoded))
	return strings.TrimSpace(C.GoString(encoded)), nil
}

func (darwinWorkspaceAccessGrant) Restore(bookmark string) (string, error) {
	encoded := C.CString(bookmark)
	defer C.free(unsafe.Pointer(encoded))
	path := C.catenar_restore_workspace_bookmark(encoded)
	if path == nil {
		return "", errors.New("macOS could not restore persistent workspace access")
	}
	defer C.free(unsafe.Pointer(path))
	return strings.TrimSpace(C.GoString(path)), nil
}
