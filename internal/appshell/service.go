package appshell

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"tether/internal/contracts"
)

const wailsVersion = "v2.12.0"

type EventEmitter interface {
	Emit(eventName contracts.EventName, payload any) error
}

type Service struct {
	emitter EventEmitter
	now     func() time.Time
}

func NewService() *Service {
	return &Service{
		now: time.Now,
	}
}

func (s *Service) SetEmitter(emitter EventEmitter) {
	s.emitter = emitter
}

func (s *Service) Bootstrap() BootstrapResponse {
	data := BootstrapData{
		App: AppMetadata{
			Name:         "tether",
			ProductLine:  "Desktop-first gRPC debugging workspace",
			Platform:     runtime.GOOS,
			Architecture: runtime.GOARCH,
			GoVersion:    runtime.Version(),
			WailsVersion: wailsVersion,
		},
		Contract: contracts.BuildContractManifest(),
		Layout: LayoutDefinition{
			Regions: []LayoutRegion{
				{ID: "navigation", Title: "Navigation rail", Purpose: "View switching without an external router."},
				{ID: "primary", Title: "Primary canvas", Purpose: "Home, workspace and session views share one shell container."},
				{ID: "secondary", Title: "Inspector rail", Purpose: "Diagnostics and session context stay visible without changing route."},
				{ID: "footer", Title: "Status strip", Purpose: "Runtime health, event bridge state and packaging readiness."},
			},
		},
		StateModel: StateModelSummary{
			PrimaryFlow: []string{
				string(contracts.AppViewHome),
				string(contracts.AppViewWorkspace),
				string(contracts.AppViewSession),
			},
			OverlayViews: []string{
				string(contracts.AppOverlayHistory),
				string(contracts.AppOverlaySettings),
				string(contracts.AppOverlayDiagnostics),
			},
			SingleActiveLiveSession: true,
		},
		EpicZero: []SliceStatus{
			{Slice: "0.1", Status: "implemented", Summary: "Wails shell, Svelte app frame, runtime binding and diagnostics event round-trip."},
			{Slice: "0.2", Status: "implemented", Summary: "Contract manifest, invoke DTOs and module boundaries encoded in shared runtime metadata."},
			{Slice: "0.3", Status: "implemented", Summary: "Canonical stream states, overlays and error taxonomy wired into shared code paths."},
		},
	}

	return BootstrapResponse{
		Ok:   true,
		Data: &data,
	}
}

func (s *Service) EmitDiagnosticsProbe() ProbeResponse {
	if s.emitter == nil {
		return ProbeResponse{
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

	now := s.now().UTC()
	event := contracts.DiagnosticsUpdateEvent{
		ID:       fmt.Sprintf("diag_%d", now.UnixNano()),
		Source:   "shell",
		Level:    "info",
		Code:     "application.runtime_ready",
		Category: contracts.ErrorCategoryApplication,
		Message:  "Frontend/backend event bridge verified successfully.",
		NextStep: "Proceed with Epic 1 runtime features on top of the validated shell.",
		Details: map[string]string{
			"eventName": string(contracts.EventDiagnosticsUpdate),
			"layer":     "runtime.EventsEmit",
		},
		Timestamp: now.Format(time.RFC3339Nano),
	}

	if err := s.emitter.Emit(contracts.EventDiagnosticsUpdate, event); err != nil {
		return ProbeResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.emit_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "Failed to emit the diagnostics probe event.",
				Details: map[string]string{
					"cause": err.Error(),
				},
			},
		}
	}

	ack := ProbeAcknowledgement{
		EventID:        event.ID,
		EventName:      string(contracts.EventDiagnosticsUpdate),
		EmittedAt:      event.Timestamp,
		Classification: event.Code,
	}

	return ProbeResponse{
		Ok:   true,
		Data: &ack,
	}
}

type failingEmitter struct{}

func (f failingEmitter) Emit(contracts.EventName, any) error {
	return errors.New("emit not supported")
}
