import { describe, expect, it } from 'vitest'

import { formatDiagnosticLocation, getDiagnosticContextRows } from './diagnostics'

describe('diagnostics helpers', () => {
  it('formats proto source positions into a single location string', () => {
    expect(
      formatDiagnosticLocation({
        file: 'demo/v1/reflection_demo.proto',
        line: '12',
        column: '4',
      }),
    ).toBe('demo/v1/reflection_demo.proto:12:4')
  })

  it('orders broken import context ahead of generic detail fields', () => {
    expect(
      getDiagnosticContextRows({
        file: 'demo/v1/reflection_demo.proto',
        line: '12',
        column: '4',
        import: 'shared/v1/proto_marker.proto',
        cause: 'open shared/v1/proto_marker.proto: no such file or directory',
      }),
    ).toEqual([
      { label: 'Source', value: 'demo/v1/reflection_demo.proto:12:4' },
      { label: 'Import', value: 'shared/v1/proto_marker.proto' },
      { label: 'Cause', value: 'open shared/v1/proto_marker.proto: no such file or directory' },
    ])
  })

  it('can localize user-facing diagnostic labels', () => {
    expect(
      getDiagnosticContextRows(
        {
          file: 'demo/v1/reflection_demo.proto',
          import: 'shared/v1/proto_marker.proto',
          cause: 'not found',
        },
        'ru',
      ),
    ).toEqual([
      { label: 'Источник', value: 'demo/v1/reflection_demo.proto' },
      { label: 'Импорт', value: 'shared/v1/proto_marker.proto' },
      { label: 'Причина', value: 'not found' },
    ])
  })
})
