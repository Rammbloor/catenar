package appshell

import (
	"errors"
	"testing"
	"time"

	"tether/internal/contracts"
)

type emitterSpy struct {
	eventName contracts.EventName
	payload   any
	err       error
}

func (e *emitterSpy) Emit(eventName contracts.EventName, payload any) error {
	e.eventName = eventName
	e.payload = payload
	return e.err
}

func TestBootstrapReturnsContractAndLayoutMetadata(t *testing.T) {
	t.Parallel()

	service := NewService()

	response := service.Bootstrap()

	if !response.Ok {
		t.Fatalf("expected bootstrap response to be ok")
	}

	if response.Data == nil {
		t.Fatalf("expected bootstrap data")
	}

	if response.Data.App.Name != "tether" {
		t.Fatalf("expected app name tether, got %q", response.Data.App.Name)
	}

	if got := len(response.Data.Layout.Regions); got != 4 {
		t.Fatalf("expected 4 layout regions, got %d", got)
	}

	if response.Data.StateModel.SingleActiveLiveSession != true {
		t.Fatalf("expected single active live session rule to be enabled")
	}
}

func TestEmitDiagnosticsProbePublishesDiagnosticsUpdate(t *testing.T) {
	t.Parallel()

	spy := &emitterSpy{}
	service := NewService()
	service.now = func() time.Time {
		return time.Date(2026, time.April, 27, 12, 30, 45, 0, time.UTC)
	}
	service.SetEmitter(spy)

	response := service.EmitDiagnosticsProbe()

	if !response.Ok {
		t.Fatalf("expected diagnostics probe to succeed: %+v", response.Error)
	}

	if response.Data == nil {
		t.Fatalf("expected probe acknowledgement")
	}

	if spy.eventName != contracts.EventDiagnosticsUpdate {
		t.Fatalf("expected diagnostics:update event, got %q", spy.eventName)
	}

	payload, ok := spy.payload.(contracts.DiagnosticsUpdateEvent)
	if !ok {
		t.Fatalf("expected diagnostics payload type, got %T", spy.payload)
	}

	if payload.Code != "application.runtime_ready" {
		t.Fatalf("expected application.runtime_ready code, got %q", payload.Code)
	}

	if response.Data.EventID != payload.ID {
		t.Fatalf("expected acknowledgement event id %q, got %q", payload.ID, response.Data.EventID)
	}
}

func TestEmitDiagnosticsProbeReturnsStructuredErrorWhenEmitterFails(t *testing.T) {
	t.Parallel()

	spy := &emitterSpy{err: errors.New("bridge offline")}
	service := NewService()
	service.SetEmitter(spy)

	response := service.EmitDiagnosticsProbe()

	if response.Ok {
		t.Fatalf("expected diagnostics probe to fail")
	}

	if response.Error == nil {
		t.Fatalf("expected structured error envelope")
	}

	if response.Error.Code != "application.emit_failed" {
		t.Fatalf("expected application.emit_failed code, got %q", response.Error.Code)
	}
}
