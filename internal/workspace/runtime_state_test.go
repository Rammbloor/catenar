package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"catenar/internal/contracts"
)

type testWorkspaceAccessGrant struct {
	bookmark     string
	resolvedPath string
}

func (grant testWorkspaceAccessGrant) Remember(string) (string, error) {
	return grant.bookmark, nil
}

func (grant testWorkspaceAccessGrant) Restore(bookmark string) (string, error) {
	if bookmark != grant.bookmark {
		return "", context.Canceled
	}
	return grant.resolvedPath, nil
}

func TestManagerRestoresLastWorkspaceFromPrivateRuntimeState(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "remembered-workspace")
	statePath := filepath.Join(t.TempDir(), "state", "runtime.json")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime, StatePath: statePath})
	created := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "remembered",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_remembered")},
	})
	if !created.Ok {
		t.Fatalf("create workspace: %+v", created.Error)
	}

	restarted := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime, StatePath: statePath})
	active, ok := restarted.Active()
	if !ok {
		t.Fatal("expected the previous workspace to be restored")
	}
	if active.ManifestPath != filepath.Join(root, manifestFileName) {
		t.Fatalf("expected restored manifest path %q, got %q", filepath.Join(root, manifestFileName), active.ManifestPath)
	}

	closed := restarted.Close(context.Background())
	if !closed.Ok {
		t.Fatalf("close workspace: %+v", closed.Error)
	}
	if _, ok := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime, StatePath: statePath}).Active(); ok {
		t.Fatal("expected an explicitly closed workspace not to reopen")
	}
}

func TestManagerCanDeferRuntimeStateRestoreUntilHostIsReady(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "deferred-workspace")
	statePath := filepath.Join(t.TempDir(), "state", "runtime.json")
	creator := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime, StatePath: statePath})
	created := creator.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "deferred",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_deferred")},
	})
	if !created.Ok {
		t.Fatalf("create workspace: %+v", created.Error)
	}

	deferred := NewManager(ManagerOptions{
		Now:          fixedWorkspaceTestTime,
		StatePath:    statePath,
		DeferRestore: true,
	})
	if _, ok := deferred.Active(); ok {
		t.Fatal("expected workspace restore to wait for the host runtime")
	}

	deferred.RestoreLastWorkspace()
	active, ok := deferred.Active()
	if !ok {
		t.Fatal("expected workspace to restore after the host runtime is ready")
	}
	if active.ManifestPath != filepath.Join(root, manifestFileName) {
		t.Fatalf("expected restored manifest path %q, got %q", filepath.Join(root, manifestFileName), active.ManifestPath)
	}
}

func TestManagerRestoresWorkspaceUsingPersistedAccessGrant(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "bookmark-workspace")
	statePath := filepath.Join(t.TempDir(), "state", "runtime.json")
	grant := testWorkspaceAccessGrant{bookmark: "bookmark-token", resolvedPath: filepath.Join(root, manifestFileName)}
	creator := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime, StatePath: statePath, AccessGrant: grant})
	created := creator.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "bookmark-restored",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_bookmark")},
	})
	if !created.Ok {
		t.Fatalf("create workspace: %+v", created.Error)
	}

	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read runtime state: %v", err)
	}
	var state persistedRuntimeState
	if err := json.Unmarshal(content, &state); err != nil {
		t.Fatalf("decode runtime state: %v", err)
	}
	if state.LastWorkspaceBookmark != grant.bookmark {
		t.Fatalf("expected bookmark %q, got %q", grant.bookmark, state.LastWorkspaceBookmark)
	}

	state.LastWorkspacePath = filepath.Join(t.TempDir(), "stale", manifestFileName)
	content, err = json.Marshal(state)
	if err != nil {
		t.Fatalf("encode stale runtime state: %v", err)
	}
	if err := os.WriteFile(statePath, content, 0o600); err != nil {
		t.Fatalf("write stale runtime state: %v", err)
	}

	restarted := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime, StatePath: statePath, AccessGrant: grant})
	active, ok := restarted.Active()
	if !ok {
		t.Fatal("expected the workspace to restore through the persistent access grant")
	}
	if active.Name != "bookmark-restored" {
		t.Fatalf("expected restored workspace name, got %q", active.Name)
	}
}

func TestManagerUpgradesPathOnlyRuntimeStateToPersistentAccessGrant(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "path-only-workspace")
	statePath := filepath.Join(t.TempDir(), "state", "runtime.json")
	grant := testWorkspaceAccessGrant{bookmark: "upgrade-token", resolvedPath: filepath.Join(root, manifestFileName)}
	creator := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime, StatePath: statePath})
	created := creator.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "upgraded",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_upgrade")},
	})
	if !created.Ok {
		t.Fatalf("create workspace: %+v", created.Error)
	}

	pathOnly, err := json.Marshal(persistedRuntimeState{LastWorkspacePath: filepath.Join(t.TempDir(), "stale", manifestFileName)})
	if err != nil {
		t.Fatalf("encode path-only state: %v", err)
	}
	if err := os.WriteFile(statePath, pathOnly, 0o600); err != nil {
		t.Fatalf("write path-only state: %v", err)
	}

	restarted := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime, StatePath: statePath, AccessGrant: grant})
	active, ok := restarted.Active()
	if !ok || active.Name != "upgraded" {
		t.Fatalf("expected path-only state to restore through access grant, got %+v, active=%t", active, ok)
	}
}
