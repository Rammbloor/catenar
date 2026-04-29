<script lang="ts">
  import { onMount } from 'svelte'
  import { EventsOn } from '../../../wailsjs/runtime/runtime'
  import type {
    BootstrapData,
    CatalogSourceKind,
    CallInvokeUnaryResult,
    CallStartStreamResult,
    CatalogMethod,
    EndpointPreset,
    EndpointTestResult,
    HistoryCallSummary,
    HistoryGetResult,
    JsonValue,
    ProtoCatalogResult,
    ReflectionCatalogResult,
    RequestSaveResult,
    StreamCompletedEvent,
    StreamErrorEvent,
    StreamEventRecord,
    StreamState,
    StreamStateEvent,
    TLSMode,
    WorkspaceSnapshot,
    WorkspaceValidationIssue,
  } from '../contracts'
  import { getDiagnosticContextRows } from '../diagnostics'
  import {
    BackendResponseError,
    cancelStream,
    createWorkspace,
    getHistory,
    invokeUnary,
    listHistory,
    loadCatalogFromProtoSources,
    loadCatalogFromReflection,
    openWorkspace,
    saveRequest,
    saveWorkspace,
    startStream,
    testEndpoint,
    validateWorkspace,
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
  let streamStartResult: CallStartStreamResult | null = null
  let streamState: StreamState = 'idle'
  let streamEvents: StreamEventRecord[] = []
  let streamError: StreamErrorEvent | null = null
  let streamCompleted: StreamCompletedEvent | null = null
  let historySummaries: HistoryCallSummary[] = []
  let historyDetail: HistoryGetResult | null = null
  let activeWorkspace: WorkspaceSnapshot | null = null
  let selectedHistoryCallId = ''
  let selectedCatalogMode: CatalogSourceKind = 'reflection'
  let workspacePath = ''
  let workspaceName = 'workspace'
  let savedRequestId = ''
  let protoDirectoriesText = ''
  let protoFilesText = ''
  let importPathsText = ''
  let requestBodyText = '{}'
  let metadataText = '{}'
  let requestBodyError = ''
  let metadataError = ''
  let actionError = ''
  let actionErrorDetails: Record<string, string> | undefined
  let infoMessage = ''
  let testPending = false
  let reflectionPending = false
  let protoPending = false
  let invokePending = false
  let streamPending = false
  let cancelPending = false
  let historyPending = false
  let historyDetailPending = false
  let workspacePending = false
  let requestSavePending = false
  let lastEndpointFingerprint = ''
  let lastProtoFingerprint = ''
  let lastCatalogMode: CatalogSourceKind = 'reflection'
  let bodyDrafts: Record<string, string> = {}
  let metadataDrafts: Record<string, string> = {}
  let workspaceIssues: WorkspaceValidationIssue[] = []
  let actionErrorContextRows: Array<{ label: string; value: string }> = []

  onMount(() => {
    void refreshHistory()

    const offState = EventsOn('stream:state', (payload: StreamStateEvent) => {
      if (!acceptStreamEvent(payload.sessionId)) {
        return
      }
      streamState = payload.state
    })
    const offEvent = EventsOn('stream:event', (payload: StreamEventRecord) => {
      if (!acceptStreamEvent(payload.sessionId)) {
        return
      }
      streamEvents = [...streamEvents, payload].sort((left, right) => left.seq - right.seq)
    })
    const offError = EventsOn('stream:error', (payload: StreamErrorEvent) => {
      if (!acceptStreamEvent(payload.sessionId)) {
        return
      }
      streamError = payload
      actionError = payload.error.message
      actionErrorDetails = payload.error.details
    })
    const offCompleted = EventsOn('stream:completed', (payload: StreamCompletedEvent) => {
      if (!acceptStreamEvent(payload.sessionId)) {
        return
      }
      streamCompleted = payload
      streamState = payload.finalState
      infoMessage =
        payload.finalState === 'closed'
          ? `Server stream saved to history as ${payload.callId}.`
          : `Server stream finished as ${payload.finalState} with ${payload.status.code}.`
      void refreshHistory()
      void loadHistoryDetail(payload.callId)
    })

    return () => {
      offState()
      offEvent()
      offError()
      offCompleted()
    }
  })

  $: actionErrorContextRows = getDiagnosticContextRows(actionErrorDetails)

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

  function buildProtoSources() {
    return [
      ...parseLineList(protoDirectoriesText).map((path) => ({ type: 'directory' as const, path })),
      ...parseLineList(protoFilesText).map((path) => ({ type: 'file' as const, path })),
    ]
  }

  function buildWorkspaceSaveInput() {
    return {
      name: workspaceName.trim() || activeWorkspace?.name || 'workspace',
      endpoints: [buildEndpointPreset()],
      protoSources: buildProtoSources(),
      importPaths: parseLineList(importPathsText),
    }
  }

  function applyWorkspaceSnapshot(snapshot: WorkspaceSnapshot): void {
    activeWorkspace = snapshot
    workspacePath = snapshot.path
    workspaceName = snapshot.name
    workspaceIssues = []

    if (snapshot.endpoints.length > 0) {
      applyEndpointPreset(snapshot.endpoints[0])
    }

    protoDirectoriesText = snapshot.protoSources
      .filter((source) => source.type === 'directory')
      .map((source) => source.path)
      .join('\n')
    protoFilesText = snapshot.protoSources
      .filter((source) => source.type === 'file')
      .map((source) => source.path)
      .join('\n')
    importPathsText = (snapshot.importPaths ?? []).join('\n')
    lastProtoFingerprint = buildProtoInputFingerprint()
  }

  function applyEndpointPreset(nextEndpoint: EndpointPreset): void {
    endpoint = {
      ...nextEndpoint,
      authority: nextEndpoint.authority ?? '',
      tls: {
        mode: nextEndpoint.tls.mode,
        serverNameOverride: nextEndpoint.tls.serverNameOverride ?? '',
        caCert: nextEndpoint.tls.caCert ?? '',
        clientCert: nextEndpoint.tls.clientCert ?? '',
        clientKey: nextEndpoint.tls.clientKey ?? '',
      },
      metadataDefaults: nextEndpoint.metadataDefaults ?? {},
    }
    lastEndpointFingerprint = buildEndpointFingerprint()
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
      clearStreamView()
      historyDetail = null
      selectedHistoryCallId = ''
      actionError = ''
      actionErrorDetails = undefined
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
    clearStreamView()
    actionError = ''
    actionErrorDetails = undefined
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

  function clearActionError(): void {
    actionError = ''
    actionErrorDetails = undefined
  }

  function setActionError(error: unknown, fallback: string): void {
    if (error instanceof BackendResponseError) {
      actionError = error.message
      actionErrorDetails = error.details
      return
    }

    actionError = error instanceof Error ? error.message : fallback
    actionErrorDetails = undefined
  }

  function setWorkspaceResultMessage(snapshot: WorkspaceSnapshot, action: string): void {
    const backupSuffix =
      snapshot.backupPaths && snapshot.backupPaths.length > 0 ? ` Backup: ${snapshot.backupPaths[0]}.` : ''
    infoMessage = `${action} ${snapshot.name} at ${snapshot.manifestPath}.${backupSuffix}`
  }

  async function runWorkspaceCreate(): Promise<void> {
    clearActionError()
    infoMessage = ''
    workspaceIssues = []
    workspacePending = true

    try {
      const result = await createWorkspace({
        path: workspacePath,
        ...buildWorkspaceSaveInput(),
      })
      applyWorkspaceSnapshot(result.workspace)
      setWorkspaceResultMessage(result.workspace, 'Created workspace')
    } catch (error) {
      setActionError(error, 'Workspace could not be created.')
    } finally {
      workspacePending = false
    }
  }

  async function runWorkspaceOpen(): Promise<void> {
    clearActionError()
    infoMessage = ''
    workspaceIssues = []
    workspacePending = true

    try {
      const result = await openWorkspace(workspacePath)
      applyWorkspaceSnapshot(result.workspace)
      activeCatalog = null
      selectedMethod = null
      invokeResult = null
      setWorkspaceResultMessage(result.workspace, 'Opened workspace')
    } catch (error) {
      setActionError(error, 'Workspace could not be opened.')
    } finally {
      workspacePending = false
    }
  }

  async function runWorkspaceSave(): Promise<void> {
    clearActionError()
    infoMessage = ''
    workspaceIssues = []
    workspacePending = true

    try {
      const result = await saveWorkspace(buildWorkspaceSaveInput())
      applyWorkspaceSnapshot(result.workspace)
      setWorkspaceResultMessage(result.workspace, 'Saved workspace')
    } catch (error) {
      setActionError(error, 'Workspace could not be saved.')
    } finally {
      workspacePending = false
    }
  }

  async function runWorkspaceValidate(): Promise<void> {
    clearActionError()
    infoMessage = ''
    workspaceIssues = []
    workspacePending = true

    try {
      const result = await validateWorkspace(buildWorkspaceSaveInput())
      workspaceIssues = result.issues
      infoMessage =
        result.issues.length === 0
          ? 'Workspace draft passed open/save validation.'
          : `Workspace draft has ${result.issues.length} validation issue${result.issues.length === 1 ? '' : 's'}.`
    } catch (error) {
      setActionError(error, 'Workspace could not be validated.')
    } finally {
      workspacePending = false
    }
  }

  async function loadHistoryDetail(callId: string): Promise<void> {
    selectedHistoryCallId = callId
    historyDetailPending = true
    try {
      historyDetail = await getHistory(callId)
    } catch (error) {
      historyDetail = null
      setActionError(error, 'History detail could not be loaded.')
    } finally {
      historyDetailPending = false
    }
  }

  async function runEndpointTest(): Promise<void> {
    clearActionError()
    infoMessage = ''
    endpointTestResult = null
    testPending = true

    try {
      endpointTestResult = await testEndpoint({ endpoint: buildEndpointPreset() })
      applyEndpointPreset(endpointTestResult.endpoint)
    } catch (error) {
      setActionError(error, 'Endpoint preflight failed unexpectedly.')
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
    clearActionError()
    infoMessage = ''
    activeCatalog = null
    selectedMethod = null
    reflectionPending = true

    try {
      const reflectionCatalog = await loadCatalogFromReflection({ endpoint: buildEndpointPreset() })
      applyEndpointPreset(reflectionCatalog.endpoint)
      const nextCatalog = toActiveCatalog('reflection', reflectionCatalog)
      activeCatalog = nextCatalog
      const initialMethod = chooseInitialMethod(nextCatalog)
      if (initialMethod) {
        selectMethod(initialMethod)
      }
      await refreshHistory()
    } catch (error) {
      activeCatalog = null
      setActionError(error, 'Reflection catalog load failed unexpectedly.')
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
    const protoSources = buildProtoSources()

    if (protoSources.length === 0) {
      actionError = 'Add at least one proto directory or file before loading the proto catalog.'
      actionErrorDetails = undefined
      return
    }

    clearActionError()
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
      applyEndpointPreset(protoCatalog.endpoint)
      const nextCatalog = toActiveCatalog('proto', protoCatalog)
      activeCatalog = nextCatalog
      const initialMethod = chooseInitialMethod(nextCatalog)
      if (initialMethod) {
        selectMethod(initialMethod)
      }
      await refreshHistory()
    } catch (error) {
      activeCatalog = null
      setActionError(error, 'Proto catalog load failed unexpectedly.')
    } finally {
      protoPending = false
    }
  }

  async function runUnaryInvoke(): Promise<void> {
    if (!activeCatalog || !selectedMethod) {
      actionError = 'Select a method from the loaded catalog before invoking.'
      actionErrorDetails = undefined
      return
    }

    clearActionError()
    infoMessage = ''
    invokeResult = null
    clearStreamView()

    if (selectedMethod.rpcType !== 'unary') {
      actionError = 'This invoke surface is still unary-only. Pick a unary method from the catalog.'
      actionErrorDetails = undefined
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
      setActionError(error, 'Unary call failed unexpectedly.')
      await refreshHistory()
    } finally {
      invokePending = false
    }
  }

  async function runServerStreamInvoke(): Promise<void> {
    if (!activeCatalog || !selectedMethod) {
      actionError = 'Select a method from the loaded catalog before starting a stream.'
      actionErrorDetails = undefined
      return
    }

    clearActionError()
    infoMessage = ''
    invokeResult = null
    clearStreamView()

    if (selectedMethod.rpcType !== 'server_stream') {
      actionError = 'Slice 3.1 can start server-streaming methods only.'
      actionErrorDetails = undefined
      return
    }

    const body = parseBodyText()
    const metadata = parseMetadataText()
    if (body === null || metadata === null) {
      return
    }

    streamPending = true
    try {
      const started = await startStream({
        catalogSource: activeCatalog.kind,
        endpointId: activeCatalog.endpoint.id ?? '',
        method: selectedMethod.fullName,
        rpcType: selectedMethod.rpcType,
        metadata,
        requestSpec: {
          mode: 'static-sequence',
          messages: [{ body }],
        },
      })
      streamStartResult = started
      if (!streamCompleted || streamCompleted.sessionId !== started.sessionId) {
        streamState = started.state
        infoMessage = `Server stream started as ${started.callId}.`
      }
    } catch (error) {
      setActionError(error, 'Server stream could not be started.')
      await refreshHistory()
    } finally {
      streamPending = false
    }
  }

  async function runStreamCancel(): Promise<void> {
    if (!streamStartResult || isTerminalState(streamState)) {
      return
    }

    clearActionError()
    cancelPending = true
    try {
      await cancelStream({ sessionId: streamStartResult.sessionId })
      infoMessage = `Cancel requested for ${streamStartResult.callId}.`
    } catch (error) {
      setActionError(error, 'Stream could not be cancelled.')
    } finally {
      cancelPending = false
    }
  }

  function defaultRequestId(method: CatalogMethod): string {
    return method.fullName.replaceAll('/', '.').replace(/[^a-zA-Z0-9_.-]+/g, '-').toLowerCase()
  }

  async function runRequestSave(): Promise<void> {
    if (!activeCatalog || !selectedMethod) {
      actionError = 'Select a loaded method before saving a reusable request.'
      actionErrorDetails = undefined
      return
    }

    clearActionError()
    infoMessage = ''

    if (selectedMethod.rpcType !== 'unary') {
      actionError = 'Saved request persistence is wired to the unary request composer in this slice.'
      actionErrorDetails = undefined
      return
    }

    const body = parseBodyText()
    const metadata = parseMetadataText()
    if (body === null || metadata === null) {
      return
    }

    requestSavePending = true
    try {
      const result: RequestSaveResult = await saveRequest({
        id: savedRequestId.trim() || defaultRequestId(selectedMethod),
        method: selectedMethod.fullName,
        rpcType: selectedMethod.rpcType,
        endpointRef: activeCatalog.endpoint.id ?? endpoint.id ?? '',
        metadataTemplate: metadata,
        requestSpec: {
          mode: 'single',
          body,
        },
      })
      applyWorkspaceSnapshot(result.workspace)
      savedRequestId = result.savedRequest.id
      infoMessage = `Saved request ${result.savedRequest.id} to ${result.savedRequest.path}.`
    } catch (error) {
      setActionError(error, 'Request could not be saved.')
    } finally {
      requestSavePending = false
    }
  }

  function handleComposerKeydown(event: KeyboardEvent): void {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      event.preventDefault()
      if (selectedMethod?.rpcType === 'server_stream') {
        void runServerStreamInvoke()
        return
      }
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

  function clearStreamView(): void {
    streamStartResult = null
    streamState = 'idle'
    streamEvents = []
    streamError = null
    streamCompleted = null
  }

  function acceptStreamEvent(sessionId: string): boolean {
    return streamStartResult?.sessionId === sessionId || (streamPending && streamStartResult === null)
  }

  function isTerminalState(state: StreamState): boolean {
    return state === 'closed' || state === 'cancelled' || state === 'error'
  }

  function formatStreamEventPreview(event: StreamEventRecord): string {
    return formatJsonValue(event.payload.preview.json)
  }
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">Slice 3.1</p>
    <h2 class="section-title">Server streaming</h2>
    <p class="section-copy">
      Reflection and proto-loaded server-streaming methods now use the live event bus, timeline and persisted history artifacts.
    </p>
  </div>

  <div class="card stack">
    <div class="card__header">
      <h3>Workspace file</h3>
      <div class="pill-row">
        <span class="pill" class:pill--accent={activeWorkspace?.version === 1}>
          {activeWorkspace ? `v${activeWorkspace.version}` : 'not open'}
        </span>
        {#if activeWorkspace}
          <span class="pill">{activeWorkspace.savedRequests?.length ?? 0} requests</span>
        {/if}
      </div>
    </div>

    <div class="form-grid">
      <label class="field">
        <span>Workspace path</span>
        <input bind:value={workspacePath} placeholder="/absolute/path/to/workspace" />
      </label>

      <label class="field">
        <span>Name</span>
        <input bind:value={workspaceName} placeholder="workspace name" />
      </label>
    </div>

    <div class="pill-row">
      <button class="ghost-button" disabled={workspacePending} on:click={runWorkspaceCreate}>
        {workspacePending ? 'Working…' : 'Create'}
      </button>
      <button class="ghost-button" disabled={workspacePending} on:click={runWorkspaceOpen}>
        {workspacePending ? 'Working…' : 'Open'}
      </button>
      <button class="action-button" disabled={workspacePending || !activeWorkspace} on:click={runWorkspaceSave}>
        {workspacePending ? 'Saving…' : 'Save'}
      </button>
      <button class="ghost-button" disabled={workspacePending || !activeWorkspace} on:click={runWorkspaceValidate}>
        {workspacePending ? 'Checking…' : 'Validate'}
      </button>
    </div>

    {#if activeWorkspace}
      <div class="table-like">
        <div class="table-row">
          <strong>Manifest</strong>
          <span>{activeWorkspace.manifestPath}</span>
        </div>
        <div class="table-row">
          <strong>Workspace id</strong>
          <span>{activeWorkspace.id}</span>
        </div>
      </div>
    {/if}

    {#if workspaceIssues.length > 0}
      <div class="empty-state">
        <div class="table-like">
          {#each workspaceIssues as issue}
            <div class="table-row">
              <strong>{issue.code}</strong>
              <span>{issue.field}: {issue.message}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if activeWorkspace?.savedRequests && activeWorkspace.savedRequests.length > 0}
      <div class="history-list">
        {#each activeWorkspace.savedRequests as request}
          <div class="history-row">
            <strong>{request.id}</strong>
            <span>{request.method}</span>
            <span>{request.path}</span>
          </div>
        {/each}
      </div>
    {/if}
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
          <div>{actionError}</div>
          {#if actionErrorContextRows.length > 0}
            <div class="table-like">
              {#each actionErrorContextRows as detail}
                <div class="table-row">
                  <strong>{detail.label}</strong>
                  <span>{detail.value}</span>
                </div>
              {/each}
            </div>
          {/if}
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
          <h3>Recent call history</h3>
          <span class="pill">{historyPending ? 'refreshing…' : `${historySummaries.length} calls`}</span>
        </div>

        {#if historySummaries.length === 0}
          <div class="empty-state">Completed calls will materialize here with persisted summaries and session log artifacts.</div>
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
      {:else if selectedMethod.rpcType !== 'unary' && selectedMethod.rpcType !== 'server_stream'}
        <div class="empty-state">
          <strong>{selectedMethod.name}</strong> is {formatRpcType(selectedMethod.rpcType)}.
          <div class="subtle">Slice 3.1 supports unary and server-streaming methods before widening into client and bidi streaming.</div>
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

          <label class="field">
            <span>Saved request id</span>
            <input bind:value={savedRequestId} placeholder={defaultRequestId(selectedMethod)} />
          </label>

          <div class="pill-row">
            <button class="ghost-button" on:click={() => selectedMethod && restoreTemplateDraft(selectedMethod)}>Reset to template</button>
            {#if selectedMethod.rpcType === 'unary'}
              <button class="ghost-button" disabled={requestSavePending || !activeWorkspace} on:click={runRequestSave}>
                {requestSavePending ? 'Saving request…' : 'Save request'}
              </button>
              <button class="action-button" disabled={invokePending || reflectionPending || protoPending} on:click={runUnaryInvoke}>
                {invokePending ? 'Invoking unary call…' : 'Invoke unary call'}
              </button>
            {:else}
              <button
                class="action-button"
                disabled={streamPending || reflectionPending || protoPending || (streamStartResult !== null && !isTerminalState(streamState))}
                on:click={runServerStreamInvoke}
              >
                {streamPending ? 'Starting stream…' : 'Start server stream'}
              </button>
              <button
                class="ghost-button"
                disabled={cancelPending || !streamStartResult || isTerminalState(streamState)}
                on:click={runStreamCancel}
              >
                {cancelPending ? 'Cancelling…' : 'Cancel stream'}
              </button>
            {/if}
          </div>

          <div class="subtle">Use <strong>Cmd/Ctrl+Enter</strong> inside the editors to run the selected unary or server-streaming method.</div>
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

      {#if streamStartResult}
        <div class="pill-row">
          <span class="pill" class:pill--accent={streamState === 'open'}>{streamState}</span>
          <span class="pill">{streamStartResult.callId}</span>
          {#if streamCompleted}
            <span class:badge--warning={streamCompleted.status.code !== 'OK'} class="badge">{streamCompleted.status.code}</span>
          {/if}
        </div>

        {#if streamError}
          <article class="diagnostic-item">
            <div class="diagnostic-item__head">
              <strong>{streamError.error.code}</strong>
              <span class="diagnostic-item__meta">{streamError.error.category}</span>
            </div>
            <div>{streamError.error.message}</div>
          </article>
        {/if}

        <div class="stream-timeline">
          {#if streamEvents.length === 0}
            <div class="empty-state">Live stream events will append here as headers, messages and trailers arrive.</div>
          {:else}
            {#each streamEvents as event}
              <article class="history-event">
                <div class="diagnostic-item__head">
                  <strong>{event.kind}</strong>
                  <span class="diagnostic-item__meta">#{event.seq} {event.direction}</span>
                </div>
                <span>{formatTimestamp(event.ts)}</span>
                <pre class="code-block">{formatStreamEventPreview(event)}</pre>
              </article>
            {/each}
          {/if}
        </div>
      {:else if !invokeResult}
        <div class="empty-state">Headers, status, trailers, unary body and streaming timeline events appear here after invocation starts.</div>
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
