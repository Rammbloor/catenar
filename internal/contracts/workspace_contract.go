package contracts

type WorkspaceCreateInput struct {
	Path         string           `json:"path"`
	Name         string           `json:"name,omitempty"`
	Endpoints    []EndpointPreset `json:"endpoints,omitempty"`
	ProtoSources []ProtoSource    `json:"protoSources,omitempty"`
	ImportPaths  []string         `json:"importPaths,omitempty"`
}

type WorkspaceSaveInput struct {
	Name         string           `json:"name,omitempty"`
	Endpoints    []EndpointPreset `json:"endpoints,omitempty"`
	ProtoSources []ProtoSource    `json:"protoSources,omitempty"`
	ImportPaths  []string         `json:"importPaths,omitempty"`
}

type WorkspaceValidateInput struct {
	Name         string           `json:"name,omitempty"`
	Endpoints    []EndpointPreset `json:"endpoints,omitempty"`
	ProtoSources []ProtoSource    `json:"protoSources,omitempty"`
	ImportPaths  []string         `json:"importPaths,omitempty"`
}

type WorkspaceValidationIssue struct {
	Field    string        `json:"field"`
	Code     string        `json:"code"`
	Category ErrorCategory `json:"category"`
	Message  string        `json:"message"`
	Path     string        `json:"path,omitempty"`
}

type WorkspaceSavedRequestSummary struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	Method      string  `json:"method"`
	RPCType     RPCType `json:"rpcType"`
	EndpointRef string  `json:"endpointRef"`
}

type WorkspaceSnapshot struct {
	ID            string                         `json:"id"`
	Version       int                            `json:"version"`
	Name          string                         `json:"name"`
	Path          string                         `json:"path"`
	ManifestPath  string                         `json:"manifestPath"`
	Endpoints     []EndpointPreset               `json:"endpoints"`
	ProtoSources  []ProtoSource                  `json:"protoSources"`
	ImportPaths   []string                       `json:"importPaths,omitempty"`
	SavedRequests []WorkspaceSavedRequestSummary `json:"savedRequests,omitempty"`
	BackupPaths   []string                       `json:"backupPaths,omitempty"`
}

type WorkspaceResult struct {
	Workspace WorkspaceSnapshot `json:"workspace"`
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

type WorkspaceValidateResponse struct {
	Ok    bool                     `json:"ok"`
	Data  *WorkspaceValidateResult `json:"data,omitempty"`
	Error *ErrorEnvelope           `json:"error,omitempty"`
}

type SavedRequestSpec struct {
	Mode     string          `json:"mode"`
	Body     any             `json:"body,omitempty"`
	Messages []StreamMessage `json:"messages,omitempty"`
}

type RequestSaveInput struct {
	ID               string            `json:"id"`
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
