package contracts

type WorkspaceCreateInput struct {
	Path         string                          `json:"path"`
	Name         string                          `json:"name,omitempty"`
	Endpoints    []EndpointPreset                `json:"endpoints,omitempty"`
	ProtoSources []ProtoSource                   `json:"protoSources,omitempty"`
	ImportPaths  []string                        `json:"importPaths,omitempty"`
	Environments map[string]WorkspaceEnvironment `json:"environments,omitempty"`
	Settings     *WorkspaceSettings              `json:"settings,omitempty"`
}

type WorkspaceSaveInput struct {
	Name         string                          `json:"name,omitempty"`
	Endpoints    []EndpointPreset                `json:"endpoints,omitempty"`
	ProtoSources []ProtoSource                   `json:"protoSources,omitempty"`
	ImportPaths  []string                        `json:"importPaths,omitempty"`
	Environments map[string]WorkspaceEnvironment `json:"environments,omitempty"`
	Settings     *WorkspaceSettings              `json:"settings,omitempty"`
}

type WorkspaceValidateInput struct {
	Name         string                          `json:"name,omitempty"`
	Endpoints    []EndpointPreset                `json:"endpoints,omitempty"`
	ProtoSources []ProtoSource                   `json:"protoSources,omitempty"`
	ImportPaths  []string                        `json:"importPaths,omitempty"`
	Environments map[string]WorkspaceEnvironment `json:"environments,omitempty"`
	Settings     *WorkspaceSettings              `json:"settings,omitempty"`
}

type WorkspaceEnvironment struct {
	Values map[string]string `json:"values,omitempty"`
}

type WorkspaceEventRetentionSettings struct {
	MaxEventsPerCall int `json:"maxEventsPerCall,omitempty"`
	MaxBytesPerCall  int `json:"maxBytesPerCall,omitempty"`
}

type WorkspaceSettings struct {
	RedactDefaults   bool                             `json:"redactDefaults"`
	CustomSecretKeys []string                         `json:"customSecretKeys,omitempty"`
	EventRetention   *WorkspaceEventRetentionSettings `json:"eventRetention,omitempty"`
}

type WorkspaceValidationIssue struct {
	Field    string        `json:"field"`
	Code     string        `json:"code"`
	Category ErrorCategory `json:"category"`
	Message  string        `json:"message"`
	Path     string        `json:"path,omitempty"`
}

type WorkspaceSavedRequestSummary struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	Method         string  `json:"method"`
	RPCType        RPCType `json:"rpcType"`
	EndpointRef    string  `json:"endpointRef"`
	EnvironmentRef string  `json:"environmentRef,omitempty"`
}

type WorkspaceSnapshot struct {
	ID            string                          `json:"id"`
	Version       int                             `json:"version"`
	Name          string                          `json:"name"`
	Path          string                          `json:"path"`
	ManifestPath  string                          `json:"manifestPath"`
	Endpoints     []EndpointPreset                `json:"endpoints"`
	ProtoSources  []ProtoSource                   `json:"protoSources"`
	ImportPaths   []string                        `json:"importPaths,omitempty"`
	Environments  map[string]WorkspaceEnvironment `json:"environments,omitempty"`
	Settings      *WorkspaceSettings              `json:"settings,omitempty"`
	SavedRequests []WorkspaceSavedRequestSummary  `json:"savedRequests,omitempty"`
	BackupPaths   []string                        `json:"backupPaths,omitempty"`
}

type WorkspaceResult struct {
	Workspace WorkspaceSnapshot          `json:"workspace"`
	Issues    []WorkspaceValidationIssue `json:"issues,omitempty"`
}

type WorkspaceValidateResult struct {
	Workspace *WorkspaceSnapshot         `json:"workspace,omitempty"`
	Issues    []WorkspaceValidationIssue `json:"issues"`
}

type WorkspaceResponse struct {
	Ok    bool             `json:"ok"`
	Data  *WorkspaceResult `json:"data,omitempty"`
	Error *ErrorEnvelope   `json:"error,omitempty"`
}

// WorkspaceActiveResponse exposes the runtime's restored workspace, if one is
// open. An empty Data value is a normal "no workspace" state, not an error.
type WorkspaceActiveResponse struct {
	Ok    bool               `json:"ok"`
	Data  *WorkspaceSnapshot `json:"data,omitempty"`
	Error *ErrorEnvelope     `json:"error,omitempty"`
}

type WorkspaceValidateResponse struct {
	Ok    bool                     `json:"ok"`
	Data  *WorkspaceValidateResult `json:"data,omitempty"`
	Error *ErrorEnvelope           `json:"error,omitempty"`
}

type WorkspaceCloseResponse struct {
	Ok    bool           `json:"ok"`
	Error *ErrorEnvelope `json:"error,omitempty"`
}

type SavedRequestSpec struct {
	Mode     string          `json:"mode"`
	Body     any             `json:"body,omitempty"`
	Messages []StreamMessage `json:"messages,omitempty"`
}

type RequestSaveInput struct {
	ID               string            `json:"id"`
	Name             string            `json:"name,omitempty"`
	Method           string            `json:"method"`
	RPCType          RPCType           `json:"rpcType"`
	EndpointRef      string            `json:"endpointRef"`
	EnvironmentRef   string            `json:"environmentRef,omitempty"`
	MetadataTemplate map[string]string `json:"metadataTemplate,omitempty"`
	CallOptions      CallOptions       `json:"callOptions,omitempty"`
	RequestSpec      SavedRequestSpec  `json:"requestSpec"`
}

type RequestSaveResult struct {
	Workspace    WorkspaceSnapshot            `json:"workspace"`
	SavedRequest WorkspaceSavedRequestSummary `json:"savedRequest"`
}

type RequestSaveResponse struct {
	Ok    bool               `json:"ok"`
	Data  *RequestSaveResult `json:"data,omitempty"`
	Error *ErrorEnvelope     `json:"error,omitempty"`
}

type RequestGetInput struct {
	ID   string `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
}

type RequestDeleteInput struct {
	ID   string `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
}

type WorkspaceSavedRequest struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Path             string            `json:"path"`
	Method           string            `json:"method"`
	RPCType          RPCType           `json:"rpcType"`
	EndpointRef      string            `json:"endpointRef"`
	EnvironmentRef   string            `json:"environmentRef,omitempty"`
	MetadataTemplate map[string]string `json:"metadataTemplate,omitempty"`
	CallOptions      CallOptions       `json:"callOptions,omitempty"`
	RequestSpec      SavedRequestSpec  `json:"requestSpec"`
}

type RequestGetResult struct {
	Workspace    WorkspaceSnapshot     `json:"workspace"`
	SavedRequest WorkspaceSavedRequest `json:"savedRequest"`
}

type RequestGetResponse struct {
	Ok    bool              `json:"ok"`
	Data  *RequestGetResult `json:"data,omitempty"`
	Error *ErrorEnvelope    `json:"error,omitempty"`
}

type RequestDeleteResult struct {
	Workspace WorkspaceSnapshot `json:"workspace"`
	DeletedID string            `json:"deletedId"`
}

type RequestDeleteResponse struct {
	Ok    bool                 `json:"ok"`
	Data  *RequestDeleteResult `json:"data,omitempty"`
	Error *ErrorEnvelope       `json:"error,omitempty"`
}
