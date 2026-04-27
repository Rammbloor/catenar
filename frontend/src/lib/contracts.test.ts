import { describe, expect, it } from 'vitest'
import {
  ENDPOINT_CHECK_OUTCOMES,
  ENDPOINT_CHECK_STAGES,
  FRONTEND_CONTRACT_MANIFEST,
  TLS_MODES,
  verifyContractManifest,
} from './contracts'

describe('verifyContractManifest', () => {
  it('accepts the matching frontend mirror', () => {
    expect(verifyContractManifest(FRONTEND_CONTRACT_MANIFEST)).toEqual([])
  })

  it('reports ordering or naming mismatches', () => {
    const mismatched = {
      ...FRONTEND_CONTRACT_MANIFEST,
      eventNames: ['diagnostics:update', ...FRONTEND_CONTRACT_MANIFEST.eventNames.slice(1)],
    }

    expect(verifyContractManifest(mismatched)).toContain(
      'eventNames[0]: expected stream:state, got diagnostics:update',
    )
  })

  it('exposes the endpoint and TLS contract enums expected by Slice 1.1', () => {
    expect(TLS_MODES).toEqual(['plaintext', 'system_ca', 'custom_ca', 'mtls'])
    expect(ENDPOINT_CHECK_STAGES).toEqual([
      'target_resolution',
      'tcp_connect',
      'tls_handshake',
      'grpc_readiness',
    ])
    expect(ENDPOINT_CHECK_OUTCOMES).toEqual(['passed', 'failed', 'skipped', 'not_proven'])
  })
})
