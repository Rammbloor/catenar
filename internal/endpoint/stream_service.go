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

const (
	DefaultMaxEventsPerCall = 10000
	DefaultMaxBytesPerCall  = 33554432
)

type EventRetentionPolicy struct {
	MaxEventsPerCall int
	MaxBytesPerCall  int64
}

type streamRetentionSnapshot struct {
	rpcType          contracts.RPCType
	truncated        bool
	reason           string
	maxEventsPerCall int
	maxBytesPerCall  int64
	retainedEvents   int
	retainedBytes    int64
	totalEvents      int64
	droppedEvents    int64
}

type recordedStreamEvent struct {
	event     contracts.HistoryLogEvent
	retained  bool
	retention streamRetentionSnapshot
}

type activeStreamSession struct {
	mu         sync.Mutex
	callID     string
	sessionID  string
	cancel     context.CancelFunc
	state      contracts.StreamState
	conditions []contracts.SessionCondition
	client     *interactiveClientStreamSession
}

type interactiveClientStreamSession struct {
	mu               sync.Mutex
	conn             GRPCClientConn
	stream           ClientStreamStartResult
	cancel           context.CancelFunc
	startedAt        time.Time
	scope            WorkspaceContext
	endpoint         contracts.EndpointPreset
	method           contracts.CatalogMethod
	methodDesc       protoreflect.MethodDescriptor
	recorder         *streamHistoryRecorder
	halfCloseStarted bool
	completed        bool
}

type streamHistoryRecorder struct {
	callID         string
	sessionID      string
	method         string
	rpcType        contracts.RPCType
	retention      EventRetentionPolicy
	nextSequence   int64
	retainedBytes  int64
	totalEvents    int64
	droppedEvents  int64
	truncated      bool
	truncatedCause string
	requestTotal   int
	responseTotal  int
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

	if input.RPCType != contracts.RPCTypeServerStream &&
		input.RPCType != contracts.RPCTypeClientStream &&
		input.RPCType != contracts.RPCTypeBidiStream {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.method_rpc_type_invalid",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Slice 3.4 can start server-streaming, client-streaming and bidirectional streaming methods.",
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
	if selectedMethod.RPCType == contracts.RPCTypeBidiStream {
		return s.callStartBidiStreamInteractive(ctx, cachedCatalog, selectedMethod, methodDescriptor, input)
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

	recorder := s.newStreamHistoryRecorder(cachedCatalog.scope, callID, sessionID, selectedMethod.FullName, contracts.RPCTypeServerStream)
	s.emitRecordedStreamEvent(session, recorder.recordCallStarted(cachedCatalog.scope.ID, cachedCatalog.endpoint.ID, requestBody, mergedMetadata, startedAt))
	s.emitRecordedStreamEvent(session, recorder.recordMessageSent(requestBody, startedAt))

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
	client := session.client
	session.mu.Unlock()
	resultState := contracts.StreamStateCancelled
	if state == contracts.StreamStateClosed ||
		state == contracts.StreamStateCancelled ||
		state == contracts.StreamStateError {
		resultState = state
	}
	cancel()
	if client != nil {
		client.mu.Lock()
		halfCloseStarted := client.halfCloseStarted
		completed := client.completed
		client.mu.Unlock()
		if !halfCloseStarted && !completed {
			cancelDiagnostic := classifyClientStreamError(client.method, context.Canceled)
			s.completeClientStreamInteractiveSession(ctx, session, client, ClientStreamInvokeResult{
				Status:     statusFromDiagnostic(cancelDiagnostic),
				FinishedAt: requestedAt,
			}, cancelDiagnostic, state)
		}
	}

	return contracts.CallCancelResponse{
		Ok: true,
		Data: &contracts.CallCancelResult{
			CallID:      session.callID,
			SessionID:   session.sessionID,
			State:       resultState,
			RequestedAt: requestedAt.Format(time.RFC3339Nano),
		},
	}
}

func (s *Service) CallSendMessage(ctx context.Context, input contracts.CallSendMessageInput) contracts.CallSendMessageResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	session, errEnvelope := s.activeStreamSession(input.SessionID)
	if errEnvelope != nil {
		return contracts.CallSendMessageResponse{
			Ok:    false,
			Error: errEnvelope,
		}
	}

	session.mu.Lock()
	client := session.client
	state := session.state
	session.mu.Unlock()
	if client == nil {
		return contracts.CallSendMessageResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.stream_session_not_interactive",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The active stream session does not accept manual client messages.",
				Details: map[string]string{
					"sessionId": session.sessionID,
				},
			},
		}
	}
	if state != contracts.StreamStateOpen {
		return contracts.CallSendMessageResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.stream_send_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "Client-streaming messages can only be sent while the stream is open.",
				Details: map[string]string{
					"sessionId": session.sessionID,
					"state":     string(state),
				},
			},
		}
	}

	client.mu.Lock()
	if client.halfCloseStarted || client.completed {
		client.mu.Unlock()
		return contracts.CallSendMessageResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.stream_send_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The client send side is already closed for this stream.",
				Details: map[string]string{
					"sessionId": session.sessionID,
				},
			},
		}
	}

	messageIndex := client.recorder.requestCount()
	sent, sendDiagnostic := s.grpcRuntime.SendClientStreamMessage(ClientStreamSendRequest{
		Method:     client.method,
		Descriptor: client.methodDesc,
		Stream:     client.stream,
		Body:       input.Message.Body,
		Index:      messageIndex,
	})
	if sendDiagnostic != nil {
		client.mu.Unlock()
		if sendDiagnostic.Category == contracts.ErrorCategoryValidation {
			s.emitDiagnostic("client-stream", s.now().UTC(), sendDiagnostic)
		} else {
			s.completeClientStreamInteractiveSession(ctx, session, client, ClientStreamInvokeResult{
				Status:     statusFromDiagnostic(sendDiagnostic),
				FinishedAt: s.now().UTC(),
			}, sendDiagnostic, state)
		}
		return contracts.CallSendMessageResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(sendDiagnostic, "application.stream_send_failed", "The client-streaming message could not be sent."),
		}
	}

	event := client.recorder.recordMessageSent(sent.Body, sent.SentAt)
	client.mu.Unlock()
	s.emitRecordedStreamEvent(session, event)

	return contracts.CallSendMessageResponse{
		Ok: true,
		Data: &contracts.CallSendMessageResult{
			CallID:       session.callID,
			SessionID:    session.sessionID,
			State:        contracts.StreamStateOpen,
			MessageIndex: event.event.MessageIndex,
			Sequence:     event.event.Sequence,
			SentAt:       sent.SentAt.Format(time.RFC3339Nano),
		},
	}
}

func (s *Service) CallHalfClose(ctx context.Context, input contracts.CallHalfCloseInput) contracts.CallHalfCloseResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	requestedAt := s.now().UTC()
	session, errEnvelope := s.activeStreamSession(input.SessionID)
	if errEnvelope != nil {
		return contracts.CallHalfCloseResponse{
			Ok:    false,
			Error: errEnvelope,
		}
	}

	session.mu.Lock()
	client := session.client
	state := session.state
	session.mu.Unlock()
	if client == nil {
		return contracts.CallHalfCloseResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.stream_session_not_interactive",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The active stream session does not expose a manual client send side.",
				Details: map[string]string{
					"sessionId": session.sessionID,
				},
			},
		}
	}
	if state != contracts.StreamStateOpen {
		return contracts.CallHalfCloseResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.stream_half_close_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The client send side can only be half-closed while the stream is open.",
				Details: map[string]string{
					"sessionId": session.sessionID,
					"state":     string(state),
				},
			},
		}
	}

	client.mu.Lock()
	if client.halfCloseStarted || client.completed {
		client.mu.Unlock()
		return contracts.CallHalfCloseResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.stream_half_close_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The client send side is already half-closed.",
				Details: map[string]string{
					"sessionId": session.sessionID,
				},
			},
		}
	}

	closeDiagnostic := s.grpcRuntime.CloseClientStreamSend(client.method, client.stream)
	if closeDiagnostic != nil {
		client.mu.Unlock()
		s.completeClientStreamInteractiveSession(ctx, session, client, ClientStreamInvokeResult{
			Status:     statusFromDiagnostic(closeDiagnostic),
			FinishedAt: requestedAt,
		}, closeDiagnostic, state)
		return contracts.CallHalfCloseResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(closeDiagnostic, "application.stream_half_close_failed", "The client send side could not be half-closed."),
		}
	}

	client.halfCloseStarted = true
	event := client.recorder.recordLocalHalfClose(requestedAt)
	client.mu.Unlock()

	s.emitRecordedStreamEvent(session, event)
	s.emitStreamState(session, contracts.StreamStateHalfClosedLocal, contracts.StreamStateOpen, requestedAt)
	if client.method.RPCType == contracts.RPCTypeClientStream {
		go s.runClientStreamInteractiveCompletion(ctx, session, client)
	}

	return contracts.CallHalfCloseResponse{
		Ok: true,
		Data: &contracts.CallHalfCloseResult{
			CallID:      session.callID,
			SessionID:   session.sessionID,
			State:       contracts.StreamStateHalfClosedLocal,
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
	if input.RequestSpec == nil {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.client_stream_static_sequence_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Client-streaming calls require either static-sequence or interactive request mode.",
			},
		}
	}

	if input.RequestSpec.Mode == contracts.RequestModeInteractive {
		return s.callStartClientStreamInteractive(ctx, cachedCatalog, selectedMethod, methodDescriptor, input)
	}

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
	recorder := s.newStreamHistoryRecorder(cachedCatalog.scope, callID, sessionID, selectedMethod.FullName, contracts.RPCTypeClientStream)
	startEvent := recorder.recordCallStarted(cachedCatalog.scope.ID, cachedCatalog.endpoint.ID, streamRequestMessagesBody(requestMessages), mergedMetadata, startedAt)
	s.emitRecordedStreamEvent(session, startEvent)

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

func (s *Service) callStartClientStreamInteractive(
	ctx context.Context,
	cachedCatalog cachedMethodCatalog,
	selectedMethod contracts.CatalogMethod,
	methodDescriptor protoreflect.MethodDescriptor,
	input contracts.CallStartStreamInput,
) contracts.CallStartStreamResponse {
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
	streamStart, startDiagnostic := s.grpcRuntime.StartClientStream(callCtx, conn, ClientStreamStartRequest{
		Method:     selectedMethod,
		Descriptor: methodDescriptor,
		Metadata:   mergedMetadata,
	})
	if startDiagnostic != nil {
		cancel()
		s.removeActiveStreamSession(sessionID)
		_ = conn.Close()
		s.emitDiagnostic("client-stream", startedAt, startDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(startDiagnostic, "application.stream_start_failed", "The client-streaming call could not be started."),
		}
	}

	recorder := s.newStreamHistoryRecorder(cachedCatalog.scope, callID, sessionID, selectedMethod.FullName, contracts.RPCTypeClientStream)
	startEvent := recorder.recordCallStarted(cachedCatalog.scope.ID, cachedCatalog.endpoint.ID, nil, mergedMetadata, startedAt)
	s.emitRecordedStreamEvent(session, startEvent)

	session.mu.Lock()
	session.client = &interactiveClientStreamSession{
		conn:       conn,
		stream:     streamStart,
		cancel:     cancel,
		startedAt:  startedAt,
		scope:      cachedCatalog.scope,
		endpoint:   cachedCatalog.endpoint,
		method:     selectedMethod,
		methodDesc: methodDescriptor,
		recorder:   recorder,
	}
	session.mu.Unlock()

	s.emitStreamState(session, contracts.StreamStateConnecting, "", startedAt)
	s.emitStreamState(session, contracts.StreamStateOpen, contracts.StreamStateConnecting, s.now().UTC())

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

func (s *Service) callStartBidiStreamInteractive(
	ctx context.Context,
	cachedCatalog cachedMethodCatalog,
	selectedMethod contracts.CatalogMethod,
	methodDescriptor protoreflect.MethodDescriptor,
	input contracts.CallStartStreamInput,
) contracts.CallStartStreamResponse {
	if input.RequestSpec == nil || input.RequestSpec.Mode != contracts.RequestModeInteractive {
		return contracts.CallStartStreamResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.bidi_stream_interactive_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Bidirectional streaming calls require interactive request mode.",
			},
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
		s.emitDiagnostic("bidi-stream", startedAt, prepDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(prepDiagnostic, "application.endpoint_preparation_failed", "The endpoint could not be prepared."),
		}
	}

	conn, runtimeDiagnostic := s.grpcRuntime.Dial(ctx, runtimeCfg)
	if runtimeDiagnostic != nil {
		cancel()
		s.removeActiveStreamSession(sessionID)
		s.emitDiagnostic("bidi-stream", startedAt, runtimeDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(runtimeDiagnostic, "transport.grpc_not_ready", "The endpoint could not establish a ready gRPC channel."),
		}
	}

	mergedMetadata := mergeInvokeMetadata(cachedCatalog.endpoint.MetadataDefaults, input.Metadata)
	streamStart, startDiagnostic := s.grpcRuntime.StartClientStream(callCtx, conn, ClientStreamStartRequest{
		Method:     selectedMethod,
		Descriptor: methodDescriptor,
		Metadata:   mergedMetadata,
	})
	if startDiagnostic != nil {
		cancel()
		s.removeActiveStreamSession(sessionID)
		_ = conn.Close()
		s.emitDiagnostic("bidi-stream", startedAt, startDiagnostic)
		return contracts.CallStartStreamResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(startDiagnostic, "application.stream_start_failed", "The bidirectional streaming call could not be started."),
		}
	}

	recorder := s.newStreamHistoryRecorder(cachedCatalog.scope, callID, sessionID, selectedMethod.FullName, contracts.RPCTypeBidiStream)
	startEvent := recorder.recordCallStarted(cachedCatalog.scope.ID, cachedCatalog.endpoint.ID, nil, mergedMetadata, startedAt)
	s.emitRecordedStreamEvent(session, startEvent)

	client := &interactiveClientStreamSession{
		conn:       conn,
		stream:     streamStart,
		cancel:     cancel,
		startedAt:  startedAt,
		scope:      cachedCatalog.scope,
		endpoint:   cachedCatalog.endpoint,
		method:     selectedMethod,
		methodDesc: methodDescriptor,
		recorder:   recorder,
	}
	session.mu.Lock()
	session.client = client
	session.mu.Unlock()

	s.emitStreamState(session, contracts.StreamStateConnecting, "", startedAt)
	s.emitStreamState(session, contracts.StreamStateOpen, contracts.StreamStateConnecting, s.now().UTC())

	go s.runBidiStreamReceiveLoop(ctx, session, client, resolveServerStreamIdleTimeout(cachedCatalog.endpoint, input.CallOptions))

	return contracts.CallStartStreamResponse{
		Ok: true,
		Data: &contracts.CallStartStreamResult{
			CallID:     callID,
			SessionID:  sessionID,
			EndpointID: cachedCatalog.endpoint.ID,
			Method:     selectedMethod.FullName,
			RPCType:    contracts.RPCTypeBidiStream,
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
			s.emitRecordedStreamEvent(input.session, event)
		},
		OnMessage: func(message ServerStreamMessage) {
			event := input.recorder.recordMessageReceived(message)
			s.emitRecordedStreamEvent(input.session, event)
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
		s.emitRecordedStreamEvent(input.session, event)
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
		RequestCount:   input.recorder.requestCount(),
		ResponseCount:  input.recorder.responseCount(),
		Truncated:      input.recorder.truncated,
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

	s.emitRecordedStreamEvent(input.session, finalEvent)
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
			s.emitRecordedStreamEvent(input.session, event)
		},
		OnHalfClose: func(ts time.Time) {
			event := input.recorder.recordLocalHalfClose(ts)
			s.emitRecordedStreamEvent(input.session, event)
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
		s.emitRecordedStreamEvent(input.session, event)
	}
	if invokeResult.ResponseBody != nil {
		message := ServerStreamMessage{
			Body:       invokeResult.ResponseBody,
			Index:      0,
			SizeBytes:  measureJSONSize(invokeResult.ResponseBody),
			ReceivedAt: finishedAt,
		}
		event := input.recorder.recordMessageReceived(message)
		s.emitRecordedStreamEvent(input.session, event)
	}
	if len(invokeResult.Trailers) > 0 || status.Code != "" {
		event := input.recorder.recordTrailers(status, invokeResult.Trailers, finishedAt)
		s.emitRecordedStreamEvent(input.session, event)
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
		ResponseCount:  input.recorder.responseCount(),
		Truncated:      input.recorder.truncated,
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

	s.emitRecordedStreamEvent(input.session, finalEvent)
	s.emitStreamState(input.session, finalState, "", finishedAt)
	s.emitStreamCompleted(input.session, finalState, status, finishedAt)
}

func (s *Service) runClientStreamInteractiveCompletion(ctx context.Context, session *activeStreamSession, client *interactiveClientStreamSession) {
	invokeResult, invokeDiagnostic := s.grpcRuntime.ReceiveClientStreamResponse(ClientStreamReceiveRequest{
		Method:     client.method,
		Descriptor: client.methodDesc,
		Stream:     client.stream,
	})
	s.completeClientStreamInteractiveSession(ctx, session, client, invokeResult, invokeDiagnostic, contracts.StreamStateHalfClosedLocal)
}

func (s *Service) runBidiStreamReceiveLoop(ctx context.Context, session *activeStreamSession, client *interactiveClientStreamSession, idleTimeout time.Duration) {
	consumeResult, consumeDiagnostic := s.grpcRuntime.ConsumeServerStream(ServerStreamConsumeRequest{
		Method:      client.method,
		Descriptor:  client.methodDesc,
		Stream:      client.stream.Stream,
		Cancel:      client.cancel,
		IdleTimeout: idleTimeout,
		OnHeaders: func(headers map[string][]string, ts time.Time) {
			client.mu.Lock()
			if client.completed {
				client.mu.Unlock()
				return
			}
			event := client.recorder.recordHeaders(headers, ts)
			client.mu.Unlock()
			s.emitRecordedStreamEvent(session, event)
		},
		OnMessage: func(message ServerStreamMessage) {
			client.mu.Lock()
			if client.completed {
				client.mu.Unlock()
				return
			}
			event := client.recorder.recordMessageReceived(message)
			client.mu.Unlock()
			s.emitRecordedStreamEvent(session, event)
		},
	})

	s.completeBidiStreamSession(ctx, session, client, consumeResult, consumeDiagnostic)
}

func (s *Service) completeBidiStreamSession(
	ctx context.Context,
	session *activeStreamSession,
	client *interactiveClientStreamSession,
	consumeResult ServerStreamConsumeResult,
	consumeDiagnostic *endpointDiagnostic,
) {
	client.mu.Lock()
	if client.completed {
		client.mu.Unlock()
		return
	}
	client.completed = true
	client.mu.Unlock()

	defer func() {
		client.cancel()
		_ = client.conn.Close()
		s.removeActiveStreamSession(session.sessionID)
	}()

	finishedAt := consumeResult.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = s.now().UTC()
	}
	duration := finishedAt.Sub(client.startedAt)
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
		client.mu.Lock()
		event := client.recorder.recordTrailers(status, consumeResult.Trailers, finishedAt)
		client.mu.Unlock()
		s.emitRecordedStreamEvent(session, event)
	}

	client.mu.Lock()
	finalEvent := client.recorder.recordFinished(finalState, status, consumeDiagnostic, duration, finishedAt)
	requestCount := client.recorder.requestCount()
	responseCount := client.recorder.responseCount()
	requestBody := client.recorder.requestBody()
	responseBodies := client.recorder.responses()
	events := client.recorder.eventsSnapshot()
	truncated := client.recorder.truncated
	client.mu.Unlock()

	if consumeDiagnostic != nil {
		diagnosticEvent := s.emitDiagnostic("bidi-stream", finishedAt, consumeDiagnostic)
		if consumeDiagnostic.Category != contracts.ErrorCategoryCancelled {
			s.emitStreamError(session, diagnosticEvent, consumeDiagnostic, finishedAt)
		}
	}

	artifacts, artifactErr := s.eventLog.WriteStreamCall(ctx, StreamEventLogRecord{
		CallID:         session.callID,
		RequestBody:    requestBody,
		ResponseBodies: responseBodies,
		Headers:        consumeResult.Headers,
		Trailers:       consumeResult.Trailers,
		Status:         status,
		Events:         events,
	})
	if artifactErr != nil {
		s.emitDiagnostic("bidi-stream", finishedAt, &endpointDiagnostic{
			Level:    "error",
			Code:     "application.history_write_artifacts_failed",
			Category: contracts.ErrorCategoryApplication,
			Message:  "The runtime could not write the streaming session history artifacts.",
			Details:  map[string]string{"cause": artifactErr.Error()},
		})
	}

	summary := contracts.HistoryCallSummary{
		CallID:         session.callID,
		SessionID:      session.sessionID,
		WorkspaceID:    client.scope.ID,
		Method:         client.method.FullName,
		RPCType:        contracts.RPCTypeBidiStream,
		EndpointID:     client.endpoint.ID,
		State:          finalState,
		GRPCStatusCode: status.Code,
		StartedAt:      client.startedAt.Format(time.RFC3339Nano),
		FinishedAt:     finishedAt.Format(time.RFC3339Nano),
		DurationMs:     duration.Milliseconds(),
		RequestCount:   requestCount,
		ResponseCount:  responseCount,
		Truncated:      truncated,
		ErrorCategory:  invokeDiagnosticCategory(consumeDiagnostic),
		ErrorCode:      invokeDiagnosticCode(consumeDiagnostic),
		SummaryPath:    artifacts.SummaryPath,
		SessionLogPath: artifacts.SessionLogPath,
	}
	if artifactErr == nil {
		if err := s.historyStore.SaveCallSummary(ctx, summary); err != nil {
			s.emitDiagnostic("bidi-stream", finishedAt, &endpointDiagnostic{
				Level:    "error",
				Code:     "application.history_save_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The runtime could not save the streaming call summary.",
				Details:  map[string]string{"cause": err.Error()},
			})
		}
	}

	session.mu.Lock()
	previousState := session.state
	session.mu.Unlock()

	s.emitRecordedStreamEvent(session, finalEvent)
	s.emitStreamState(session, finalState, previousState, finishedAt)
	s.emitStreamCompleted(session, finalState, status, finishedAt)
}

func (s *Service) completeClientStreamInteractiveSession(
	ctx context.Context,
	session *activeStreamSession,
	client *interactiveClientStreamSession,
	invokeResult ClientStreamInvokeResult,
	invokeDiagnostic *endpointDiagnostic,
	previousState contracts.StreamState,
) {
	client.mu.Lock()
	if client.completed {
		client.mu.Unlock()
		return
	}
	client.completed = true
	client.mu.Unlock()

	defer func() {
		client.cancel()
		_ = client.conn.Close()
		s.removeActiveStreamSession(session.sessionID)
	}()

	finishedAt := invokeResult.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = s.now().UTC()
	}
	duration := finishedAt.Sub(client.startedAt)
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
		event := client.recorder.recordHeaders(invokeResult.Headers, finishedAt)
		s.emitRecordedStreamEvent(session, event)
	}
	if invokeResult.ResponseBody != nil {
		message := ServerStreamMessage{
			Body:       invokeResult.ResponseBody,
			Index:      0,
			SizeBytes:  measureJSONSize(invokeResult.ResponseBody),
			ReceivedAt: finishedAt,
		}
		event := client.recorder.recordMessageReceived(message)
		s.emitRecordedStreamEvent(session, event)
	}
	if len(invokeResult.Trailers) > 0 || status.Code != "" {
		event := client.recorder.recordTrailers(status, invokeResult.Trailers, finishedAt)
		s.emitRecordedStreamEvent(session, event)
	}
	finalEvent := client.recorder.recordFinished(finalState, status, invokeDiagnostic, duration, finishedAt)

	if invokeDiagnostic != nil {
		diagnosticEvent := s.emitDiagnostic(streamDiagnosticSource(client.method.RPCType), finishedAt, invokeDiagnostic)
		if invokeDiagnostic.Category != contracts.ErrorCategoryCancelled {
			s.emitStreamError(session, diagnosticEvent, invokeDiagnostic, finishedAt)
		}
	}

	artifacts, artifactErr := s.eventLog.WriteStreamCall(ctx, StreamEventLogRecord{
		CallID:         session.callID,
		RequestBody:    client.recorder.requestBody(),
		ResponseBodies: client.recorder.responses(),
		Headers:        invokeResult.Headers,
		Trailers:       invokeResult.Trailers,
		Status:         status,
		Events:         client.recorder.eventsSnapshot(),
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

	rpcType := client.method.RPCType
	if rpcType == "" {
		rpcType = contracts.RPCTypeClientStream
	}

	summary := contracts.HistoryCallSummary{
		CallID:         session.callID,
		SessionID:      session.sessionID,
		WorkspaceID:    client.scope.ID,
		Method:         client.method.FullName,
		RPCType:        rpcType,
		EndpointID:     client.endpoint.ID,
		State:          finalState,
		GRPCStatusCode: status.Code,
		StartedAt:      client.startedAt.Format(time.RFC3339Nano),
		FinishedAt:     finishedAt.Format(time.RFC3339Nano),
		DurationMs:     duration.Milliseconds(),
		RequestCount:   client.recorder.requestCount(),
		ResponseCount:  client.recorder.responseCount(),
		Truncated:      client.recorder.truncated,
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

	s.emitRecordedStreamEvent(session, finalEvent)
	s.emitStreamState(session, finalState, previousState, finishedAt)
	s.emitStreamCompleted(session, finalState, status, finishedAt)
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
			Message:  "Client-streaming calls require a static request message sequence in static-sequence mode.",
		}
	}

	if input.RequestSpec.Mode != contracts.RequestModeStaticSequence {
		return nil, &contracts.ErrorEnvelope{
			Code:     "validation.client_stream_static_sequence_required",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Client-streaming calls require static-sequence request mode for static sequence execution.",
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

func streamDiagnosticSource(rpcType contracts.RPCType) string {
	if rpcType == contracts.RPCTypeBidiStream {
		return "bidi-stream"
	}
	if rpcType == contracts.RPCTypeServerStream {
		return "server-stream"
	}
	return "client-stream"
}

func normalizeEventRetentionPolicy(policy EventRetentionPolicy) EventRetentionPolicy {
	if policy.MaxEventsPerCall <= 0 {
		policy.MaxEventsPerCall = DefaultMaxEventsPerCall
	}
	if policy.MaxBytesPerCall <= 0 {
		policy.MaxBytesPerCall = DefaultMaxBytesPerCall
	}

	return policy
}

func (s *Service) eventRetentionForScope(scope WorkspaceContext) EventRetentionPolicy {
	if scope.EventRetention.MaxEventsPerCall > 0 || scope.EventRetention.MaxBytesPerCall > 0 {
		return normalizeEventRetentionPolicy(scope.EventRetention)
	}

	return s.eventRetention
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
	conditions := append([]contracts.SessionCondition(nil), session.conditions...)
	session.mu.Unlock()

	if s.emitter == nil {
		return
	}

	_ = s.emitter.Emit(contracts.EventStreamState, contracts.StreamStateEvent{
		SessionID:     session.sessionID,
		CallID:        session.callID,
		State:         state,
		PreviousState: previous,
		Conditions:    conditions,
		Timestamp:     ts.Format(time.RFC3339Nano),
	})
}

func (s *Service) emitRecordedStreamEvent(session *activeStreamSession, recorded recordedStreamEvent) {
	s.emitHistoryBackedStreamEvent(recorded.event)
	if recorded.retention.truncated {
		s.markStreamTruncated(session, recorded.retention)
	}
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

func (s *Service) markStreamTruncated(session *activeStreamSession, retention streamRetentionSnapshot) {
	session.mu.Lock()
	if sessionHasCondition(session.conditions, contracts.SessionConditionTruncated) {
		session.mu.Unlock()
		return
	}
	session.conditions = append(session.conditions, contracts.SessionConditionTruncated)
	state := session.state
	session.mu.Unlock()

	ts := s.now().UTC()
	s.emitDiagnostic(streamDiagnosticSource(retention.rpcType), ts, &endpointDiagnostic{
		Level:    "warning",
		Code:     "application.stream_event_retention_truncated",
		Category: contracts.ErrorCategoryApplication,
		Message:  "The streaming event log reached the configured retention limit and was marked truncated.",
		NextStep: "Export the session before increasing stream volume, or raise event retention limits in workspace settings.",
		Details:  retention.details(),
	})
	s.emitStreamState(session, state, state, ts)
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
	session.mu.Lock()
	conditions := append([]contracts.SessionCondition(nil), session.conditions...)
	session.mu.Unlock()

	if s.emitter == nil {
		return
	}

	_ = s.emitter.Emit(contracts.EventStreamCompleted, contracts.StreamCompletedEvent{
		SessionID:  session.sessionID,
		CallID:     session.callID,
		FinalState: finalState,
		Conditions: conditions,
		Status:     status,
		Timestamp:  ts.Format(time.RFC3339Nano),
	})
}

func (s *Service) newStreamHistoryRecorder(scope WorkspaceContext, callID, sessionID, method string, rpcType contracts.RPCType) *streamHistoryRecorder {
	return &streamHistoryRecorder{
		callID:       callID,
		sessionID:    sessionID,
		method:       method,
		rpcType:      rpcType,
		retention:    s.eventRetentionForScope(scope),
		nextSequence: 1,
	}
}

func (r *streamHistoryRecorder) append(event contracts.HistoryLogEvent) recordedStreamEvent {
	event.CallID = r.callID
	event.SessionID = r.sessionID
	event.Sequence = r.nextSequence
	r.nextSequence++
	r.totalEvents++

	eventSize := measureJSONSize(event)
	retained := r.retainNextEvent(eventSize)
	if retained {
		r.events = append(r.events, event)
		r.retainedBytes += eventSize
	} else {
		r.droppedEvents++
	}

	return recordedStreamEvent{
		event:     event,
		retained:  retained,
		retention: r.retentionSnapshot(),
	}
}

func (r *streamHistoryRecorder) retainNextEvent(eventSize int64) bool {
	if r.truncated {
		return false
	}
	if r.retention.MaxEventsPerCall > 0 && len(r.events)+1 > r.retention.MaxEventsPerCall {
		r.markTruncated("max_events_per_call")
		return false
	}
	if r.retention.MaxBytesPerCall > 0 && r.retainedBytes+eventSize > r.retention.MaxBytesPerCall {
		r.markTruncated("max_bytes_per_call")
		return false
	}

	return true
}

func (r *streamHistoryRecorder) markTruncated(reason string) {
	if r.truncated {
		return
	}
	r.truncated = true
	r.truncatedCause = reason
}

func (r *streamHistoryRecorder) retentionSnapshot() streamRetentionSnapshot {
	return streamRetentionSnapshot{
		rpcType:          r.rpcType,
		truncated:        r.truncated,
		reason:           r.truncatedCause,
		maxEventsPerCall: r.retention.MaxEventsPerCall,
		maxBytesPerCall:  r.retention.MaxBytesPerCall,
		retainedEvents:   len(r.events),
		retainedBytes:    r.retainedBytes,
		totalEvents:      r.totalEvents,
		droppedEvents:    r.droppedEvents,
	}
}

func (r streamRetentionSnapshot) details() map[string]string {
	reason := r.reason
	if reason == "" {
		reason = "retention_limit_reached"
	}

	return map[string]string{
		"reason":           reason,
		"maxEventsPerCall": fmt.Sprintf("%d", r.maxEventsPerCall),
		"maxBytesPerCall":  fmt.Sprintf("%d", r.maxBytesPerCall),
		"retainedEvents":   fmt.Sprintf("%d", r.retainedEvents),
		"retainedBytes":    fmt.Sprintf("%d", r.retainedBytes),
		"totalEvents":      fmt.Sprintf("%d", r.totalEvents),
		"droppedEvents":    fmt.Sprintf("%d", r.droppedEvents),
	}
}

func sessionHasCondition(conditions []contracts.SessionCondition, condition contracts.SessionCondition) bool {
	for _, existing := range conditions {
		if existing == condition {
			return true
		}
	}

	return false
}

func (r *streamHistoryRecorder) recordCallStarted(workspaceID, endpointID string, requestBody any, metadata map[string]string, ts time.Time) recordedStreamEvent {
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

func (r *streamHistoryRecorder) recordMessageSent(requestBody any, ts time.Time) recordedStreamEvent {
	messageIndex := r.requestTotal
	r.requestTotal++
	recorded := r.append(contracts.HistoryLogEvent{
		Kind:         "message_sent",
		Direction:    "sent",
		MessageIndex: messageIndex,
		SizeBytes:    measureJSONSize(requestBody),
		Preview: &contracts.HistoryLogPreview{
			JSON: requestBody,
		},
		Timestamp: ts.Format(time.RFC3339Nano),
	})
	if recorded.retained {
		r.requestBodies = append(r.requestBodies, requestBody)
	}

	return recorded
}

func (r *streamHistoryRecorder) recordLocalHalfClose(ts time.Time) recordedStreamEvent {
	return r.append(contracts.HistoryLogEvent{
		Kind:      "send_half_closed",
		Direction: "sent",
		Details: map[string]string{
			"state": string(contracts.StreamStateHalfClosedLocal),
		},
		Timestamp: ts.Format(time.RFC3339Nano),
	})
}

func (r *streamHistoryRecorder) recordHeaders(headers map[string][]string, ts time.Time) recordedStreamEvent {
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

func (r *streamHistoryRecorder) recordMessageReceived(message ServerStreamMessage) recordedStreamEvent {
	r.responseTotal++
	recorded := r.append(contracts.HistoryLogEvent{
		Kind:         "message_received",
		Direction:    "received",
		MessageIndex: message.Index,
		SizeBytes:    message.SizeBytes,
		Preview: &contracts.HistoryLogPreview{
			JSON: message.Body,
		},
		Timestamp: message.ReceivedAt.Format(time.RFC3339Nano),
	})
	if recorded.retained {
		r.responseBodies = append(r.responseBodies, message.Body)
	}

	return recorded
}

func (r *streamHistoryRecorder) recordTrailers(status contracts.StreamStatus, trailers map[string][]string, ts time.Time) recordedStreamEvent {
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

func (r *streamHistoryRecorder) recordFinished(finalState contracts.StreamState, streamStatus contracts.StreamStatus, diagnostic *endpointDiagnostic, duration time.Duration, ts time.Time) recordedStreamEvent {
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
	return r.requestTotal
}

func (r *streamHistoryRecorder) responseCount() int {
	return r.responseTotal
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
