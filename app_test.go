package main

import (
	"context"
	"path/filepath"
	"testing"

	"catenar/internal/contracts"
	"catenar/internal/workspace"
)

func TestAppRestoresWorkspaceDuringShellBootstrap(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "workspace")
	statePath := filepath.Join(t.TempDir(), "state", "runtime.json")
	creator := workspace.NewManager(workspace.ManagerOptions{StatePath: statePath})
	created := creator.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path: root,
		Name: "restored-after-startup",
		Endpoints: []contracts.EndpointPreset{{
			ID:                  "local",
			Name:                "Local",
			Target:              "localhost:50051",
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		}},
	})
	if !created.Ok {
		t.Fatalf("create workspace: %+v", created.Error)
	}

	app := newApp(appOptions{workspaceStatePath: statePath, deferWorkspaceRestore: true})
	if _, ok := app.workspace.Active(); ok {
		t.Fatal("expected workspace restore to wait for Wails startup")
	}

	app.startup(context.Background())
	if _, ok := app.workspace.Active(); ok {
		t.Fatal("expected workspace restore to wait for shell bootstrap")
	}

	bootstrap := app.ShellBootstrap()
	active, ok := app.workspace.Active()
	if !ok {
		t.Fatal("expected bootstrap to restore the remembered workspace")
	}
	if active.Name != "restored-after-startup" {
		t.Fatalf("expected restored workspace name, got %q", active.Name)
	}
	if bootstrap.Data == nil || bootstrap.Data.Workspace == nil {
		t.Fatal("expected bootstrap to include the restored workspace")
	}
	if bootstrap.Data.Workspace.Name != active.Name {
		t.Fatalf("expected bootstrap workspace name %q, got %q", active.Name, bootstrap.Data.Workspace.Name)
	}
}
