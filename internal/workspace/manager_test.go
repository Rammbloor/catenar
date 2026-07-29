package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"catenar/internal/contracts"
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
		Name:        "Say hello to world",
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
	if !strings.Contains(string(content), "name: Say hello to world") {
		t.Fatalf("expected saved request display name in request file, got %s", content)
	}
	if !strings.Contains(string(content), "mode: single") {
		t.Fatalf("expected unary single request mode in request file, got %s", content)
	}
	if !strings.Contains(string(content), "x-tenant-id: dev") {
		t.Fatalf("expected request-level metadata template in request file, got %s", content)
	}

	manifestContent, err := os.ReadFile(filepath.Join(root, manifestFileName))
	if err != nil {
		t.Fatalf("read manifest file: %v", err)
	}
	if !strings.Contains(string(manifestContent), "requests/greeter.say-hello.yaml") {
		t.Fatalf("expected saved request reference in manifest, got %s", manifestContent)
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
	if openResponse.Data.Workspace.SavedRequests[0].Name != "Say hello to world" {
		t.Fatalf("expected saved request name, got %+v", openResponse.Data.Workspace.SavedRequests[0])
	}

	loadResponse := reopened.RequestGet(context.Background(), contracts.RequestGetInput{ID: "greeter.say-hello"})
	if !loadResponse.Ok || loadResponse.Data == nil {
		t.Fatalf("expected saved request load to succeed, got %+v", loadResponse.Error)
	}
	if loadResponse.Data.SavedRequest.Method != "demo.v1.Greeter/SayHello" {
		t.Fatalf("expected loaded saved request method, got %+v", loadResponse.Data.SavedRequest)
	}
	if loadResponse.Data.SavedRequest.MetadataTemplate["x-tenant-id"] != "dev" {
		t.Fatalf("expected loaded metadata template, got %+v", loadResponse.Data.SavedRequest.MetadataTemplate)
	}
	body, ok := loadResponse.Data.SavedRequest.RequestSpec.Body.(map[string]interface{})
	if !ok || body["name"] != "world" {
		t.Fatalf("expected loaded request body, got %#v", loadResponse.Data.SavedRequest.RequestSpec.Body)
	}
	if loadResponse.Data.SavedRequest.CallOptions.RequestTimeoutMs != 1500 {
		t.Fatalf("expected loaded call options, got %+v", loadResponse.Data.SavedRequest.CallOptions)
	}
}

func TestDeleteRemovesOnlyWorkspaceManagedFilesAndClearsActiveWorkspace(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "delete-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	created := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "delete-workspace",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !created.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", created.Error)
	}

	saved := manager.RequestSave(context.Background(), contracts.RequestSaveInput{
		ID:          "greeter.say-hello",
		Method:      "demo.v1.Greeter/SayHello",
		RPCType:     contracts.RPCTypeUnary,
		EndpointRef: "ep_local",
		RequestSpec: contracts.SavedRequestSpec{Mode: "single", Body: map[string]any{}},
	})
	if !saved.Ok {
		t.Fatalf("expected saved request create to succeed, got %+v", saved.Error)
	}

	keepPath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(keepPath, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}

	deleted := manager.Delete(context.Background())
	if !deleted.Ok {
		t.Fatalf("expected workspace delete to succeed, got %+v", deleted.Error)
	}
	if _, err := os.Stat(filepath.Join(root, manifestFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected manifest to be removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "requests")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected saved requests directory to be removed, got %v", err)
	}
	if content, err := os.ReadFile(keepPath); err != nil || string(content) != "keep me" {
		t.Fatalf("expected unrelated file to stay intact, content=%q err=%v", content, err)
	}
	if _, active := manager.Active(); active {
		t.Fatal("expected deleted workspace to be closed")
	}
}

func TestRequestSaveRejectsDuplicateNameForEndpoint(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "duplicate-request-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "duplicates",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	request := func(id string) contracts.RequestSaveResponse {
		return manager.RequestSave(context.Background(), contracts.RequestSaveInput{
			ID:          id,
			Name:        "Create user",
			Method:      "demo.v1.Greeter/CreateUser",
			RPCType:     contracts.RPCTypeUnary,
			EndpointRef: "ep_local",
			RequestSpec: contracts.SavedRequestSpec{Mode: "single", Body: map[string]any{"name": "world"}},
		})
	}
	if response := request("greeter.create-user-1"); !response.Ok {
		t.Fatalf("expected first request save to succeed, got %+v", response.Error)
	}

	duplicate := request("greeter.create-user-2")
	if duplicate.Ok {
		t.Fatalf("expected duplicate request save to fail")
	}
	if duplicate.Error == nil || duplicate.Error.Code != "validation.saved_request_name_duplicate" {
		t.Fatalf("expected duplicate name validation error, got %+v", duplicate.Error)
	}
	if snapshot, ok := manager.Active(); ok && len(snapshot.SavedRequests) != 1 {
		t.Fatalf("expected duplicate request not to be added, got %+v", snapshot.SavedRequests)
	}
}

func TestRequestSaveAllowsRenamingSameSavedRequest(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "rename-request-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "rename",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	save := func(name string) contracts.RequestSaveResponse {
		return manager.RequestSave(context.Background(), contracts.RequestSaveInput{
			ID:          "greeter.create-user",
			Name:        name,
			Method:      "demo.v1.Greeter/CreateUser",
			RPCType:     contracts.RPCTypeUnary,
			EndpointRef: "ep_local",
			RequestSpec: contracts.SavedRequestSpec{Mode: "single", Body: map[string]any{"name": "world"}},
		})
	}
	if response := save("Create user"); !response.Ok {
		t.Fatalf("expected original request save to succeed, got %+v", response.Error)
	}
	if response := save("Create user with test data"); !response.Ok || response.Data == nil {
		t.Fatalf("expected saved request rename to succeed, got %+v", response.Error)
	}

	snapshot, ok := manager.Active()
	if !ok || len(snapshot.SavedRequests) != 1 || snapshot.SavedRequests[0].Name != "Create user with test data" {
		t.Fatalf("expected one renamed saved request, got %+v", snapshot.SavedRequests)
	}
}

func TestRequestGetRejectsMissingSavedRequest(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "missing-request-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "missing-request",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	response := manager.RequestGet(context.Background(), contracts.RequestGetInput{ID: "missing"})
	if response.Ok {
		t.Fatalf("expected missing saved request load to fail")
	}
	if response.Error == nil || response.Error.Code != "workspace.saved_request_not_found" {
		t.Fatalf("expected missing saved request error, got %+v", response.Error)
	}
}

func TestRequestDeleteRemovesSavedRequestFileAndManifestReference(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "delete-request-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "delete-request",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	saveResponse := manager.RequestSave(context.Background(), contracts.RequestSaveInput{
		ID:          "greeter.say-hello",
		Method:      "demo.v1.Greeter/SayHello",
		RPCType:     contracts.RPCTypeUnary,
		EndpointRef: "ep_local",
		RequestSpec: contracts.SavedRequestSpec{
			Mode: "single",
			Body: map[string]any{"name": "world"},
		},
	})
	if !saveResponse.Ok || saveResponse.Data == nil {
		t.Fatalf("expected request save to succeed, got %+v", saveResponse.Error)
	}
	requestPath := filepath.Join(root, saveResponse.Data.SavedRequest.Path)

	deleteResponse := manager.RequestDelete(context.Background(), contracts.RequestDeleteInput{
		ID: "greeter.say-hello",
	})
	if !deleteResponse.Ok || deleteResponse.Data == nil {
		t.Fatalf("expected request delete to succeed, got %+v", deleteResponse.Error)
	}
	if deleteResponse.Data.DeletedID != "greeter.say-hello" {
		t.Fatalf("expected deleted id, got %+v", deleteResponse.Data)
	}
	if len(deleteResponse.Data.Workspace.SavedRequests) != 0 {
		t.Fatalf("expected saved request summary to be removed, got %+v", deleteResponse.Data.Workspace.SavedRequests)
	}
	if _, err := os.Stat(requestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected saved request file to be removed, got %v", err)
	}
	manifestContent, err := os.ReadFile(filepath.Join(root, manifestFileName))
	if err != nil {
		t.Fatalf("read manifest file: %v", err)
	}
	if strings.Contains(string(manifestContent), "greeter.say-hello") {
		t.Fatalf("expected saved request reference to be removed, got %s", manifestContent)
	}
}

func TestSavePrunesSavedRequestsForDeletedEndpoints(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "pruned-request-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "pruned-request",
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
		RequestSpec: contracts.SavedRequestSpec{
			Mode: "single",
			Body: map[string]any{"name": "world"},
		},
	})
	if !requestResponse.Ok || requestResponse.Data == nil {
		t.Fatalf("expected request save to succeed, got %+v", requestResponse.Error)
	}

	saveResponse := manager.Save(context.Background(), contracts.WorkspaceSaveInput{
		Endpoints: []contracts.EndpointPreset{},
	})
	if !saveResponse.Ok || saveResponse.Data == nil {
		t.Fatalf("expected workspace save with deleted endpoint to succeed, got %+v", saveResponse.Error)
	}
	if len(saveResponse.Data.Workspace.SavedRequests) != 0 {
		t.Fatalf("expected saved request references to be pruned, got %+v", saveResponse.Data.Workspace.SavedRequests)
	}

	content, err := os.ReadFile(filepath.Join(root, manifestFileName))
	if err != nil {
		t.Fatalf("read manifest file: %v", err)
	}
	if strings.Contains(string(content), "greeter.say-hello") {
		t.Fatalf("expected manifest saved request reference to be pruned, got %s", content)
	}
}

func TestWorkspaceCloseClearsActiveWorkspace(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "close-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path: root,
		Name: "close-workspace",
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	closeResponse := manager.Close(context.Background())
	if !closeResponse.Ok {
		t.Fatalf("expected workspace close to succeed, got %+v", closeResponse.Error)
	}

	saveResponse := manager.Save(context.Background(), contracts.WorkspaceSaveInput{Name: "still-open"})
	if saveResponse.Ok {
		t.Fatalf("expected save after close to fail")
	}
	if saveResponse.Error == nil || saveResponse.Error.Code != "workspace.not_open" {
		t.Fatalf("expected no active workspace error, got %+v", saveResponse.Error)
	}
}

func TestWorkspaceEnvironmentsPersistAndAllowSavedRequestRef(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "environment-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "environments",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
		Environments: map[string]contracts.WorkspaceEnvironment{
			"local": {
				Values: map[string]string{
					"token":  "dev-token",
					"tenant": "dev",
				},
			},
		},
		Settings: &contracts.WorkspaceSettings{
			RedactDefaults:   true,
			CustomSecretKeys: []string{"x-internal-token"},
			EventRetention: &contracts.WorkspaceEventRetentionSettings{
				MaxEventsPerCall: 9,
				MaxBytesPerCall:  2048,
			},
		},
	})
	if !createResponse.Ok || createResponse.Data == nil {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}
	if createResponse.Data.Workspace.Environments["local"].Values["token"] != "dev-token" {
		t.Fatalf("expected environment values in snapshot, got %+v", createResponse.Data.Workspace.Environments)
	}

	requestResponse := manager.RequestSave(context.Background(), contracts.RequestSaveInput{
		ID:             "greeter.with-env",
		Method:         "demo.v1.Greeter/SayHello",
		RPCType:        contracts.RPCTypeUnary,
		EndpointRef:    "ep_local",
		EnvironmentRef: "local",
		MetadataTemplate: map[string]string{
			"authorization": "Bearer {{ env.token }}",
		},
		RequestSpec: contracts.SavedRequestSpec{
			Mode: "single",
			Body: map[string]any{"name": "world"},
		},
	})
	if !requestResponse.Ok || requestResponse.Data == nil {
		t.Fatalf("expected request save with environmentRef to succeed, got %+v", requestResponse.Error)
	}
	if requestResponse.Data.SavedRequest.EnvironmentRef != "local" {
		t.Fatalf("expected environment ref in summary, got %+v", requestResponse.Data.SavedRequest)
	}

	reopened := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	openResponse := reopened.Open(context.Background(), root)
	if !openResponse.Ok || openResponse.Data == nil {
		t.Fatalf("expected workspace reopen to succeed, got %+v", openResponse.Error)
	}
	if openResponse.Data.Workspace.Environments["local"].Values["tenant"] != "dev" {
		t.Fatalf("expected reopened environment values, got %+v", openResponse.Data.Workspace.Environments)
	}
	if openResponse.Data.Workspace.Settings == nil ||
		openResponse.Data.Workspace.Settings.EventRetention == nil ||
		len(openResponse.Data.Workspace.Settings.CustomSecretKeys) != 1 ||
		openResponse.Data.Workspace.Settings.CustomSecretKeys[0] != "x-internal-token" ||
		openResponse.Data.Workspace.Settings.EventRetention.MaxEventsPerCall != 9 {
		t.Fatalf("expected workspace settings in snapshot, got %+v", openResponse.Data.Workspace.Settings)
	}
	if len(openResponse.Data.Workspace.SavedRequests) != 1 ||
		openResponse.Data.Workspace.SavedRequests[0].EnvironmentRef != "local" {
		t.Fatalf("expected reopened saved request environmentRef, got %+v", openResponse.Data.Workspace.SavedRequests)
	}
}

func TestRequestSaveRejectsDanglingEnvironmentRef(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "dangling-environment-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "dangling-environment",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	response := manager.RequestSave(context.Background(), contracts.RequestSaveInput{
		ID:             "greeter.dangling-env",
		Method:         "demo.v1.Greeter/SayHello",
		RPCType:        contracts.RPCTypeUnary,
		EndpointRef:    "ep_local",
		EnvironmentRef: "missing",
		RequestSpec: contracts.SavedRequestSpec{
			Mode: "single",
			Body: map[string]any{"name": "world"},
		},
	})

	if response.Ok {
		t.Fatalf("expected dangling environmentRef save to fail")
	}
	if response.Error == nil || response.Error.Code != "validation.saved_request_environment_ref_invalid" {
		t.Fatalf("expected dangling environment validation error, got %+v", response.Error)
	}
}

func TestRequestSaveRejectsInvalidUnaryMode(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "request-mode-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "request-mode",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	response := manager.RequestSave(context.Background(), contracts.RequestSaveInput{
		ID:          "greeter.say-hello",
		Method:      "demo.v1.Greeter/SayHello",
		RPCType:     contracts.RPCTypeUnary,
		EndpointRef: "ep_local",
		RequestSpec: contracts.SavedRequestSpec{
			Mode: "static-sequence",
			Body: map[string]any{"name": "world"},
		},
	})

	if response.Ok {
		t.Fatalf("expected unary static sequence save to fail")
	}
	if response.Error == nil || response.Error.Code != "validation.saved_request_spec_mode_invalid" {
		t.Fatalf("expected saved request mode validation error, got %+v", response.Error)
	}
	if _, err := os.Stat(filepath.Join(root, "requests", "greeter.say-hello.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected invalid saved request not to be written, stat err=%v", err)
	}
}

func TestRequestSaveRejectsSensitiveInlineMetadataBeforeWriting(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "request-secret-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "request-secret",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	response := manager.RequestSave(context.Background(), contracts.RequestSaveInput{
		ID:          "greeter.say-hello",
		Method:      "demo.v1.Greeter/SayHello",
		RPCType:     contracts.RPCTypeUnary,
		EndpointRef: "ep_local",
		MetadataTemplate: map[string]string{
			"authorization": "Bearer raw-token",
		},
		RequestSpec: contracts.SavedRequestSpec{
			Mode: "single",
			Body: map[string]any{"name": "world"},
		},
	})

	if response.Ok {
		t.Fatalf("expected inline sensitive request metadata save to fail")
	}
	if response.Error == nil || response.Error.Code != "validation.workspace_metadata_secret_inline" {
		t.Fatalf("expected inline secret metadata validation error, got %+v", response.Error)
	}
	if _, err := os.Stat(filepath.Join(root, "requests", "greeter.say-hello.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected invalid saved request not to be written, stat err=%v", err)
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

func TestSaveRejectsInlineCustomSensitiveMetadataBeforeWritingWorkspace(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "custom-metadata-workspace")
	manager := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime})
	createResponse := manager.Create(context.Background(), contracts.WorkspaceCreateInput{
		Path:      root,
		Name:      "custom-metadata",
		Endpoints: []contracts.EndpointPreset{workspaceTestEndpoint("ep_local")},
		Settings: &contracts.WorkspaceSettings{
			RedactDefaults:   false,
			CustomSecretKeys: []string{"x-workspace-token"},
		},
	})
	if !createResponse.Ok {
		t.Fatalf("expected workspace create to succeed, got %+v", createResponse.Error)
	}

	endpointPreset := workspaceTestEndpoint("ep_local")
	endpointPreset.MetadataDefaults = map[string]string{
		"x-workspace-token": "raw-token",
	}
	response := manager.Save(context.Background(), contracts.WorkspaceSaveInput{
		Name:      "custom-metadata",
		Endpoints: []contracts.EndpointPreset{endpointPreset},
		Settings: &contracts.WorkspaceSettings{
			RedactDefaults:   false,
			CustomSecretKeys: []string{"x-workspace-token"},
		},
	})
	if response.Ok {
		t.Fatalf("expected inline custom secret metadata to fail save")
	}
	if response.Error == nil || response.Error.Code != "validation.workspace_metadata_secret_inline" {
		t.Fatalf("expected inline custom secret metadata validation error, got %+v", response.Error)
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

func TestOpenDegradesMalformedSavedRequestFile(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "degraded-workspace")
	requestsDir := filepath.Join(root, "requests")
	if err := os.MkdirAll(requestsDir, 0o755); err != nil {
		t.Fatalf("mkdir requests: %v", err)
	}
	writeWorkspaceYAML(t, filepath.Join(root, manifestFileName), map[string]any{
		"version": 1,
		"name":    "degraded",
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
		"savedRequests": []string{
			"requests/good.yaml",
			"requests/broken.yaml",
		},
	})
	writeWorkspaceYAML(t, filepath.Join(requestsDir, "good.yaml"), map[string]any{
		"id":          "good",
		"method":      "demo.v1.Greeter/SayHello",
		"rpcType":     "unary",
		"endpointRef": "ep_local",
		"requestSpec": map[string]any{
			"mode": "single",
			"body": map[string]any{"name": "world"},
		},
	})
	if err := os.WriteFile(filepath.Join(requestsDir, "broken.yaml"), []byte("id: [broken\n"), 0o644); err != nil {
		t.Fatalf("write broken saved request: %v", err)
	}

	response := NewManager(ManagerOptions{Now: fixedWorkspaceTestTime}).Open(context.Background(), root)
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected degraded workspace open to succeed, got %+v", response.Error)
	}
	if len(response.Data.Workspace.SavedRequests) != 1 || response.Data.Workspace.SavedRequests[0].ID != "good" {
		t.Fatalf("expected good saved request to load, got %+v", response.Data.Workspace.SavedRequests)
	}
	if len(response.Data.Issues) != 1 || response.Data.Issues[0].Code != "workspace.saved_request_invalid" {
		t.Fatalf("expected degraded saved request issue, got %+v", response.Data.Issues)
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
