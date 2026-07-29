package endpoint

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"catenar/internal/contracts"
)

const diagnosticsExportHistoryLimit = 100

type diagnosticsExportMetadata struct {
	ExportedAt       string   `json:"exportedAt"`
	DiagnosticsCount int      `json:"diagnosticsCount"`
	IncludeHistory   bool     `json:"includeHistory"`
	RequestedCallIDs []string `json:"requestedCallIds,omitempty"`
	IncludedCalls    []string `json:"includedCalls,omitempty"`
}

type diagnosticsExportCallArtifact struct {
	Summary contracts.HistoryCallSummary `json:"summary"`
	Detail  *storedUnaryHistoryDetail    `json:"detail,omitempty"`
}

func (s *Service) DiagnosticsExport(ctx context.Context, input contracts.DiagnosticsExportInput) contracts.DiagnosticsExportResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	exportPath := strings.TrimSpace(input.Path)
	if exportPath == "" {
		return contracts.DiagnosticsExportResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.diagnostics_export_path_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Choose a destination path before exporting diagnostics.",
			},
		}
	}

	includeHistory := input.IncludeHistory || len(input.CallIDs) > 0
	calls, includedCallIDs, errEnvelope := s.diagnosticsExportCalls(ctx, includeHistory, input.CallIDs)
	if errEnvelope != nil {
		return contracts.DiagnosticsExportResponse{Ok: false, Error: errEnvelope}
	}

	diagnostics := s.diagnosticsSnapshot()
	exportedAt := s.now().UTC()
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		return contracts.DiagnosticsExportResponse{
			Ok:    false,
			Error: diagnosticsExportFileError("application.diagnostics_export_directory_failed", "The diagnostics export directory could not be created.", err),
		}
	}

	file, err := os.Create(exportPath)
	if err != nil {
		return contracts.DiagnosticsExportResponse{
			Ok:    false,
			Error: diagnosticsExportFileError("application.diagnostics_export_create_failed", "The diagnostics export file could not be created.", err),
		}
	}

	fileCount := 0
	closeFile := true
	defer func() {
		if closeFile {
			_ = file.Close()
		}
	}()

	zipWriter := zip.NewWriter(file)
	addJSON := func(name string, value any) error {
		writer, createErr := zipWriter.Create(name)
		if createErr != nil {
			return createErr
		}
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(value); encodeErr != nil {
			return encodeErr
		}
		fileCount++
		return nil
	}
	addFile := func(name, path string) error {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				return nil
			}
			return readErr
		}
		writer, createErr := zipWriter.Create(name)
		if createErr != nil {
			return createErr
		}
		if _, writeErr := writer.Write(content); writeErr != nil {
			return writeErr
		}
		fileCount++
		return nil
	}

	metadata := diagnosticsExportMetadata{
		ExportedAt:       exportedAt.Format(time.RFC3339Nano),
		DiagnosticsCount: len(diagnostics),
		IncludeHistory:   includeHistory,
		RequestedCallIDs: uniqueTrimmedStrings(input.CallIDs),
		IncludedCalls:    includedCallIDs,
	}
	if err := addJSON("metadata.json", metadata); err != nil {
		return diagnosticsExportWriteFailure(file, zipWriter, exportPath, err)
	}
	if err := addJSON("diagnostics.json", diagnostics); err != nil {
		return diagnosticsExportWriteFailure(file, zipWriter, exportPath, err)
	}

	if includeHistory {
		if err := addJSON("history/calls.json", calls); err != nil {
			return diagnosticsExportWriteFailure(file, zipWriter, exportPath, err)
		}
		for _, call := range calls {
			segment := safeArchiveSegment(call.CallID)
			artifact := diagnosticsExportCallArtifact{Summary: call}
			if strings.TrimSpace(call.SummaryPath) != "" {
				if detail, detailErr := readStoredUnaryHistoryDetail(call.SummaryPath); detailErr == nil {
					artifact.Detail = &detail
				}
			}
			if err := addJSON(fmt.Sprintf("history/%s/summary.json", segment), artifact); err != nil {
				return diagnosticsExportWriteFailure(file, zipWriter, exportPath, err)
			}
			if strings.TrimSpace(call.SessionLogPath) != "" {
				if err := addFile(fmt.Sprintf("history/%s/session.jsonl", segment), call.SessionLogPath); err != nil {
					return diagnosticsExportWriteFailure(file, zipWriter, exportPath, err)
				}
			}
		}
	}

	if err := zipWriter.Close(); err != nil {
		return diagnosticsExportWriteFailure(file, nil, exportPath, err)
	}
	if err := file.Close(); err != nil {
		closeFile = false
		_ = os.Remove(exportPath)
		return contracts.DiagnosticsExportResponse{
			Ok:    false,
			Error: diagnosticsExportFileError("application.diagnostics_export_write_failed", "The diagnostics export file could not be finalized.", err),
		}
	}
	closeFile = false

	return contracts.DiagnosticsExportResponse{
		Ok: true,
		Data: &contracts.DiagnosticsExportResult{
			Path:          exportPath,
			FileCount:     fileCount,
			IncludedCalls: includedCallIDs,
			ExportedAt:    exportedAt.Format(time.RFC3339Nano),
		},
	}
}

func (s *Service) diagnosticsExportCalls(ctx context.Context, includeHistory bool, callIDs []string) ([]contracts.HistoryCallSummary, []string, *contracts.ErrorEnvelope) {
	if !includeHistory {
		return nil, nil, nil
	}
	if s.initializationErr != nil || s.historyStore == nil {
		return nil, nil, wrapHistoryStoreError("initialize", s.initializationErr)
	}

	ids := uniqueTrimmedStrings(callIDs)
	calls := make([]contracts.HistoryCallSummary, 0, len(ids))
	if len(ids) > 0 {
		for _, id := range ids {
			summary, err := s.historyStore.GetCallSummary(ctx, id)
			if err != nil {
				return nil, nil, wrapHistoryStoreError("get", err)
			}
			calls = append(calls, summary)
		}
		return calls, historyCallIDs(calls), nil
	}

	calls, err := s.historyStore.ListCalls(ctx, contracts.HistoryListInput{Limit: diagnosticsExportHistoryLimit})
	if err != nil {
		return nil, nil, wrapHistoryStoreError("list", err)
	}
	return calls, historyCallIDs(calls), nil
}

func diagnosticsExportWriteFailure(file *os.File, zipWriter *zip.Writer, path string, err error) contracts.DiagnosticsExportResponse {
	if zipWriter != nil {
		_ = zipWriter.Close()
	}
	if file != nil {
		_ = file.Close()
	}
	_ = os.Remove(path)

	return contracts.DiagnosticsExportResponse{
		Ok:    false,
		Error: diagnosticsExportFileError("application.diagnostics_export_write_failed", "The diagnostics export file could not be written.", err),
	}
}

func diagnosticsExportFileError(code, message string, err error) *contracts.ErrorEnvelope {
	return &contracts.ErrorEnvelope{
		Code:     code,
		Category: contracts.ErrorCategoryApplication,
		Message:  message,
		Details: map[string]string{
			"cause": err.Error(),
		},
	}
}

func historyCallIDs(calls []contracts.HistoryCallSummary) []string {
	if len(calls) == 0 {
		return nil
	}

	ids := make([]string, 0, len(calls))
	for _, call := range calls {
		ids = append(ids, call.CallID)
	}
	return ids
}

func uniqueTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func safeArchiveSegment(value string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	result := strings.Trim(builder.String(), "._")
	if result == "" {
		return "call"
	}
	return result
}
