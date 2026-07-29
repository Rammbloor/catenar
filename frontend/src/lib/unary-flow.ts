import type { CallOptions, CatalogMethod, JsonValue, RequestSaveInput } from './contracts'
import type { MonitorConnection } from './monitor-flow'

export interface MetadataDraftRow {
  id: string
  key: string
  value: string
}

export type MetadataValidationError =
  | { code: 'metadata.keyRequired' }
  | { code: 'metadata.duplicateKey'; key: string }

export type JsonDraftParseResult =
  | { ok: true; value: JsonValue }
  | { ok: false; message: string; code?: 'json.number_leading_zero' }

export type MetadataBuildResult =
  | { ok: true; value: Record<string, string> }
  | { ok: false; error: MetadataValidationError }

export function unaryDraftKey(input: {
  endpointRef: string
  methodFullName: string
  environmentRef?: string
}): string {
  return `${input.endpointRef}::${input.environmentRef ?? 'default'}::${input.methodFullName}`
}

export function formatJsonValue(value: unknown): string {
  if (value === undefined) {
    return '{}'
  }

  return JSON.stringify(value, null, 2)
}

export function starterBodyDraft(connection: MonitorConnection, method: CatalogMethod): string {
  return formatJsonValue(connection.requestTemplates?.[method.fullName])
}

export function parseJsonDraft(value: string): JsonDraftParseResult {
  try {
    return {
      ok: true,
      value: JSON.parse(value) as JsonValue,
    }
  } catch (error) {
    // Return a semantic code so the UI can use a translated, generic message
    // instead of hardcoding a particular number into the validation feedback.
    if (/(?:^|[[:,]\s*)-?0\d+(?=\s*[,}\]])/.test(value)) {
      return {
        ok: false,
        message: '',
        code: 'json.number_leading_zero',
      }
    }

    return {
      ok: false,
      message: error instanceof Error ? error.message : '',
    }
  }
}

export function createEmptyMetadataRow(id: string): MetadataDraftRow {
  return { id, key: '', value: '' }
}

export function cloneMetadataRows(rows: MetadataDraftRow[]): MetadataDraftRow[] {
  return rows.map((row) => ({ ...row }))
}

export function buildMetadataFromRows(rows: MetadataDraftRow[]): MetadataBuildResult {
  const result: Record<string, string> = {}
  const seen = new Set<string>()

  for (const row of rows) {
    const key = row.key.trim()
    const value = row.value

    if (!key && !value.trim()) {
      continue
    }

    if (!key) {
      return { ok: false, error: { code: 'metadata.keyRequired' } }
    }

    if (seen.has(key)) {
      return { ok: false, error: { code: 'metadata.duplicateKey', key } }
    }

    seen.add(key)
    result[key] = value
  }

  return { ok: true, value: result }
}

export function defaultUnaryRequestId(method: CatalogMethod): string {
  return method.fullName.replaceAll('/', '.').replace(/[^a-zA-Z0-9_.-]+/g, '-').toLowerCase()
}

function compactStringMap(values: Record<string, string>): Record<string, string> | undefined {
  return Object.keys(values).length > 0 ? { ...values } : undefined
}

function compactCallOptions(values: CallOptions | undefined): CallOptions | undefined {
  if (!values) {
    return undefined
  }

  const result: CallOptions = {}
  if (values.requestTimeoutMs !== undefined) {
    result.requestTimeoutMs = values.requestTimeoutMs
  }
  if (values.streamIdleTimeoutMs !== undefined) {
    result.streamIdleTimeoutMs = values.streamIdleTimeoutMs
  }

  return Object.keys(result).length > 0 ? result : undefined
}

export function buildUnaryRequestSaveInput(input: {
  connection: MonitorConnection
  method: CatalogMethod
  body: JsonValue
  metadata: Record<string, string>
  callOptions?: CallOptions
  id?: string
  name?: string
}): RequestSaveInput {
  if (input.method.rpcType !== 'unary') {
    throw new Error('Only unary methods can be saved with a single request body.')
  }

  return {
    id: input.id?.trim() || defaultUnaryRequestId(input.method),
    name: input.name?.trim() || undefined,
    method: input.method.fullName,
    rpcType: 'unary',
    endpointRef: input.connection.id,
    metadataTemplate: compactStringMap(input.metadata),
    callOptions: compactCallOptions(input.callOptions),
    requestSpec: {
      mode: 'single',
      body: input.body,
    },
  }
}
