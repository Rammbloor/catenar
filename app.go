package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"tether/internal/appshell"
	"tether/internal/contracts"
	"tether/internal/endpoint"
	"tether/internal/workspace"
)

// App wires Wails lifecycle hooks to the application shell service.
type App struct {
	ctx       context.Context
	shell     *appshell.Service
	workspace *workspace.Manager
	endpoint  *endpoint.Service
}

// NewApp creates the application root.
func NewApp() *App {
	workspaceManager := workspace.NewManager(workspace.ManagerOptions{})
	return &App{
		shell:     appshell.NewService(),
		workspace: workspaceManager,
		endpoint: endpoint.NewService(endpoint.ServiceDependencies{
			WorkspaceManager: workspaceManager,
		}),
	}
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
	return a.shell.Bootstrap()
}

// ShellEmitDiagnosticsProbe verifies the Wails event bridge by emitting a
// diagnostics update and returning an acknowledgement to the frontend.
func (a *App) ShellEmitDiagnosticsProbe() appshell.ProbeResponse {
	return a.shell.EmitDiagnosticsProbe()
}

// WorkspaceCreate creates a file-backed workspace manifest at the requested path.
func (a *App) WorkspaceCreate(input contracts.WorkspaceCreateInput) contracts.WorkspaceResponse {
	return a.workspace.Create(contextOrBackground(a.ctx), input)
}

// WorkspaceOpen opens a file-backed workspace manifest by directory or manifest path.
func (a *App) WorkspaceOpen(path string) contracts.WorkspaceResponse {
	return a.workspace.Open(contextOrBackground(a.ctx), path)
}

// WorkspaceSave persists the active workspace manifest with the latest editor draft.
func (a *App) WorkspaceSave(input contracts.WorkspaceSaveInput) contracts.WorkspaceResponse {
	return a.workspace.Save(contextOrBackground(a.ctx), input)
}

// WorkspaceValidate validates the active workspace draft without writing it.
func (a *App) WorkspaceValidate(input contracts.WorkspaceValidateInput) contracts.WorkspaceValidateResponse {
	return a.workspace.Validate(contextOrBackground(a.ctx), input)
}

// RequestSave writes a reusable saved request document and updates the workspace manifest.
func (a *App) RequestSave(input contracts.RequestSaveInput) contracts.RequestSaveResponse {
	return a.workspace.RequestSave(contextOrBackground(a.ctx), input)
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
