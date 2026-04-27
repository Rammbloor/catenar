<script lang="ts">
  import { onMount } from 'svelte'
  import type {
    BootstrapData,
    CatalogSourceKind,
    CallInvokeUnaryResult,
    CatalogMethod,
    EndpointPreset,
    EndpointTestResult,
    HistoryCallSummary,
    HistoryGetResult,
    JsonValue,
    ProtoCatalogResult,
    ReflectionCatalogResult,
    TLSMode,
  } from '../contracts'
  import {
    getHistory,
    invokeUnary,
    listHistory,
    loadCatalogFromProtoSources,
    loadCatalogFromReflection,
    testEndpoint,
  } from '../wails/backend'

  export let bootstrap: BootstrapData
  export let contractMismatch: string[]

  type ActiveCatalog = {
    kind: CatalogSourceKind
    endpoint: EndpointPreset
    services: ReflectionCatalogResult['services']
    wellKnownTypes?: ReflectionCatalogResult['wellKnownTypes']
    requestTemplates?: ReflectionCatalogResult['requestTemplates']
    diagnostic?: ReflectionCatalogResult['diagnostic']
    loadedAt: string
    durationMs: number
  }

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
  let activeCatalog: ActiveCatalog | null = null
  let selectedMethod: CatalogMethod | null = null
  let invokeResult: CallInvokeUnaryResult | null = null
  let historySummaries: HistoryCallSummary[] = []
  let historyDetail: HistoryGetResult | null = null
  let selectedHistoryCallId = ''
  let selectedCatalogMode: CatalogSourceKind = 'reflection'
  let protoDirectoriesText = ''
  let protoFilesText = ''
  let importPathsText = ''
  let requestBodyText = '{}'
  let metadataText = '{}'
  let requestBodyError = ''
  let metadataError = ''
  let actionError = ''
  let infoMessage = ''
  let testPending = false
  let reflectionPending = false
  let protoPending = false
  let invokePending = false
  let historyPending = false
  let historyDetailPending = false
  let lastEndpointFingerprint = ''
  let lastProtoFingerprint = ''
  let lastCatalogMode: CatalogSourceKind = 'reflection'
  let bodyDrafts: Record<string, string> = {}
  let metadataDrafts: Record<string, string> = {}

  onMount(() => {
    void refreshHistory()
  })

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

  function buildEndpointFingerprint(): string {
    return JSON.stringify(buildEndpointPreset())
  }

  function buildProtoInputFingerprint(): string {
    return JSON.stringify({
      directories: protoDirectoriesText,
      files: protoFilesText,
      importPaths: importPathsText,
    })
  }

  $: {
    const nextEndpointFingerprint = buildEndpointFingerprint()
    const nextProtoFingerprint = buildProtoInputFingerprint()
    const endpointChanged = lastEndpointFingerprint !== '' && nextEndpointFingerprint !== lastEndpointFingerprint
    const catalogModeChanged = lastCatalogMode !== selectedCatalogMode
    const protoInputsChanged =
      lastProtoFingerprint !== '' &&
      nextProtoFingerprint !== lastProtoFingerprint &&
      activeCatalog?.kind === 'proto'

    if (endpointChanged || protoInputsChanged || catalogModeChanged) {
      endpointTestResult = null
      activeCatalog = null
      selectedMethod = null
      invokeResult = null
      historyDetail = null
      selectedHistoryCallId = ''
      actionError = ''
      infoMessage = ''
      requestBodyError = ''
      metadataError = ''
      bodyDrafts = {}
      metadataDrafts = {}
    }

    lastEndpointFingerprint = nextEndpointFingerprint
    lastProtoFingerprint = nextProtoFingerprint
    lastCatalogMode = selectedCatalogMode
  }

  function formatRpcType(rpcType: CatalogMethod['rpcType']): string {
    return rpcType.replaceAll('_', ' ')
  }

  function isTLSMode(mode: TLSMode): boolean {
    return endpoint.tls.mode === mode
  }

  function methodDraftKey(method: CatalogMethod): string {
    return `${activeCatalog?.kind ?? 'reflection'}::${activeCatalog?.endpoint.id ?? 'transient'}::${method.fullName}`
  }

  function formatJsonValue(value: JsonValue | undefined): string {
    if (value === undefined) {
      return '{}'
    }

    return JSON.stringify(value, null, 2)
  }

  function restoreTemplateDraft(method: CatalogMethod): void {
    const template = activeCatalog?.requestTemplates?.[method.fullName]
    requestBodyText = formatJsonValue(template)
    requestBodyError = ''
    const key = methodDraftKey(method)
    bodyDrafts = {
      ...bodyDrafts,
      [key]: requestBodyText,
    }
  }

  function selectMethod(method: CatalogMethod): void {
    selectedMethod = method
    invokeResult = null
    actionError = ''
    infoMessage = ''
    requestBodyError = ''
    metadataError = ''

    const key = methodDraftKey(method)
    requestBodyText = bodyDrafts[key] ?? formatJsonValue(activeCatalog?.requestTemplates?.[method.fullName])
    metadataText = metadataDrafts[key] ?? '{}'
  }

  function updateBodyDraft(value: string): void {
    requestBodyText = value
    if (selectedMethod) {
      bodyDrafts = {
        ...bodyDrafts,
        [methodDraftKey(selectedMethod)]: value,
      }
    }
  }

  function updateMetadataDraft(value: string): void {
    metadataText = value
    if (selectedMethod) {
      metadataDrafts = {
        ...metadataDrafts,
        [methodDraftKey(selectedMethod)]: value,
      }
    }
  }

  function parseBodyText(): JsonValue | null {
    requestBodyError = ''

    try {
      return JSON.parse(requestBodyText) as JsonValue
    } catch (error) {
      requestBodyError = error instanceof Error ? error.message : 'Request body must be valid JSON.'
      return null
    }
  }

  function parseMetadataText(): Record<string, string> | null {
    metadataError = ''

    try {
      const parsed = JSON.parse(metadataText) as JsonValue
      if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') {
        metadataError = 'Metadata overrides must be a JSON object with string values.'
        return null
      }

      const metadataEntries = Object.entries(parsed)
      const result: Record<string, string> = {}
      for (const [key, value] of metadataEntries) {
        if (typeof value !== 'string') {
          metadataError = `Metadata value for "${key}" must be a string.`
          return null
        }
        result[key] = value
      }

      return result
    } catch (error) {
      metadataError = error instanceof Error ? error.message : 'Metadata overrides must be valid JSON.'
      return null
    }
  }

  async function refreshHistory(): Promise<void> {
    historyPending = true
    try {
      const result = await listHistory({ limit: 6 })
      historySummaries = result.calls
    } catch {
      historySummaries = []
    } finally {
      historyPending = false
    }
  }

  async function loadHistoryDetail(callId: string): Promise<void> {
    selectedHistoryCallId = callId
    historyDetailPending = true
    try {
      historyDetail = await getHistory(callId)
    } catch (error) {
      historyDetail = null
      actionError = error instanceof Error ? error.message : 'History detail could not be loaded.'
    } finally {
      historyDetailPending = false
    }
  }

  async function runEndpointTest(): Promise<void> {
    actionError = ''
    infoMessage = ''
    endpointTestResult = null
    testPending = true

    try {
      endpointTestResult = await testEndpoint({ endpoint: buildEndpointPreset() })
    } catch (error) {
      actionError = error instanceof Error ? error.message : 'Endpoint preflight failed unexpectedly.'
    } finally {
      testPending = false
    }
  }

  function toActiveCatalog(kind: CatalogSourceKind, catalog: ReflectionCatalogResult | ProtoCatalogResult): ActiveCatalog {
    return {
      kind,
      endpoint: catalog.endpoint,
      services: catalog.services,
      wellKnownTypes: catalog.wellKnownTypes,
      requestTemplates: catalog.requestTemplates,
      diagnostic: catalog.diagnostic,
      loadedAt: catalog.loadedAt,
      durationMs: catalog.durationMs,
    }
  }

  function chooseInitialMethod(catalog: ActiveCatalog): CatalogMethod | null {
    const allMethods = catalog.services.flatMap((service) => service.methods)
    return allMethods.find((method) => method.rpcType === 'unary') ?? allMethods[0] ?? null
  }

  async function runReflectionLoad(): Promise<void> {
    actionError = ''
    infoMessage = ''
    activeCatalog = null
    selectedMethod = null
    reflectionPending = true

    try {
      const reflectionCatalog = await loadCatalogFromReflection({ endpoint: buildEndpointPreset() })
      const nextCatalog = toActiveCatalog('reflection', reflectionCatalog)
      activeCatalog = nextCatalog
      const initialMethod = chooseInitialMethod(nextCatalog)
      if (initialMethod) {
        selectMethod(initialMethod)
      }
      await refreshHistory()
    } catch (error) {
      activeCatalog = null
      actionError = error instanceof Error ? error.message : 'Reflection catalog load failed unexpectedly.'
    } finally {
      reflectionPending = false
    }
  }

  function parseLineList(value: string): string[] {
    return value
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0)
  }

  async function runProtoLoad(): Promise<void> {
    const protoSources = [
      ...parseLineList(protoDirectoriesText).map((path) => ({ type: 'directory' as const, path })),
      ...parseLineList(protoFilesText).map((path) => ({ type: 'file' as const, path })),
    ]

    if (protoSources.length === 0) {
      actionError = 'Add at least one proto directory or file before loading the proto catalog.'
      return
    }

    actionError = ''
    infoMessage = ''
    activeCatalog = null
    selectedMethod = null
    protoPending = true

    try {
      const protoCatalog = await loadCatalogFromProtoSources({
        endpoint: buildEndpointPreset(),
        protoSources,
        importPaths: parseLineList(importPathsText),
      })
      const nextCatalog = toActiveCatalog('proto', protoCatalog)
      activeCatalog = nextCatalog
      const initialMethod = chooseInitialMethod(nextCatalog)
      if (initialMethod) {
        selectMethod(initialMethod)
      }
      await refreshHistory()
    } catch (error) {
      activeCatalog = null
      actionError = error instanceof Error ? error.message : 'Proto catalog load failed unexpectedly.'
    } finally {
      protoPending = false
    }
  }

  async function runUnaryInvoke(): Promise<void> {
    if (!activeCatalog || !selectedMethod) {
      actionError = 'Select a method from the loaded catalog before invoking.'
      return
    }

    actionError = ''
    infoMessage = ''
    invokeResult = null

    if (selectedMethod.rpcType !== 'unary') {
      actionError = 'This invoke surface is still unary-only. Pick a unary method from the catalog.'
      return
    }

    const body = parseBodyText()
    const metadata = parseMetadataText()
    if (body === null || metadata === null) {
      return
    }

    invokePending = true
    try {
      invokeResult = await invokeUnary({
        catalogSource: activeCatalog.kind,
        endpointId: activeCatalog.endpoint.id ?? '',
        method: selectedMethod.fullName,
        metadata,
        body,
      })
      infoMessage =
        invokeResult.finalState === 'closed'
          ? `Unary call saved to history as ${invokeResult.callId}.`
          : `Unary call completed with ${invokeResult.status.code} and was saved to history as ${invokeResult.callId}.`
      await refreshHistory()
      await loadHistoryDetail(invokeResult.callId)
    } catch (error) {
      actionError = error instanceof Error ? error.message : 'Unary call failed unexpectedly.'
      await refreshHistory()
    } finally {
      invokePending = false
    }
  }

  function handleComposerKeydown(event: KeyboardEvent): void {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      event.preventDefault()
      void runUnaryInvoke()
    }
  }

  function formatDuration(durationMs: number | undefined): string {
    if (!durationMs || durationMs <= 0) {
      return '0 ms'
    }

    return `${durationMs} ms`
  }

  function formatTimestamp(value: string | undefined): string {
    if (!value) {
      return 'n/a'
    }

    return new Date(value).toLocaleString()
  }

  function isSelectedMethod(method: CatalogMethod): boolean {
    return selectedMethod?.fullName === method.fullName
  }

  function isSelectedHistory(callId: string): boolean {
    return selectedHistoryCallId === callId
  }
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">Slice 2.1</p>
    <h2 class="section-title">Proto sources and unary flow</h2>
    <p class="section-copy">
      Reflection and local proto sources now feed the same method picker, request templates and unary
      runtime contract, while proto reload stays explicitly manual for MVP.
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

      <label class="field">
        <span>Catalog source</span>
        <select bind:value={selectedCatalogMode}>
          <option value="reflection">reflection</option>
          <option value="proto">proto sources</option>
        </select>
      </label>

      {#if selectedCatalogMode === 'proto'}
        <div class="stack">
          <label class="field field--textarea">
            <span>Proto directories (one per line)</span>
            <textarea rows="4" bind:value={protoDirectoriesText} placeholder="/absolute/path/to/proto"></textarea>
          </label>

          <label class="field field--textarea">
            <span>Proto files (optional, one per line)</span>
            <textarea rows="4" bind:value={protoFilesText} placeholder="/absolute/path/to/service.proto"></textarea>
          </label>

          <label class="field field--textarea">
            <span>Import paths (optional, one per line)</span>
            <textarea rows="4" bind:value={importPathsText} placeholder="/absolute/path/to/import-root"></textarea>
          </label>

          <div class="subtle">Proto catalogs reload only when you click the button below. File watching is intentionally out of scope for this slice.</div>
        </div>
      {/if}

      <div class="pill-row">
        <button class="ghost-button" disabled={testPending || reflectionPending || protoPending || invokePending} on:click={runEndpointTest}>
          {testPending ? 'Testing endpoint…' : 'Run endpoint preflight'}
        </button>
        {#if selectedCatalogMode === 'reflection'}
          <button class="action-button" disabled={testPending || reflectionPending || protoPending || invokePending} on:click={runReflectionLoad}>
            {reflectionPending ? 'Loading reflection…' : 'Load reflection catalog'}
          </button>
        {:else}
          <button class="action-button" disabled={testPending || reflectionPending || protoPending || invokePending} on:click={runProtoLoad}>
            {protoPending ? 'Loading proto catalog…' : 'Load proto catalog'}
          </button>
        {/if}
      </div>

      {#if actionError}
        <div class="empty-state">
          {actionError}
          <div class="subtle">Structured diagnostics still flow into the diagnostics panel when the runtime can classify the failure.</div>
        </div>
      {/if}

      {#if infoMessage}
        <div class="info-banner">{infoMessage}</div>
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
          <h3>Recent unary history</h3>
          <span class="pill">{historyPending ? 'refreshing…' : `${historySummaries.length} calls`}</span>
        </div>

        {#if historySummaries.length === 0}
          <div class="empty-state">Completed unary calls will materialize here with persisted summaries and session log artifacts.</div>
        {:else}
          <div class="history-list">
            {#each historySummaries as summary}
              <button class:selected-row={isSelectedHistory(summary.callId)} class="history-row" on:click={() => loadHistoryDetail(summary.callId)}>
                <strong>{summary.method}</strong>
                <span>{summary.grpcStatusCode || summary.state}</span>
                <span>{formatDuration(summary.durationMs)}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>

  <div class="three-column">
    <div class="card stack">
      <div class="card__header">
        <h3>Method catalog</h3>
        {#if activeCatalog}
          <div class="pill-row">
            <span class="pill pill--accent">{activeCatalog.services.length} services</span>
            <span class="pill">{activeCatalog.kind}</span>
          </div>
        {/if}
      </div>

      {#if !activeCatalog}
        <div class="empty-state">
          {#if selectedCatalogMode === 'reflection'}
            Load reflection to build the service tree, request templates and RPC metadata that drive the unary flow.
          {:else}
            Load local proto sources and import paths to build the same method tree without depending on server reflection.
          {/if}
        </div>
      {:else}
        <div class="stack">
          {#each activeCatalog.services as service}
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
                  <button class:selected-method={isSelectedMethod(method)} class="catalog-method selectable-card" on:click={() => selectMethod(method)}>
                    <div class="catalog-method__head">
                      <strong>{method.name}</strong>
                      <span class="badge">{formatRpcType(method.rpcType)}</span>
                    </div>
                    <div class="subtle">{method.fullName}</div>
                    <div class="catalog-method__types">
                      <span>request: <strong>{method.requestType.fullName}</strong></span>
                    </div>
                    <div class="catalog-method__types">
                      <span>response: <strong>{method.responseType.fullName}</strong></span>
                    </div>
                  </button>
                {/each}
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </div>

    <div class="card stack">
      <div class="card__header">
        <h3>Request composer</h3>
        {#if selectedMethod}
          <span class="pill">{selectedMethod.rpcType}</span>
        {/if}
      </div>

      {#if !selectedMethod}
        <div class="empty-state">Pick a method from the loaded catalog to materialize the starter JSON payload.</div>
      {:else if selectedMethod.rpcType !== 'unary'}
        <div class="empty-state">
          <strong>{selectedMethod.name}</strong> is {formatRpcType(selectedMethod.rpcType)}.
          <div class="subtle">Slice 2.1 still keeps the invoke surface unary-only before widening into streaming layouts.</div>
        </div>
      {:else}
        <div class="stack">
          <div class="table-like">
            <div class="table-row">
              <strong>Method</strong>
              <span>{selectedMethod.fullName}</span>
            </div>
            <div class="table-row">
              <strong>Request type</strong>
              <span>{selectedMethod.requestType.fullName}</span>
            </div>
            <div class="table-row">
              <strong>Response type</strong>
              <span>{selectedMethod.responseType.fullName}</span>
            </div>
          </div>

          <label class="field field--textarea">
            <span>Request body JSON</span>
            <textarea
              rows="14"
              value={requestBodyText}
              on:input={(event) => updateBodyDraft((event.currentTarget as HTMLTextAreaElement).value)}
              on:keydown={handleComposerKeydown}
            ></textarea>
          </label>
          {#if requestBodyError}
            <div class="inline-error">{requestBodyError}</div>
          {/if}

          <label class="field field--textarea">
            <span>Metadata overrides JSON</span>
            <textarea
              rows="6"
              value={metadataText}
              on:input={(event) => updateMetadataDraft((event.currentTarget as HTMLTextAreaElement).value)}
              on:keydown={handleComposerKeydown}
            ></textarea>
          </label>
          {#if metadataError}
            <div class="inline-error">{metadataError}</div>
          {/if}

          <div class="pill-row">
            <button class="ghost-button" on:click={() => selectedMethod && restoreTemplateDraft(selectedMethod)}>Reset to template</button>
            <button class="action-button" disabled={invokePending || reflectionPending || protoPending} on:click={runUnaryInvoke}>
              {invokePending ? 'Invoking unary call…' : 'Invoke unary call'}
            </button>
          </div>

          <div class="subtle">Use <strong>Cmd/Ctrl+Enter</strong> inside the editors to invoke the selected unary method.</div>
        </div>
      {/if}
    </div>

    <div class="card stack">
      <div class="card__header">
        <h3>Response panel</h3>
        {#if invokeResult}
          <span class:badge--warning={invokeResult.status.code !== 'OK'} class="badge">{invokeResult.status.code}</span>
        {/if}
      </div>

      {#if !invokeResult}
        <div class="empty-state">Headers, status, trailers and body appear here after a unary invocation completes.</div>
      {:else}
        <div class="pill-row">
          <span class="pill pill--accent">{invokeResult.finalState}</span>
          <span class="pill">{formatDuration(invokeResult.durationMs)}</span>
          <span class="pill">{invokeResult.callId}</span>
        </div>

        <div class="table-like">
          <div class="table-row">
            <strong>Started</strong>
            <span>{formatTimestamp(invokeResult.startedAt)}</span>
          </div>
          <div class="table-row">
            <strong>Finished</strong>
            <span>{formatTimestamp(invokeResult.finishedAt)}</span>
          </div>
          <div class="table-row">
            <strong>Status</strong>
            <span>{invokeResult.status.code}{invokeResult.status.message ? ` — ${invokeResult.status.message}` : ''}</span>
          </div>
        </div>

        <div class="stack">
          <h4>Response body</h4>
          <pre class="code-block">{formatJsonValue(invokeResult.responseBody)}</pre>
        </div>

        <div class="stack">
          <h4>Headers</h4>
          <pre class="code-block">{formatJsonValue(invokeResult.headers as unknown as JsonValue)}</pre>
        </div>

        <div class="stack">
          <h4>Trailers</h4>
          <pre class="code-block">{formatJsonValue(invokeResult.trailers as unknown as JsonValue)}</pre>
        </div>

        {#if invokeResult.diagnostic}
          <article class="diagnostic-item">
            <div class="diagnostic-item__head">
              <strong>{invokeResult.diagnostic.code}</strong>
              <span class="diagnostic-item__meta">{invokeResult.diagnostic.category}</span>
            </div>
            <div>{invokeResult.diagnostic.message}</div>
          </article>
        {/if}
      {/if}
    </div>
  </div>

  {#if historyDetail || historyDetailPending}
    <div class="card stack">
      <div class="card__header">
        <h3>Persisted history detail</h3>
        {#if historyDetail}
          <span class="pill">{historyDetail.summary.callId}</span>
        {/if}
      </div>

      {#if historyDetailPending}
        <div class="empty-state">Loading persisted session artifact…</div>
      {:else if historyDetail}
        <div class="grid-cards">
          <article class="card list-block">
            <h4>Stored summary</h4>
            <div class="table-like">
              <div class="table-row">
                <strong>Method</strong>
                <span>{historyDetail.summary.method}</span>
              </div>
              <div class="table-row">
                <strong>Status</strong>
                <span>{historyDetail.status.code}</span>
              </div>
              <div class="table-row">
                <strong>Artifacts</strong>
                <span>{historyDetail.summary.sessionLogPath}</span>
              </div>
            </div>
          </article>

          <article class="card list-block">
            <h4>Structured log events</h4>
            <div class="history-events">
              {#each historyDetail.events as event}
                <div class="history-event">
                  <strong>{event.kind}</strong>
                  <span>{event.ts}</span>
                </div>
              {/each}
            </div>
          </article>
        </div>
      {/if}
    </div>
  {/if}
</section>
