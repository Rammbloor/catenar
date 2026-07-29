import type { MonitorConnectionDraft } from './monitor-flow'

export const RUNTIME_SESSION_STORAGE_KEY = 'catenar.runtime-session.v1'

export interface RuntimeSession {
  workspaceManifestPath?: string
  selectedConnectionId?: string
  connections: MonitorConnectionDraft[]
}

function storage(): Storage | undefined {
  return typeof window === 'undefined' ? undefined : window.localStorage
}

function text(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value : undefined
}

function draft(value: unknown): MonitorConnectionDraft | null {
  if (!value || typeof value !== 'object') {
    return null
  }
  const candidate = value as Partial<MonitorConnectionDraft>
  const name = text(candidate.name)
  const endpoint = text(candidate.endpoint)
  if (!name || !endpoint || (candidate.catalogSource !== 'reflection' && candidate.catalogSource !== 'proto')) {
    return null
  }

  const result = {
    id: text(candidate.id),
    name,
    endpoint,
    tlsEnabled: candidate.tlsEnabled,
    tlsMode: candidate.tlsMode,
    authority: text(candidate.authority),
    serverNameOverride: text(candidate.serverNameOverride),
    caCertRef: text(candidate.caCertRef),
    clientCertRef: text(candidate.clientCertRef),
    clientKeyRef: text(candidate.clientKeyRef),
    connectTimeoutMs: candidate.connectTimeoutMs,
    requestTimeoutMs: candidate.requestTimeoutMs,
    streamIdleTimeoutMs: candidate.streamIdleTimeoutMs,
    catalogSource: candidate.catalogSource,
    protoSources: Array.isArray(candidate.protoSources) ? candidate.protoSources : [],
    importPaths: Array.isArray(candidate.importPaths) ? candidate.importPaths.filter((path): path is string => typeof path === 'string') : [],
  }
  return Object.fromEntries(Object.entries(result).filter(([, item]) => item !== undefined)) as unknown as MonitorConnectionDraft
}

export function loadRuntimeSession(): RuntimeSession {
  const raw = storage()?.getItem(RUNTIME_SESSION_STORAGE_KEY)
  if (!raw) {
    return { connections: [] }
  }
  try {
    const candidate = JSON.parse(raw) as { workspaceManifestPath?: unknown; selectedConnectionId?: unknown; connections?: unknown }
    return {
      workspaceManifestPath: text(candidate.workspaceManifestPath),
      selectedConnectionId: text(candidate.selectedConnectionId),
      connections: Array.isArray(candidate.connections) ? candidate.connections.map(draft).filter((value): value is MonitorConnectionDraft => value !== null) : [],
    }
  } catch {
    return { connections: [] }
  }
}

export function saveRuntimeSession(session: RuntimeSession): void {
  const safeConnections = session.connections
    .map(draft)
    .filter((value): value is MonitorConnectionDraft => value !== null)
    .map(({ metadataDefaults: _metadataDefaults, ...connection }) => connection)
  storage()?.setItem(
    RUNTIME_SESSION_STORAGE_KEY,
    JSON.stringify({
      workspaceManifestPath: text(session.workspaceManifestPath),
      selectedConnectionId: text(session.selectedConnectionId),
      connections: safeConnections,
    }),
  )
}
