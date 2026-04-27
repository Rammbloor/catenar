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
	Endpoint       EndpointPreset          `json:"endpoint"`
	Services       []CatalogService        `json:"services"`
	WellKnownTypes []CatalogMessageRef     `json:"wellKnownTypes,omitempty"`
	Diagnostic     *DiagnosticsUpdateEvent `json:"diagnostic,omitempty"`
	LoadedAt       string                  `json:"loadedAt"`
	DurationMs     int64                   `json:"durationMs"`
}

type CatalogLoadFromReflectionResponse struct {
	Ok    bool                     `json:"ok"`
	Data  *ReflectionCatalogResult `json:"data,omitempty"`
	Error *ErrorEnvelope           `json:"error,omitempty"`
}
