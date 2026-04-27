<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type {
    BootstrapData,
    CatalogMethod,
    EndpointPreset,
    EndpointTestResult,
    ReflectionCatalogResult,
    TLSMode,
  } from '../contracts'
  import { loadCatalogFromReflection, testEndpoint } from '../wails/backend'

  export let bootstrap: BootstrapData
  export let contractMismatch: string[]

  const dispatch = createEventDispatcher<{
    opensession: void
  }>()

  let endpoint: EndpointPreset = {
    target: '127.0.0.1:50051',
    authority: '',
    tls: {
      mode: 'plaintext',
      serverNameOverride: '',
      caCert: '',
      clientCert: '',
      clientKey: '',
    },
    connectTimeoutMs: 3000,
    requestTimeoutMs: 1000,
    streamIdleTimeoutMs: 0,
    metadataDefaults: {},
  }

  let endpointTestResult: EndpointTestResult | null = null
  let reflectionCatalog: ReflectionCatalogResult | null = null
  let testPending = false
  let reflectionPending = false
  let actionError = ''

  function buildEndpointPreset(): EndpointPreset {
    const tlsMode = endpoint.tls.mode

    return {
      ...endpoint,
      target: endpoint.target.trim(),
      authority: endpoint.authority?.trim(),
      tls: {
        mode: tlsMode,
        serverNameOverride: tlsMode === 'plaintext' ? '' : endpoint.tls.serverNameOverride?.trim(),
        caCert: tlsMode === 'custom_ca' || tlsMode === 'mtls' ? endpoint.tls.caCert?.trim() : '',
        clientCert: tlsMode === 'mtls' ? endpoint.tls.clientCert?.trim() : '',
        clientKey: tlsMode === 'mtls' ? endpoint.tls.clientKey?.trim() : '',
      },
      metadataDefaults: {},
    }
  }

  async function runEndpointTest(): Promise<void> {
    actionError = ''
    testPending = true

    try {
      endpointTestResult = await testEndpoint({ endpoint: buildEndpointPreset() })
    } catch (error) {
      actionError = error instanceof Error ? error.message : 'Endpoint preflight failed unexpectedly.'
    } finally {
      testPending = false
    }
  }

  async function runReflectionLoad(): Promise<void> {
    actionError = ''
    reflectionPending = true

    try {
      reflectionCatalog = await loadCatalogFromReflection({ endpoint: buildEndpointPreset() })
    } catch (error) {
      reflectionCatalog = null
      actionError = error instanceof Error ? error.message : 'Reflection catalog load failed unexpectedly.'
    } finally {
      reflectionPending = false
    }
  }

  function formatRpcType(rpcType: CatalogMethod['rpcType']): string {
    return rpcType.replaceAll('_', ' ')
  }

  function isTLSMode(mode: TLSMode): boolean {
    return endpoint.tls.mode === mode
  }
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">Slice 1.2</p>
    <h2 class="section-title">Reflection-based exploration</h2>
    <p class="section-copy">
      This workspace path reuses the Slice 1.1 endpoint and TLS model, then loads a service tree
      through gRPC Server Reflection without leaking transport internals into the UI contract.
    </p>
  </div>

  <div class="two-column reflection-layout">
    <div class="card stack">
      <div class="card__header">
        <h3>Endpoint</h3>
        <div class="pill-row">
          <span class="pill pill--accent">{bootstrap.contract.boundMethods.length} bindings</span>
          <span class="pill">{contractMismatch.length === 0 ? 'contract verified' : 'contract drift'}</span>
        </div>
      </div>

      <div class="form-grid">
        <label class="field">
          <span>Target</span>
          <input bind:value={endpoint.target} placeholder="127.0.0.1:50051" />
        </label>

        <label class="field">
          <span>Authority</span>
          <input bind:value={endpoint.authority} placeholder="optional override" />
        </label>

        <label class="field">
          <span>TLS mode</span>
          <select bind:value={endpoint.tls.mode}>
            <option value="plaintext">plaintext</option>
            <option value="system_ca">system_ca</option>
            <option value="custom_ca">custom_ca</option>
            <option value="mtls">mtls</option>
          </select>
        </label>

        <label class="field">
          <span>Server name override</span>
          <input bind:value={endpoint.tls.serverNameOverride} placeholder="optional SAN override" />
        </label>

        <label class="field">
          <span>Connect timeout (ms)</span>
          <input bind:value={endpoint.connectTimeoutMs} min="1" type="number" />
        </label>

        <label class="field">
          <span>Request timeout (ms)</span>
          <input bind:value={endpoint.requestTimeoutMs} min="0" type="number" />
        </label>

        <label class="field">
          <span>Stream idle timeout (ms)</span>
          <input bind:value={endpoint.streamIdleTimeoutMs} min="0" type="number" />
        </label>

        {#if isTLSMode('custom_ca') || isTLSMode('mtls')}
          <label class="field field--span-2">
            <span>CA secret-ref</span>
            <input bind:value={endpoint.tls.caCert} placeholder="secret-ref:file/tls/ca.pem" />
          </label>
        {/if}

        {#if isTLSMode('mtls')}
          <label class="field">
            <span>Client cert secret-ref</span>
            <input bind:value={endpoint.tls.clientCert} placeholder="secret-ref:file/tls/client.crt" />
          </label>

          <label class="field">
            <span>Client key secret-ref</span>
            <input bind:value={endpoint.tls.clientKey} placeholder="secret-ref:file/tls/client.key" />
          </label>
        {/if}
      </div>

      <div class="pill-row">
        <button class="ghost-button" disabled={testPending} on:click={runEndpointTest}>
          {testPending ? 'Testing endpoint…' : 'Run endpoint preflight'}
        </button>
        <button class="action-button" disabled={reflectionPending} on:click={runReflectionLoad}>
          {reflectionPending ? 'Loading reflection…' : 'Load reflection catalog'}
        </button>
      </div>

      {#if actionError}
        <div class="empty-state">
          {actionError}
          <div class="subtle">Structured details are emitted into the diagnostics panel when the runtime can classify the failure.</div>
        </div>
      {/if}
    </div>

    <div class="stack">
      <div class="card stack">
        <h3>Transport preflight</h3>
        {#if !endpointTestResult}
          <div class="empty-state">Run endpoint preflight to reuse the Slice 1.1 transport, TLS and gRPC readiness checks.</div>
        {:else}
          <div class="pill-row">
            <span class="pill" class:pill--accent={endpointTestResult.transportReachable}>
              transport {endpointTestResult.transportReachable ? 'reachable' : 'blocked'}
            </span>
            <span class="pill" class:pill--accent={endpointTestResult.grpcReadyProven}>
              gRPC {endpointTestResult.grpcReadyProven ? 'ready' : 'not proven'}
            </span>
            <span class="pill" class:pill--accent={endpointTestResult.tlsOk || !endpointTestResult.tlsConfigured}>
              TLS {endpointTestResult.tlsConfigured ? (endpointTestResult.tlsOk ? 'ok' : 'failed') : 'off'}
            </span>
          </div>

          <div class="stack">
            {#each endpointTestResult.checks as check}
              <article class="diagnostic-item">
                <div class="diagnostic-item__head">
                  <strong>{check.stage}</strong>
                  <span class="diagnostic-item__meta">{check.outcome}</span>
                </div>
                <div>{check.message}</div>
              </article>
            {/each}
          </div>
        {/if}
      </div>

      <div class="card stack">
        <div class="card__header">
          <h3>Reflection catalog</h3>
          {#if reflectionCatalog}
            <button class="ghost-button" on:click={() => dispatch('opensession')}>Open session shell</button>
          {/if}
        </div>

        {#if !reflectionCatalog}
          <div class="empty-state">
            Load reflection to build the service tree, RPC metadata and well-known protobuf type summary.
          </div>
        {:else}
          <div class="pill-row">
            <span class="pill pill--accent">{reflectionCatalog.services.length} services</span>
            <span class="pill">{reflectionCatalog.wellKnownTypes?.length ?? 0} well-known types</span>
            <span class="pill">{reflectionCatalog.durationMs} ms</span>
          </div>

          <div class="stack">
            {#each reflectionCatalog.services as service}
              <article class="catalog-service">
                <div class="catalog-service__head">
                  <div>
                    <h4>{service.name}</h4>
                    <div class="subtle">{service.fullName}</div>
                  </div>
                  <span class="badge">{service.methods.length} methods</span>
                </div>

                <div class="catalog-methods">
                  {#each service.methods as method}
                    <article class="catalog-method">
                      <div class="catalog-method__head">
                        <strong>{method.name}</strong>
                        <span class="badge">{formatRpcType(method.rpcType)}</span>
                      </div>
                      <div class="subtle">{method.fullName}</div>
                      <div class="catalog-method__types">
                        <span>request: <strong>{method.requestType.fullName}</strong></span>
                        {#if method.requestType.isWellKnown}
                          <span class="pill pill--accent">well-known</span>
                        {/if}
                      </div>
                      <div class="catalog-method__types">
                        <span>response: <strong>{method.responseType.fullName}</strong></span>
                        {#if method.responseType.isWellKnown}
                          <span class="pill pill--accent">well-known</span>
                        {/if}
                      </div>
                    </article>
                  {/each}
                </div>
              </article>
            {/each}
          </div>

          <div class="divider"></div>

          <div class="stack">
            <h4>Referenced well-known types</h4>
            {#if !reflectionCatalog.wellKnownTypes || reflectionCatalog.wellKnownTypes.length === 0}
              <div class="empty-state">This reflection catalog does not reference well-known protobuf types yet.</div>
            {:else}
              <div class="pill-row">
                {#each reflectionCatalog.wellKnownTypes as typeRef}
                  <span class="pill">{typeRef.fullName}</span>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>
</section>
