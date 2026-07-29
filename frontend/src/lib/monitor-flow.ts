import type {
  CatalogService,
  CatalogSourceKind,
  EndpointPreset,
  EndpointTLSSettings,
  EndpointTestResult,
  JsonValue,
  ProtoCatalogResult,
  ProtoSource,
  ReflectionCatalogResult,
  TLSMode,
  WorkspaceSaveInput,
  WorkspaceSnapshot,
} from './contracts'

export type MonitorConnectionStatus = 'online' | 'warning' | 'offline'

export interface MonitorConnectionDraft {
  id?: string
  name: string
  endpoint: string
  tlsEnabled?: boolean
  tlsMode?: TLSMode
  authority?: string
  serverNameOverride?: string
  caCertRef?: string
  clientCertRef?: string
  clientKeyRef?: string
  connectTimeoutMs?: number
  requestTimeoutMs?: number
  streamIdleTimeoutMs?: number
  metadataDefaults?: Record<string, string>
  catalogSource: CatalogSourceKind
  protoSources: ProtoSource[]
  importPaths?: string[]
}

export interface MonitorConnection {
  id: string
  name: string
  endpoint: string
  status: MonitorConnectionStatus
  streams: number
  updatedAt: string
  catalogSource: CatalogSourceKind
  services: CatalogService[]
  selectedMethodFullName?: string
  endpointCheck?: EndpointTestResult
  catalogLoadedAt?: string
  catalogDurationMs?: number
  requestTemplates?: Record<string, JsonValue>
  protoSources?: ProtoSource[]
  importPaths?: string[]
}

const DEFAULT_CONNECT_TIMEOUT_MS = 5000
const DEFAULT_REQUEST_TIMEOUT_MS = 30000
const DEFAULT_STREAM_IDLE_TIMEOUT_MS = 120000

export function buildEndpointPreset(draft: MonitorConnectionDraft): EndpointPreset {
  const target = draft.endpoint.trim()
  const name = draft.name.trim()
  const tlsMode = draft.tlsMode ?? (draft.tlsEnabled ? 'system_ca' : 'plaintext')
  const tls: EndpointTLSSettings = { mode: tlsMode }

  if (tlsMode !== 'plaintext' && draft.serverNameOverride?.trim()) {
    tls.serverNameOverride = draft.serverNameOverride.trim()
  }
  if ((tlsMode === 'custom_ca' || tlsMode === 'mtls') && draft.caCertRef?.trim()) {
    tls.caCert = draft.caCertRef.trim()
  }
  if (tlsMode === 'mtls') {
    if (draft.clientCertRef?.trim()) {
      tls.clientCert = draft.clientCertRef.trim()
    }
    if (draft.clientKeyRef?.trim()) {
      tls.clientKey = draft.clientKeyRef.trim()
    }
  }

  return {
    id: draft.id?.trim() || undefined,
    name: name || target,
    target,
    authority: draft.authority?.trim() || undefined,
    tls,
    connectTimeoutMs: draft.connectTimeoutMs ?? DEFAULT_CONNECT_TIMEOUT_MS,
    requestTimeoutMs: draft.requestTimeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS,
    streamIdleTimeoutMs: draft.streamIdleTimeoutMs ?? DEFAULT_STREAM_IDLE_TIMEOUT_MS,
    metadataDefaults: compactStringMap(draft.metadataDefaults),
  }
}

export function endpointStatusFromTest(result: EndpointTestResult): MonitorConnectionStatus {
  if (!result.transportReachable) {
    return 'offline'
  }

  if (!result.grpcReady || !result.grpcReadyProven || (result.tlsConfigured && !result.tlsOk)) {
    return 'warning'
  }

  return 'online'
}

export function endpointReadinessError(result: EndpointTestResult): string | null {
  if (endpointStatusFromTest(result) === 'online') {
    return null
  }

  const failedCheck = result.checks.find((check) => check.outcome === 'failed')
  const notProvenCheck = result.checks.find((check) => check.outcome === 'not_proven')
  const diagnosticMessage = result.diagnostic?.message
  const checkMessage = failedCheck?.message ?? notProvenCheck?.message

  return checkMessage ?? diagnosticMessage ?? 'Endpoint is not ready for catalog loading.'
}

export function protoSourceLabel(source: ProtoSource): string {
  const pathParts = source.path.split(/[\\/]/).filter(Boolean)
  const basename = pathParts[pathParts.length - 1]

  return basename ? `${basename} (${source.type})` : source.path
}

export function firstCatalogMethodFullName(services: CatalogService[]): string | undefined {
  for (const service of services) {
    const firstMethod = service.methods[0]
    if (firstMethod) {
      return firstMethod.fullName
    }
  }

  return undefined
}

export function createMonitorConnection(
  draft: MonitorConnectionDraft,
  endpointCheck: EndpointTestResult,
  catalog: ReflectionCatalogResult | ProtoCatalogResult,
  now: Date = new Date(),
): MonitorConnection {
  const services = catalog.services ?? []

  return {
    id: catalog.endpoint.id ?? endpointCheck.endpoint.id ?? catalog.endpoint.target,
    name: catalog.endpoint.name || draft.name.trim() || catalog.endpoint.target,
    endpoint: catalog.endpoint.target,
    status: services.length > 0 ? endpointStatusFromTest(endpointCheck) : 'warning',
    streams: 0,
    updatedAt: now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    catalogSource: draft.catalogSource,
    services,
    selectedMethodFullName: firstCatalogMethodFullName(services),
    endpointCheck,
    catalogLoadedAt: catalog.loadedAt,
    catalogDurationMs: catalog.durationMs,
    requestTemplates: catalog.requestTemplates,
    protoSources: draft.protoSources,
    importPaths: draft.importPaths,
  }
}

export function monitorConnectionToEndpointPreset(connection: MonitorConnection): EndpointPreset {
  if (connection.endpointCheck?.endpoint) {
    return connection.endpointCheck.endpoint
  }

  return {
    id: connection.id,
    name: connection.name,
    target: connection.endpoint,
    tls: { mode: 'plaintext' },
    connectTimeoutMs: DEFAULT_CONNECT_TIMEOUT_MS,
    requestTimeoutMs: DEFAULT_REQUEST_TIMEOUT_MS,
    streamIdleTimeoutMs: DEFAULT_STREAM_IDLE_TIMEOUT_MS,
  }
}

export function workspaceDraftFromConnections(input: {
  name?: string
  connections: MonitorConnection[]
  settings?: WorkspaceSnapshot['settings']
}): WorkspaceSaveInput {
  const protoSources = uniqueProtoSources(input.connections.flatMap((connection) => connection.protoSources ?? []))
  const importPaths = uniqueStrings(input.connections.flatMap((connection) => connection.importPaths ?? []))

  return {
    name: input.name?.trim() || undefined,
    endpoints: input.connections.map(monitorConnectionToEndpointPreset),
    protoSources,
    importPaths,
    settings: input.settings,
  }
}

export function workspaceEndpointDraft(
  endpoint: EndpointPreset,
  workspace: Pick<WorkspaceSnapshot, 'protoSources' | 'importPaths'>,
): MonitorConnectionDraft {
  return {
    id: endpoint.id,
    name: endpoint.name ?? endpoint.target,
    endpoint: endpoint.target,
    authority: endpoint.authority,
    tlsEnabled: endpoint.tls.mode !== 'plaintext',
    tlsMode: endpoint.tls.mode,
    serverNameOverride: endpoint.tls.serverNameOverride,
    caCertRef: endpoint.tls.caCert,
    clientCertRef: endpoint.tls.clientCert,
    clientKeyRef: endpoint.tls.clientKey,
    connectTimeoutMs: endpoint.connectTimeoutMs,
    requestTimeoutMs: endpoint.requestTimeoutMs,
    streamIdleTimeoutMs: endpoint.streamIdleTimeoutMs,
    metadataDefaults: endpoint.metadataDefaults,
    catalogSource: workspace.protoSources.length > 0 ? 'proto' : 'reflection',
    protoSources: workspace.protoSources,
    importPaths: workspace.importPaths,
  }
}

export function monitorConnectionDraft(connection: MonitorConnection): MonitorConnectionDraft {
  const endpoint = monitorConnectionToEndpointPreset(connection)

  return {
    ...workspaceEndpointDraft(endpoint, {
      protoSources: connection.protoSources ?? [],
      importPaths: connection.importPaths ?? [],
    }),
    catalogSource: connection.catalogSource,
  }
}

export function createWorkspaceFallbackConnection(input: {
  endpoint: EndpointPreset
  workspace: Pick<WorkspaceSnapshot, 'protoSources' | 'importPaths'>
  status: MonitorConnectionStatus
  endpointCheck?: EndpointTestResult
  now?: Date
}): MonitorConnection {
  const draft = workspaceEndpointDraft(input.endpoint, input.workspace)

  return {
    id: input.endpoint.id ?? input.endpoint.target,
    name: input.endpoint.name || input.endpoint.target,
    endpoint: input.endpoint.target,
    status: input.status,
    streams: 0,
    updatedAt: (input.now ?? new Date()).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    catalogSource: draft.catalogSource,
    services: [],
    endpointCheck: input.endpointCheck,
    protoSources: draft.protoSources,
    importPaths: draft.importPaths,
  }
}

function uniqueStrings(values: string[]): string[] {
  return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)))
}

function uniqueProtoSources(values: ProtoSource[]): ProtoSource[] {
  const seen = new Set<string>()
  const result: ProtoSource[] = []

  for (const source of values) {
    const path = source.path.trim()
    if (!path) {
      continue
    }

    const key = `${source.type}:${path}`
    if (seen.has(key)) {
      continue
    }

    seen.add(key)
    result.push({ type: source.type, path })
  }

  return result
}

function compactStringMap(values: Record<string, string> | undefined): Record<string, string> | undefined {
  if (!values) {
    return undefined
  }

  const result = Object.fromEntries(
    Object.entries(values)
      .map(([key, value]) => [key.trim(), value] as const)
      .filter(([key, value]) => key.length > 0 && value.trim().length > 0),
  )

  return Object.keys(result).length > 0 ? result : undefined
}
