package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"tether/internal/appshell"
	"tether/internal/contracts"
	"tether/internal/endpoint"
)

// App wires Wails lifecycle hooks to the application shell service.
type App struct {
	ctx      context.Context
	shell    *appshell.Service
	endpoint *endpoint.Service
}

// NewApp creates the application root.
func NewApp() *App {
	return &App{
		shell:    appshell.NewService(),
		endpoint: endpoint.NewService(endpoint.ServiceDependencies{}),
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

type wailsEmitter struct {
	ctx context.Context
}

func (e wailsEmitter) Emit(eventName contracts.EventName, payload any) error {
	wailsruntime.EventsEmit(e.ctx, string(eventName), payload)
	return nil
}
