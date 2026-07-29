import { describe, expect, it } from 'vitest'
import type { CatalogMethod } from './contracts'
import type { MonitorConnection } from './monitor-flow'
import {
  buildMetadataFromRows,
  buildUnaryRequestSaveInput,
  createEmptyMetadataRow,
  defaultUnaryRequestId,
  parseJsonDraft,
  starterBodyDraft,
  unaryDraftKey,
} from './unary-flow'

const method: CatalogMethod = {
  name: 'SayHello',
  fullName: 'demo.v1.Greeter.SayHello',
  rpcType: 'unary',
  requestType: { name: 'HelloRequest', fullName: 'demo.v1.HelloRequest', isWellKnown: false },
  responseType: { name: 'HelloReply', fullName: 'demo.v1.HelloReply', isWellKnown: false },
}

const connection: MonitorConnection = {
  id: 'ep_local',
  name: 'Greeter',
  endpoint: 'localhost:50051',
  status: 'online',
  streams: 0,
  updatedAt: '10:00:00',
  catalogSource: 'reflection',
  services: [],
  requestTemplates: {
    [method.fullName]: {
      name: 'world',
      labels: ['mvp'],
    },
  },
}

describe('unary flow helpers', () => {
  it('keys drafts by endpoint, environment and method', () => {
    expect(
      unaryDraftKey({
        endpointRef: 'ep_local',
        environmentRef: 'local',
        methodFullName: 'demo.v1.Greeter.SayHello',
      }),
    ).toBe('ep_local::local::demo.v1.Greeter.SayHello')
  })

  it('uses the selected connection request template as the starter body', () => {
    expect(starterBodyDraft(connection, method)).toBe('{\n  "name": "world",\n  "labels": [\n    "mvp"\n  ]\n}')
  })

  it('validates the JSON body before invoke', () => {
    expect(parseJsonDraft('{ "name": "world" }')).toEqual({
      ok: true,
      value: { name: 'world' },
    })
    expect(parseJsonDraft('{')).toMatchObject({ ok: false })
  })

  it('explains that a leading zero makes a JSON number invalid', () => {
    expect(parseJsonDraft('{ "age": 010 }')).toMatchObject({
      ok: false,
      code: 'json.number_leading_zero',
    })
  })

  it('builds metadata from editable key value rows', () => {
    expect(
      buildMetadataFromRows([
        { id: 'row-1', key: 'authorization', value: 'Bearer {{ env.token }}' },
        createEmptyMetadataRow('row-2'),
      ]),
    ).toEqual({
      ok: true,
      value: {
        authorization: 'Bearer {{ env.token }}',
      },
    })
  })

  it('rejects incomplete and duplicate metadata keys', () => {
    expect(buildMetadataFromRows([{ id: 'row-1', key: '', value: 'token' }])).toEqual({
      ok: false,
      error: { code: 'metadata.keyRequired' },
    })

    expect(
      buildMetadataFromRows([
        { id: 'row-1', key: 'authorization', value: 'a' },
        { id: 'row-2', key: 'authorization', value: 'b' },
      ]),
    ).toEqual({
      ok: false,
      error: { code: 'metadata.duplicateKey', key: 'authorization' },
    })
  })

  it('builds unary saved request input with single mode and request-level metadata only', () => {
    const input = buildUnaryRequestSaveInput({
      connection: {
        ...connection,
        endpointCheck: {
          endpoint: {
            id: 'ep_local',
            target: 'localhost:50051',
            tls: { mode: 'plaintext' },
            metadataDefaults: {
              authorization: 'Bearer {{ env.default_token }}',
            },
          },
          transportReachable: true,
          tlsConfigured: false,
          tlsOk: false,
          grpcReady: true,
          grpcReadyProven: true,
          checks: [],
          testedAt: '2026-05-04T10:00:00Z',
          durationMs: 8,
        },
      },
      method: {
        ...method,
        fullName: 'demo.v1.Greeter/SayHello',
      },
      name: 'List test users',
      body: { name: 'world' },
      metadata: {
        authorization: 'Bearer {{ env.token }}',
      },
      callOptions: {
        requestTimeoutMs: 1500,
      },
    })

    expect(input).toEqual({
      id: 'demo.v1.greeter.sayhello',
      name: 'List test users',
      method: 'demo.v1.Greeter/SayHello',
      rpcType: 'unary',
      endpointRef: 'ep_local',
      metadataTemplate: {
        authorization: 'Bearer {{ env.token }}',
      },
      callOptions: {
        requestTimeoutMs: 1500,
      },
      requestSpec: {
        mode: 'single',
        body: { name: 'world' },
      },
    })
  })

  it('uses the canonical method-derived request id when no explicit id is provided', () => {
    expect(defaultUnaryRequestId({ ...method, fullName: 'demo.v1.Greeter/Say Hello' })).toBe(
      'demo.v1.greeter.say-hello',
    )
  })
})
