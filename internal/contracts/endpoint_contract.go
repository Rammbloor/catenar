package contracts

type TLSMode string

const (
	TLSModePlaintext TLSMode = "plaintext"
	TLSModeSystemCA  TLSMode = "system_ca"
	TLSModeCustomCA  TLSMode = "custom_ca"
	TLSModeMTLS      TLSMode = "mtls"
)

type EndpointCheckStage string

const (
	EndpointCheckStageTargetResolution EndpointCheckStage = "target_resolution"
	EndpointCheckStageTCPConnect       EndpointCheckStage = "tcp_connect"
	EndpointCheckStageTLSHandshake     EndpointCheckStage = "tls_handshake"
	EndpointCheckStageGRPCReadiness    EndpointCheckStage = "grpc_readiness"
)

type EndpointCheckOutcome string

const (
	EndpointCheckOutcomePassed    EndpointCheckOutcome = "passed"
	EndpointCheckOutcomeFailed    EndpointCheckOutcome = "failed"
	EndpointCheckOutcomeSkipped   EndpointCheckOutcome = "skipped"
	EndpointCheckOutcomeNotProven EndpointCheckOutcome = "not_proven"
)

type EndpointTLSSettings struct {
	Mode               TLSMode `json:"mode"`
	ServerNameOverride string  `json:"serverNameOverride,omitempty"`
	CACert             string  `json:"caCert,omitempty"`
	ClientCert         string  `json:"clientCert,omitempty"`
	ClientKey          string  `json:"clientKey,omitempty"`
}

type EndpointPreset struct {
	ID                  string              `json:"id,omitempty"`
	Name                string              `json:"name,omitempty"`
	Target              string              `json:"target"`
	Authority           string              `json:"authority,omitempty"`
	TLS                 EndpointTLSSettings `json:"tls"`
	ConnectTimeoutMs    int                 `json:"connectTimeoutMs,omitempty"`
	RequestTimeoutMs    int                 `json:"requestTimeoutMs,omitempty"`
	StreamIdleTimeoutMs int                 `json:"streamIdleTimeoutMs,omitempty"`
	MetadataDefaults    map[string]string   `json:"metadataDefaults,omitempty"`
}

type EndpointValidationIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type EndpointTestInput struct {
	Endpoint EndpointPreset `json:"endpoint"`
}

type EndpointCheck struct {
	Stage   EndpointCheckStage   `json:"stage"`
	Outcome EndpointCheckOutcome `json:"outcome"`
	Message string               `json:"message"`
	Details map[string]string    `json:"details,omitempty"`
}

type EndpointTestResult struct {
	Endpoint           EndpointPreset          `json:"endpoint"`
	TransportReachable bool                    `json:"transportReachable"`
	TLSConfigured      bool                    `json:"tlsConfigured"`
	TLSOK              bool                    `json:"tlsOk"`
	GRPCReady          bool                    `json:"grpcReady"`
	GRPCReadyProven    bool                    `json:"grpcReadyProven"`
	Checks             []EndpointCheck         `json:"checks"`
	Diagnostic         *DiagnosticsUpdateEvent `json:"diagnostic,omitempty"`
	TestedAt           string                  `json:"testedAt"`
	DurationMs         int64                   `json:"durationMs"`
}

type EndpointTestResponse struct {
	Ok    bool                `json:"ok"`
	Data  *EndpointTestResult `json:"data,omitempty"`
	Error *ErrorEnvelope      `json:"error,omitempty"`
}

type CatalogLoadFromReflectionInput struct {
	Endpoint EndpointPreset `json:"endpoint"`
}

type ProtoSourceType string

const (
	ProtoSourceTypeDirectory ProtoSourceType = "directory"
	ProtoSourceTypeFile      ProtoSourceType = "file"
)

type ProtoSource struct {
	Type ProtoSourceType `json:"type"`
	Path string          `json:"path"`
}

type CatalogLoadFromProtoSourcesInput struct {
	Endpoint     EndpointPreset `json:"endpoint"`
	ProtoSources []ProtoSource  `json:"protoSources"`
	ImportPaths  []string       `json:"importPaths,omitempty"`
}

type CatalogMessageRef struct {
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	IsWellKnown bool   `json:"isWellKnown"`
}

type CatalogMethod struct {
	Name         string            `json:"name"`
	FullName     string            `json:"fullName"`
	RPCType      RPCType           `json:"rpcType"`
	RequestType  CatalogMessageRef `json:"requestType"`
	ResponseType CatalogMessageRef `json:"responseType"`
}

type CatalogService struct {
	Name     string          `json:"name"`
	FullName string          `json:"fullName"`
	Methods  []CatalogMethod `json:"methods"`
}

type ReflectionCatalogResult struct {
	Endpoint         EndpointPreset          `json:"endpoint"`
	Services         []CatalogService        `json:"services"`
	WellKnownTypes   []CatalogMessageRef     `json:"wellKnownTypes,omitempty"`
	RequestTemplates map[string]any          `json:"requestTemplates,omitempty"`
	Diagnostic       *DiagnosticsUpdateEvent `json:"diagnostic,omitempty"`
	LoadedAt         string                  `json:"loadedAt"`
	DurationMs       int64                   `json:"durationMs"`
}

type CatalogLoadFromReflectionResponse struct {
	Ok    bool                     `json:"ok"`
	Data  *ReflectionCatalogResult `json:"data,omitempty"`
	Error *ErrorEnvelope           `json:"error,omitempty"`
}

type ProtoCatalogResult struct {
	Endpoint         EndpointPreset          `json:"endpoint"`
	ProtoSources     []ProtoSource           `json:"protoSources"`
	ImportPaths      []string                `json:"importPaths,omitempty"`
	Services         []CatalogService        `json:"services"`
	WellKnownTypes   []CatalogMessageRef     `json:"wellKnownTypes,omitempty"`
	RequestTemplates map[string]any          `json:"requestTemplates,omitempty"`
	Diagnostic       *DiagnosticsUpdateEvent `json:"diagnostic,omitempty"`
	LoadedAt         string                  `json:"loadedAt"`
	DurationMs       int64                   `json:"durationMs"`
}

type CatalogLoadFromProtoSourcesResponse struct {
	Ok    bool                `json:"ok"`
	Data  *ProtoCatalogResult `json:"data,omitempty"`
	Error *ErrorEnvelope      `json:"error,omitempty"`
}

type CallInvokeUnaryResult struct {
	CallID       string                  `json:"callId"`
	SessionID    string                  `json:"sessionId"`
	EndpointID   string                  `json:"endpointId"`
	Method       string                  `json:"method"`
	RPCType      RPCType                 `json:"rpcType"`
	FinalState   StreamState             `json:"finalState"`
	RequestBody  any                     `json:"requestBody"`
	ResponseBody any                     `json:"responseBody,omitempty"`
	Headers      map[string][]string     `json:"headers,omitempty"`
	Trailers     map[string][]string     `json:"trailers,omitempty"`
	Status       StreamStatus            `json:"status"`
	Diagnostic   *DiagnosticsUpdateEvent `json:"diagnostic,omitempty"`
	StartedAt    string                  `json:"startedAt"`
	FinishedAt   string                  `json:"finishedAt"`
	DurationMs   int64                   `json:"durationMs"`
}

type CallInvokeUnaryResponse struct {
	Ok    bool                   `json:"ok"`
	Data  *CallInvokeUnaryResult `json:"data,omitempty"`
	Error *ErrorEnvelope         `json:"error,omitempty"`
}

type CallStartStreamResult struct {
	CallID     string      `json:"callId"`
	SessionID  string      `json:"sessionId"`
	EndpointID string      `json:"endpointId"`
	Method     string      `json:"method"`
	RPCType    RPCType     `json:"rpcType"`
	State      StreamState `json:"state"`
	StartedAt  string      `json:"startedAt"`
}

type CallStartStreamResponse struct {
	Ok    bool                   `json:"ok"`
	Data  *CallStartStreamResult `json:"data,omitempty"`
	Error *ErrorEnvelope         `json:"error,omitempty"`
}

type CallSendMessageResult struct {
	CallID       string      `json:"callId"`
	SessionID    string      `json:"sessionId"`
	State        StreamState `json:"state"`
	MessageIndex int         `json:"messageIndex"`
	Sequence     int64       `json:"seq"`
	SentAt       string      `json:"sentAt"`
}

type CallSendMessageResponse struct {
	Ok    bool                   `json:"ok"`
	Data  *CallSendMessageResult `json:"data,omitempty"`
	Error *ErrorEnvelope         `json:"error,omitempty"`
}

type CallHalfCloseResult struct {
	CallID      string      `json:"callId"`
	SessionID   string      `json:"sessionId"`
	State       StreamState `json:"state"`
	RequestedAt string      `json:"requestedAt"`
}

type CallHalfCloseResponse struct {
	Ok    bool                 `json:"ok"`
	Data  *CallHalfCloseResult `json:"data,omitempty"`
	Error *ErrorEnvelope       `json:"error,omitempty"`
}

type CallCancelResult struct {
	CallID      string      `json:"callId"`
	SessionID   string      `json:"sessionId"`
	State       StreamState `json:"state"`
	RequestedAt string      `json:"requestedAt"`
}

type CallCancelResponse struct {
	Ok    bool              `json:"ok"`
	Data  *CallCancelResult `json:"data,omitempty"`
	Error *ErrorEnvelope    `json:"error,omitempty"`
}

type HistoryListInput struct {
	Limit int `json:"limit,omitempty"`
}

type HistoryCallSummary struct {
	CallID         string        `json:"callId"`
	SessionID      string        `json:"sessionId,omitempty"`
	WorkspaceID    string        `json:"workspaceId,omitempty"`
	Method         string        `json:"method"`
	RPCType        RPCType       `json:"rpcType"`
	EndpointID     string        `json:"endpointId"`
	State          StreamState   `json:"state"`
	GRPCStatusCode string        `json:"grpcStatusCode,omitempty"`
	StartedAt      string        `json:"startedAt"`
	FinishedAt     string        `json:"finishedAt,omitempty"`
	DurationMs     int64         `json:"durationMs,omitempty"`
	RequestCount   int           `json:"requestCount"`
	ResponseCount  int           `json:"responseCount"`
	Truncated      bool          `json:"truncated"`
	ErrorCategory  ErrorCategory `json:"errorCategory,omitempty"`
	ErrorCode      string        `json:"errorCode,omitempty"`
	SummaryPath    string        `json:"summaryPath,omitempty"`
	SessionLogPath string        `json:"sessionLogPath,omitempty"`
}

type HistoryListResult struct {
	Calls []HistoryCallSummary `json:"calls"`
}

type HistoryListResponse struct {
	Ok    bool               `json:"ok"`
	Data  *HistoryListResult `json:"data,omitempty"`
	Error *ErrorEnvelope     `json:"error,omitempty"`
}

type HistoryLogPreview struct {
	JSON any `json:"json,omitempty"`
}

type HistoryLogGRPC struct {
	Method     string              `json:"method,omitempty"`
	RPCType    RPCType             `json:"rpcType,omitempty"`
	StatusCode string              `json:"statusCode,omitempty"`
	Metadata   map[string][]string `json:"metadata,omitempty"`
}

type HistoryLogEvent struct {
	CallID       string             `json:"callId"`
	SessionID    string             `json:"sessionId,omitempty"`
	Sequence     int64              `json:"seq"`
	Kind         string             `json:"kind"`
	Direction    string             `json:"direction,omitempty"`
	MessageIndex int                `json:"messageIndex,omitempty"`
	SizeBytes    int64              `json:"sizeBytes,omitempty"`
	Preview      *HistoryLogPreview `json:"preview,omitempty"`
	GRPC         *HistoryLogGRPC    `json:"grpc,omitempty"`
	Details      map[string]string  `json:"details,omitempty"`
	Timestamp    string             `json:"ts"`
}

type HistoryGetResult struct {
	Summary      HistoryCallSummary  `json:"summary"`
	RequestBody  any                 `json:"requestBody"`
	ResponseBody any                 `json:"responseBody,omitempty"`
	Headers      map[string][]string `json:"headers,omitempty"`
	Trailers     map[string][]string `json:"trailers,omitempty"`
	Status       StreamStatus        `json:"status"`
	Events       []HistoryLogEvent   `json:"events"`
}

type HistoryGetResponse struct {
	Ok    bool              `json:"ok"`
	Data  *HistoryGetResult `json:"data,omitempty"`
	Error *ErrorEnvelope    `json:"error,omitempty"`
}
