import { describe, expect, it } from 'vitest'
import { get } from 'svelte/store'
import { FRONTEND_CONTRACT_MANIFEST, type BootstrapData } from '../contracts'
import { canTransition, createAppShellStore } from './app-shell'

const bootstrapFixture: BootstrapData = {
  app: {
    name: 'tether',
    productLine: 'Desktop-first gRPC debugging workspace',
    platform: 'darwin',
    architecture: 'arm64',
    goVersion: 'go1.25.6',
    wailsVersion: 'v2.12.0',
  },
  contract: FRONTEND_CONTRACT_MANIFEST,
  layout: {
    regions: [
      { id: 'navigation', title: 'Navigation rail', purpose: 'View switching without an external router.' },
    ],
  },
  stateModel: {
    primaryFlow: ['home', 'workspace', 'session'],
    overlayViews: ['history-overlay', 'settings-overlay', 'diagnostics-overlay'],
    singleActiveLiveSession: true,
  },
  epicZero: [
    { slice: '0.1', status: 'implemented', summary: 'Shell' },
    { slice: '0.2', status: 'implemented', summary: 'Contract' },
    { slice: '0.3', status: 'implemented', summary: 'State and errors' },
  ],
}

describe('createAppShellStore', () => {
  it('hydrates bootstrap and keeps the frontend/backend contract verified', () => {
    const store = createAppShellStore()
    store.hydrateBootstrap(bootstrapFixture)

    const state = get(store)
    expect(state.bootstrap?.app.name).toBe('tether')
    expect(state.contractMismatch).toEqual([])
  })

  it('opens views and overlays without a router dependency', () => {
    const store = createAppShellStore()
    store.openView('workspace')
    store.openOverlay('diagnostics-overlay')

    const state = get(store)
    expect(state.currentView).toBe('workspace')
    expect(state.activeOverlay).toBe('diagnostics-overlay')
  })

  it('records diagnostics when an invalid stream transition is attempted', () => {
    const store = createAppShellStore()
    store.setStreamState('closed')

    const state = get(store)
    expect(state.activeStreamState).toBe('idle')
    expect(state.diagnostics[0]?.code).toBe('application.invalid_state_transition')
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
