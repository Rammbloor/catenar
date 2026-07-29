package endpoint

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"catenar/internal/contracts"
)

var secretMetadataKeys = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"x-api-key":           {},
	"api-key":             {},
	"apikey":              {},
	"cookie":              {},
	"set-cookie":          {},
}

type MetadataRedactionPolicy struct {
	redactDefaults   bool
	customSecretKeys map[string]struct{}
	configured       bool
}

func DefaultMetadataRedactionPolicy() MetadataRedactionPolicy {
	return NewMetadataRedactionPolicy(true, nil)
}

func NewMetadataRedactionPolicy(redactDefaults bool, customSecretKeys []string) MetadataRedactionPolicy {
	policy := MetadataRedactionPolicy{
		redactDefaults:   redactDefaults,
		customSecretKeys: make(map[string]struct{}),
		configured:       true,
	}
	for _, key := range customSecretKeys {
		normalized := normalizeMetadataKey(key)
		if normalized == "" {
			continue
		}
		policy.customSecretKeys[normalized] = struct{}{}
	}
	if len(policy.customSecretKeys) == 0 {
		policy.customSecretKeys = nil
	}

	return policy
}

func MetadataRedactionPolicyFromWorkspaceSettings(settings *contracts.WorkspaceSettings) MetadataRedactionPolicy {
	if settings == nil {
		return DefaultMetadataRedactionPolicy()
	}

	return NewMetadataRedactionPolicy(settings.RedactDefaults, settings.CustomSecretKeys)
}

func MetadataKeyIsSensitive(key string, policy MetadataRedactionPolicy) bool {
	return isSensitiveMetadataKey(key, policy)
}

func (p MetadataRedactionPolicy) normalized() MetadataRedactionPolicy {
	if p.configured {
		return p
	}

	return DefaultMetadataRedactionPolicy()
}

type UnaryEventLogArtifacts struct {
	SummaryPath    string
	SessionLogPath string
	Events         []contracts.HistoryLogEvent
}

type StreamEventLogArtifacts struct {
	SummaryPath    string
	SessionLogPath string
}

type UnaryEventLogRecord struct {
	CallID          string
	SessionID       string
	EndpointID      string
	WorkspaceID     string
	Method          string
	RPCType         contracts.RPCType
	FinalState      contracts.StreamState
	StartedAt       time.Time
	FinishedAt      time.Time
	Duration        time.Duration
	RequestBody     any
	ResponseBody    any
	RequestMetadata map[string]string
	Headers         map[string][]string
	Trailers        map[string][]string
	Status          contracts.StreamStatus
	ErrorCategory   contracts.ErrorCategory
	ErrorCode       string
	RedactionPolicy MetadataRedactionPolicy
}

type storedUnaryHistoryDetail struct {
	RequestBody  any                         `json:"requestBody"`
	ResponseBody any                         `json:"responseBody,omitempty"`
	Headers      map[string][]string         `json:"headers,omitempty"`
	Trailers     map[string][]string         `json:"trailers,omitempty"`
	Status       contracts.StreamStatus      `json:"status"`
	Events       []contracts.HistoryLogEvent `json:"events"`
}

type StreamEventLogRecord struct {
	CallID          string
	RequestBody     any
	ResponseBodies  []any
	Headers         map[string][]string
	Trailers        map[string][]string
	Status          contracts.StreamStatus
	Events          []contracts.HistoryLogEvent
	RedactionPolicy MetadataRedactionPolicy
}

type EventLog interface {
	WriteUnaryCall(ctx context.Context, record UnaryEventLogRecord) (UnaryEventLogArtifacts, error)
	WriteStreamCall(ctx context.Context, record StreamEventLogRecord) (StreamEventLogArtifacts, error)
}

type fileEventLog struct {
	baseDir string
}

func newFileEventLog(baseDir string) EventLog {
	return &fileEventLog{baseDir: filepath.Join(baseDir, "history")}
}

func (l *fileEventLog) WriteUnaryCall(_ context.Context, record UnaryEventLogRecord) (UnaryEventLogArtifacts, error) {
	if err := os.MkdirAll(filepath.Join(l.baseDir, "session-logs"), 0o755); err != nil {
		return UnaryEventLogArtifacts{}, err
	}
	if err := os.MkdirAll(filepath.Join(l.baseDir, "summaries"), 0o755); err != nil {
		return UnaryEventLogArtifacts{}, err
	}

	events := buildUnaryHistoryEvents(record)
	sessionLogPath := filepath.Join(l.baseDir, "session-logs", record.CallID+".jsonl")
	if err := writeHistoryEvents(sessionLogPath, events); err != nil {
		return UnaryEventLogArtifacts{}, err
	}

	summaryPath := filepath.Join(l.baseDir, "summaries", record.CallID+".json")
	summaryPayload := storedUnaryHistoryDetail{
		RequestBody:  record.RequestBody,
		ResponseBody: record.ResponseBody,
		Headers:      redactMetadataValues(record.Headers, record.RedactionPolicy),
		Trailers:     redactMetadataValues(record.Trailers, record.RedactionPolicy),
		Status:       record.Status,
		Events:       events,
	}
	if err := writeJSONFile(summaryPath, summaryPayload); err != nil {
		return UnaryEventLogArtifacts{}, err
	}

	return UnaryEventLogArtifacts{
		SummaryPath:    summaryPath,
		SessionLogPath: sessionLogPath,
		Events:         events,
	}, nil
}

func (l *fileEventLog) WriteStreamCall(_ context.Context, record StreamEventLogRecord) (StreamEventLogArtifacts, error) {
	if err := os.MkdirAll(filepath.Join(l.baseDir, "session-logs"), 0o755); err != nil {
		return StreamEventLogArtifacts{}, err
	}
	if err := os.MkdirAll(filepath.Join(l.baseDir, "summaries"), 0o755); err != nil {
		return StreamEventLogArtifacts{}, err
	}

	sessionLogPath := filepath.Join(l.baseDir, "session-logs", record.CallID+".jsonl")
	if err := writeHistoryEvents(sessionLogPath, record.Events); err != nil {
		return StreamEventLogArtifacts{}, err
	}

	summaryPath := filepath.Join(l.baseDir, "summaries", record.CallID+".json")
	summaryPayload := storedUnaryHistoryDetail{
		RequestBody:  record.RequestBody,
		ResponseBody: record.ResponseBodies,
		Headers:      redactMetadataValues(record.Headers, record.RedactionPolicy),
		Trailers:     redactMetadataValues(record.Trailers, record.RedactionPolicy),
		Status:       record.Status,
		Events:       record.Events,
	}
	if err := writeJSONFile(summaryPath, summaryPayload); err != nil {
		return StreamEventLogArtifacts{}, err
	}

	return StreamEventLogArtifacts{
		SummaryPath:    summaryPath,
		SessionLogPath: sessionLogPath,
	}, nil
}

func buildUnaryHistoryEvents(record UnaryEventLogRecord) []contracts.HistoryLogEvent {
	redactedRequestMetadata := redactMetadataMap(record.RequestMetadata, record.RedactionPolicy)
	redactedHeaders := redactMetadataValues(record.Headers, record.RedactionPolicy)
	redactedTrailers := redactMetadataValues(record.Trailers, record.RedactionPolicy)

	events := make([]contracts.HistoryLogEvent, 0, 6)
	nextSequence := int64(1)
	appendEvent := func(event contracts.HistoryLogEvent) {
		event.CallID = record.CallID
		event.SessionID = record.SessionID
		event.Sequence = nextSequence
		nextSequence++
		events = append(events, event)
	}

	appendEvent(contracts.HistoryLogEvent{
		Kind:      "call_started",
		Direction: "sent",
		Preview: &contracts.HistoryLogPreview{
			JSON: record.RequestBody,
		},
		GRPC: &contracts.HistoryLogGRPC{
			Method:   record.Method,
			RPCType:  record.RPCType,
			Metadata: toRepeatedMetadata(redactedRequestMetadata),
		},
		Timestamp: record.StartedAt.Format(time.RFC3339Nano),
	})

	appendEvent(contracts.HistoryLogEvent{
		Kind:         "message_sent",
		Direction:    "sent",
		MessageIndex: 0,
		SizeBytes:    measureJSONSize(record.RequestBody),
		Preview: &contracts.HistoryLogPreview{
			JSON: record.RequestBody,
		},
		Timestamp: record.StartedAt.Format(time.RFC3339Nano),
	})

	if len(redactedHeaders) > 0 {
		appendEvent(contracts.HistoryLogEvent{
			Kind:      "headers_received",
			Direction: "received",
			GRPC: &contracts.HistoryLogGRPC{
				Metadata: redactedHeaders,
			},
			Timestamp: record.FinishedAt.Format(time.RFC3339Nano),
		})
	}

	if record.ResponseBody != nil {
		appendEvent(contracts.HistoryLogEvent{
			Kind:         "message_received",
			Direction:    "received",
			MessageIndex: 0,
			SizeBytes:    measureJSONSize(record.ResponseBody),
			Preview: &contracts.HistoryLogPreview{
				JSON: record.ResponseBody,
			},
			Timestamp: record.FinishedAt.Format(time.RFC3339Nano),
		})
	}

	appendEvent(contracts.HistoryLogEvent{
		Kind:      "trailers_received",
		Direction: "received",
		GRPC: &contracts.HistoryLogGRPC{
			StatusCode: record.Status.Code,
			Metadata:   redactedTrailers,
		},
		Timestamp: record.FinishedAt.Format(time.RFC3339Nano),
	})

	finalKind := "call_finished"
	if record.FinalState == contracts.StreamStateError {
		finalKind = "call_failed"
	}

	appendEvent(contracts.HistoryLogEvent{
		Kind:      finalKind,
		Direction: "received",
		GRPC: &contracts.HistoryLogGRPC{
			StatusCode: record.Status.Code,
		},
		Details: map[string]string{
			"durationMs":    fmt.Sprintf("%d", record.Duration.Milliseconds()),
			"finalState":    string(record.FinalState),
			"errorCode":     record.ErrorCode,
			"errorCategory": string(record.ErrorCategory),
		},
		Timestamp: record.FinishedAt.Format(time.RFC3339Nano),
	})

	return events
}

func writeHistoryEvents(path string, events []contracts.HistoryLogEvent) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	writer := bufio.NewWriter(file)
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := writer.Write(payload); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func writeJSONFile(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, payload, 0o600)
}

func measureJSONSize(value any) int64 {
	payload, err := json.Marshal(value)
	if err != nil {
		return 0
	}

	return int64(len(payload))
}

func redactMetadataMap(values map[string]string, policy MetadataRedactionPolicy) map[string]string {
	if len(values) == 0 {
		return nil
	}

	redacted := make(map[string]string, len(values))
	for key, value := range values {
		if isSensitiveMetadataKey(key, policy) {
			redacted[key] = "[REDACTED]"
			continue
		}
		redacted[key] = value
	}

	return redacted
}

func redactMetadataValues(values map[string][]string, policy MetadataRedactionPolicy) map[string][]string {
	if len(values) == 0 {
		return nil
	}

	redacted := make(map[string][]string, len(values))
	for key, items := range values {
		if isSensitiveMetadataKey(key, policy) {
			redacted[key] = []string{"[REDACTED]"}
			continue
		}
		redacted[key] = append([]string(nil), items...)
	}

	return redacted
}

func isSensitiveMetadataKey(key string, policy MetadataRedactionPolicy) bool {
	normalized := normalizeMetadataKey(key)
	if normalized == "" {
		return false
	}

	normalizedPolicy := policy.normalized()
	if _, secret := normalizedPolicy.customSecretKeys[normalized]; secret {
		return true
	}
	if normalizedPolicy.redactDefaults {
		if _, secret := secretMetadataKeys[normalized]; secret {
			return true
		}

		return strings.Contains(normalized, "token") ||
			strings.Contains(normalized, "secret") ||
			strings.Contains(normalized, "credential") ||
			strings.Contains(normalized, "password")
	}

	return false
}

func normalizeMetadataKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func metadataValuesEqual(left map[string][]string, right map[string][]string) bool {
	if len(left) != len(right) {
		return false
	}

	for key, leftItems := range left {
		rightItems, ok := right[key]
		if !ok || len(leftItems) != len(rightItems) {
			return false
		}
		for index := range leftItems {
			if leftItems[index] != rightItems[index] {
				return false
			}
		}
	}

	return true
}

func toRepeatedMetadata(values map[string]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string][]string, len(values))
	for key, value := range values {
		result[key] = []string{value}
	}

	return result
}
