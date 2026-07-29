import { describe, expect, it } from 'vitest'
import { get } from 'svelte/store'
import { FRONTEND_CONTRACT_MANIFEST, type BootstrapData } from '../contracts'
import { canTransition, createAppShellStore } from './app-shell'

const bootstrapFixture: BootstrapData = {
  app: {
    name: 'Catenar',
    version: '0.0.0',
    productLine: 'gRPC client & traffic inspector',
    platform: 'darwin',
    architecture: 'arm64',
    goVersion: 'go1.25.6',
    wailsVersion: 'v2.12.0',
  },
  contract: FRONTEND_CONTRACT_MANIFEST,
}

describe('createAppShellStore', () => {
  it('hydrates bootstrap and keeps the runtime contract verified', () => {
    const store = createAppShellStore()
    store.hydrateBootstrap(bootstrapFixture)

    const state = get(store)
    expect(state.bootstrap?.app.name).toBe('Catenar')
    expect(state.contractMismatch).toEqual([])
  })

  it('records diagnostics when an invalid stream transition is attempted', () => {
    const store = createAppShellStore()
    store.setStreamState('closed')

    const state = get(store)
    expect(state.activeStreamState).toBe('idle')
    expect(state.diagnostics[0]?.code).toBe('application.invalid_state_transition')
  })

  it('allows a new stream session to start after a terminal state', () => {
    const store = createAppShellStore()
    store.setStreamState('connecting')
    store.setStreamState('open')
    store.setStreamState('closed')
    store.setStreamState('connecting')

    const state = get(store)
    expect(state.activeStreamState).toBe('connecting')
    expect(state.diagnostics).toEqual([])
  })

  it('keeps stream events ordered by sequence', () => {
    const store = createAppShellStore()
    const baseEvent = {
      sessionId: 'sess_1',
      callId: 'call_1',
      kind: 'message_received',
      direction: 'received',
      ts: '2026-05-10T09:00:00.000Z',
      payload: {
        preview: {
          json: {},
        },
      },
    }

    store.applyStreamEvent({ ...baseEvent, seq: 2 })
    store.applyStreamEvent({ ...baseEvent, seq: 1 })

    const state = get(store)
    expect(state.streamEvents.map((event) => event.seq)).toEqual([1, 2])
    expect(state.activeStreamSessionId).toBe('sess_1')
  })
})

describe('canTransition', () => {
  it('accepts allowed canonical transitions', () => {
    expect(canTransition('idle', 'connecting')).toBe(true)
    expect(canTransition('open', 'half_closed_local')).toBe(true)
  })

  it('rejects forbidden canonical transitions', () => {
    expect(canTransition('closed', 'open')).toBe(false)
    expect(canTransition('cancelled', 'connecting')).toBe(false)
  })
})
