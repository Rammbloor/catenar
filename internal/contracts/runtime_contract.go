package contracts

import "slices"

type EventName string

const (
	EventStreamState       EventName = "stream:state"
	EventStreamEvent       EventName = "stream:event"
	EventStreamError       EventName = "stream:error"
	EventStreamCompleted   EventName = "stream:completed"
	EventDiagnosticsUpdate EventName = "diagnostics:update"
)

type BoundMethod string

const (
	MethodWorkspaceCreate             BoundMethod = "WorkspaceCreate"
	MethodWorkspaceOpen               BoundMethod = "WorkspaceOpen"
	MethodWorkspaceSave               BoundMethod = "WorkspaceSave"
	MethodWorkspaceValidate           BoundMethod = "WorkspaceValidate"
	MethodEndpointTest                BoundMethod = "EndpointTest"
	MethodCatalogLoadFromReflection   BoundMethod = "CatalogLoadFromReflection"
	MethodCatalogLoadFromProtoSources BoundMethod = "CatalogLoadFromProtoSources"
	MethodRequestSave                 BoundMethod = "RequestSave"
	MethodCallInvokeUnary             BoundMethod = "CallInvokeUnary"
	MethodCallStartStream             BoundMethod = "CallStartStream"
	MethodCallSendMessage             BoundMethod = "CallSendMessage"
	MethodCallHalfClose               BoundMethod = "CallHalfClose"
	MethodCallCancel                  BoundMethod = "CallCancel"
	MethodHistoryList                 BoundMethod = "HistoryList"
	MethodHistoryGet                  BoundMethod = "HistoryGet"
	MethodDiagnosticsExport           BoundMethod = "DiagnosticsExport"
)

type ErrorCategory string

const (
	ErrorCategoryTransport   ErrorCategory = "transport"
	ErrorCategoryGRPCStatus  ErrorCategory = "grpc_status"
	ErrorCategoryReflection  ErrorCategory = "reflection"
	ErrorCategoryProto       ErrorCategory = "proto"
	ErrorCategoryValidation  ErrorCategory = "validation"
	ErrorCategoryWorkspace   ErrorCategory = "workspace"
	ErrorCategoryApplication ErrorCategory = "application"
	ErrorCategoryCancelled   ErrorCategory = "cancelled"
)

type StreamState string

const (
	StreamStateIdle             StreamState = "idle"
	StreamStateConnecting       StreamState = "connecting"
	StreamStateOpen             StreamState = "open"
	StreamStateHalfClosedLocal  StreamState = "half_closed_local"
	StreamStateHalfClosedRemote StreamState = "half_closed_remote"
	StreamStateClosed           StreamState = "closed"
	StreamStateCancelled        StreamState = "cancelled"
	StreamStateError            StreamState = "error"
)

type SessionCondition string

const (
	SessionConditionTruncated SessionCondition = "truncated"
)

type AppView string

const (
	AppViewHome      AppView = "home"
	AppViewWorkspace AppView = "workspace"
	AppViewSession   AppView = "session"
)

type AppOverlay string

const (
	AppOverlayHistory     AppOverlay = "history-overlay"
	AppOverlaySettings    AppOverlay = "settings-overlay"
	AppOverlayDiagnostics AppOverlay = "diagnostics-overlay"
)

type RPCType string

const (
	RPCTypeUnary        RPCType = "unary"
	RPCTypeServerStream RPCType = "server_stream"
	RPCTypeClientStream RPCType = "client_stream"
	RPCTypeBidiStream   RPCType = "bidi_stream"
)

type RequestMode string

const (
	RequestModeStaticSequence RequestMode = "static-sequence"
	RequestModeInteractive    RequestMode = "interactive"
)

type ErrorEnvelope struct {
	Code     string            `json:"code"`
	Category ErrorCategory     `json:"category"`
	Message  string            `json:"message"`
	Details  map[string]string `json:"details,omitempty"`
}

type StreamStateEvent struct {
	SessionID     string             `json:"sessionId"`
	CallID        string             `json:"callId"`
	State         StreamState        `json:"state"`
	PreviousState StreamState        `json:"previousState,omitempty"`
	Conditions    []SessionCondition `json:"conditions"`
	Timestamp     string             `json:"ts"`
}

type StreamEventPreview struct {
	JSON map[string]any `json:"json,omitempty"`
}

type StreamEventPayload struct {
	Preview   StreamEventPreview `json:"preview"`
	SizeBytes int64              `json:"sizeBytes,omitempty"`
}

type StreamEventRecord struct {
	SessionID string             `json:"sessionId"`
	CallID    string             `json:"callId"`
	Sequence  int64              `json:"seq"`
	Kind      string             `json:"kind"`
	Direction string             `json:"direction"`
	Timestamp string             `json:"ts"`
	Payload   StreamEventPayload `json:"payload"`
}

type StreamErrorEvent struct {
	SessionID string        `json:"sessionId"`
	CallID    string        `json:"callId"`
	Error     ErrorEnvelope `json:"error"`
	Timestamp string        `json:"ts"`
}

type StreamStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StreamCompletedEvent struct {
	SessionID  string             `json:"sessionId"`
	CallID     string             `json:"callId"`
	FinalState StreamState        `json:"finalState"`
	Conditions []SessionCondition `json:"conditions"`
	Status     StreamStatus       `json:"status"`
	Timestamp  string             `json:"ts"`
}

type DiagnosticsUpdateEvent struct {
	ID        string            `json:"id"`
	Source    string            `json:"source"`
	Level     string            `json:"level"`
	Code      string            `json:"code"`
	Category  ErrorCategory     `json:"category"`
	Message   string            `json:"message"`
	NextStep  string            `json:"nextStep,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
	Timestamp string            `json:"ts"`
}

type CallOptions struct {
	RequestTimeoutMs    int `json:"requestTimeoutMs,omitempty"`
	StreamIdleTimeoutMs int `json:"streamIdleTimeoutMs,omitempty"`
}

type StreamMessage struct {
	Body map[string]any `json:"body"`
}

type StreamRequestSpec struct {
	Mode     RequestMode     `json:"mode"`
	Messages []StreamMessage `json:"messages,omitempty"`
}

type CallInvokeUnaryInput struct {
	EndpointID     string            `json:"endpointId"`
	Method         string            `json:"method"`
	EnvironmentRef string            `json:"environmentRef,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Body           map[string]any    `json:"body"`
	CallOptions    CallOptions       `json:"callOptions,omitempty"`
}

type CallStartStreamInput struct {
	EndpointID     string             `json:"endpointId"`
	Method         string             `json:"method"`
	RPCType        RPCType            `json:"rpcType"`
	EnvironmentRef string             `json:"environmentRef,omitempty"`
	Metadata       map[string]string  `json:"metadata,omitempty"`
	RequestSpec    *StreamRequestSpec `json:"requestSpec,omitempty"`
	CallOptions    CallOptions        `json:"callOptions,omitempty"`
}

type CallSendMessageInput struct {
	SessionID string        `json:"sessionId"`
	Message   StreamMessage `json:"message"`
}

type CallHalfCloseInput struct {
	SessionID string `json:"sessionId"`
}

type CallCancelInput struct {
	SessionID string `json:"sessionId"`
}

type ModuleContract struct {
	Name           string `json:"name"`
	Responsibility string `json:"responsibility"`
}

type TransitionRule struct {
	From  StreamState   `json:"from"`
	Event string        `json:"event"`
	To    []StreamState `json:"to"`
	Notes string        `json:"notes,omitempty"`
}

type ContractManifest struct {
	Version              string           `json:"version"`
	BoundMethods         []string         `json:"boundMethods"`
	EventNames           []string         `json:"eventNames"`
	ErrorCategories      []string         `json:"errorCategories"`
	TopLevelViews        []string         `json:"topLevelViews"`
	Overlays             []string         `json:"overlays"`
	StreamStates         []string         `json:"streamStates"`
	TerminalStreamStates []string         `json:"terminalStreamStates"`
	SessionConditions    []string         `json:"sessionConditions"`
	Transitions          []TransitionRule `json:"transitions"`
	Modules              []ModuleContract `json:"modules"`
}

func BuildContractManifest() ContractManifest {
	return ContractManifest{
		Version: "v1",
		BoundMethods: toStrings([]BoundMethod{
			MethodWorkspaceCreate,
			MethodWorkspaceOpen,
			MethodWorkspaceSave,
			MethodWorkspaceValidate,
			MethodEndpointTest,
			MethodCatalogLoadFromReflection,
			MethodCatalogLoadFromProtoSources,
			MethodRequestSave,
			MethodCallInvokeUnary,
			MethodCallStartStream,
			MethodCallSendMessage,
			MethodCallHalfClose,
			MethodCallCancel,
			MethodHistoryList,
			MethodHistoryGet,
			MethodDiagnosticsExport,
		}),
		EventNames: toStrings([]EventName{
			EventStreamState,
			EventStreamEvent,
			EventStreamError,
			EventStreamCompleted,
			EventDiagnosticsUpdate,
		}),
		ErrorCategories: toStrings([]ErrorCategory{
			ErrorCategoryTransport,
			ErrorCategoryGRPCStatus,
			ErrorCategoryReflection,
			ErrorCategoryProto,
			ErrorCategoryValidation,
			ErrorCategoryWorkspace,
			ErrorCategoryApplication,
			ErrorCategoryCancelled,
		}),
		TopLevelViews: toStrings([]AppView{
			AppViewHome,
			AppViewWorkspace,
			AppViewSession,
		}),
		Overlays: toStrings([]AppOverlay{
			AppOverlayHistory,
			AppOverlaySettings,
			AppOverlayDiagnostics,
		}),
		StreamStates: toStrings([]StreamState{
			StreamStateIdle,
			StreamStateConnecting,
			StreamStateOpen,
			StreamStateHalfClosedLocal,
			StreamStateHalfClosedRemote,
			StreamStateClosed,
			StreamStateCancelled,
			StreamStateError,
		}),
		TerminalStreamStates: toStrings([]StreamState{
			StreamStateClosed,
			StreamStateCancelled,
			StreamStateError,
		}),
		SessionConditions: toStrings([]SessionCondition{
			SessionConditionTruncated,
		}),
		Transitions: BuildStreamTransitionTable(),
		Modules: []ModuleContract{
			{Name: "workspace-manager", Responsibility: "Workspace lifecycle, saved requests, history references and import path ownership."},
			{Name: "proto-loader", Responsibility: "Reads .proto files, import roots and builds the descriptor graph."},
			{Name: "reflection-client", Responsibility: "Loads dynamic service catalogs from gRPC Server Reflection."},
			{Name: "grpc-runtime", Responsibility: "Owns connections, TLS, metadata, RPC invocation and stream lifecycles."},
			{Name: "event-log", Responsibility: "Appends sent, received, info and error records for each call session."},
			{Name: "secret-store", Responsibility: "Resolves secret references without persisting raw secret values in workspace files."},
			{Name: "transport-adapter", Responsibility: "Keeps the domain model gRPC-aware without leaking transport implementation details into the UI contract."},
		},
	}
}

func BuildStreamTransitionTable() []TransitionRule {
	return []TransitionRule{
		{From: StreamStateIdle, Event: "start_stream", To: []StreamState{StreamStateConnecting}, Notes: "session created"},
		{From: StreamStateConnecting, Event: "transport_ready", To: []StreamState{StreamStateOpen}, Notes: "stream established"},
		{From: StreamStateConnecting, Event: "user_cancel", To: []StreamState{StreamStateCancelled}, Notes: "before open"},
		{From: StreamStateConnecting, Event: "transport_failure", To: []StreamState{StreamStateError}, Notes: "include diagnostics"},
		{From: StreamStateOpen, Event: "user_half_close_local_send", To: []StreamState{StreamStateHalfClosedLocal}, Notes: "for client and bidi streams"},
		{From: StreamStateOpen, Event: "remote_half_close", To: []StreamState{StreamStateHalfClosedRemote}, Notes: "remote finished sending"},
		{From: StreamStateOpen, Event: "stream_completed", To: []StreamState{StreamStateClosed}, Notes: "normal completion without intermediate half-close"},
		{From: StreamStateOpen, Event: "user_cancel", To: []StreamState{StreamStateCancelled}, Notes: "terminal"},
		{From: StreamStateOpen, Event: "transport_failure", To: []StreamState{StreamStateError}, Notes: "terminal"},
		{From: StreamStateHalfClosedLocal, Event: "remote_completes_send_side", To: []StreamState{StreamStateClosed, StreamStateHalfClosedRemote}, Notes: "remote half-close may be observed explicitly"},
		{From: StreamStateHalfClosedLocal, Event: "user_cancel", To: []StreamState{StreamStateCancelled}, Notes: "terminal"},
		{From: StreamStateHalfClosedLocal, Event: "transport_failure", To: []StreamState{StreamStateError}, Notes: "terminal"},
		{From: StreamStateHalfClosedRemote, Event: "local_finalization", To: []StreamState{StreamStateClosed}, Notes: "terminal success"},
		{From: StreamStateHalfClosedRemote, Event: "user_cancel", To: []StreamState{StreamStateCancelled}, Notes: "terminal"},
		{From: StreamStateHalfClosedRemote, Event: "transport_failure", To: []StreamState{StreamStateError}, Notes: "terminal"},
	}
}

func CanTransition(from StreamState, to StreamState) bool {
	if from == to {
		return true
	}

	for _, rule := range BuildStreamTransitionTable() {
		if rule.From == from && slices.Contains(rule.To, to) {
			return true
		}
	}

	return false
}

func NewErrorEnvelope(code string, category ErrorCategory, message string, details map[string]string) ErrorEnvelope {
	return ErrorEnvelope{
		Code:     code,
		Category: category,
		Message:  message,
		Details:  details,
	}
}

func toStrings[T ~string](items []T) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, string(item))
	}
	return result
}
