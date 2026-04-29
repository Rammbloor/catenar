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

export function getDiagnosticContextRows(details: Record<string, string> | undefined): DiagnosticContextRow[] {
  if (!details) {
    return []
  }

  const rows: DiagnosticContextRow[] = []
  const location = formatDiagnosticLocation(details)
  if (location) {
    rows.push({ label: 'Source', value: location })
  }

  const importedPath = readDetail(details, 'import')
  if (importedPath) {
    rows.push({ label: 'Import', value: importedPath })
  }

  const consumedKeys = new Set(['file', 'line', 'column', 'import'])
  const extraKeys = Object.keys(details)
    .filter((key) => !consumedKeys.has(key) && readDetail(details, key) !== '')
    .sort()

  for (const key of extraKeys) {
    rows.push({
      label: humanizeDiagnosticKey(key),
      value: details[key],
    })
  }

  return rows
}
