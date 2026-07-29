const JSON_TOKEN = /"(?:\\.|[^"\\])*"(?=\s*:)|"(?:\\.|[^"\\])*"|\b(?:true|false|null)\b|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/g

function escapeHtml(value: string): string {
  return value.replace(/[&<>'"]/g, (character) => {
    switch (character) {
      case '&': return '&amp;'
      case '<': return '&lt;'
      case '>': return '&gt;'
      case "'": return '&#39;'
      case '"': return '&quot;'
      default: return character
    }
  })
}

/**
 * Returns escaped, display-only JSON markup for code blocks. Input is always
 * escaped before it reaches the template, including malformed JSON excerpts.
 */
export function highlightJson(value: string): string {
  let result = ''
  let lastIndex = 0

  for (const match of value.matchAll(JSON_TOKEN)) {
    const token = match[0]
    const index = match.index ?? 0
    result += escapeHtml(value.slice(lastIndex, index))

    const className = token.startsWith('"')
      ? /^\s*:/.test(value.slice(index + token.length))
        ? 'json-token json-token--key'
        : 'json-token json-token--string'
      : token === 'true' || token === 'false' || token === 'null'
        ? 'json-token json-token--literal'
        : 'json-token json-token--number'

    result += `<span class="${className}">${escapeHtml(token)}</span>`
    lastIndex = index + token.length
  }

  return result + escapeHtml(value.slice(lastIndex))
}
