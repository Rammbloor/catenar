import { afterEach, describe, expect, it, vi } from 'vitest'

describe('runtime session', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.resetModules()
  })

  it('persists only reconnectable connection data and restores the selected connection', async () => {
    const values = new Map<string, string>()
    vi.stubGlobal('window', {
      localStorage: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, value),
      },
    })
    const { loadRuntimeSession, saveRuntimeSession } = await import('./runtime-session')

    saveRuntimeSession({
      workspaceManifestPath: '/tmp/demo/workspace.yaml',
      selectedConnectionId: 'local',
      connections: [
        {
          id: 'local',
          name: 'Local',
          endpoint: 'localhost:50051',
          catalogSource: 'reflection',
          protoSources: [],
          metadataDefaults: { authorization: 'secret' },
        },
      ],
    })

    expect(loadRuntimeSession()).toEqual({
      workspaceManifestPath: '/tmp/demo/workspace.yaml',
      selectedConnectionId: 'local',
      connections: [
        {
          id: 'local',
          name: 'Local',
          endpoint: 'localhost:50051',
          catalogSource: 'reflection',
          protoSources: [],
          importPaths: [],
        },
      ],
    })
  })

  it('ignores malformed stored data', async () => {
    vi.stubGlobal('window', {
      localStorage: { getItem: () => '{broken', setItem: () => undefined },
    })
    const { loadRuntimeSession } = await import('./runtime-session')

    expect(loadRuntimeSession()).toEqual({ connections: [] })
  })
})
