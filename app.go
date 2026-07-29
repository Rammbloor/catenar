package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"catenar/internal/appshell"
	"catenar/internal/appupdate"
	"catenar/internal/contracts"
	"catenar/internal/endpoint"
	"catenar/internal/githubsync"
	"catenar/internal/workspace"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var appVersion = "0.0.0"
var githubRepo = "Rammbloor/catenar"

// App wires Wails lifecycle hooks to the application shell service.
type App struct {
	ctx       context.Context
	shell     *appshell.Service
	updates   *appupdate.Service
	workspace *workspace.Manager
	github    *githubsync.Service
	endpoint  *endpoint.Service
}

type appOptions struct {
	workspaceStatePath    string
	deferWorkspaceRestore bool
}

// NewApp creates the application root.
func NewApp() *App {
	return newApp(appOptions{
		workspaceStatePath:    workspaceRuntimeStatePath(),
		deferWorkspaceRestore: true,
	})
}

func newApp(options appOptions) *App {
	workspaceManager := workspace.NewManager(workspace.ManagerOptions{
		StatePath:    options.workspaceStatePath,
		DeferRestore: options.deferWorkspaceRestore,
	})
	return &App{
		shell:     appshell.NewService(appVersion),
		updates:   appupdate.NewService(appupdate.Options{AppVersion: appVersion, GitHubRepo: githubRepo}),
		workspace: workspaceManager,
		github:    githubsync.NewService(githubsync.Options{Manager: workspaceManager}),
		endpoint: endpoint.NewService(endpoint.ServiceDependencies{
			WorkspaceManager: workspaceManager,
		}),
	}
}

func workspaceRuntimeStatePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "Catenar", "runtime-session.json")
}

// GitHubWorkspaceLink connects the active workspace to an existing GitHub repository.
func (a *App) GitHubWorkspaceLink(input contracts.GitHubWorkspaceLinkInput) contracts.GitHubSyncResponse {
	return a.github.Link(contextOrBackground(a.ctx), input)
}

// GitHubWorkspaceSetToken stores an HTTPS Personal Access Token for the active
// workspace in the system credential store. The token never enters workspace files.
func (a *App) GitHubWorkspaceSetToken(input contracts.GitHubWorkspaceCredentialInput) contracts.GitHubSyncResponse {
	return a.github.SetToken(contextOrBackground(a.ctx), input)
}

// GitHubWorkspaceStatus returns the current local/remote sync state.
func (a *App) GitHubWorkspaceStatus() contracts.GitHubSyncResponse {
	return a.github.Status(contextOrBackground(a.ctx))
}

// GitHubWorkspacePull applies remote workspace changes only when there is no local conflict.
func (a *App) GitHubWorkspacePull(input contracts.GitHubSyncActionInput) contracts.GitHubSyncResponse {
	return a.github.Pull(contextOrBackground(a.ctx), input)
}

// GitHubWorkspacePush commits and pushes only the active workspace directory.
func (a *App) GitHubWorkspacePush(input contracts.GitHubSyncActionInput) contracts.GitHubSyncResponse {
	return a.github.Push(contextOrBackground(a.ctx), input)
}

// GitHubWorkspaceUnlink removes the local GitHub link and its private cache.
func (a *App) GitHubWorkspaceUnlink() contracts.GitHubSyncResponse {
	return a.github.Unlink(contextOrBackground(a.ctx))
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	emitter := wailsEmitter{ctx: ctx}
	a.shell.SetEmitter(emitter)
	a.endpoint.SetEmitter(emitter)
}

// ShellBootstrap returns frontend bootstrap metadata for the app shell.
func (a *App) ShellBootstrap() appshell.BootstrapResponse {
	// Restore only after the Wails bridge is ready. This makes the workspace
	// snapshot and the initial frontend bootstrap one coherent operation.
	a.workspace.RestoreLastWorkspace()
	response := a.shell.Bootstrap()
	if response.Data == nil {
		return response
	}
	if snapshot, ok := a.workspace.Active(); ok {
		response.Data.Workspace = &snapshot
	}
	return response
}

// ShellEmitDiagnosticsProbe verifies the Wails event bridge by emitting a
// diagnostics update and returning an acknowledgement to the frontend.
func (a *App) ShellEmitDiagnosticsProbe() appshell.ProbeResponse {
	return a.shell.EmitDiagnosticsProbe()
}

// UpdateCheck checks the latest published GitHub release for the application.
func (a *App) UpdateCheck() contracts.UpdateCheckResponse {
	return a.updates.Check(contextOrBackground(a.ctx))
}

// WorkspaceCreate creates a file-backed workspace manifest at the requested path.
func (a *App) WorkspaceCreate(input contracts.WorkspaceCreateInput) contracts.WorkspaceResponse {
	return a.workspace.Create(contextOrBackground(a.ctx), input)
}

// WorkspaceOpen opens a file-backed workspace manifest by directory or manifest path.
func (a *App) WorkspaceOpen(path string) contracts.WorkspaceResponse {
	return a.workspace.Open(contextOrBackground(a.ctx), path)
}

// WorkspaceActive returns the restored active workspace without making the
// frontend infer it from browser storage.
func (a *App) WorkspaceActive() contracts.WorkspaceActiveResponse {
	snapshot, ok := a.workspace.Active()
	if !ok {
		return contracts.WorkspaceActiveResponse{Ok: true}
	}
	return contracts.WorkspaceActiveResponse{Ok: true, Data: &snapshot}
}

// WorkspaceClose clears the active file-backed workspace from the runtime session.
func (a *App) WorkspaceClose() contracts.WorkspaceCloseResponse {
	return a.workspace.Close(contextOrBackground(a.ctx))
}

// WorkspaceDelete removes the active workspace manifest and its saved requests.
// Other files in the workspace folder are intentionally preserved.
func (a *App) WorkspaceDelete() contracts.WorkspaceCloseResponse {
	if unlinked := a.github.Unlink(contextOrBackground(a.ctx)); !unlinked.Ok {
		return contracts.WorkspaceCloseResponse{Ok: false, Error: unlinked.Error}
	}
	return a.workspace.Delete(contextOrBackground(a.ctx))
}

// WorkspaceSave persists the active workspace manifest with the latest editor draft.
func (a *App) WorkspaceSave(input contracts.WorkspaceSaveInput) contracts.WorkspaceResponse {
	return a.workspace.Save(contextOrBackground(a.ctx), input)
}

// WorkspaceValidate validates the active workspace draft without writing it.
func (a *App) WorkspaceValidate(input contracts.WorkspaceValidateInput) contracts.WorkspaceValidateResponse {
	return a.workspace.Validate(contextOrBackground(a.ctx), input)
}

// DialogSelectWorkspaceCreatePath opens the native save dialog for a workspace manifest path.
func (a *App) DialogSelectWorkspaceCreatePath() string {
	if a.ctx == nil {
		return ""
	}

	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:                "Create workspace",
		DefaultFilename:      "workspace.yaml",
		CanCreateDirectories: true,
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Workspace manifest (*.yaml)",
				Pattern:     "*.yaml",
			},
			{
				DisplayName: "YAML files (*.yml)",
				Pattern:     "*.yml",
			},
		},
	})
	if err != nil {
		return ""
	}

	return path
}

// DialogSelectWorkspaceOpenPath opens the native picker for an existing workspace manifest.
func (a *App) DialogSelectWorkspaceOpenPath() string {
	if a.ctx == nil {
		return ""
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open workspace",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Workspace manifest (*.yaml)",
				Pattern:     "*.yaml",
			},
			{
				DisplayName: "YAML files (*.yml)",
				Pattern:     "*.yml",
			},
		},
		ResolvesAliases:            true,
		TreatPackagesAsDirectories: true,
	})
	if err != nil {
		return ""
	}

	return path
}

// RequestSave writes a reusable saved request document and updates the workspace manifest.
func (a *App) RequestSave(input contracts.RequestSaveInput) contracts.RequestSaveResponse {
	return a.workspace.RequestSave(contextOrBackground(a.ctx), input)
}

// RequestGet returns a reusable saved request document from the active workspace.
func (a *App) RequestGet(input contracts.RequestGetInput) contracts.RequestGetResponse {
	return a.workspace.RequestGet(contextOrBackground(a.ctx), input)
}

// RequestDelete removes a reusable saved request document from the active workspace.
func (a *App) RequestDelete(input contracts.RequestDeleteInput) contracts.RequestDeleteResponse {
	return a.workspace.RequestDelete(contextOrBackground(a.ctx), input)
}

// EndpointTest runs transport and TLS preflight checks for a single endpoint preset.
func (a *App) EndpointTest(input contracts.EndpointTestInput) contracts.EndpointTestResponse {
	if a.ctx == nil {
		return contracts.EndpointTestResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.EndpointTest(a.ctx, input)
}

// MaterialRegisterFile stores a local file-backed material handle and returns a secret-ref.
func (a *App) MaterialRegisterFile(input contracts.MaterialRegisterFileInput) contracts.MaterialRegisterFileResponse {
	return a.endpoint.MaterialRegisterFile(contextOrBackground(a.ctx), input)
}

// CatalogLoadFromReflection loads a service and method catalog from gRPC Server Reflection.
func (a *App) CatalogLoadFromReflection(input contracts.CatalogLoadFromReflectionInput) contracts.CatalogLoadFromReflectionResponse {
	if a.ctx == nil {
		return contracts.CatalogLoadFromReflectionResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.CatalogLoadFromReflection(a.ctx, input)
}

// CatalogLoadFromProtoSources loads a service and method catalog from local proto sources and import paths.
func (a *App) CatalogLoadFromProtoSources(input contracts.CatalogLoadFromProtoSourcesInput) contracts.CatalogLoadFromProtoSourcesResponse {
	if a.ctx == nil {
		return contracts.CatalogLoadFromProtoSourcesResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.CatalogLoadFromProtoSources(a.ctx, input)
}

// CallInvokeUnary executes a unary RPC using the cached method catalog for the selected endpoint.
func (a *App) CallInvokeUnary(input contracts.CallInvokeUnaryInput) contracts.CallInvokeUnaryResponse {
	if a.ctx == nil {
		return contracts.CallInvokeUnaryResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.CallInvokeUnary(a.ctx, input)
}

// CallStartStream starts a server-streaming RPC or a client-streaming static sequence and emits live session updates.
func (a *App) CallStartStream(input contracts.CallStartStreamInput) contracts.CallStartStreamResponse {
	if a.ctx == nil {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.CallStartStream(a.ctx, input)
}

// CallSendMessage sends one manual request message to an active interactive client stream.
func (a *App) CallSendMessage(input contracts.CallSendMessageInput) contracts.CallSendMessageResponse {
	if a.ctx == nil {
		return contracts.CallSendMessageResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.CallSendMessage(a.ctx, input)
}

// CallHalfClose closes the local send side for an active interactive client stream.
func (a *App) CallHalfClose(input contracts.CallHalfCloseInput) contracts.CallHalfCloseResponse {
	if a.ctx == nil {
		return contracts.CallHalfCloseResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.CallHalfClose(a.ctx, input)
}

// CallCancel requests cancellation for the active live stream session.
func (a *App) CallCancel(input contracts.CallCancelInput) contracts.CallCancelResponse {
	if a.ctx == nil {
		return contracts.CallCancelResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.CallCancel(a.ctx, input)
}

// HistoryList returns recent persisted unary call summaries from the local history store.
func (a *App) HistoryList(input contracts.HistoryListInput) contracts.HistoryListResponse {
	if a.ctx == nil {
		return contracts.HistoryListResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.HistoryList(a.ctx, input)
}

// HistoryGet returns one persisted unary history entry together with its stored session artifact.
func (a *App) HistoryGet(callID string) contracts.HistoryGetResponse {
	if a.ctx == nil {
		return contracts.HistoryGetResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.runtime_not_ready",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The Wails runtime context is not ready yet.",
				Details: map[string]string{
					"expectedHook": "startup",
				},
			},
		}
	}

	return a.endpoint.HistoryGet(a.ctx, callID)
}

// DiagnosticsExport writes a redacted diagnostics bundle for troubleshooting.
func (a *App) DiagnosticsExport(input contracts.DiagnosticsExportInput) contracts.DiagnosticsExportResponse {
	if strings.TrimSpace(input.Path) == "" {
		if a.ctx == nil {
			return contracts.DiagnosticsExportResponse{
				Ok: false,
				Error: &contracts.ErrorEnvelope{
					Code:     "application.runtime_not_ready",
					Category: contracts.ErrorCategoryApplication,
					Message:  "The Wails runtime context is not ready yet.",
					Details: map[string]string{
						"expectedHook": "startup",
					},
				},
			}
		}

		path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
			Title:                "Export diagnostics",
			DefaultFilename:      "catenar-diagnostics.zip",
			CanCreateDirectories: true,
			Filters: []wailsruntime.FileFilter{
				{
					DisplayName: "ZIP archive (*.zip)",
					Pattern:     "*.zip",
				},
			},
		})
		if err != nil {
			return contracts.DiagnosticsExportResponse{
				Ok: false,
				Error: &contracts.ErrorEnvelope{
					Code:     "application.diagnostics_export_dialog_failed",
					Category: contracts.ErrorCategoryApplication,
					Message:  "The diagnostics export dialog could not be opened.",
					Details: map[string]string{
						"cause": err.Error(),
					},
				},
			}
		}
		if strings.TrimSpace(path) == "" {
			return contracts.DiagnosticsExportResponse{
				Ok: false,
				Error: &contracts.ErrorEnvelope{
					Code:     "cancelled.export_dialog",
					Category: contracts.ErrorCategoryCancelled,
					Message:  "Diagnostics export was cancelled.",
				},
			}
		}
		input.Path = path
	}

	return a.endpoint.DiagnosticsExport(contextOrBackground(a.ctx), input)
}

// DialogSelectProtoFiles opens the native Wails picker and returns absolute .proto file paths.
func (a *App) DialogSelectProtoFiles() []string {
	if a.ctx == nil {
		return nil
	}

	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select .proto files",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Protocol buffers (*.proto)",
				Pattern:     "*.proto",
			},
		},
		ResolvesAliases:            true,
		TreatPackagesAsDirectories: true,
	})
	if err != nil {
		return nil
	}

	return paths
}

// DialogSelectProtoDirectory opens the native Wails picker and returns one proto source directory path.
func (a *App) DialogSelectProtoDirectory() string {
	if a.ctx == nil {
		return ""
	}

	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                      "Select proto directory",
		CanCreateDirectories:       false,
		ResolvesAliases:            true,
		TreatPackagesAsDirectories: true,
	})
	if err != nil {
		return ""
	}

	return path
}

// DialogSelectMaterialFile opens the native Wails picker and returns one material file path.
func (a *App) DialogSelectMaterialFile() string {
	if a.ctx == nil {
		return ""
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                      "Select material file",
		CanCreateDirectories:       false,
		ResolvesAliases:            true,
		TreatPackagesAsDirectories: true,
	})
	if err != nil {
		return ""
	}

	return path
}

type wailsEmitter struct {
	ctx context.Context
}

func (e wailsEmitter) Emit(eventName contracts.EventName, payload any) error {
	wailsruntime.EventsEmit(e.ctx, string(eventName), payload)
	return nil
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}

	return context.Background()
}
