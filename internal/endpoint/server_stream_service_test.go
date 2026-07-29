package endpoint

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"catenar/internal/contracts"
)

type emittedStreamEvent struct {
	name    contracts.EventName
	payload any
}

type streamEmitterSpy struct {
	mu     sync.Mutex
	events []emittedStreamEvent
	ch     chan emittedStreamEvent
}

type retentionWorkspaceManager struct {
	retention EventRetentionPolicy
}

func (m retentionWorkspaceManager) PrepareEndpointTest(_ context.Context, input contracts.EndpointTestInput) (WorkspaceContext, contracts.EndpointPreset, error) {
	return WorkspaceContext{
		ID:             "ws_retention",
		Kind:           "file-workspace",
		EventRetention: m.retention,
	}, EnsureEndpointIdentity(NormalizeEndpointPreset(input.Endpoint)), nil
}

func newStreamEmitterSpy() *streamEmitterSpy {
	return &streamEmitterSpy{
		ch: make(chan emittedStreamEvent, 64),
	}
}

func (s *streamEmitterSpy) Emit(eventName contracts.EventName, payload any) error {
	event := emittedStreamEvent{name: eventName, payload: payload}
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	s.ch <- event
	return nil
}

func TestCallStartStreamFromReflectionEmitsLifecycleEventsAndPersistsHistory(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		watchMessages: []int64{1, 2},
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Watch",
		RPCType:       contracts.RPCTypeServerStream,
		Metadata: map[string]string{
			"authorization": "Bearer streaming-secret",
			"x-auth-token":  "streaming-auth-token-secret",
			"x-request-id":  "stream-123",
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected stream start acknowledgement, got %+v", response.Error)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateClosed {
		t.Fatalf("expected closed stream completion, got %+v", completed)
	}
	if completed.Status.Code != codes.OK.String() {
		t.Fatalf("expected OK final status, got %+v", completed.Status)
	}

	stateSequence := streamStatesForSession(emitter, response.Data.SessionID)
	expectedStates := []contracts.StreamState{
		contracts.StreamStateConnecting,
		contracts.StreamStateOpen,
		contracts.StreamStateClosed,
	}
	if !slices.Equal(stateSequence, expectedStates) {
		t.Fatalf("expected state sequence %+v, got %+v", expectedStates, stateSequence)
	}

	messageEvents := streamEventsForSession(emitter, response.Data.SessionID, "message_received")
	if len(messageEvents) != 2 {
		t.Fatalf("expected two received message events, got %+v", messageEvents)
	}
	if messageEvents[0].Payload.Preview.JSON != "1970-01-01T00:00:01Z" {
		t.Fatalf("expected first timestamp preview, got %+v", messageEvents[0].Payload.Preview.JSON)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one history summary, got %+v", historyList)
	}

	summary := historyList.Data.Calls[0]
	if summary.CallID != response.Data.CallID || summary.RPCType != contracts.RPCTypeServerStream {
		t.Fatalf("unexpected streaming summary identity: %+v", summary)
	}
	if summary.State != contracts.StreamStateClosed || summary.ResponseCount != 2 || summary.RequestCount != 1 {
		t.Fatalf("unexpected streaming summary counts/state: %+v", summary)
	}

	historyGet := service.HistoryGet(context.Background(), response.Data.CallID)
	if !historyGet.Ok || historyGet.Data == nil {
		t.Fatalf("expected streaming history detail, got %+v", historyGet.Error)
	}

	eventKinds := make([]string, 0, len(historyGet.Data.Events))
	for _, event := range historyGet.Data.Events {
		eventKinds = append(eventKinds, event.Kind)
	}
	for _, expectedKind := range []string{
		"call_started",
		"message_sent",
		"headers_received",
		"message_received",
		"trailers_received",
		"call_finished",
	} {
		if !slices.Contains(eventKinds, expectedKind) {
			t.Fatalf("expected history event %q, got %+v", expectedKind, eventKinds)
		}
	}

	if historyGet.Data.Events[0].GRPC.Metadata["authorization"][0] != "[REDACTED]" {
		t.Fatalf("expected request metadata to be redacted, got %+v", historyGet.Data.Events[0].GRPC.Metadata)
	}
	if historyGet.Data.Events[0].GRPC.Metadata["x-auth-token"][0] != "[REDACTED]" {
		t.Fatalf("expected token-like request metadata to be redacted, got %+v", historyGet.Data.Events[0].GRPC.Metadata)
	}

	summaryPayload, err := os.ReadFile(summary.SummaryPath)
	if err != nil {
		t.Fatalf("read streaming summary: %v", err)
	}
	if strings.Contains(string(summaryPayload), "streaming-secret") ||
		strings.Contains(string(summaryPayload), "streaming-auth-token-secret") ||
		strings.Contains(string(summaryPayload), "stream-cookie-secret") ||
		strings.Contains(string(summaryPayload), "stream-token-secret") ||
		strings.Contains(string(summaryPayload), "stream-trailer-secret") {
		t.Fatalf("expected persisted streaming artifact to omit raw secret metadata, got %s", summaryPayload)
	}
}

func TestCallStartStreamMarksTruncatedWhenRetentionLimitReached(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		watchMessages: []int64{1, 2, 3},
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
		EventRetention: EventRetentionPolicy{
			MaxEventsPerCall: 3,
			MaxBytesPerCall:  DefaultMaxBytesPerCall,
		},
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Watch",
		RPCType:       contracts.RPCTypeServerStream,
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected stream start acknowledgement, got %+v", response.Error)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateClosed {
		t.Fatalf("expected closed stream completion, got %+v", completed)
	}
	if !slices.Contains(completed.Conditions, contracts.SessionConditionTruncated) {
		t.Fatalf("expected completed event to carry truncated condition, got %+v", completed.Conditions)
	}

	stateEvents := streamStateEventsForSession(emitter, response.Data.SessionID)
	hasLiveTruncatedState := false
	for _, stateEvent := range stateEvents {
		if stateEvent.State == contracts.StreamStateOpen &&
			slices.Contains(stateEvent.Conditions, contracts.SessionConditionTruncated) {
			hasLiveTruncatedState = true
			break
		}
	}
	if !hasLiveTruncatedState {
		t.Fatalf("expected live stream state to expose truncated condition, got %+v", stateEvents)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one history summary, got %+v", historyList)
	}
	summary := historyList.Data.Calls[0]
	if !summary.Truncated || summary.State != contracts.StreamStateClosed {
		t.Fatalf("expected truncated closed history summary, got %+v", summary)
	}
	if summary.ResponseCount != 3 || summary.RequestCount != 1 {
		t.Fatalf("expected summary counts to survive truncation, got %+v", summary)
	}

	historyGet := service.HistoryGet(context.Background(), response.Data.CallID)
	if !historyGet.Ok || historyGet.Data == nil {
		t.Fatalf("expected truncated history detail, got %+v", historyGet.Error)
	}
	if len(historyGet.Data.Events) > 3 {
		t.Fatalf("expected retained event log to respect limit, got %d events", len(historyGet.Data.Events))
	}
	sessionLogPayload, err := os.ReadFile(summary.SessionLogPath)
	if err != nil {
		t.Fatalf("read retained stream session log: %v", err)
	}
	if lineCount := strings.Count(string(sessionLogPayload), "\n"); lineCount > 3 {
		t.Fatalf("expected persisted JSONL stream log to respect retention limit, got %d events", lineCount)
	}

	truncationDiagnostics := diagnosticsForCode(emitter, "application.stream_event_retention_truncated")
	if len(truncationDiagnostics) != 1 {
		t.Fatalf("expected one retention truncation diagnostic, got %+v", truncationDiagnostics)
	}
	for _, stateEvent := range stateEvents {
		if stateEvent.State == contracts.StreamState("truncated") {
			t.Fatalf("expected truncated to remain a condition, not a canonical state: %+v", stateEvent)
		}
	}
}

func TestCallStartStreamUsesWorkspaceEventRetentionSettings(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		watchMessages: []int64{1, 2, 3},
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
		WorkspaceManager: retentionWorkspaceManager{
			retention: EventRetentionPolicy{
				MaxEventsPerCall: 3,
				MaxBytesPerCall:  DefaultMaxBytesPerCall,
			},
		},
		EventRetention: EventRetentionPolicy{
			MaxEventsPerCall: 100,
			MaxBytesPerCall:  DefaultMaxBytesPerCall,
		},
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Watch",
		RPCType:       contracts.RPCTypeServerStream,
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected stream start acknowledgement, got %+v", response.Error)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if !slices.Contains(completed.Conditions, contracts.SessionConditionTruncated) {
		t.Fatalf("expected workspace retention policy to mark stream truncated, got %+v", completed.Conditions)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one history summary, got %+v", historyList)
	}
	summary := historyList.Data.Calls[0]
	if !summary.Truncated || summary.ResponseCount != 3 {
		t.Fatalf("expected workspace retention to reach runtime while preserving counts, got %+v", summary)
	}
}

func TestCallStartStreamDoesNotUseUnaryRequestTimeoutAsStreamDeadline(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		watchMessages:  []int64{1},
		watchSendDelay: 75 * time.Millisecond,
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1,
			StreamIdleTimeoutMs: 0,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Watch",
		RPCType:       contracts.RPCTypeServerStream,
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected stream start acknowledgement, got %+v", response.Error)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateClosed || completed.Status.Code != codes.OK.String() {
		t.Fatalf("expected stream to ignore unary request timeout and close normally, got %+v", completed)
	}
}

func TestCallStartStreamAppliesStreamIdleTimeout(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		watchBlockUntilCancel: true,
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    0,
			StreamIdleTimeoutMs: 30,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Watch",
		RPCType:       contracts.RPCTypeServerStream,
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected stream start acknowledgement, got %+v", response.Error)
	}

	errorEvent := waitForStreamError(t, emitter, response.Data.SessionID)
	if errorEvent.Error.Code != "transport.stream_idle_timeout" || errorEvent.Error.Category != contracts.ErrorCategoryTransport {
		t.Fatalf("expected idle timeout transport diagnostic, got %+v", errorEvent)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateError || completed.Status.Code != codes.DeadlineExceeded.String() {
		t.Fatalf("expected idle timeout completion with DEADLINE_EXCEEDED, got %+v", completed)
	}
}

func TestCallStartStreamCancelCompletesLiveSessionAsCancelled(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		watchBlockUntilCancel: true,
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 0,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Watch",
		RPCType:       contracts.RPCTypeServerStream,
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected stream start acknowledgement, got %+v", response.Error)
	}

	waitForStreamState(t, emitter, response.Data.SessionID, contracts.StreamStateOpen)
	cancelResponse := service.CallCancel(context.Background(), contracts.CallCancelInput{SessionID: response.Data.SessionID})
	if !cancelResponse.Ok {
		t.Fatalf("expected cancel acknowledgement, got %+v", cancelResponse.Error)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateCancelled {
		t.Fatalf("expected cancelled completion, got %+v", completed)
	}
	if completed.Status.Code != codes.Canceled.String() {
		t.Fatalf("expected CANCELLED final status, got %+v", completed.Status)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected cancelled stream history summary, got %+v", historyList)
	}
	if historyList.Data.Calls[0].State != contracts.StreamStateCancelled {
		t.Fatalf("expected cancelled history state, got %+v", historyList.Data.Calls[0])
	}
}

func TestCallStartStreamPersistsGRPCStatusFailureAsDiagnostic(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		watchStatusCode:    codes.InvalidArgument,
		watchStatusMessage: "watch rejected",
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Watch",
		RPCType:       contracts.RPCTypeServerStream,
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected stream start acknowledgement, got %+v", response.Error)
	}

	errorEvent := waitForStreamError(t, emitter, response.Data.SessionID)
	if errorEvent.Error.Code != "grpc_status.invalid_argument" || errorEvent.Error.Category != contracts.ErrorCategoryGRPCStatus {
		t.Fatalf("expected grpc status stream error, got %+v", errorEvent)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateError || completed.Status.Code != codes.InvalidArgument.String() {
		t.Fatalf("expected error completion with INVALID_ARGUMENT, got %+v", completed)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected failed stream history summary, got %+v", historyList)
	}
	summary := historyList.Data.Calls[0]
	if summary.ErrorCategory != contracts.ErrorCategoryGRPCStatus || summary.ErrorCode != "grpc_status.invalid_argument" {
		t.Fatalf("expected grpc status classification in history, got %+v", summary)
	}
}

func TestCallStartStreamClientStreamingStaticSequencePersistsHistory(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Upload",
		RPCType:       contracts.RPCTypeClientStream,
		Metadata: map[string]string{
			"authorization": "Bearer client-stream-secret",
			"x-request-id":  "client-stream-123",
		},
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeStaticSequence,
			Messages: []contracts.StreamMessage{
				{Body: "1970-01-01T00:00:01Z"},
				{Body: "1970-01-01T00:00:03Z"},
			},
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected client stream start acknowledgement, got %+v", response.Error)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateClosed || completed.Status.Code != codes.OK.String() {
		t.Fatalf("expected closed client stream completion, got %+v", completed)
	}

	stateSequence := streamStatesForSession(emitter, response.Data.SessionID)
	expectedStates := []contracts.StreamState{
		contracts.StreamStateConnecting,
		contracts.StreamStateOpen,
		contracts.StreamStateHalfClosedLocal,
		contracts.StreamStateClosed,
	}
	if !slices.Equal(stateSequence, expectedStates) {
		t.Fatalf("expected state sequence %+v, got %+v", expectedStates, stateSequence)
	}

	sentEvents := streamEventsForSession(emitter, response.Data.SessionID, "message_sent")
	if len(sentEvents) != 2 {
		t.Fatalf("expected two sent message events, got %+v", sentEvents)
	}
	if sentEvents[1].Payload.Preview.JSON != "1970-01-01T00:00:03Z" {
		t.Fatalf("expected second sent message preview, got %+v", sentEvents[1].Payload.Preview.JSON)
	}

	receivedEvents := streamEventsForSession(emitter, response.Data.SessionID, "message_received")
	if len(receivedEvents) != 1 {
		t.Fatalf("expected one received response event, got %+v", receivedEvents)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one history summary, got %+v", historyList)
	}

	summary := historyList.Data.Calls[0]
	if summary.CallID != response.Data.CallID || summary.RPCType != contracts.RPCTypeClientStream {
		t.Fatalf("unexpected client streaming summary identity: %+v", summary)
	}
	if summary.State != contracts.StreamStateClosed || summary.RequestCount != 2 || summary.ResponseCount != 1 {
		t.Fatalf("unexpected client streaming summary counts/state: %+v", summary)
	}

	historyGet := service.HistoryGet(context.Background(), response.Data.CallID)
	if !historyGet.Ok || historyGet.Data == nil {
		t.Fatalf("expected client streaming history detail, got %+v", historyGet.Error)
	}

	eventKinds := make([]string, 0, len(historyGet.Data.Events))
	for _, event := range historyGet.Data.Events {
		eventKinds = append(eventKinds, event.Kind)
	}
	for _, expectedKind := range []string{
		"call_started",
		"message_sent",
		"send_half_closed",
		"headers_received",
		"message_received",
		"trailers_received",
		"call_finished",
	} {
		if !slices.Contains(eventKinds, expectedKind) {
			t.Fatalf("expected history event %q, got %+v", expectedKind, eventKinds)
		}
	}

	requestBodies, ok := historyGet.Data.RequestBody.([]any)
	if !ok || len(requestBodies) != 2 {
		t.Fatalf("expected static sequence request body, got %#v", historyGet.Data.RequestBody)
	}
	if historyGet.Data.Events[0].GRPC.Metadata["authorization"][0] != "[REDACTED]" {
		t.Fatalf("expected request metadata to be redacted, got %+v", historyGet.Data.Events[0].GRPC.Metadata)
	}
}

func TestCallStartStreamClientStreamingInteractiveSendAndHalfClosePersistsHistory(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Upload",
		RPCType:       contracts.RPCTypeClientStream,
		Metadata: map[string]string{
			"authorization": "Bearer interactive-secret",
			"x-request-id":  "client-stream-interactive-123",
		},
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeInteractive,
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected interactive client stream start acknowledgement, got %+v", response.Error)
	}

	firstSend := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:02Z"},
	})
	if !firstSend.Ok || firstSend.Data == nil {
		t.Fatalf("expected first interactive send acknowledgement, got %+v", firstSend.Error)
	}
	if firstSend.Data.MessageIndex != 0 || firstSend.Data.State != contracts.StreamStateOpen {
		t.Fatalf("unexpected first send acknowledgement: %+v", firstSend.Data)
	}

	secondSend := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:05Z"},
	})
	if !secondSend.Ok || secondSend.Data == nil {
		t.Fatalf("expected second interactive send acknowledgement, got %+v", secondSend.Error)
	}
	if secondSend.Data.MessageIndex != 1 || secondSend.Data.State != contracts.StreamStateOpen {
		t.Fatalf("unexpected second send acknowledgement: %+v", secondSend.Data)
	}

	halfClose := service.CallHalfClose(context.Background(), contracts.CallHalfCloseInput{
		SessionID: response.Data.SessionID,
	})
	if !halfClose.Ok || halfClose.Data == nil {
		t.Fatalf("expected interactive half-close acknowledgement, got %+v", halfClose.Error)
	}
	if halfClose.Data.State != contracts.StreamStateHalfClosedLocal {
		t.Fatalf("expected half-closed local acknowledgement, got %+v", halfClose.Data)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateClosed || completed.Status.Code != codes.OK.String() {
		t.Fatalf("expected closed interactive client stream completion, got %+v", completed)
	}

	stateSequence := streamStatesForSession(emitter, response.Data.SessionID)
	expectedStates := []contracts.StreamState{
		contracts.StreamStateConnecting,
		contracts.StreamStateOpen,
		contracts.StreamStateHalfClosedLocal,
		contracts.StreamStateClosed,
	}
	if !slices.Equal(stateSequence, expectedStates) {
		t.Fatalf("expected state sequence %+v, got %+v", expectedStates, stateSequence)
	}

	sentEvents := streamEventsForSession(emitter, response.Data.SessionID, "message_sent")
	if len(sentEvents) != 2 {
		t.Fatalf("expected two interactive sent message events, got %+v", sentEvents)
	}
	if sentEvents[0].Payload.Preview.JSON != "1970-01-01T00:00:02Z" ||
		sentEvents[1].Payload.Preview.JSON != "1970-01-01T00:00:05Z" {
		t.Fatalf("unexpected interactive sent previews: %+v", sentEvents)
	}

	receivedEvents := streamEventsForSession(emitter, response.Data.SessionID, "message_received")
	if len(receivedEvents) != 1 {
		t.Fatalf("expected one interactive response event, got %+v", receivedEvents)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one interactive history summary, got %+v", historyList)
	}

	summary := historyList.Data.Calls[0]
	if summary.CallID != response.Data.CallID || summary.RPCType != contracts.RPCTypeClientStream {
		t.Fatalf("unexpected interactive client streaming summary identity: %+v", summary)
	}
	if summary.State != contracts.StreamStateClosed || summary.RequestCount != 2 || summary.ResponseCount != 1 {
		t.Fatalf("unexpected interactive client streaming summary counts/state: %+v", summary)
	}

	historyGet := service.HistoryGet(context.Background(), response.Data.CallID)
	if !historyGet.Ok || historyGet.Data == nil {
		t.Fatalf("expected interactive client streaming history detail, got %+v", historyGet.Error)
	}

	requestBodies, ok := historyGet.Data.RequestBody.([]any)
	if !ok || len(requestBodies) != 2 {
		t.Fatalf("expected interactive request body sequence, got %#v", historyGet.Data.RequestBody)
	}
	if historyGet.Data.Events[0].GRPC.Metadata["authorization"][0] != "[REDACTED]" {
		t.Fatalf("expected interactive request metadata to be redacted, got %+v", historyGet.Data.Events[0].GRPC.Metadata)
	}
}

func TestCallStartStreamBidiInteractiveSendAndHalfClosePersistsHistory(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Chat",
		RPCType:       contracts.RPCTypeBidiStream,
		Metadata: map[string]string{
			"authorization": "Bearer bidi-secret",
			"x-request-id":  "bidi-interactive-123",
		},
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeInteractive,
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected bidi stream start acknowledgement, got %+v", response.Error)
	}
	if response.Data.RPCType != contracts.RPCTypeBidiStream || response.Data.State != contracts.StreamStateOpen {
		t.Fatalf("unexpected bidi start acknowledgement: %+v", response.Data)
	}

	firstSend := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:02Z"},
	})
	if !firstSend.Ok || firstSend.Data == nil {
		t.Fatalf("expected first bidi send acknowledgement, got %+v", firstSend.Error)
	}

	secondSend := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:05Z"},
	})
	if !secondSend.Ok || secondSend.Data == nil {
		t.Fatalf("expected second bidi send acknowledgement, got %+v", secondSend.Error)
	}

	receivedBeforeHalfClose := waitForStreamEventCount(t, emitter, response.Data.SessionID, "message_received", 2)
	firstPreview, ok := receivedBeforeHalfClose[0].Payload.Preview.JSON.(map[string]any)
	if !ok || firstPreview["echoSeconds"] != float64(2) {
		t.Fatalf("expected first bidi response before half-close, got %+v", receivedBeforeHalfClose[0].Payload.Preview.JSON)
	}

	halfClose := service.CallHalfClose(context.Background(), contracts.CallHalfCloseInput{
		SessionID: response.Data.SessionID,
	})
	if !halfClose.Ok || halfClose.Data == nil {
		t.Fatalf("expected bidi half-close acknowledgement, got %+v", halfClose.Error)
	}
	if halfClose.Data.State != contracts.StreamStateHalfClosedLocal {
		t.Fatalf("expected half-closed local acknowledgement, got %+v", halfClose.Data)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateClosed || completed.Status.Code != codes.OK.String() {
		t.Fatalf("expected closed bidi stream completion, got %+v", completed)
	}

	stateSequence := streamStatesForSession(emitter, response.Data.SessionID)
	expectedStates := []contracts.StreamState{
		contracts.StreamStateConnecting,
		contracts.StreamStateOpen,
		contracts.StreamStateHalfClosedLocal,
		contracts.StreamStateClosed,
	}
	if !slices.Equal(stateSequence, expectedStates) {
		t.Fatalf("expected state sequence %+v, got %+v", expectedStates, stateSequence)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one bidi history summary, got %+v", historyList)
	}

	summary := historyList.Data.Calls[0]
	if summary.CallID != response.Data.CallID || summary.RPCType != contracts.RPCTypeBidiStream {
		t.Fatalf("unexpected bidi streaming summary identity: %+v", summary)
	}
	if summary.State != contracts.StreamStateClosed || summary.RequestCount != 2 || summary.ResponseCount != 2 {
		t.Fatalf("unexpected bidi streaming summary counts/state: %+v", summary)
	}

	historyGet := service.HistoryGet(context.Background(), response.Data.CallID)
	if !historyGet.Ok || historyGet.Data == nil {
		t.Fatalf("expected bidi streaming history detail, got %+v", historyGet.Error)
	}

	requestBodies, ok := historyGet.Data.RequestBody.([]any)
	if !ok || len(requestBodies) != 2 {
		t.Fatalf("expected bidi request body sequence, got %#v", historyGet.Data.RequestBody)
	}
	if historyGet.Data.Events[0].GRPC.Metadata["authorization"][0] != "[REDACTED]" {
		t.Fatalf("expected bidi request metadata to be redacted, got %+v", historyGet.Data.Events[0].GRPC.Metadata)
	}
}

func TestCallStartStreamBidiMidStreamFailurePreservesReceivedEvents(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		chatStatusCode:         codes.InvalidArgument,
		chatStatusMessage:      "chat rejected after first response",
		chatFailAfterResponses: 1,
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Chat",
		RPCType:       contracts.RPCTypeBidiStream,
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeInteractive,
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected bidi stream start acknowledgement, got %+v", response.Error)
	}

	send := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:07Z"},
	})
	if !send.Ok || send.Data == nil {
		t.Fatalf("expected bidi send acknowledgement before server failure, got %+v", send.Error)
	}

	errorEvent := waitForStreamError(t, emitter, response.Data.SessionID)
	if errorEvent.Error.Code != "grpc_status.invalid_argument" || errorEvent.Error.Category != contracts.ErrorCategoryGRPCStatus {
		t.Fatalf("expected grpc status bidi stream error, got %+v", errorEvent)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateError || completed.Status.Code != codes.InvalidArgument.String() {
		t.Fatalf("expected error completion with INVALID_ARGUMENT, got %+v", completed)
	}

	receivedEvents := streamEventsForSession(emitter, response.Data.SessionID, "message_received")
	if len(receivedEvents) != 1 {
		t.Fatalf("expected one received bidi event to survive mid-stream failure, got %+v", receivedEvents)
	}
	preview, ok := receivedEvents[0].Payload.Preview.JSON.(map[string]any)
	if !ok || preview["echoSeconds"] != float64(7) {
		t.Fatalf("expected received bidi response preview, got %+v", receivedEvents[0].Payload.Preview.JSON)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected failed bidi history summary, got %+v", historyList)
	}
	summary := historyList.Data.Calls[0]
	if summary.RPCType != contracts.RPCTypeBidiStream ||
		summary.State != contracts.StreamStateError ||
		summary.RequestCount != 1 ||
		summary.ResponseCount != 1 ||
		summary.ErrorCode != "grpc_status.invalid_argument" {
		t.Fatalf("expected failed bidi summary with preserved counts, got %+v", summary)
	}
}

func TestCallStartStreamBidiCancelOpenSessionPersistsCancelledHistory(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Chat",
		RPCType:       contracts.RPCTypeBidiStream,
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeInteractive,
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected bidi stream start acknowledgement, got %+v", response.Error)
	}

	waitForStreamState(t, emitter, response.Data.SessionID, contracts.StreamStateOpen)
	cancelResponse := service.CallCancel(context.Background(), contracts.CallCancelInput{SessionID: response.Data.SessionID})
	if !cancelResponse.Ok || cancelResponse.Data == nil {
		t.Fatalf("expected bidi cancel acknowledgement, got %+v", cancelResponse.Error)
	}
	if cancelResponse.Data.State != contracts.StreamStateCancelled {
		t.Fatalf("expected cancel acknowledgement to report cancelled, got %+v", cancelResponse.Data)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateCancelled || completed.Status.Code != codes.Canceled.String() {
		t.Fatalf("expected cancelled bidi completion, got %+v", completed)
	}

	stateSequence := streamStatesForSession(emitter, response.Data.SessionID)
	if !slices.Contains(stateSequence, contracts.StreamStateCancelled) {
		t.Fatalf("expected cancelled state event, got %+v", stateSequence)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected cancelled bidi history summary, got %+v", historyList)
	}
	summary := historyList.Data.Calls[0]
	if summary.RPCType != contracts.RPCTypeBidiStream ||
		summary.State != contracts.StreamStateCancelled ||
		summary.RequestCount != 0 ||
		summary.ResponseCount != 0 ||
		summary.ErrorCode != "cancelled.stream_cancelled" {
		t.Fatalf("expected cancelled bidi summary, got %+v", summary)
	}

	sendAfterCancel := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:09Z"},
	})
	if sendAfterCancel.Ok || sendAfterCancel.Error == nil || sendAfterCancel.Error.Code != "application.stream_session_not_found" {
		t.Fatalf("expected send after cancelled bidi stream to be rejected cleanly, got %+v", sendAfterCancel)
	}
}

func TestCallStartStreamBidiRejectsCommandsAfterHalfCloseWhileReceiving(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		chatBlockAfterClientHalfClose: true,
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Chat",
		RPCType:       contracts.RPCTypeBidiStream,
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeInteractive,
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected bidi stream start acknowledgement, got %+v", response.Error)
	}

	send := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:04Z"},
	})
	if !send.Ok || send.Data == nil {
		t.Fatalf("expected bidi send acknowledgement, got %+v", send.Error)
	}
	waitForStreamEventCount(t, emitter, response.Data.SessionID, "message_received", 1)

	halfClose := service.CallHalfClose(context.Background(), contracts.CallHalfCloseInput{
		SessionID: response.Data.SessionID,
	})
	if !halfClose.Ok || halfClose.Data == nil {
		t.Fatalf("expected bidi half-close acknowledgement, got %+v", halfClose.Error)
	}
	if halfClose.Data.State != contracts.StreamStateHalfClosedLocal {
		t.Fatalf("expected half-closed local state, got %+v", halfClose.Data)
	}
	waitForStreamState(t, emitter, response.Data.SessionID, contracts.StreamStateHalfClosedLocal)

	sendAfterHalfClose := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:05Z"},
	})
	if sendAfterHalfClose.Ok || sendAfterHalfClose.Error == nil {
		t.Fatalf("expected send after half-close to fail, got %+v", sendAfterHalfClose)
	}
	if sendAfterHalfClose.Error.Code != "application.stream_send_unavailable" ||
		sendAfterHalfClose.Error.Details["state"] != string(contracts.StreamStateHalfClosedLocal) {
		t.Fatalf("expected send unavailable while half-closed local, got %+v", sendAfterHalfClose.Error)
	}

	secondHalfClose := service.CallHalfClose(context.Background(), contracts.CallHalfCloseInput{
		SessionID: response.Data.SessionID,
	})
	if secondHalfClose.Ok || secondHalfClose.Error == nil {
		t.Fatalf("expected second half-close to fail, got %+v", secondHalfClose)
	}
	if secondHalfClose.Error.Code != "application.stream_half_close_unavailable" ||
		secondHalfClose.Error.Details["state"] != string(contracts.StreamStateHalfClosedLocal) {
		t.Fatalf("expected half-close unavailable while half-closed local, got %+v", secondHalfClose.Error)
	}

	cancelResponse := service.CallCancel(context.Background(), contracts.CallCancelInput{SessionID: response.Data.SessionID})
	if !cancelResponse.Ok || cancelResponse.Data == nil {
		t.Fatalf("expected cancel acknowledgement for half-closed bidi stream, got %+v", cancelResponse.Error)
	}
	if cancelResponse.Data.State != contracts.StreamStateCancelled {
		t.Fatalf("expected cancel acknowledgement to report cancelled, got %+v", cancelResponse.Data)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateCancelled {
		t.Fatalf("expected cancelled completion after half-closed bidi cancel, got %+v", completed)
	}
}

func TestCallStartStreamBidiRejectsSendAfterTerminalError(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		chatStatusCode:         codes.InvalidArgument,
		chatStatusMessage:      "chat rejected after first response",
		chatFailAfterResponses: 1,
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Chat",
		RPCType:       contracts.RPCTypeBidiStream,
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeInteractive,
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected bidi stream start acknowledgement, got %+v", response.Error)
	}

	send := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:08Z"},
	})
	if !send.Ok || send.Data == nil {
		t.Fatalf("expected bidi send acknowledgement before server failure, got %+v", send.Error)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateError || completed.Status.Code != codes.InvalidArgument.String() {
		t.Fatalf("expected terminal error completion, got %+v", completed)
	}

	sendAfterError := service.CallSendMessage(context.Background(), contracts.CallSendMessageInput{
		SessionID: response.Data.SessionID,
		Message:   contracts.StreamMessage{Body: "1970-01-01T00:00:09Z"},
	})
	if sendAfterError.Ok || sendAfterError.Error == nil {
		t.Fatalf("expected send after terminal error to be rejected cleanly, got %+v", sendAfterError)
	}
	if sendAfterError.Error.Code != "application.stream_session_not_found" &&
		(sendAfterError.Error.Code != "application.stream_send_unavailable" ||
			sendAfterError.Error.Details["state"] != string(contracts.StreamStateError)) {
		t.Fatalf("expected terminal send rejection after bidi error, got %+v", sendAfterError.Error)
	}
}

func TestCallStartStreamBidiValidatesInteractiveRequestMode(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Chat",
		RPCType:       contracts.RPCTypeBidiStream,
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeStaticSequence,
			Messages: []contracts.StreamMessage{
				{Body: "1970-01-01T00:00:01Z"},
			},
		},
	})
	if response.Ok || response.Error == nil {
		t.Fatalf("expected non-interactive bidi start to fail validation, got %+v", response)
	}
	if response.Error.Code != "validation.bidi_stream_interactive_required" ||
		response.Error.Category != contracts.ErrorCategoryValidation {
		t.Fatalf("expected bidi interactive validation error, got %+v", response.Error)
	}
}

func TestCallStartStreamClientStreamingValidatesStaticSequence(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	baseInput := contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Upload",
		RPCType:       contracts.RPCTypeClientStream,
	}

	tests := []struct {
		name string
		spec *contracts.StreamRequestSpec
		code string
	}{
		{
			name: "missing spec",
			spec: nil,
			code: "validation.client_stream_static_sequence_required",
		},
		{
			name: "empty messages",
			spec: &contracts.StreamRequestSpec{Mode: contracts.RequestModeStaticSequence},
			code: "validation.client_stream_messages_required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := baseInput
			input.RequestSpec = tt.spec
			response := service.CallStartStream(context.Background(), input)
			if response.Ok || response.Error == nil {
				t.Fatalf("expected validation error, got %+v", response)
			}
			if response.Error.Code != tt.code || response.Error.Category != contracts.ErrorCategoryValidation {
				t.Fatalf("expected %s validation error, got %+v", tt.code, response.Error)
			}
		})
	}
}

func TestCallStartStreamClientStreamingPersistsGRPCStatusFailure(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		uploadStatusCode:    codes.InvalidArgument,
		uploadStatusMessage: "upload rejected",
	})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	emitter := newStreamEmitterSpy()
	service.SetEmitter(emitter)

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:           address,
			TLS:              contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs: 3000,
			RequestTimeoutMs: 1000,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Upload",
		RPCType:       contracts.RPCTypeClientStream,
		RequestSpec: &contracts.StreamRequestSpec{
			Mode: contracts.RequestModeStaticSequence,
			Messages: []contracts.StreamMessage{
				{Body: "1970-01-01T00:00:01Z"},
			},
		},
	})
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected client stream start acknowledgement, got %+v", response.Error)
	}

	errorEvent := waitForStreamError(t, emitter, response.Data.SessionID)
	if errorEvent.Error.Code != "grpc_status.invalid_argument" || errorEvent.Error.Category != contracts.ErrorCategoryGRPCStatus {
		t.Fatalf("expected grpc status client stream error, got %+v", errorEvent)
	}

	completed := waitForStreamCompleted(t, emitter, response.Data.SessionID)
	if completed.FinalState != contracts.StreamStateError || completed.Status.Code != codes.InvalidArgument.String() {
		t.Fatalf("expected error completion with INVALID_ARGUMENT, got %+v", completed)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected failed client stream history summary, got %+v", historyList)
	}
	summary := historyList.Data.Calls[0]
	if summary.ErrorCategory != contracts.ErrorCategoryGRPCStatus || summary.ErrorCode != "grpc_status.invalid_argument" {
		t.Fatalf("expected grpc status classification in history, got %+v", summary)
	}
	if summary.RPCType != contracts.RPCTypeClientStream || summary.RequestCount != 1 {
		t.Fatalf("expected client stream summary with one request, got %+v", summary)
	}
}

func TestCallStartStreamRejectsUnavailableGRPCChannelWithTransportDiagnostic(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})
	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})
	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}
	stop()

	response := service.CallStartStream(context.Background(), contracts.CallStartStreamInput{
		CatalogSource: contracts.CatalogSourceReflection,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "catenar.demo.v1.ReflectionDemo.Watch",
		RPCType:       contracts.RPCTypeServerStream,
	})
	if response.Ok {
		t.Fatalf("expected stream start to fail when transport is unavailable")
	}
	if response.Error == nil || response.Error.Category != contracts.ErrorCategoryTransport {
		t.Fatalf("expected transport diagnostic, got %+v", response.Error)
	}
}

func waitForStreamCompleted(t *testing.T, emitter *streamEmitterSpy, sessionID string) contracts.StreamCompletedEvent {
	t.Helper()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-emitter.ch:
			if event.name != contracts.EventStreamCompleted {
				continue
			}
			completed, ok := event.payload.(contracts.StreamCompletedEvent)
			if !ok || completed.SessionID != sessionID {
				continue
			}
			return completed
		case <-deadline:
			t.Fatalf("timed out waiting for stream:completed for %s", sessionID)
		}
	}
}

func waitForStreamError(t *testing.T, emitter *streamEmitterSpy, sessionID string) contracts.StreamErrorEvent {
	t.Helper()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-emitter.ch:
			if event.name != contracts.EventStreamError {
				continue
			}
			streamError, ok := event.payload.(contracts.StreamErrorEvent)
			if !ok || streamError.SessionID != sessionID {
				continue
			}
			return streamError
		case <-deadline:
			t.Fatalf("timed out waiting for stream:error for %s", sessionID)
		}
	}
}

func waitForStreamState(t *testing.T, emitter *streamEmitterSpy, sessionID string, state contracts.StreamState) contracts.StreamStateEvent {
	t.Helper()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-emitter.ch:
			if event.name != contracts.EventStreamState {
				continue
			}
			stateEvent, ok := event.payload.(contracts.StreamStateEvent)
			if !ok || stateEvent.SessionID != sessionID || stateEvent.State != state {
				continue
			}
			return stateEvent
		case <-deadline:
			t.Fatalf("timed out waiting for stream state %s for %s", state, sessionID)
		}
	}
}

func waitForStreamEventCount(t *testing.T, emitter *streamEmitterSpy, sessionID string, kind string, count int) []contracts.StreamEventRecord {
	t.Helper()

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		records := streamEventsForSession(emitter, sessionID, kind)
		if len(records) >= count {
			return records
		}

		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("timed out waiting for %d stream events of kind %s for %s; got %+v", count, kind, sessionID, records)
		}
	}
}

func streamStatesForSession(emitter *streamEmitterSpy, sessionID string) []contracts.StreamState {
	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	states := make([]contracts.StreamState, 0)
	for _, event := range emitter.events {
		stateEvent, ok := event.payload.(contracts.StreamStateEvent)
		if event.name == contracts.EventStreamState && ok && stateEvent.SessionID == sessionID {
			states = append(states, stateEvent.State)
		}
	}

	return states
}

func streamStateEventsForSession(emitter *streamEmitterSpy, sessionID string) []contracts.StreamStateEvent {
	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	states := make([]contracts.StreamStateEvent, 0)
	for _, event := range emitter.events {
		stateEvent, ok := event.payload.(contracts.StreamStateEvent)
		if event.name == contracts.EventStreamState && ok && stateEvent.SessionID == sessionID {
			states = append(states, stateEvent)
		}
	}

	return states
}

func streamEventsForSession(emitter *streamEmitterSpy, sessionID string, kind string) []contracts.StreamEventRecord {
	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	records := make([]contracts.StreamEventRecord, 0)
	for _, event := range emitter.events {
		record, ok := event.payload.(contracts.StreamEventRecord)
		if event.name == contracts.EventStreamEvent && ok && record.SessionID == sessionID && record.Kind == kind {
			records = append(records, record)
		}
	}

	return records
}

func diagnosticsForCode(emitter *streamEmitterSpy, code string) []contracts.DiagnosticsUpdateEvent {
	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	diagnostics := make([]contracts.DiagnosticsUpdateEvent, 0)
	for _, event := range emitter.events {
		diagnostic, ok := event.payload.(contracts.DiagnosticsUpdateEvent)
		if event.name == contracts.EventDiagnosticsUpdate && ok && diagnostic.Code == code {
			diagnostics = append(diagnostics, diagnostic)
		}
	}

	return diagnostics
}
