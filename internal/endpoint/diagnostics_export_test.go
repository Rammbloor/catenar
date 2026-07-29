package endpoint

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"catenar/internal/contracts"
)

func TestDiagnosticsExportWritesDiagnosticsAndSelectedHistory(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	appDataDir := t.TempDir()
	service := NewService(ServiceDependencies{
		AppDataDir: appDataDir,
		Now: func() time.Time {
			return now
		},
	})

	service.emitDiagnostic("test", now, &endpointDiagnostic{
		Level:    "error",
		Code:     "transport.grpc_not_ready",
		Category: contracts.ErrorCategoryTransport,
		Message:  "test diagnostic",
	})

	summaryPath := filepath.Join(appDataDir, "history", "summaries", "call-1.json")
	sessionLogPath := filepath.Join(appDataDir, "history", "session-logs", "call-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(summaryPath), 0o755); err != nil {
		t.Fatalf("mkdir summary dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sessionLogPath), 0o755); err != nil {
		t.Fatalf("mkdir session log dir: %v", err)
	}
	if err := writeJSONFile(summaryPath, storedUnaryHistoryDetail{
		RequestBody:  map[string]any{"name": "world"},
		ResponseBody: map[string]any{"message": "hello"},
		Status:       contracts.StreamStatus{Code: "OK"},
	}); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.WriteFile(sessionLogPath, []byte(`{"kind":"call_started"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session log: %v", err)
	}
	if err := service.historyStore.SaveCallSummary(context.Background(), contracts.HistoryCallSummary{
		CallID:         "call-1",
		SessionID:      "session-1",
		WorkspaceID:    "workspace-1",
		Method:         "demo.v1.Greeter/SayHello",
		RPCType:        contracts.RPCTypeUnary,
		EndpointID:     "endpoint-1",
		State:          contracts.StreamStateClosed,
		GRPCStatusCode: "OK",
		StartedAt:      now.Format(time.RFC3339Nano),
		FinishedAt:     now.Format(time.RFC3339Nano),
		RequestCount:   1,
		ResponseCount:  1,
		SummaryPath:    summaryPath,
		SessionLogPath: sessionLogPath,
	}); err != nil {
		t.Fatalf("save call summary: %v", err)
	}

	exportPath := filepath.Join(t.TempDir(), "diagnostics.zip")
	response := service.DiagnosticsExport(context.Background(), contracts.DiagnosticsExportInput{
		Path:    exportPath,
		CallIDs: []string{"call-1"},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected diagnostics export to succeed, got %+v", response.Error)
	}
	if response.Data.Path != exportPath {
		t.Fatalf("expected export path in response, got %+v", response.Data)
	}
	if !slices.Contains(response.Data.IncludedCalls, "call-1") {
		t.Fatalf("expected selected call in export response, got %+v", response.Data.IncludedCalls)
	}

	archive, err := zip.OpenReader(exportPath)
	if err != nil {
		t.Fatalf("open export zip: %v", err)
	}
	defer func() {
		_ = archive.Close()
	}()

	names := make([]string, 0, len(archive.File))
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	for _, expected := range []string{
		"metadata.json",
		"diagnostics.json",
		"history/calls.json",
		"history/call-1/summary.json",
		"history/call-1/session.jsonl",
	} {
		if !slices.Contains(names, expected) {
			t.Fatalf("expected %s in export zip, got %+v", expected, names)
		}
	}
}
