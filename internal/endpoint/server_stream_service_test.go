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

	"tether/internal/contracts"
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
		Method:        "tether.demo.v1.ReflectionDemo.Watch",
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
		Method:        "tether.demo.v1.ReflectionDemo.Watch",
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
		Method:        "tether.demo.v1.ReflectionDemo.Watch",
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
		Method:        "tether.demo.v1.ReflectionDemo.Watch",
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
		Method:        "tether.demo.v1.ReflectionDemo.Watch",
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
		Method:        "tether.demo.v1.ReflectionDemo.Watch",
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
