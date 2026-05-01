import { translate, type Language } from './i18n'

export interface DiagnosticContextRow {
  label: string
  value: string
}

function readDetail(details: Record<string, string> | undefined, key: string): string {
  return details?.[key]?.trim() ?? ''
}

function humanizeDiagnosticKey(key: string): string {
  return key
    .split(/[_-]+/)
    .filter((segment) => segment.length > 0)
    .map((segment) => segment[0].toUpperCase() + segment.slice(1))
    .join(' ')
}

export function formatDiagnosticLocation(details: Record<string, string> | undefined): string | undefined {
  const file = readDetail(details, 'file')
  if (!file) {
    return undefined
  }

  const line = readDetail(details, 'line')
  const column = readDetail(details, 'column')
  if (line && column) {
    return `${file}:${line}:${column}`
  }
  if (line) {
    return `${file}:${line}`
  }

  return file
}

function formatDiagnosticLabel(language: Language, key: string): string {
  if (key === 'cause') {
    return translate(language, 'diagnostics.detail.cause')
  }

  return humanizeDiagnosticKey(key)
}

export function getDiagnosticContextRows(
  details: Record<string, string> | undefined,
  language: Language = 'en',
): DiagnosticContextRow[] {
  if (!details) {
    return []
  }

  const rows: DiagnosticContextRow[] = []
  const location = formatDiagnosticLocation(details)
  if (location) {
    rows.push({ label: translate(language, 'diagnostics.detail.source'), value: location })
  }

  const importedPath = readDetail(details, 'import')
  if (importedPath) {
    rows.push({ label: translate(language, 'diagnostics.detail.import'), value: importedPath })
  }

  const consumedKeys = new Set(['file', 'line', 'column', 'import'])
  const extraKeys = Object.keys(details)
    .filter((key) => !consumedKeys.has(key) && readDetail(details, key) !== '')
    .sort()

  for (const key of extraKeys) {
    rows.push({
      label: formatDiagnosticLabel(language, key),
      value: details[key],
    })
  }

  return rows
}
