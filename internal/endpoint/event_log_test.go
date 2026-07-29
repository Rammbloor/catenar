package endpoint

import (
	"context"
	"testing"
	"time"

	"catenar/internal/contracts"
)

func TestUnaryEventLogAppliesWorkspaceRedactionPolicy(t *testing.T) {
	t.Parallel()

	log := newFileEventLog(t.TempDir())
	startedAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	policy := NewMetadataRedactionPolicy(false, []string{"x-workspace-token"})
	artifacts, err := log.WriteUnaryCall(context.Background(), UnaryEventLogRecord{
		CallID:      "call_custom_redaction",
		SessionID:   "session_custom_redaction",
		EndpointID:  "endpoint-local",
		WorkspaceID: "workspace-local",
		Method:      "demo.v1.Greeter/SayHello",
		RPCType:     contracts.RPCTypeUnary,
		FinalState:  contracts.StreamStateClosed,
		StartedAt:   startedAt,
		FinishedAt:  startedAt.Add(3 * time.Millisecond),
		Duration:    3 * time.Millisecond,
		RequestBody: map[string]any{"name": "Ada"},
		ResponseBody: map[string]any{
			"message": "hello",
		},
		RequestMetadata: map[string]string{
			"authorization":     "Bearer visible-when-defaults-disabled",
			"x-workspace-token": "custom-secret",
			"x-request-id":      "req-1",
		},
		Headers: map[string][]string{
			"x-workspace-token": {"response-secret"},
			"set-cookie":        {"visible-cookie"},
		},
		Trailers: map[string][]string{
			"x-workspace-token": {"trailer-secret"},
		},
		Status:          contracts.StreamStatus{Code: "OK"},
		RedactionPolicy: policy,
	})
	if err != nil {
		t.Fatalf("write unary call: %v", err)
	}

	detail, err := readStoredUnaryHistoryDetail(artifacts.SummaryPath)
	if err != nil {
		t.Fatalf("read history detail: %v", err)
	}
	if detail.Headers["x-workspace-token"][0] != "[REDACTED]" {
		t.Fatalf("expected custom response header to be redacted, got %+v", detail.Headers)
	}

	startEvent := artifacts.Events[0]
	if startEvent.GRPC == nil {
		t.Fatalf("expected grpc metadata on call started event")
	}
	if startEvent.GRPC.Metadata["x-workspace-token"][0] != "[REDACTED]" {
		t.Fatalf("expected custom request metadata to be redacted, got %+v", startEvent.GRPC.Metadata)
	}
	if startEvent.GRPC.Metadata["authorization"][0] != "Bearer visible-when-defaults-disabled" {
		t.Fatalf("expected defaults-disabled metadata to remain visible, got %+v", startEvent.GRPC.Metadata)
	}
	if startEvent.GRPC.Metadata["x-request-id"][0] != "req-1" {
		t.Fatalf("expected ordinary metadata to remain visible, got %+v", startEvent.GRPC.Metadata)
	}
}
