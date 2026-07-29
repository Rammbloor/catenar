import { describe, expect, it } from 'vitest'
import type { EndpointTestResult, ReflectionCatalogResult } from './contracts'
import {
  buildEndpointPreset,
  createWorkspaceFallbackConnection,
  createMonitorConnection,
  endpointReadinessError,
  endpointStatusFromTest,
  monitorConnectionToEndpointPreset,
  protoSourceLabel,
  workspaceDraftFromConnections,
  workspaceEndpointDraft,
} from './monitor-flow'

function endpointTest(overrides: Partial<EndpointTestResult> = {}): EndpointTestResult {
  return {
    endpoint: {
      id: 'ep_reflection',
      name: 'Greeter',
      target: 'localhost:50051',
      tls: { mode: 'plaintext' },
      connectTimeoutMs: 5000,
      requestTimeoutMs: 30000,
      streamIdleTimeoutMs: 120000,
    },
    transportReachable: true,
    tlsConfigured: false,
    tlsOk: false,
    grpcReady: true,
    grpcReadyProven: true,
    checks: [],
    testedAt: '2026-05-04T10:00:00Z',
    durationMs: 12,
    ...overrides,
  }
}

describe('monitor flow helpers', () => {
  it('builds a backend-ready endpoint preset from the connection draft', () => {
    expect(
      buildEndpointPreset({
        name: ' Greeter ',
        endpoint: ' localhost:50051 ',
        tlsEnabled: true,
        catalogSource: 'reflection',
        protoSources: [],
      }),
    ).toEqual({
      name: 'Greeter',
      target: 'localhost:50051',
      tls: { mode: 'system_ca' },
      connectTimeoutMs: 5000,
      requestTimeoutMs: 30000,
      streamIdleTimeoutMs: 120000,
    })
  })

  it('builds custom TLS material references without leaking file paths', () => {
    expect(
      buildEndpointPreset({
        name: 'Secure',
        endpoint: 'secure.local:443',
        tlsMode: 'mtls',
        serverNameOverride: 'secure.local',
        caCertRef: 'secret-ref:file/secure/ca',
        clientCertRef: 'secret-ref:file/secure/client-cert',
        clientKeyRef: 'secret-ref:file/secure/client-key',
        catalogSource: 'reflection',
        protoSources: [],
      }),
    ).toMatchObject({
      tls: {
        mode: 'mtls',
        serverNameOverride: 'secure.local',
        caCert: 'secret-ref:file/secure/ca',
        clientCert: 'secret-ref:file/secure/client-cert',
        clientKey: 'secret-ref:file/secure/client-key',
      },
    })
  })

  it('drops stale material references when TLS mode does not allow them', () => {
    expect(
      buildEndpointPreset({
        name: 'Secure',
        endpoint: 'secure.local:443',
        tlsMode: 'system_ca',
        caCertRef: 'secret-ref:file/secure/ca',
        clientCertRef: 'secret-ref:file/secure/client-cert',
        clientKeyRef: 'secret-ref:file/secure/client-key',
        catalogSource: 'reflection',
        protoSources: [],
      }).tls,
    ).toEqual({ mode: 'system_ca' })
  })

  it('blocks catalog loading when endpoint test did not prove gRPC readiness', () => {
    const result = endpointTest({
      grpcReady: false,
      grpcReadyProven: false,
      checks: [
        {
          stage: 'grpc_readiness',
          outcome: 'not_proven',
          message: 'gRPC readiness could not be proven.',
        },
      ],
    })

    expect(endpointStatusFromTest(result)).toBe('warning')
    expect(endpointReadinessError(result)).toBe('gRPC readiness could not be proven.')
  })

  it('creates a connection with the normalized backend endpoint id and first method selected', () => {
    const catalog: ReflectionCatalogResult = {
      endpoint: endpointTest().endpoint,
      services: [
        {
          name: 'Greeter',
          fullName: 'demo.v1.Greeter',
          methods: [
            {
              name: 'SayHello',
              fullName: 'demo.v1.Greeter.SayHello',
              rpcType: 'unary',
              requestType: { name: 'HelloRequest', fullName: 'demo.v1.HelloRequest', isWellKnown: false },
              responseType: { name: 'HelloReply', fullName: 'demo.v1.HelloReply', isWellKnown: false },
            },
          ],
        },
      ],
      loadedAt: '2026-05-04T10:00:01Z',
      durationMs: 21,
    }

    expect(
      createMonitorConnection(
        {
          name: 'Greeter',
          endpoint: 'localhost:50051',
          tlsEnabled: false,
          catalogSource: 'reflection',
          protoSources: [],
        },
        endpointTest(),
        catalog,
        new Date('2026-05-04T10:00:02Z'),
      ),
    ).toMatchObject({
      id: 'ep_reflection',
      status: 'online',
      services: catalog.services,
      selectedMethodFullName: 'demo.v1.Greeter.SayHello',
      catalogSource: 'reflection',
    })
  })

  it('keeps real proto source paths while using compact labels in the UI', () => {
    expect(protoSourceLabel({ type: 'file', path: '/workspace/proto/demo/v1/greeter.proto' })).toBe(
      'greeter.proto (file)',
    )
  })

  it('builds a workspace draft from real monitor connections without duplicating proto paths', () => {
    const connection = createMonitorConnection(
      {
        name: 'Greeter',
        endpoint: 'localhost:50051',
        tlsEnabled: false,
        catalogSource: 'proto',
        protoSources: [{ type: 'file', path: '/workspace/proto/demo/v1/greeter.proto' }],
        importPaths: ['/workspace/proto'],
      },
      endpointTest(),
      {
        endpoint: endpointTest().endpoint,
        services: [],
        protoSources: [{ type: 'file', path: '/workspace/proto/demo/v1/greeter.proto' }],
        importPaths: ['/workspace/proto'],
        loadedAt: '2026-05-04T10:00:01Z',
        durationMs: 21,
      },
    )

    expect(
      workspaceDraftFromConnections({
        name: '  demo  ',
        connections: [
          connection,
          {
            ...connection,
            id: 'ep_second',
            protoSources: [{ type: 'file', path: '/workspace/proto/demo/v1/greeter.proto' }],
          },
        ],
      }),
    ).toMatchObject({
      name: 'demo',
      endpoints: [monitorConnectionToEndpointPreset(connection), monitorConnectionToEndpointPreset(connection)],
      protoSources: [{ type: 'file', path: '/workspace/proto/demo/v1/greeter.proto' }],
      importPaths: ['/workspace/proto'],
    })
  })

  it('recreates endpoint drafts and fallback rows from a workspace snapshot', () => {
    const endpoint = endpointTest().endpoint
    const workspace = {
      protoSources: [{ type: 'file' as const, path: '/workspace/proto/demo/v1/greeter.proto' }],
      importPaths: ['/workspace/proto'],
    }

    expect(workspaceEndpointDraft(endpoint, workspace)).toMatchObject({
      name: 'Greeter',
      endpoint: 'localhost:50051',
      catalogSource: 'proto',
      protoSources: workspace.protoSources,
      importPaths: workspace.importPaths,
    })
    expect(
      createWorkspaceFallbackConnection({
        endpoint,
        workspace,
        status: 'warning',
        now: new Date('2026-05-04T10:00:02Z'),
      }),
    ).toMatchObject({
      id: 'ep_reflection',
      status: 'warning',
      services: [],
      catalogSource: 'proto',
    })
  })
})
