export const CONTRACT_VERSION = 'v1' as const

export const BOUND_METHODS = [
  'WorkspaceCreate',
  'WorkspaceOpen',
  'WorkspaceSave',
  'WorkspaceValidate',
  'EndpointTest',
  'CatalogLoadFromReflection',
  'CatalogLoadFromProtoSources',
  'RequestSave',
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

export const TOP_LEVEL_VIEWS = ['home', 'workspace', 'session'] as const

export const APP_OVERLAYS = [
  'history-overlay',
  'settings-overlay',
  'diagnostics-overlay',
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
export const TLS_MODES = ['plaintext', 'system_ca', 'custom_ca', 'mtls'] as const
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
export type AppView = (typeof TOP_LEVEL_VIEWS)[number]
export type AppOverlay = (typeof APP_OVERLAYS)[number]
export type StreamState = (typeof STREAM_STATES)[number]
export type TerminalStreamState = (typeof TERMINAL_STREAM_STATES)[number]
export type SessionCondition = (typeof SESSION_CONDITIONS)[number]
export type RPCType = (typeof RPC_TYPES)[number]
export type TLSMode = (typeof TLS_MODES)[number]
export type EndpointCheckStage = (typeof ENDPOINT_CHECK_STAGES)[number]
export type EndpointCheckOutcome = (typeof ENDPOINT_CHECK_OUTCOMES)[number]

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
  topLevelViews: string[]
  overlays: string[]
  streamStates: string[]
  terminalStreamStates: string[]
  sessionConditions: string[]
  transitions: TransitionRule[]
  modules: ModuleContract[]
}

export interface AppMetadata {
  name: string
  productLine: string
  platform: string
  architecture: string
  goVersion: string
  wailsVersion: string
}

export interface LayoutRegion {
  id: string
  title: string
  purpose: string
}

export interface LayoutDefinition {
  regions: LayoutRegion[]
}

export interface StateModelSummary {
  primaryFlow: string[]
  overlayViews: string[]
  singleActiveLiveSession: boolean
}

export interface SliceStatus {
  slice: string
  status: string
  summary: string
}

export interface BootstrapData {
  app: AppMetadata
  contract: ContractManifest
  layout: LayoutDefinition
  stateModel: StateModelSummary
  epicZero: SliceStatus[]
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

export interface CatalogLoadFromReflectionInput {
  endpoint: EndpointPreset
}

export interface CatalogMessageRef {
  name: string
  fullName: string
  isWellKnown: boolean
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
  diagnostic?: DiagnosticsUpdateEvent
  loadedAt: string
  durationMs: number
}

export interface CatalogLoadFromReflectionResponse {
  ok: boolean
  data?: ReflectionCatalogResult
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
  topLevelViews: [...TOP_LEVEL_VIEWS],
  overlays: [...APP_OVERLAYS],
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
    ...compareArray('topLevelViews', [...TOP_LEVEL_VIEWS], manifest.topLevelViews),
    ...compareArray('overlays', [...APP_OVERLAYS], manifest.overlays),
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
