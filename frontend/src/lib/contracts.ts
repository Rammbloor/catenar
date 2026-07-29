export const CONTRACT_VERSION = 'v1' as const

export const BOUND_METHODS = [
  'WorkspaceCreate',
  'WorkspaceOpen',
  'WorkspaceClose',
  'WorkspaceSave',
  'WorkspaceValidate',
  'GitHubWorkspaceLink',
	'GitHubWorkspaceSetToken',
  'GitHubWorkspaceStatus',
  'GitHubWorkspacePull',
  'GitHubWorkspacePush',
  'GitHubWorkspaceUnlink',
  'DialogSelectWorkspaceCreatePath',
  'DialogSelectWorkspaceOpenPath',
  'UpdateCheck',
  'EndpointTest',
  'MaterialRegisterFile',
  'CatalogLoadFromReflection',
  'CatalogLoadFromProtoSources',
  'DialogSelectProtoFiles',
  'DialogSelectProtoDirectory',
  'DialogSelectMaterialFile',
  'RequestSave',
  'RequestGet',
  'RequestDelete',
  'CallInvokeUnary',
  'CallStartStream',
  'CallSendMessage',
  'CallHalfClose',
  'CallCancel',
  'HistoryList',
  'HistoryGet',
  'DiagnosticsExport',
] as const

export const EVENT_NAMES = [
  'stream:state',
  'stream:event',
  'stream:error',
  'stream:completed',
  'diagnostics:update',
] as const

export const ERROR_CATEGORIES = [
  'transport',
  'grpc_status',
  'reflection',
  'proto',
  'validation',
  'workspace',
  'application',
  'cancelled',
] as const

export const STREAM_STATES = [
  'idle',
  'connecting',
  'open',
  'half_closed_local',
  'half_closed_remote',
  'closed',
  'cancelled',
  'error',
] as const

export const TERMINAL_STREAM_STATES = ['closed', 'cancelled', 'error'] as const

export const SESSION_CONDITIONS = ['truncated'] as const
export const RPC_TYPES = ['unary', 'server_stream', 'client_stream', 'bidi_stream'] as const
export const CATALOG_SOURCES = ['reflection', 'proto'] as const
export const REQUEST_MODES = ['static-sequence', 'interactive'] as const
export const SAVED_REQUEST_MODES = ['single', 'static-sequence'] as const
export const PROTO_SOURCE_TYPES = ['directory', 'file'] as const
export const TLS_MODES = ['plaintext', 'system_ca', 'custom_ca', 'mtls'] as const
export const MATERIAL_KINDS = ['tls_ca', 'tls_client_cert', 'tls_client_key', 'metadata'] as const
export const ENDPOINT_CHECK_STAGES = [
  'target_resolution',
  'tcp_connect',
  'tls_handshake',
  'grpc_readiness',
] as const
export const ENDPOINT_CHECK_OUTCOMES = ['passed', 'failed', 'skipped', 'not_proven'] as const

export type BoundMethod = (typeof BOUND_METHODS)[number]
export type EventName = (typeof EVENT_NAMES)[number]
export type ErrorCategory = (typeof ERROR_CATEGORIES)[number]
export type StreamState = (typeof STREAM_STATES)[number]
export type TerminalStreamState = (typeof TERMINAL_STREAM_STATES)[number]
export type SessionCondition = (typeof SESSION_CONDITIONS)[number]
export type RPCType = (typeof RPC_TYPES)[number]
export type CatalogSourceKind = (typeof CATALOG_SOURCES)[number]
export type RequestMode = (typeof REQUEST_MODES)[number]
export type SavedRequestMode = (typeof SAVED_REQUEST_MODES)[number]
export type ProtoSourceType = (typeof PROTO_SOURCE_TYPES)[number]
export type TLSMode = (typeof TLS_MODES)[number]
export type MaterialKind = (typeof MATERIAL_KINDS)[number]
export type EndpointCheckStage = (typeof ENDPOINT_CHECK_STAGES)[number]
export type EndpointCheckOutcome = (typeof ENDPOINT_CHECK_OUTCOMES)[number]
export type JsonPrimitive = string | number | boolean | null
export type JsonValue = JsonPrimitive | JsonValue[] | { [key: string]: JsonValue }

export interface CallOptions {
  requestTimeoutMs?: number
  streamIdleTimeoutMs?: number
}

export interface StreamMessage {
  body: JsonValue
}

export interface StreamStatus {
  code: string
  message: string
}

export interface StreamStateEvent {
  sessionId: string
  callId: string
  state: StreamState
  previousState?: StreamState
  conditions: SessionCondition[]
  ts: string
}

export interface StreamEventPreview {
  json?: JsonValue
}

export interface StreamEventPayload {
  preview: StreamEventPreview
  sizeBytes?: number
}

export interface StreamEventRecord {
  sessionId: string
  callId: string
  seq: number
  kind: string
  direction: string
  ts: string
  payload: StreamEventPayload
}

export interface StreamErrorEvent {
  sessionId: string
  callId: string
  error: ErrorEnvelope
  ts: string
}

export interface StreamCompletedEvent {
  sessionId: string
  callId: string
  finalState: StreamState
  conditions: SessionCondition[]
  status: StreamStatus
  ts: string
}

export interface StreamRequestSpec {
  mode: RequestMode
  messages?: StreamMessage[]
}

export interface TransitionRule {
  from: StreamState
  event: string
  to: StreamState[]
  notes?: string
}

export interface ModuleContract {
  name: string
  responsibility: string
}

export interface ContractManifest {
  version: string
  boundMethods: string[]
  eventNames: string[]
  errorCategories: string[]
  streamStates: string[]
  terminalStreamStates: string[]
  sessionConditions: string[]
  transitions: TransitionRule[]
  modules: ModuleContract[]
}

export interface AppMetadata {
  name: string
  version: string
  productLine: string
  platform: string
  architecture: string
  goVersion: string
  wailsVersion: string
}

export interface BootstrapData {
  app: AppMetadata
  contract: ContractManifest
  workspace?: WorkspaceSnapshot
}

export interface ErrorEnvelope {
  code: string
  category: ErrorCategory
  message: string
  details?: Record<string, string>
}

export interface BootstrapResponse {
  ok: boolean
  data?: BootstrapData
  error?: ErrorEnvelope
}

export interface ProbeAcknowledgement {
  eventId: string
  eventName: string
  emittedAt: string
  classification: string
}

export interface ProbeResponse {
  ok: boolean
  data?: ProbeAcknowledgement
  error?: ErrorEnvelope
}

export interface UpdateCheckResult {
  currentVersion: string
  latestVersion: string
  updateAvailable: boolean
  releaseUrl?: string
  downloadUrl?: string
  downloadName?: string
  publishedAt?: string
}

export interface UpdateCheckResponse {
  ok: boolean
  data?: UpdateCheckResult
  error?: ErrorEnvelope
}

export interface EventProbeState {
  pending: boolean
  lastAcknowledgement?: ProbeAcknowledgement
  error?: string
}

export interface DiagnosticsUpdateEvent {
  id: string
  source: string
  level: string
  code: string
  category: ErrorCategory
  message: string
  nextStep?: string
  details?: Record<string, string>
  ts: string
}

export interface EndpointTLSSettings {
  mode: TLSMode
  serverNameOverride?: string
  caCert?: string
  clientCert?: string
  clientKey?: string
}

export interface EndpointPreset {
  id?: string
  name?: string
  target: string
  authority?: string
  tls: EndpointTLSSettings
  connectTimeoutMs?: number
  requestTimeoutMs?: number
  streamIdleTimeoutMs?: number
  metadataDefaults?: Record<string, string>
}

export interface WorkspaceEnvironment {
  values?: Record<string, string>
}

export interface WorkspaceEventRetentionSettings {
  maxEventsPerCall?: number
  maxBytesPerCall?: number
}

export interface WorkspaceSettings {
  redactDefaults: boolean
  customSecretKeys?: string[]
  eventRetention?: WorkspaceEventRetentionSettings
}

export interface WorkspaceCreateInput {
  path: string
  name?: string
  endpoints?: EndpointPreset[]
  protoSources?: ProtoSource[]
  importPaths?: string[]
  environments?: Record<string, WorkspaceEnvironment>
  settings?: WorkspaceSettings
}

export interface WorkspaceSaveInput {
  name?: string
  endpoints?: EndpointPreset[]
  protoSources?: ProtoSource[]
  importPaths?: string[]
  environments?: Record<string, WorkspaceEnvironment>
  settings?: WorkspaceSettings
}

export interface WorkspaceValidateInput {
  name?: string
  endpoints?: EndpointPreset[]
  protoSources?: ProtoSource[]
  importPaths?: string[]
  environments?: Record<string, WorkspaceEnvironment>
  settings?: WorkspaceSettings
}

export interface WorkspaceValidationIssue {
  field: string
  code: string
  category: ErrorCategory
  message: string
  path?: string
}

export interface WorkspaceSavedRequestSummary {
  id: string
  name: string
  path: string
  method: string
  rpcType: RPCType
  endpointRef: string
  environmentRef?: string
}

export interface WorkspaceSnapshot {
  id: string
  version: number
  name: string
  path: string
  manifestPath: string
  endpoints: EndpointPreset[]
  protoSources: ProtoSource[]
  importPaths?: string[]
  environments?: Record<string, WorkspaceEnvironment>
  settings?: WorkspaceSettings
  savedRequests?: WorkspaceSavedRequestSummary[]
  backupPaths?: string[]
}

export interface WorkspaceResult {
  workspace: WorkspaceSnapshot
  issues?: WorkspaceValidationIssue[]
}

export interface WorkspaceValidateResult {
  workspace?: WorkspaceSnapshot
  issues: WorkspaceValidationIssue[]
}

export interface WorkspaceResponse {
  ok: boolean
  data?: WorkspaceResult
  error?: ErrorEnvelope
}

export interface WorkspaceActiveResponse {
  ok: boolean
  data?: WorkspaceSnapshot
  error?: ErrorEnvelope
}

export interface WorkspaceValidateResponse {
  ok: boolean
  data?: WorkspaceValidateResult
  error?: ErrorEnvelope
}

export interface WorkspaceCloseResponse {
  ok: boolean
  error?: ErrorEnvelope
}

export interface GitHubWorkspaceLinkInput {
  repositoryUrl: string
  branch: string
  workspacePath: string
  accessToken?: string
}

export interface GitHubWorkspaceCredentialInput {
  accessToken: string
}

export interface GitHubWorkspaceLink {
  repositoryUrl: string
  branch: string
  workspacePath: string
  lastSyncedCommit?: string
  lastSyncedAt?: string
}

export interface GitHubSyncActionInput {
  overwrite: boolean
}

export interface GitHubSyncStatus {
  linked: boolean
  localChanges: boolean
  remoteChanges: boolean
  conflict: boolean
  initialSyncRequired: boolean
  remoteCommit?: string
  tokenConfigured: boolean
  link?: GitHubWorkspaceLink
  workspace?: WorkspaceSnapshot
}

export interface GitHubSyncResponse {
  ok: boolean
  data?: GitHubSyncStatus
  error?: {
    code?: string
    category?: ErrorCategory
    message: string
    details?: Record<string, string>
  }
}

export interface SavedRequestSpec {
  mode: SavedRequestMode
  body?: JsonValue
  messages?: StreamMessage[]
}

export interface RequestSaveInput {
  id: string
  name?: string
  method: string
  rpcType: RPCType
  endpointRef: string
  environmentRef?: string
  metadataTemplate?: Record<string, string>
  callOptions?: CallOptions
  requestSpec: SavedRequestSpec
}

export interface RequestSaveResult {
  workspace: WorkspaceSnapshot
  savedRequest: WorkspaceSavedRequestSummary
}

export interface RequestSaveResponse {
  ok: boolean
  data?: RequestSaveResult
  error?: ErrorEnvelope
}

export interface RequestGetInput {
  id?: string
  path?: string
}

export interface RequestDeleteInput {
  id?: string
  path?: string
}

export interface WorkspaceSavedRequest {
  id: string
  name: string
  path: string
  method: string
  rpcType: RPCType
  endpointRef: string
  environmentRef?: string
  metadataTemplate?: Record<string, string>
  callOptions?: CallOptions
  requestSpec: SavedRequestSpec
}

export interface RequestGetResult {
  workspace: WorkspaceSnapshot
  savedRequest: WorkspaceSavedRequest
}

export interface RequestGetResponse {
  ok: boolean
  data?: RequestGetResult
  error?: ErrorEnvelope
}

export interface RequestDeleteResult {
  workspace: WorkspaceSnapshot
  deletedId: string
}

export interface RequestDeleteResponse {
  ok: boolean
  data?: RequestDeleteResult
  error?: ErrorEnvelope
}

export interface EndpointValidationIssue {
  field: string
  code: string
  message: string
}

export interface EndpointTestInput {
  endpoint: EndpointPreset
}

export interface EndpointCheck {
  stage: EndpointCheckStage
  outcome: EndpointCheckOutcome
  message: string
  details?: Record<string, string>
}

export interface EndpointTestResult {
  endpoint: EndpointPreset
  transportReachable: boolean
  tlsConfigured: boolean
  tlsOk: boolean
  grpcReady: boolean
  grpcReadyProven: boolean
  checks: EndpointCheck[]
  diagnostic?: DiagnosticsUpdateEvent
  testedAt: string
  durationMs: number
}

export interface EndpointTestResponse {
  ok: boolean
  data?: EndpointTestResult
  error?: ErrorEnvelope
}

export interface MaterialRegisterFileInput {
  namespace: string
  key: string
  path: string
  kind: MaterialKind
}

export interface MaterialFileRecord {
  backend: string
  namespace: string
  key: string
  path: string
  kind: MaterialKind | string
  createdAt?: string
  updatedAt?: string
}

export interface MaterialRegisterFileResult {
  ref: string
  record: MaterialFileRecord
}

export interface MaterialRegisterFileResponse {
  ok: boolean
  data?: MaterialRegisterFileResult
  error?: ErrorEnvelope
}

export interface CatalogLoadFromReflectionInput {
  endpoint: EndpointPreset
}

export interface ProtoSource {
  type: ProtoSourceType
  path: string
}

export interface CatalogLoadFromProtoSourcesInput {
  endpoint: EndpointPreset
  protoSources: ProtoSource[]
  importPaths?: string[]
}

export interface CatalogMessageRef {
  name: string
  fullName: string
  isWellKnown: boolean
  fields?: CatalogField[]
}

export interface CatalogField {
  name: string
  jsonName: string
  type: string
  repeated?: boolean
  required?: boolean
  oneof?: string
  fields?: CatalogField[]
}

export interface CatalogMethod {
  name: string
  fullName: string
  rpcType: RPCType
  requestType: CatalogMessageRef
  responseType: CatalogMessageRef
}

export interface CatalogService {
  name: string
  fullName: string
  methods: CatalogMethod[]
}

export interface ReflectionCatalogResult {
  endpoint: EndpointPreset
  services: CatalogService[]
  wellKnownTypes?: CatalogMessageRef[]
  requestTemplates?: Record<string, JsonValue>
  diagnostic?: DiagnosticsUpdateEvent
  loadedAt: string
  durationMs: number
}

export interface CatalogLoadFromReflectionResponse {
  ok: boolean
  data?: ReflectionCatalogResult
  error?: ErrorEnvelope
}

export interface ProtoCatalogResult {
  endpoint: EndpointPreset
  protoSources: ProtoSource[]
  importPaths?: string[]
  services: CatalogService[]
  wellKnownTypes?: CatalogMessageRef[]
  requestTemplates?: Record<string, JsonValue>
  diagnostic?: DiagnosticsUpdateEvent
  loadedAt: string
  durationMs: number
}

export interface CatalogLoadFromProtoSourcesResponse {
  ok: boolean
  data?: ProtoCatalogResult
  error?: ErrorEnvelope
}

export interface CallInvokeUnaryInput {
  catalogSource?: CatalogSourceKind
  endpointId: string
  method: string
  environmentRef?: string
  metadata?: Record<string, string>
  body: JsonValue
  callOptions?: CallOptions
}

export interface CallInvokeUnaryResult {
  callId: string
  sessionId: string
  endpointId: string
  method: string
  rpcType: RPCType
  finalState: StreamState
  requestBody: JsonValue
  responseBody?: JsonValue
  headers?: Record<string, string[]>
  trailers?: Record<string, string[]>
  status: StreamStatus
  diagnostic?: DiagnosticsUpdateEvent
  startedAt: string
  finishedAt: string
  durationMs: number
}

export interface CallInvokeUnaryResponse {
  ok: boolean
  data?: CallInvokeUnaryResult
  error?: ErrorEnvelope
}

export interface CallStartStreamInput {
  catalogSource?: CatalogSourceKind
  endpointId: string
  method: string
  rpcType: RPCType
  environmentRef?: string
  metadata?: Record<string, string>
  requestSpec?: StreamRequestSpec
  callOptions?: CallOptions
}

export interface CallStartStreamResult {
  callId: string
  sessionId: string
  endpointId: string
  method: string
  rpcType: RPCType
  state: StreamState
  startedAt: string
}

export interface CallStartStreamResponse {
  ok: boolean
  data?: CallStartStreamResult
  error?: ErrorEnvelope
}

export interface CallSendMessageInput {
  sessionId: string
  message: StreamMessage
}

export interface CallSendMessageResult {
  callId: string
  sessionId: string
  state: StreamState
  messageIndex: number
  seq: number
  sentAt: string
}

export interface CallSendMessageResponse {
  ok: boolean
  data?: CallSendMessageResult
  error?: ErrorEnvelope
}

export interface CallHalfCloseInput {
  sessionId: string
}

export interface CallHalfCloseResult {
  callId: string
  sessionId: string
  state: StreamState
  requestedAt: string
}

export interface CallHalfCloseResponse {
  ok: boolean
  data?: CallHalfCloseResult
  error?: ErrorEnvelope
}

export interface CallCancelInput {
  sessionId: string
}

export interface CallCancelResult {
  callId: string
  sessionId: string
  state: StreamState
  requestedAt: string
}

export interface CallCancelResponse {
  ok: boolean
  data?: CallCancelResult
  error?: ErrorEnvelope
}

export interface HistoryListInput {
  endpointId?: string
  workspaceId?: string
  environmentRef?: string
  limit?: number
}

export interface HistoryCallSummary {
  callId: string
  sessionId?: string
  workspaceId?: string
  environmentRef?: string
  method: string
  rpcType: RPCType
  endpointId: string
  state: StreamState
  grpcStatusCode?: string
  startedAt: string
  finishedAt?: string
  durationMs?: number
  requestCount: number
  responseCount: number
  truncated: boolean
  errorCategory?: ErrorCategory
  errorCode?: string
  summaryPath?: string
  sessionLogPath?: string
}

export interface HistoryListResult {
  calls: HistoryCallSummary[]
}

export interface HistoryListResponse {
  ok: boolean
  data?: HistoryListResult
  error?: ErrorEnvelope
}

export interface HistoryLogPreview {
  json?: JsonValue
}

export interface HistoryLogGRPC {
  method?: string
  rpcType?: RPCType
  statusCode?: string
  metadata?: Record<string, string[]>
}

export interface HistoryLogEvent {
  callId: string
  sessionId?: string
  seq: number
  kind: string
  direction?: string
  messageIndex?: number
  sizeBytes?: number
  preview?: HistoryLogPreview
  grpc?: HistoryLogGRPC
  details?: Record<string, string>
  ts: string
}

export interface HistoryGetResult {
  summary: HistoryCallSummary
  requestBody: JsonValue
  responseBody?: JsonValue
  headers?: Record<string, string[]>
  trailers?: Record<string, string[]>
  status: StreamStatus
  events: HistoryLogEvent[]
}

export interface HistoryGetResponse {
  ok: boolean
  data?: HistoryGetResult
  error?: ErrorEnvelope
}

export interface DiagnosticsExportInput {
  path?: string
  callIds?: string[]
  includeHistory?: boolean
}

export interface DiagnosticsExportResult {
  path: string
  fileCount: number
  includedCalls?: string[]
  exportedAt: string
}

export interface DiagnosticsExportResponse {
  ok: boolean
  data?: DiagnosticsExportResult
  error?: ErrorEnvelope
}

export const STREAM_TRANSITIONS: TransitionRule[] = [
  { from: 'idle', event: 'start_stream', to: ['connecting'], notes: 'session created' },
  { from: 'connecting', event: 'transport_ready', to: ['open'], notes: 'stream established' },
  { from: 'connecting', event: 'user_cancel', to: ['cancelled'], notes: 'before open' },
  { from: 'connecting', event: 'transport_failure', to: ['error'], notes: 'include diagnostics' },
  { from: 'open', event: 'user_half_close_local_send', to: ['half_closed_local'], notes: 'for client and bidi streams' },
  { from: 'open', event: 'remote_half_close', to: ['half_closed_remote'], notes: 'remote finished sending' },
  { from: 'open', event: 'stream_completed', to: ['closed'], notes: 'normal completion without intermediate half-close' },
  { from: 'open', event: 'user_cancel', to: ['cancelled'], notes: 'terminal' },
  { from: 'open', event: 'transport_failure', to: ['error'], notes: 'terminal' },
  { from: 'half_closed_local', event: 'remote_completes_send_side', to: ['closed', 'half_closed_remote'], notes: 'remote half-close may be observed explicitly' },
  { from: 'half_closed_local', event: 'user_cancel', to: ['cancelled'], notes: 'terminal' },
  { from: 'half_closed_local', event: 'transport_failure', to: ['error'], notes: 'terminal' },
  { from: 'half_closed_remote', event: 'local_finalization', to: ['closed'], notes: 'terminal success' },
  { from: 'half_closed_remote', event: 'user_cancel', to: ['cancelled'], notes: 'terminal' },
  { from: 'half_closed_remote', event: 'transport_failure', to: ['error'], notes: 'terminal' },
] as const

export const FRONTEND_CONTRACT_MANIFEST: ContractManifest = {
  version: CONTRACT_VERSION,
  boundMethods: [...BOUND_METHODS],
  eventNames: [...EVENT_NAMES],
  errorCategories: [...ERROR_CATEGORIES],
  streamStates: [...STREAM_STATES],
  terminalStreamStates: [...TERMINAL_STREAM_STATES],
  sessionConditions: [...SESSION_CONDITIONS],
  transitions: STREAM_TRANSITIONS,
  modules: [
    {
      name: 'workspace-manager',
      responsibility: 'Workspace lifecycle, saved requests, history references and import path ownership.',
    },
    {
      name: 'proto-loader',
      responsibility: 'Reads .proto files, import roots and builds the descriptor graph.',
    },
    {
      name: 'reflection-client',
      responsibility: 'Loads dynamic service catalogs from gRPC Server Reflection.',
    },
    {
      name: 'grpc-runtime',
      responsibility: 'Owns connections, TLS, metadata, RPC invocation and stream lifecycles.',
    },
    {
      name: 'event-log',
      responsibility: 'Appends sent, received, info and error records for each call session.',
    },
    {
      name: 'secret-store',
      responsibility: 'Resolves secret references without persisting raw secret values in workspace files.',
    },
    {
      name: 'transport-adapter',
      responsibility:
        'Keeps the domain model gRPC-aware without leaking transport implementation details into the UI contract.',
    },
  ],
}

function compareArray(name: string, expected: string[], actual: string[]): string[] {
  if (expected.length !== actual.length) {
    return [`${name}: expected ${expected.length} entries, got ${actual.length}`]
  }

  return expected.flatMap((value, index) =>
    actual[index] === value ? [] : [`${name}[${index}]: expected ${value}, got ${actual[index]}`],
  )
}

export function verifyContractManifest(manifest: ContractManifest): string[] {
  return [
    ...(manifest.version === CONTRACT_VERSION ? [] : [`version: expected ${CONTRACT_VERSION}, got ${manifest.version}`]),
    ...compareArray('boundMethods', [...BOUND_METHODS], manifest.boundMethods),
    ...compareArray('eventNames', [...EVENT_NAMES], manifest.eventNames),
    ...compareArray('errorCategories', [...ERROR_CATEGORIES], manifest.errorCategories),
    ...compareArray('streamStates', [...STREAM_STATES], manifest.streamStates),
    ...compareArray('terminalStreamStates', [...TERMINAL_STREAM_STATES], manifest.terminalStreamStates),
    ...compareArray('sessionConditions', [...SESSION_CONDITIONS], manifest.sessionConditions),
    ...compareArray(
      'moduleNames',
      FRONTEND_CONTRACT_MANIFEST.modules.map((module) => module.name),
      manifest.modules.map((module) => module.name),
    ),
  ]
}
