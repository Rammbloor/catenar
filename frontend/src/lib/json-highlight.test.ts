import { describe, expect, it } from 'vitest'
import { highlightJson } from './json-highlight'

describe('highlightJson', () => {
  it('distinguishes keys, values, numbers and literals', () => {
    expect(highlightJson('{ "name": "Catenar", "age": 4, "active": true }')).toBe(
      '{ <span class="json-token json-token--key">&quot;name&quot;</span>: <span class="json-token json-token--string">&quot;Catenar&quot;</span>, <span class="json-token json-token--key">&quot;age&quot;</span>: <span class="json-token json-token--number">4</span>, <span class="json-token json-token--key">&quot;active&quot;</span>: <span class="json-token json-token--literal">true</span> }',
    )
  })

  it('escapes content before it is rendered as HTML', () => {
    expect(highlightJson('{ "value": "<script>alert(1)</script>" }')).not.toContain('<script>')
    expect(highlightJson('{ "value": "<script>alert(1)</script>" }')).toContain('&lt;script&gt;')
  })
})
