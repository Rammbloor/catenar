package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"tether/internal/contracts"
)

func TestCreateSaveOpenReopenPersistsWorkspaceManifestV1(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "demo-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path: root,
		Name: "demo",
		Endpoints: []contracts.EndpointPreset{
			workspaceTestEndpoint("ep_local"),
		},
		ProtoSources: []contracts.ProtoSource{
			{Type: contracts.ProtoSourceTypeDirectory, Path: "./proto"},
			{Type: contracts.ProtoSourceTypeFile, Path: "./contracts/greeter.proto"},
		},
		ImportPaths: []string{"./proto", "./contracts"},
	})

	if !createResponse.Ok || createResponse.Data == nil {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	saveResponse := manager.Save(context.Background(), contracts.WorkspaceSaveInput{
		Name: "demo-renamed",
		Endpoints: []contracts.EndpointPreset{
			workspaceTestEndpoint("ep_local"),
		},
		ProtoSources: []contracts.ProtoSource{
			{Type: contracts.ProtoSourceTypeDirectory, Path: "./proto"},
		},
		ImportPaths: []string{"./proto"},
	})
	if !saveResponse.Ok || saveResponse.Data == nil {
		t.Fatalf("expected workspace save to succeed, got %+v", saveResponse.Error)
	}

	reopened := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	openResponse := reopened.Open(context.Background(), root)
	if !openResponse.Ok || openResponse.Data == nil {
		t.Fatalf("expected workspace reopen to succeed, got %+v", openResponse.Error)
	}

	snapshot := openResponse.Data.Workspace
	if snapshot.Version != CurrentManifestVersion {
		t.Fatalf("expected version %d, got %d", CurrentManifestVersion, snapshot.Version)
	}
	if snapshot.Name != "demo-renamed" {
		t.Fatalf("expected saved workspace name, got %q", snapshot.Name)
	}
	if len(snapshot.Endpoints) != 1 || snapshot.Endpoints[0].ID != "ep_local" {
		t.Fatalf("expected persisted endpoint, got %+v", snapshot.Endpoints)
	}
	if len(snapshot.ProtoSources) != 1 || snapshot.ProtoSources[0].Path != "./proto" {
		t.Fatalf("expected persisted proto sources, got %+v", snapshot.ProtoSources)
	}
	if len(snapshot.ImportPaths) != 1 || snapshot.ImportPaths[0] != "./proto" {
		t.Fatalf("expected persisted import paths, got %+v", snapshot.ImportPaths)
	}
}

func TestRequestSavePersistsSavedRequestAndReopensSummary(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "request-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "requests",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	requestResponse := manager.RequestSave(context.Background(), contracts.RequestSaveInput{
		ID:          "greeter.say-hello",
		Method:      "demo.v1.Greeter/SayHello",
		RPCType:     contracts.RPCTypeUnary,
		EndpointRef: "ep_local",
		MetadataTemplate: map[string]string{
			"x-tenant-id": "dev",
		},
		CallOptions: contracts.CallOptions{
			RequestTimeoutMs: 1500,
		},
		RequestSpec: contracts.SavedRequestSpec{
			Mode: "single",
			Body: map[string]any{"name": "world"},
		},
	})
	if !requestResponse.Ok || requestResponse.Data == nil {
		t.Fatalf("expected request save to succeed, got %+v", requestResponse.Error)
	}

	requestPath := filepath.Join(root, requestResponse.Data.SavedRequest.Path)
	content, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request file: %v", err)
	}
	if !strings.Contains(string(content), "endpointRef: ep_local") {
		t.Fatalf("expected endpoint ref in request file, got %s", content)
	}

	reopened := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	openResponse := reopened.Open(context.Background(), root)
	if !openResponse.Ok || openResponse.Data == nil {
		t.Fatalf("expected workspace reopen to succeed, got %+v", openResponse.Error)
	}
	if len(openResponse.Data.Workspace.SavedRequests) != 1 {
		t.Fatalf("expected reopened saved request summary, got %+v", openResponse.Data.Workspace.SavedRequests)
	}
	if openResponse.Data.Workspace.SavedRequests[0].ID != "greeter.say-hello" {
		t.Fatalf("expected saved request id, got %+v", openResponse.Data.Workspace.SavedRequests[0])
	}
}

func TestPrepareEndpointTestCarriesWorkspaceEventRetentionSettings(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "settings-workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	writeWorkspaceYAML(t, filepath.Join(root, manifestFileName), map[string]any{
		"version": CurrentManifestVersion,
		"name":    "settings",
		"endpoints": []map[string]any{
			{
				"id":               "ep_local",
				"name":             "local",
				"target":           "127.0.0.1:50051",
				"connectTimeoutMs": 3000,
				"tls": map[string]any{
					"mode": "plaintext",
				},
			},
		},
		"settings": map[string]any{
			"eventRetention": map[string]any{
				"maxEventsPerCall": 7,
				"maxBytesPerCall":  512,
			},
		},
	})

	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	openResponse := manager.Open(context.Background(), root)
	if !openResponse.Ok {
		t.Fatalf("expected workspace open to succeed, got %+v", openResponse.Error)
	}

	scope, _, err := manager.PrepareEndpointTest(context.Background(), contracts.EndpointTestInput{
		Endpoint: workspaceTestEndpoint("ep_local"),
	})
	if err != nil {
		t.Fatalf("prepare endpoint: %v", err)
	}
	if scope.EventRetention.MaxEventsPerCall != 7 || scope.EventRetention.MaxBytesPerCall != 512 {
		t.Fatalf("expected workspace retention settings in runtime scope, got %+v", scope.EventRetention)
	}
}

func TestOpenMigratesVersionZeroManifestAndCreatesBackup(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "legacy-workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeWorkspaceYAML(t, filepath.Join(root, manifestFileName), map[string]any{
		"version": 0,
		"name":    "legacy",
		"endpoints": []map[string]any{
			{
				"id":               "ep_legacy",
				"name":             "legacy",
				"target":           "127.0.0.1:50051",
				"connectTimeoutMs": 3000,
				"requestTimeoutMs": 1000,
				"tls": map[string]any{
					"mode": "plaintext",
				},
			},
		},
	})

	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	response := manager.Open(context.Background(), root)
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected migrated workspace to open, got %+v", response.Error)
	}
	if len(response.Data.Workspace.BackupPaths) != 1 {
		t.Fatalf("expected migration backup path, got %+v", response.Data.Workspace.BackupPaths)
	}
	if _, err := os.Stat(response.Data.Workspace.BackupPaths[0]); err != nil {
		t.Fatalf("expected backup file: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, manifestFileName))
	if err != nil {
		t.Fatalf("read migrated manifest: %v", err)
	}
	if !strings.Contains(string(content), "version: 1") {
		t.Fatalf("expected manifest to be rewritten at v1, got %s", content)
	}
}

func TestOpenRejectsMalformedSecretRefsBeforeInvoke(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "secret-workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeWorkspaceYAML(t, filepath.Join(root, manifestFileName), map[string]any{
		"version": 1,
		"name":    "secret",
		"endpoints": []map[string]any{
			{
				"id":               "ep_secret",
				"name":             "secret",
				"target":           "127.0.0.1:50051",
				"connectTimeoutMs": 3000,
				"requestTimeoutMs": 1000,
				"tls": map[string]any{
					"mode":   "custom_ca",
					"caCert": "raw-ca.pem",
				},
			},
		},
	})

	response := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime}).Open(context.Background(), root)
	if response.Ok {
		t.Fatalf("expected malformed secret ref to fail open")
	}
	if response.Error == nil || response.Error.Code != "validation.secret_ref_malformed" {
		t.Fatalf("expected malformed secret ref validation error, got %+v", response.Error)
	}
}

func TestSaveRejectsInlineSensitiveMetadataBeforeWritingWorkspace(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "metadata-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "metadata",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	endpointPreset := workspaceTestEndpoint("ep_local")
	endpointPreset.MetadataDefaults = map[string]string{
		"authorization": "Bearer raw-token",
	}
	response := manager.Save(context.Background(), contracts.WorkspaceSaveInput{
		Name:      "metadata",
		Endpoints: []contracts.EndpointPreset{endpointPreset},
	})
	if response.Ok {
		t.Fatalf("expected inline secret metadata to fail save")
	}
	if response.Error == nil || response.Error.Code != "validation.workspace_metadata_secret_inline" {
		t.Fatalf("expected inline secret metadata validation error, got %+v", response.Error)
	}
}

func TestSaveRejectsInvalidTimeoutValuesBeforeInvoke(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "timeout-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "timeouts",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	invalid := workspaceTestEndpoint("ep_local")
	invalid.ConnectTimeoutMs = 0
	response := manager.Save(context.Background(), contracts.WorkspaceSaveInput{
		Name:      "timeouts",
		Endpoints: []contracts.EndpointPreset{invalid},
	})
	if response.Ok {
		t.Fatalf("expected invalid timeout save to fail")
	}
	if response.Error == nil || response.Error.Code != "validation.endpoint_connect_timeout_invalid" {
		t.Fatalf("expected invalid timeout validation error, got %+v", response.Error)
	}
}

func TestOpenRejectsInvalidSavedRequestRefsBeforeInvoke(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "refs-workspace")
	requestsDir := filepath.Join(root, "requests")
	if err := os.MkdirAll(requestsDir, 0o755); err != nil {
		t.Fatalf("mkdir requests: %v", err)
	}
	writeWorkspaceYAML(t, filepath.Join(root, manifestFileName), map[string]any{
		"version": 1,
		"name":    "refs",
		"endpoints": []map[string]any{
			{
				"id":               "ep_local",
				"name":             "local",
				"target":           "127.0.0.1:50051",
				"connectTimeoutMs": 3000,
				"requestTimeoutMs": 1000,
				"tls": map[string]any{
					"mode": "plaintext",
				},
			},
		},
		"savedRequests": []string{"requests/broken.yaml"},
	})
	writeWorkspaceYAML(t, filepath.Join(requestsDir, "broken.yaml"), map[string]any{
		"id":          "broken",
		"method":      "demo.v1.Greeter/SayHello",
		"rpcType":     "unary",
		"endpointRef": "ep_missing",
		"requestSpec": map[string]any{
			"mode": "single",
			"body": map[string]any{"name": "world"},
		},
	})

	response := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime}).Open(context.Background(), root)
	if response.Ok {
		t.Fatalf("expected dangling endpointRef to fail open")
	}
	if response.Error == nil || response.Error.Code != "validation.saved_request_endpoint_ref_invalid" {
		t.Fatalf("expected dangling endpointRef validation error, got %+v", response.Error)
	}
}

func TestOpenRejectsInvalidSavedRequestTimeoutBeforeInvoke(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "request-timeout-workspace")
	requestsDir := filepath.Join(root, "requests")
	if err := os.MkdirAll(requestsDir, 0o755); err != nil {
		t.Fatalf("mkdir requests: %v", err)
	}
	writeWorkspaceYAML(t, filepath.Join(root, manifestFileName), map[string]any{
		"version": 1,
		"name":    "request-timeout",
		"endpoints": []map[string]any{
			{
				"id":               "ep_local",
				"name":             "local",
				"target":           "127.0.0.1:50051",
				"connectTimeoutMs": 3000,
				"requestTimeoutMs": 1000,
				"tls": map[string]any{
					"mode": "plaintext",
				},
			},
		},
		"savedRequests": []string{"requests/timeout.yaml"},
	})
	writeWorkspaceYAML(t, filepath.Join(requestsDir, "timeout.yaml"), map[string]any{
		"id":          "timeout",
		"method":      "demo.v1.Greeter/SayHello",
		"rpcType":     "unary",
		"endpointRef": "ep_local",
		"callOptions": map[string]any{
			"requestTimeoutMs": -1,
		},
		"requestSpec": map[string]any{
			"mode": "single",
			"body": map[string]any{"name": "world"},
		},
	})

	response := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime}).Open(context.Background(), root)
	if response.Ok {
		t.Fatalf("expected invalid saved request timeout to fail open")
	}
	if response.Error == nil || response.Error.Code != "validation.saved_request_request_timeout_invalid" {
		t.Fatalf("expected saved request timeout validation error, got %+v", response.Error)
	}
}

func workspaceTestEndpoint(id string) contracts.EndpointPreset {
	return contracts.EndpointPreset{
		ID:                  id,
		Name:                "local",
		Target:              "127.0.0.1:50051",
		TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
		ConnectTimeoutMs:    3000,
		RequestTimeoutMs:    1000,
		StreamIdleTimeoutMs: 0,
	}
}

func fixedWorkspaceTestTime() time.Time {
	return time.Date(2026, 4, 29, 12, 13, 14, 0, time.UTC)
}

func writeWorkspaceYAML(t *testing.T, path string, value any) {
	t.Helper()

	content, err := yaml.Marshal(value)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
}
