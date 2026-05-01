package endpoint

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/reflect/protoreflect"

	"tether/internal/contracts"
)

type activeStreamSession struct {
	mu        sync.Mutex
	callID    string
	sessionID string
	cancel    context.CancelFunc
	state     contracts.StreamState
}

type streamHistoryRecorder struct {
	callID         string
	sessionID      string
	method         string
	rpcType        contracts.RPCType
	nextSequence   int64
	requestBodies  []any
	events         []contracts.HistoryLogEvent
	responseBodies []any
}

func (s *Service) CallStartStream(ctx context.Context, input contracts.CallStartStreamInput) contracts.CallStartStreamResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.initializationErr != nil || s.historyStore == nil || s.eventLog == nil {
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: wrapHistoryStoreError("initialize", s.initializationErr),
		}
	}

	if input.RPCType != contracts.RPCTypeServerStream && input.RPCType != contracts.RPCTypeClientStream {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.method_rpc_type_invalid",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Slice 3.2 can start server-streaming and client-streaming static sequence methods only.",
				Details: map[string]string{
					"rpcType": string(input.RPCType),
				},
			},
		}
	}

	endpointID := strings.TrimSpace(input.EndpointID)
	if endpointID == "" {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.endpoint_id_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "A loaded endpoint id is required before starting a stream.",
			},
		}
	}

	methodName := strings.TrimSpace(input.Method)
	if methodName == "" {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.method_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "A method full name is required before starting a stream.",
			},
		}
	}

	catalogSource := normalizeCatalogSource(input.CatalogSource)
	cachedCatalog, ok := s.loadMethodCatalog(endpointID, catalogSource)
	if !ok {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     catalogCacheStateCode("catalog_not_loaded"),
				Category: contracts.ErrorCategoryApplication,
				Message:  fmt.Sprintf("Load the %s catalog for this endpoint again before starting a stream.", catalogSourceLabel(catalogSource)),
				Details: map[string]string{
					"endpointId":    endpointID,
					"catalogSource": string(catalogSource),
				},
			},
		}
	}

	methodDescriptor, found := cachedCatalog.catalog.methods[methodName]
	if !found {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     catalogCacheStateCode("method_not_found"),
				Category: contracts.ErrorCategoryApplication,
				Message:  fmt.Sprintf("The selected method is no longer present in the cached %s catalog.", catalogSourceLabel(catalogSource)),
				Details: map[string]string{
					"endpointId":    endpointID,
					"method":        methodName,
					"catalogSource": string(catalogSource),
				},
			},
		}
	}

	selectedMethod := buildCatalogMethod(methodDescriptor)
	if selectedMethod.RPCType != input.RPCType {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.method_rpc_type_invalid",
				Category: contracts.ErrorCategoryValidation,
				Message:  fmt.Sprintf("The selected %s method cannot be executed as %s.", catalogSourceLabel(catalogSource), input.RPCType),
				Details: map[string]string{
					"method":  selectedMethod.FullName,
					"rpcType": string(selectedMethod.RPCType),
					"input":   string(input.RPCType),
				},
			},
		}
	}

	if selectedMethod.RPCType == contracts.RPCTypeClientStream {
		return s.callStartClientStream(ctx, cachedCatalog, selectedMethod, methodDescriptor, input)
	}

	requestBody, requestBodyErr := serverStreamRequestBody(methodDescriptor, input)
	if requestBodyErr != nil {
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: requestBodyErr,
		}
	}

	startedAt := s.now().UTC()
	callID, sessionID := newCallIdentity(startedAt)
	session := &activeStreamSession{
		callID:    callID,
		sessionID: sessionID,
		cancel:    func() {},
		state:     contracts.StreamStateIdle,
	}
	if errEnvelope := s.reserveActiveStreamSession(session); errEnvelope != nil {
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errEnvelope,
		}
	}

	runtimeCfg, prepDiagnostic := s.resolveRuntimeConfig(ctx, cachedCatalog.scope, cachedCatalog.endpoint)
	if prepDiagnostic != nil {
		s.removeActiveStreamSession(sessionID)
		s.emitDiagnostic("server-stream", startedAt, prepDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(prepDiagnostic, "application.endpoint_preparation_failed", "The endpoint could not be prepared."),
		}
	}

	conn, runtimeDiagnostic := s.grpcRuntime.Dial(ctx, runtimeCfg)
	if runtimeDiagnostic != nil {
		s.removeActiveStreamSession(sessionID)
		s.emitDiagnostic("server-stream", startedAt, runtimeDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(runtimeDiagnostic, "transport.grpc_not_ready", "The endpoint could not establish a ready gRPC channel."),
		}
	}

	mergedMetadata := mergeInvokeMetadata(cachedCatalog.endpoint.MetadataDefaults, input.Metadata)
	streamIdleTimeout := resolveServerStreamIdleTimeout(cachedCatalog.endpoint, input.CallOptions)
	streamStart, startDiagnostic := s.grpcRuntime.StartServerStream(ctx, conn, ServerStreamStartRequest{
		Method:     selectedMethod,
		Descriptor: methodDescriptor,
		Metadata:   mergedMetadata,
		Body:       requestBody,
	})
	if startDiagnostic != nil {
		s.removeActiveStreamSession(sessionID)
		_ = conn.Close()
		s.emitDiagnostic("server-stream", startedAt, startDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(startDiagnostic, "application.stream_start_failed", "The server-streaming call could not be started."),
		}
	}
	session.mu.Lock()
	session.cancel = streamStart.Cancel
	session.mu.Unlock()

	recorder := newStreamHistoryRecorder(callID, sessionID, selectedMethod.FullName, contracts.RPCTypeServerStream)
	recorder.recordCallStarted(cachedCatalog.scope.ID, cachedCatalog.endpoint.ID, requestBody, mergedMetadata, startedAt)
	s.emitHistoryBackedStreamEvent(recorder.events[0])
	recorder.recordMessageSent(requestBody, startedAt)
	s.emitHistoryBackedStreamEvent(recorder.events[1])

	s.emitStreamState(session, contracts.StreamStateConnecting, "", startedAt)
	s.emitStreamState(session, contracts.StreamStateOpen, contracts.StreamStateConnecting, s.now().UTC())

	go s.runServerStreamSession(ctx, streamSessionRunInput{
		session:      session,
		conn:         conn,
		startedAt:    startedAt,
		scope:        cachedCatalog.scope,
		endpoint:     cachedCatalog.endpoint,
		method:       selectedMethod,
		methodDesc:   methodDescriptor,
		recorder:     recorder,
		serverStream: streamStart,
		idleTimeout:  streamIdleTimeout,
	})

	return contracts.CallStartStreamResponse{
		Ok: true,
		Data: &contracts.CallStartStreamResult{
			CallID:     callID,
			SessionID:  sessionID,
			EndpointID: cachedCatalog.endpoint.ID,
			Method:     selectedMethod.FullName,
			RPCType:    contracts.RPCTypeServerStream,
			State:      contracts.StreamStateOpen,
			StartedAt:  startedAt.Format(time.RFC3339Nano),
		},
	}
}

func (s *Service) CallCancel(ctx context.Context, input contracts.CallCancelInput) contracts.CallCancelResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	requestedAt := s.now().UTC()
	session, errEnvelope := s.activeStreamSession(input.SessionID)
	if errEnvelope != nil {
		return contracts.CallCancelResponse{
			Ok:    false,
			Error: errEnvelope,
		}
	}

	session.mu.Lock()
	cancel := session.cancel
	state := session.state
	session.mu.Unlock()
	cancel()

	return contracts.CallCancelResponse{
		Ok: true,
		Data: &contracts.CallCancelResult{
			CallID:      session.callID,
			SessionID:   session.sessionID,
			State:       state,
			RequestedAt: requestedAt.Format(time.RFC3339Nano),
		},
	}
}

func (s *Service) callStartClientStream(
	ctx context.Context,
	cachedCatalog cachedMethodCatalog,
	selectedMethod contracts.CatalogMethod,
	methodDescriptor protoreflect.MethodDescriptor,
	input contracts.CallStartStreamInput,
) contracts.CallStartStreamResponse {
	requestMessages, requestMessagesErr := clientStreamRequestMessages(input)
	if requestMessagesErr != nil {
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: requestMessagesErr,
		}
	}

	startedAt := s.now().UTC()
	callID, sessionID := newCallIdentity(startedAt)
	callCtx, cancel := context.WithCancel(ctx)
	session := &activeStreamSession{
		callID:    callID,
		sessionID: sessionID,
		cancel:    cancel,
		state:     contracts.StreamStateIdle,
	}
	if errEnvelope := s.reserveActiveStreamSession(session); errEnvelope != nil {
		cancel()
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errEnvelope,
		}
	}

	runtimeCfg, prepDiagnostic := s.resolveRuntimeConfig(ctx, cachedCatalog.scope, cachedCatalog.endpoint)
	if prepDiagnostic != nil {
		cancel()
		s.removeActiveStreamSession(sessionID)
		s.emitDiagnostic("client-stream", startedAt, prepDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(prepDiagnostic, "application.endpoint_preparation_failed", "The endpoint could not be prepared."),
		}
	}

	conn, runtimeDiagnostic := s.grpcRuntime.Dial(ctx, runtimeCfg)
	if runtimeDiagnostic != nil {
		cancel()
		s.removeActiveStreamSession(sessionID)
		s.emitDiagnostic("client-stream", startedAt, runtimeDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(runtimeDiagnostic, "transport.grpc_not_ready", "The endpoint could not establish a ready gRPC channel."),
		}
	}

	mergedMetadata := mergeInvokeMetadata(cachedCatalog.endpoint.MetadataDefaults, input.Metadata)
	recorder := newStreamHistoryRecorder(callID, sessionID, selectedMethod.FullName, contracts.RPCTypeClientStream)
	startEvent := recorder.recordCallStarted(cachedCatalog.scope.ID, cachedCatalog.endpoint.ID, streamRequestMessagesBody(requestMessages), mergedMetadata, startedAt)
	s.emitHistoryBackedStreamEvent(startEvent)

	s.emitStreamState(session, contracts.StreamStateConnecting, "", startedAt)
	s.emitStreamState(session, contracts.StreamStateOpen, contracts.StreamStateConnecting, s.now().UTC())

	go s.runClientStreamStaticSequence(callCtx, clientStreamRunInput{
		session:     session,
		conn:        conn,
		cancel:      cancel,
		startedAt:   startedAt,
		scope:       cachedCatalog.scope,
		endpoint:    cachedCatalog.endpoint,
		method:      selectedMethod,
		methodDesc:  methodDescriptor,
		metadata:    mergedMetadata,
		messages:    requestMessages,
		callOptions: input.CallOptions,
		recorder:    recorder,
	})

	return contracts.CallStartStreamResponse{
		Ok: true,
		Data: &contracts.CallStartStreamResult{
			CallID:     callID,
			SessionID:  sessionID,
			EndpointID: cachedCatalog.endpoint.ID,
			Method:     selectedMethod.FullName,
			RPCType:    contracts.RPCTypeClientStream,
			State:      contracts.StreamStateOpen,
			StartedAt:  startedAt.Format(time.RFC3339Nano),
		},
	}
}

type streamSessionRunInput struct {
	session      *activeStreamSession
	conn         GRPCClientConn
	startedAt    time.Time
	scope        WorkspaceContext
	endpoint     contracts.EndpointPreset
	method       contracts.CatalogMethod
	methodDesc   protoreflect.MethodDescriptor
	recorder     *streamHistoryRecorder
	serverStream ServerStreamStartResult
	idleTimeout  time.Duration
}

type clientStreamRunInput struct {
	session     *activeStreamSession
	conn        GRPCClientConn
	cancel      context.CancelFunc
	startedAt   time.Time
	scope       WorkspaceContext
	endpoint    contracts.EndpointPreset
	method      contracts.CatalogMethod
	methodDesc  protoreflect.MethodDescriptor
	metadata    map[string]string
	messages    []any
	callOptions contracts.CallOptions
	recorder    *streamHistoryRecorder
}

func (s *Service) runServerStreamSession(ctx context.Context, input streamSessionRunInput) {
	defer func() {
		input.serverStream.Cancel()
		_ = input.conn.Close()
		s.removeActiveStreamSession(input.session.sessionID)
	}()

	consumeResult, consumeDiagnostic := s.grpcRuntime.ConsumeServerStream(ServerStreamConsumeRequest{
		Method:      input.method,
		Descriptor:  input.methodDesc,
		Stream:      input.serverStream.Stream,
		Cancel:      input.serverStream.Cancel,
		IdleTimeout: input.idleTimeout,
		OnHeaders: func(headers map[string][]string, ts time.Time) {
			event := input.recorder.recordHeaders(headers, ts)
			s.emitHistoryBackedStreamEvent(event)
		},
		OnMessage: func(message ServerStreamMessage) {
			event := input.recorder.recordMessageReceived(message)
			s.emitHistoryBackedStreamEvent(event)
		},
	})

	finishedAt := consumeResult.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = s.now().UTC()
	}
	duration := finishedAt.Sub(input.startedAt)
	status := consumeResult.Status
	if status.Code == "" {
		status = statusFromDiagnostic(consumeDiagnostic)
	}

	finalState := contracts.StreamStateClosed
	if consumeDiagnostic != nil {
		finalState = contracts.StreamStateError
		if consumeDiagnostic.Category == contracts.ErrorCategoryCancelled {
			finalState = contracts.StreamStateCancelled
		}
	}

	if len(consumeResult.Trailers) > 0 || status.Code != "" {
		event := input.recorder.recordTrailers(status, consumeResult.Trailers, finishedAt)
		s.emitHistoryBackedStreamEvent(event)
	}
	finalEvent := input.recorder.recordFinished(finalState, status, consumeDiagnostic, duration, finishedAt)

	if consumeDiagnostic != nil {
		diagnosticEvent := s.emitDiagnostic("server-stream", finishedAt, consumeDiagnostic)
		if consumeDiagnostic.Category != contracts.ErrorCategoryCancelled {
			s.emitStreamError(input.session, diagnosticEvent, consumeDiagnostic, finishedAt)
		}
	}

	artifacts, artifactErr := s.eventLog.WriteStreamCall(ctx, StreamEventLogRecord{
		CallID:         input.session.callID,
		RequestBody:    input.recorder.requestBody(),
		ResponseBodies: input.recorder.responses(),
		Headers:        consumeResult.Headers,
		Trailers:       consumeResult.Trailers,
		Status:         status,
		Events:         input.recorder.eventsSnapshot(),
	})
	if artifactErr != nil {
		s.emitDiagnostic("server-stream", finishedAt, &endpointDiagnostic{
			Level:    "error",
			Code:     "application.history_write_artifacts_failed",
			Category: contracts.ErrorCategoryApplication,
			Message:  "The runtime could not write the streaming session history artifacts.",
			Details:  map[string]string{"cause": artifactErr.Error()},
		})
	}

	summary := contracts.HistoryCallSummary{
		CallID:         input.session.callID,
		SessionID:      input.session.sessionID,
		WorkspaceID:    input.scope.ID,
		Method:         input.method.FullName,
		RPCType:        contracts.RPCTypeServerStream,
		EndpointID:     input.endpoint.ID,
		State:          finalState,
		GRPCStatusCode: status.Code,
		StartedAt:      input.startedAt.Format(time.RFC3339Nano),
		FinishedAt:     finishedAt.Format(time.RFC3339Nano),
		DurationMs:     duration.Milliseconds(),
		RequestCount:   1,
		ResponseCount:  len(input.recorder.responses()),
		Truncated:      false,
		ErrorCategory:  invokeDiagnosticCategory(consumeDiagnostic),
		ErrorCode:      invokeDiagnosticCode(consumeDiagnostic),
		SummaryPath:    artifacts.SummaryPath,
		SessionLogPath: artifacts.SessionLogPath,
	}
	if artifactErr == nil {
		if err := s.historyStore.SaveCallSummary(ctx, summary); err != nil {
			s.emitDiagnostic("server-stream", finishedAt, &endpointDiagnostic{
				Level:    "error",
				Code:     "application.history_save_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The runtime could not save the streaming call summary.",
				Details:  map[string]string{"cause": err.Error()},
			})
		}
	}

	s.emitHistoryBackedStreamEvent(finalEvent)
	s.emitStreamState(input.session, finalState, contracts.StreamStateOpen, finishedAt)
	s.emitStreamCompleted(input.session, finalState, status, finishedAt)
}

func (s *Service) runClientStreamStaticSequence(ctx context.Context, input clientStreamRunInput) {
	defer func() {
		input.cancel()
		_ = input.conn.Close()
		s.removeActiveStreamSession(input.session.sessionID)
	}()

	invokeResult, invokeDiagnostic := s.grpcRuntime.InvokeClientStream(ctx, input.conn, ClientStreamInvokeRequest{
		Method:         input.method,
		Descriptor:     input.methodDesc,
		Metadata:       input.metadata,
		Messages:       input.messages,
		RequestTimeout: resolveUnaryRequestTimeout(input.endpoint, input.callOptions),
		OnMessageSent: func(message ClientStreamSentMessage) {
			event := input.recorder.recordMessageSent(message.Body, message.SentAt)
			s.emitHistoryBackedStreamEvent(event)
		},
		OnHalfClose: func(ts time.Time) {
			event := input.recorder.recordLocalHalfClose(ts)
			s.emitHistoryBackedStreamEvent(event)
			s.emitStreamState(input.session, contracts.StreamStateHalfClosedLocal, contracts.StreamStateOpen, ts)
		},
	})

	finishedAt := invokeResult.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = s.now().UTC()
	}
	duration := finishedAt.Sub(input.startedAt)
	if invokeResult.Duration > 0 {
		duration = invokeResult.Duration
	}
	status := invokeResult.Status
	if status.Code == "" {
		status = statusFromDiagnostic(invokeDiagnostic)
	}

	finalState := contracts.StreamStateClosed
	if invokeDiagnostic != nil {
		finalState = contracts.StreamStateError
		if invokeDiagnostic.Category == contracts.ErrorCategoryCancelled {
			finalState = contracts.StreamStateCancelled
		}
	}

	if len(invokeResult.Headers) > 0 {
		event := input.recorder.recordHeaders(invokeResult.Headers, finishedAt)
		s.emitHistoryBackedStreamEvent(event)
	}
	if invokeResult.ResponseBody != nil {
		message := ServerStreamMessage{
			Body:       invokeResult.ResponseBody,
			Index:      0,
			SizeBytes:  measureJSONSize(invokeResult.ResponseBody),
			ReceivedAt: finishedAt,
		}
		event := input.recorder.recordMessageReceived(message)
		s.emitHistoryBackedStreamEvent(event)
	}
	if len(invokeResult.Trailers) > 0 || status.Code != "" {
		event := input.recorder.recordTrailers(status, invokeResult.Trailers, finishedAt)
		s.emitHistoryBackedStreamEvent(event)
	}
	finalEvent := input.recorder.recordFinished(finalState, status, invokeDiagnostic, duration, finishedAt)

	if invokeDiagnostic != nil {
		diagnosticEvent := s.emitDiagnostic("client-stream", finishedAt, invokeDiagnostic)
		if invokeDiagnostic.Category != contracts.ErrorCategoryCancelled {
			s.emitStreamError(input.session, diagnosticEvent, invokeDiagnostic, finishedAt)
		}
	}

	artifacts, artifactErr := s.eventLog.WriteStreamCall(ctx, StreamEventLogRecord{
		CallID:         input.session.callID,
		RequestBody:    input.recorder.requestBody(),
		ResponseBodies: input.recorder.responses(),
		Headers:        invokeResult.Headers,
		Trailers:       invokeResult.Trailers,
		Status:         status,
		Events:         input.recorder.eventsSnapshot(),
	})
	if artifactErr != nil {
		s.emitDiagnostic("client-stream", finishedAt, &endpointDiagnostic{
			Level:    "error",
			Code:     "application.history_write_artifacts_failed",
			Category: contracts.ErrorCategoryApplication,
			Message:  "The runtime could not write the streaming session history artifacts.",
			Details:  map[string]string{"cause": artifactErr.Error()},
		})
	}

	summary := contracts.HistoryCallSummary{
		CallID:         input.session.callID,
		SessionID:      input.session.sessionID,
		WorkspaceID:    input.scope.ID,
		Method:         input.method.FullName,
		RPCType:        contracts.RPCTypeClientStream,
		EndpointID:     input.endpoint.ID,
		State:          finalState,
		GRPCStatusCode: status.Code,
		StartedAt:      input.startedAt.Format(time.RFC3339Nano),
		FinishedAt:     finishedAt.Format(time.RFC3339Nano),
		DurationMs:     duration.Milliseconds(),
		RequestCount:   input.recorder.requestCount(),
		ResponseCount:  len(input.recorder.responses()),
		Truncated:      false,
		ErrorCategory:  invokeDiagnosticCategory(invokeDiagnostic),
		ErrorCode:      invokeDiagnosticCode(invokeDiagnostic),
		SummaryPath:    artifacts.SummaryPath,
		SessionLogPath: artifacts.SessionLogPath,
	}
	if artifactErr == nil {
		if err := s.historyStore.SaveCallSummary(ctx, summary); err != nil {
			s.emitDiagnostic("client-stream", finishedAt, &endpointDiagnostic{
				Level:    "error",
				Code:     "application.history_save_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The runtime could not save the streaming call summary.",
				Details:  map[string]string{"cause": err.Error()},
			})
		}
	}

	s.emitHistoryBackedStreamEvent(finalEvent)
	s.emitStreamState(input.session, finalState, "", finishedAt)
	s.emitStreamCompleted(input.session, finalState, status, finishedAt)
}

func serverStreamRequestBody(methodDescriptor protoreflect.MethodDescriptor, input contracts.CallStartStreamInput) (any, *contracts.ErrorEnvelope) {
	if input.RequestSpec == nil || len(input.RequestSpec.Messages) == 0 {
		return buildStarterJSONValue(methodDescriptor.Input()), nil
	}

	if len(input.RequestSpec.Messages) > 1 {
		return nil, &contracts.ErrorEnvelope{
			Code:     "validation.server_stream_single_request_required",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Server-streaming calls accept one request message before receiving the stream.",
			Details: map[string]string{
				"messageCount": fmt.Sprintf("%d", len(input.RequestSpec.Messages)),
			},
		}
	}

	return input.RequestSpec.Messages[0].Body, nil
}

func clientStreamRequestMessages(input contracts.CallStartStreamInput) ([]any, *contracts.ErrorEnvelope) {
	if input.RequestSpec == nil {
		return nil, &contracts.ErrorEnvelope{
			Code:     "validation.client_stream_static_sequence_required",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Client-streaming calls require a static request message sequence for Slice 3.2.",
		}
	}

	if input.RequestSpec.Mode != contracts.RequestModeStaticSequence {
		return nil, &contracts.ErrorEnvelope{
			Code:     "validation.client_stream_static_sequence_required",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Client-streaming calls require static-sequence request mode for Slice 3.2.",
			Details: map[string]string{
				"mode": string(input.RequestSpec.Mode),
			},
		}
	}

	if len(input.RequestSpec.Messages) == 0 {
		return nil, &contracts.ErrorEnvelope{
			Code:     "validation.client_stream_messages_required",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Client-streaming calls require at least one request message.",
		}
	}

	messages := make([]any, 0, len(input.RequestSpec.Messages))
	for _, message := range input.RequestSpec.Messages {
		messages = append(messages, message.Body)
	}

	return messages, nil
}

func streamRequestMessagesBody(messages []any) any {
	if len(messages) == 0 {
		return nil
	}

	if len(messages) == 1 {
		return messages[0]
	}

	return append([]any(nil), messages...)
}

func resolveServerStreamIdleTimeout(endpointPreset contracts.EndpointPreset, callOptions contracts.CallOptions) time.Duration {
	timeoutMs := endpointPreset.StreamIdleTimeoutMs
	if callOptions.StreamIdleTimeoutMs > 0 {
		timeoutMs = callOptions.StreamIdleTimeoutMs
	}

	if timeoutMs <= 0 {
		return 0
	}

	return time.Duration(timeoutMs) * time.Millisecond
}

func (s *Service) reserveActiveStreamSession(session *activeStreamSession) *contracts.ErrorEnvelope {
	s.streamSessionsMu.Lock()
	defer s.streamSessionsMu.Unlock()

	if len(s.streamSessions) > 0 {
		return &contracts.ErrorEnvelope{
			Code:     "application.stream_session_active",
			Category: contracts.ErrorCategoryApplication,
			Message:  "Only one active live stream session is supported in the MVP.",
		}
	}

	s.streamSessions[session.sessionID] = session
	return nil
}

func (s *Service) activeStreamSession(sessionID string) (*activeStreamSession, *contracts.ErrorEnvelope) {
	if sessionID == "" {
		return nil, &contracts.ErrorEnvelope{
			Code:     "validation.session_id_required",
			Category: contracts.ErrorCategoryValidation,
			Message:  "A stream session id is required.",
		}
	}

	s.streamSessionsMu.Lock()
	defer s.streamSessionsMu.Unlock()

	session, ok := s.streamSessions[sessionID]
	if !ok {
		return nil, &contracts.ErrorEnvelope{
			Code:     "application.stream_session_not_found",
			Category: contracts.ErrorCategoryApplication,
			Message:  "The stream session is no longer active.",
			Details: map[string]string{
				"sessionId": sessionID,
			},
		}
	}

	return session, nil
}

func (s *Service) removeActiveStreamSession(sessionID string) {
	s.streamSessionsMu.Lock()
	defer s.streamSessionsMu.Unlock()
	delete(s.streamSessions, sessionID)
}

func (s *Service) emitStreamState(session *activeStreamSession, state contracts.StreamState, previous contracts.StreamState, ts time.Time) {
	session.mu.Lock()
	if previous == "" {
		previous = session.state
	}
	session.state = state
	session.mu.Unlock()

	if s.emitter == nil {
		return
	}

	_ = s.emitter.Emit(contracts.EventStreamState, contracts.StreamStateEvent{
		SessionID:     session.sessionID,
		CallID:        session.callID,
		State:         state,
		PreviousState: previous,
		Conditions:    []contracts.SessionCondition{},
		Timestamp:     ts.Format(time.RFC3339Nano),
	})
}

func (s *Service) emitHistoryBackedStreamEvent(event contracts.HistoryLogEvent) {
	if s.emitter == nil {
		return
	}

	_ = s.emitter.Emit(contracts.EventStreamEvent, contracts.StreamEventRecord{
		SessionID: event.SessionID,
		CallID:    event.CallID,
		Sequence:  event.Sequence,
		Kind:      event.Kind,
		Direction: event.Direction,
		Timestamp: event.Timestamp,
		Payload: contracts.StreamEventPayload{
			Preview: contracts.StreamEventPreview{
				JSON: streamEventPreviewJSON(event),
			},
			SizeBytes: event.SizeBytes,
		},
	})
}

func (s *Service) emitStreamError(session *activeStreamSession, diagnosticEvent *contracts.DiagnosticsUpdateEvent, diagnostic *endpointDiagnostic, ts time.Time) {
	if s.emitter == nil {
		return
	}

	envelope := errorEnvelopeFromDiagnostic(diagnostic, "application.stream_failed", "The streaming call failed.")
	if diagnosticEvent != nil {
		envelope.Details = copyDetails(diagnosticEvent.Details)
	}

	_ = s.emitter.Emit(contracts.EventStreamError, contracts.StreamErrorEvent{
		SessionID: session.sessionID,
		CallID:    session.callID,
		Error:     *envelope,
		Timestamp: ts.Format(time.RFC3339Nano),
	})
}

func (s *Service) emitStreamCompleted(session *activeStreamSession, finalState contracts.StreamState, status contracts.StreamStatus, ts time.Time) {
	if s.emitter == nil {
		return
	}

	_ = s.emitter.Emit(contracts.EventStreamCompleted, contracts.StreamCompletedEvent{
		SessionID:  session.sessionID,
		CallID:     session.callID,
		FinalState: finalState,
		Conditions: []contracts.SessionCondition{},
		Status:     status,
		Timestamp:  ts.Format(time.RFC3339Nano),
	})
}

func newStreamHistoryRecorder(callID, sessionID, method string, rpcType contracts.RPCType) *streamHistoryRecorder {
	return &streamHistoryRecorder{
		callID:       callID,
		sessionID:    sessionID,
		method:       method,
		rpcType:      rpcType,
		nextSequence: 1,
	}
}

func (r *streamHistoryRecorder) append(event contracts.HistoryLogEvent) contracts.HistoryLogEvent {
	event.CallID = r.callID
	event.SessionID = r.sessionID
	event.Sequence = r.nextSequence
	r.nextSequence++
	r.events = append(r.events, event)
	return event
}

func (r *streamHistoryRecorder) recordCallStarted(workspaceID, endpointID string, requestBody any, metadata map[string]string, ts time.Time) contracts.HistoryLogEvent {
	return r.append(contracts.HistoryLogEvent{
		Kind:      "call_started",
		Direction: "sent",
		Preview: &contracts.HistoryLogPreview{
			JSON: requestBody,
		},
		GRPC: &contracts.HistoryLogGRPC{
			Method:   r.method,
			RPCType:  r.rpcType,
			Metadata: toRepeatedMetadata(redactMetadataMap(metadata)),
		},
		Details: map[string]string{
			"workspaceId": workspaceID,
			"endpointId":  endpointID,
		},
		Timestamp: ts.Format(time.RFC3339Nano),
	})
}

func (r *streamHistoryRecorder) recordMessageSent(requestBody any, ts time.Time) contracts.HistoryLogEvent {
	messageIndex := len(r.requestBodies)
	r.requestBodies = append(r.requestBodies, requestBody)

	return r.append(contracts.HistoryLogEvent{
		Kind:         "message_sent",
		Direction:    "sent",
		MessageIndex: messageIndex,
		SizeBytes:    measureJSONSize(requestBody),
		Preview: &contracts.HistoryLogPreview{
			JSON: requestBody,
		},
		Timestamp: ts.Format(time.RFC3339Nano),
	})
}

func (r *streamHistoryRecorder) recordLocalHalfClose(ts time.Time) contracts.HistoryLogEvent {
	return r.append(contracts.HistoryLogEvent{
		Kind:      "send_half_closed",
		Direction: "sent",
		Details: map[string]string{
			"state": string(contracts.StreamStateHalfClosedLocal),
		},
		Timestamp: ts.Format(time.RFC3339Nano),
	})
}

func (r *streamHistoryRecorder) recordHeaders(headers map[string][]string, ts time.Time) contracts.HistoryLogEvent {
	redactedHeaders := redactMetadataValues(headers)
	return r.append(contracts.HistoryLogEvent{
		Kind:      "headers_received",
		Direction: "received",
		Preview: &contracts.HistoryLogPreview{
			JSON: redactedHeaders,
		},
		GRPC: &contracts.HistoryLogGRPC{
			Metadata: redactedHeaders,
		},
		Timestamp: ts.Format(time.RFC3339Nano),
	})
}

func (r *streamHistoryRecorder) recordMessageReceived(message ServerStreamMessage) contracts.HistoryLogEvent {
	r.responseBodies = append(r.responseBodies, message.Body)
	return r.append(contracts.HistoryLogEvent{
		Kind:         "message_received",
		Direction:    "received",
		MessageIndex: message.Index,
		SizeBytes:    message.SizeBytes,
		Preview: &contracts.HistoryLogPreview{
			JSON: message.Body,
		},
		Timestamp: message.ReceivedAt.Format(time.RFC3339Nano),
	})
}

func (r *streamHistoryRecorder) recordTrailers(status contracts.StreamStatus, trailers map[string][]string, ts time.Time) contracts.HistoryLogEvent {
	redactedTrailers := redactMetadataValues(trailers)
	return r.append(contracts.HistoryLogEvent{
		Kind:      "trailers_received",
		Direction: "received",
		Preview: &contracts.HistoryLogPreview{
			JSON: map[string]any{
				"metadata":   redactedTrailers,
				"statusCode": status.Code,
			},
		},
		GRPC: &contracts.HistoryLogGRPC{
			StatusCode: status.Code,
			Metadata:   redactedTrailers,
		},
		Timestamp: ts.Format(time.RFC3339Nano),
	})
}

func (r *streamHistoryRecorder) recordFinished(finalState contracts.StreamState, streamStatus contracts.StreamStatus, diagnostic *endpointDiagnostic, duration time.Duration, ts time.Time) contracts.HistoryLogEvent {
	kind := "call_finished"
	if finalState == contracts.StreamStateError {
		kind = "call_failed"
	}
	if finalState == contracts.StreamStateCancelled {
		kind = "call_cancelled"
	}

	return r.append(contracts.HistoryLogEvent{
		Kind:      kind,
		Direction: "received",
		GRPC: &contracts.HistoryLogGRPC{
			StatusCode: streamStatus.Code,
		},
		Details: map[string]string{
			"durationMs":    fmt.Sprintf("%d", duration.Milliseconds()),
			"finalState":    string(finalState),
			"errorCode":     invokeDiagnosticCode(diagnostic),
			"errorCategory": string(invokeDiagnosticCategory(diagnostic)),
		},
		Timestamp: ts.Format(time.RFC3339Nano),
	})
}

func (r *streamHistoryRecorder) requestBody() any {
	return streamRequestMessagesBody(r.requestBodies)
}

func (r *streamHistoryRecorder) requestCount() int {
	return len(r.requestBodies)
}

func (r *streamHistoryRecorder) responses() []any {
	return append([]any(nil), r.responseBodies...)
}

func (r *streamHistoryRecorder) eventsSnapshot() []contracts.HistoryLogEvent {
	return append([]contracts.HistoryLogEvent(nil), r.events...)
}

func streamEventPreviewJSON(event contracts.HistoryLogEvent) any {
	if event.Preview != nil {
		return event.Preview.JSON
	}

	if event.GRPC != nil && len(event.GRPC.Metadata) > 0 {
		return event.GRPC.Metadata
	}

	return nil
}

func statusFromDiagnostic(diagnostic *endpointDiagnostic) contracts.StreamStatus {
	if diagnostic == nil {
		return contracts.StreamStatus{Code: codes.OK.String()}
	}

	if diagnostic.Category == contracts.ErrorCategoryCancelled {
		return contracts.StreamStatus{Code: codes.Canceled.String(), Message: diagnostic.Details["cause"]}
	}

	if diagnostic.Category == contracts.ErrorCategoryGRPCStatus {
		return contracts.StreamStatus{
			Code:    diagnostic.Details["grpcStatusCode"],
			Message: diagnostic.Details["cause"],
		}
	}

	return contracts.StreamStatus{
		Code:    codes.Unknown.String(),
		Message: diagnostic.Message,
	}
}
