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
    StreamMessage,
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
    i18n,
    translateDiagnosticMessage,
    translateEndpointCheckMessage,
    translateEndpointCheckOutcome,
    translateEndpointCheckStage,
    translateStreamStateLabel,
  } from '../i18n'
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
      const rpcType = streamStartResult?.rpcType ? formatRpcType(streamStartResult.rpcType) : $i18n.t('footer.stream')
      infoMessage =
        payload.finalState === 'closed'
          ? $i18n.t('stream.completedClosed', { rpcType, callId: payload.callId })
          : $i18n.t('stream.completedOther', { rpcType, state: payload.finalState, status: payload.status.code })
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

  $: actionErrorContextRows = getDiagnosticContextRows(actionErrorDetails, $i18n.language)

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
    return $i18n.t(`method.rpc.${rpcType}`)
  }

  function formatCatalogSource(kind: CatalogSourceKind): string {
    return $i18n.t(`source.${kind}`)
  }

  function formatTLSMode(mode: TLSMode): string {
    return $i18n.t(`tls.${mode}`)
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

  function defaultDraftForMethod(method: CatalogMethod): string {
    const template = activeCatalog?.requestTemplates?.[method.fullName]
    if (method.rpcType === 'client_stream') {
      return formatJsonValue([template ?? {}])
    }

    return formatJsonValue(template)
  }

  function restoreTemplateDraft(method: CatalogMethod): void {
    requestBodyText = defaultDraftForMethod(method)
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
    requestBodyText = bodyDrafts[key] ?? defaultDraftForMethod(method)
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
      requestBodyError = error instanceof Error ? error.message : $i18n.t('errors.requestBodyJson')
      return null
    }
  }

  function parseClientStreamMessages(): StreamMessage[] | null {
    requestBodyError = ''

    try {
      const parsed = JSON.parse(requestBodyText) as JsonValue
      if (!Array.isArray(parsed)) {
        requestBodyError = $i18n.t('errors.clientStreamMessagesArray')
        return null
      }
      if (parsed.length === 0) {
        requestBodyError = $i18n.t('errors.clientStreamMessagesRequired')
        return null
      }

      return parsed.map((body) => ({ body }))
    } catch (error) {
      requestBodyError = error instanceof Error ? error.message : $i18n.t('errors.requestBodyJson')
      return null
    }
  }

  function parseMetadataText(): Record<string, string> | null {
    metadataError = ''

    try {
      const parsed = JSON.parse(metadataText) as JsonValue
      if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') {
        metadataError = $i18n.t('errors.metadataObject')
        return null
      }

      const metadataEntries = Object.entries(parsed)
      const result: Record<string, string> = {}
      for (const [key, value] of metadataEntries) {
        if (typeof value !== 'string') {
          metadataError = $i18n.t('errors.metadataStringValue', { key })
          return null
        }
        result[key] = value
      }

      return result
    } catch (error) {
      metadataError = error instanceof Error ? error.message : $i18n.t('errors.metadataValidJson')
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
      const translated = error.code
        ? translateDiagnosticMessage($i18n.language, error.code, error.backendMessage)
        : error.backendMessage
      actionError = error.code ? `${error.code}: ${translated}` : translated
      actionErrorDetails = error.details
      return
    }

    actionError = error instanceof Error ? error.message : fallback
    actionErrorDetails = undefined
  }

  function setWorkspaceResultMessage(snapshot: WorkspaceSnapshot, messageKey: string): void {
    const backupSuffix =
      snapshot.backupPaths && snapshot.backupPaths.length > 0
        ? $i18n.t('workspace.backupSuffix', { path: snapshot.backupPaths[0] })
        : ''
    infoMessage = $i18n.t(messageKey, {
      name: snapshot.name,
      manifestPath: snapshot.manifestPath,
      backup: backupSuffix,
    })
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
      setWorkspaceResultMessage(result.workspace, 'workspace.created')
    } catch (error) {
      setActionError(error, $i18n.t('errors.workspaceCreate'))
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
      setWorkspaceResultMessage(result.workspace, 'workspace.opened')
    } catch (error) {
      setActionError(error, $i18n.t('errors.workspaceOpen'))
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
      setWorkspaceResultMessage(result.workspace, 'workspace.saved')
    } catch (error) {
      setActionError(error, $i18n.t('errors.workspaceSave'))
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
          ? $i18n.t('workspace.validationPassed')
          : $i18n.t('workspace.validationIssues', { count: result.issues.length })
    } catch (error) {
      setActionError(error, $i18n.t('errors.workspaceValidate'))
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
      setActionError(error, $i18n.t('errors.historyDetail'))
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
      setActionError(error, $i18n.t('errors.endpointPreflight'))
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
      setActionError(error, $i18n.t('errors.reflectionCatalog'))
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
      actionError = $i18n.t('errors.addProtoSource')
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
      setActionError(error, $i18n.t('errors.protoCatalog'))
    } finally {
      protoPending = false
    }
  }

  async function runUnaryInvoke(): Promise<void> {
    if (!activeCatalog || !selectedMethod) {
      actionError = $i18n.t('errors.selectMethodInvoke')
      actionErrorDetails = undefined
      return
    }

    clearActionError()
    infoMessage = ''
    invokeResult = null
    clearStreamView()

    if (selectedMethod.rpcType !== 'unary') {
      actionError = $i18n.t('errors.unaryOnly')
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
          ? $i18n.t('call.unarySaved', { callId: invokeResult.callId })
          : $i18n.t('call.unarySavedWithStatus', { status: invokeResult.status.code, callId: invokeResult.callId })
      await refreshHistory()
      await loadHistoryDetail(invokeResult.callId)
    } catch (error) {
      setActionError(error, $i18n.t('errors.unaryInvoke'))
      await refreshHistory()
    } finally {
      invokePending = false
    }
  }

  async function runServerStreamInvoke(): Promise<void> {
    if (!activeCatalog || !selectedMethod) {
      actionError = $i18n.t('errors.selectMethodStream')
      actionErrorDetails = undefined
      return
    }

    clearActionError()
    infoMessage = ''
    invokeResult = null
    clearStreamView()

    if (selectedMethod.rpcType !== 'server_stream') {
      actionError = $i18n.t('errors.streamUnaryOnly')
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
        infoMessage = $i18n.t('stream.started', { rpcType: formatRpcType(started.rpcType), callId: started.callId })
      }
    } catch (error) {
      setActionError(error, $i18n.t('errors.streamStart'))
      await refreshHistory()
    } finally {
      streamPending = false
    }
  }

  async function runClientStreamInvoke(): Promise<void> {
    if (!activeCatalog || !selectedMethod) {
      actionError = $i18n.t('errors.selectMethodStream')
      actionErrorDetails = undefined
      return
    }

    clearActionError()
    infoMessage = ''
    invokeResult = null
    clearStreamView()

    if (selectedMethod.rpcType !== 'client_stream') {
      actionError = $i18n.t('errors.streamUnaryOnly')
      actionErrorDetails = undefined
      return
    }

    const messages = parseClientStreamMessages()
    const metadata = parseMetadataText()
    if (messages === null || metadata === null) {
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
          messages,
        },
      })
      streamStartResult = started
      if (!streamCompleted || streamCompleted.sessionId !== started.sessionId) {
        streamState = started.state
        infoMessage = $i18n.t('stream.started', { rpcType: formatRpcType(started.rpcType), callId: started.callId })
      }
    } catch (error) {
      setActionError(error, $i18n.t('errors.clientStreamStart'))
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
      infoMessage = $i18n.t('stream.cancelRequested', { callId: streamStartResult.callId })
    } catch (error) {
      setActionError(error, $i18n.t('errors.streamCancel'))
    } finally {
      cancelPending = false
    }
  }

  function defaultRequestId(method: CatalogMethod): string {
    return method.fullName.replaceAll('/', '.').replace(/[^a-zA-Z0-9_.-]+/g, '-').toLowerCase()
  }

  async function runRequestSave(): Promise<void> {
    if (!activeCatalog || !selectedMethod) {
      actionError = $i18n.t('errors.requestSaveLoadedMethod')
      actionErrorDetails = undefined
      return
    }

    clearActionError()
    infoMessage = ''

    if (selectedMethod.rpcType !== 'unary') {
      actionError = $i18n.t('errors.requestSaveUnaryOnly')
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
      infoMessage = $i18n.t('call.requestSaved', { id: result.savedRequest.id, path: result.savedRequest.path })
    } catch (error) {
      setActionError(error, $i18n.t('errors.requestSave'))
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
      if (selectedMethod?.rpcType === 'client_stream') {
        void runClientStreamInvoke()
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
      return $i18n.t('common.notAvailable')
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

  function supportsStaticComposer(method: CatalogMethod): boolean {
    return method.rpcType === 'unary' || method.rpcType === 'server_stream' || method.rpcType === 'client_stream'
  }

  function bodyEditorLabel(method: CatalogMethod): string {
    return method.rpcType === 'client_stream' ? $i18n.t('request.clientSequenceJson') : $i18n.t('request.bodyJson')
  }

  function formatStreamEventPreview(event: StreamEventRecord): string {
    return formatJsonValue(event.payload.preview.json)
  }

  function formatCheckStage(stage: string): string {
    return translateEndpointCheckStage($i18n.language, stage)
  }

  function formatCheckOutcome(outcome: string): string {
    return translateEndpointCheckOutcome($i18n.language, outcome)
  }

  function formatCheckMessage(stage: string, outcome: string, message: string): string {
    return translateEndpointCheckMessage($i18n.language, stage, outcome, message)
  }
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">{$i18n.t('workspaceView.eyebrow')}</p>
    <h2 class="section-title">{$i18n.t('workspaceView.title')}</h2>
    <p class="section-copy">
      {$i18n.t('workspaceView.copy')}
    </p>
  </div>

  <div class="card stack">
    <div class="card__header">
      <h3>{$i18n.t('workspace.fileTitle')}</h3>
      <div class="pill-row">
        <span class="pill" class:pill--accent={activeWorkspace?.version === 1}>
          {activeWorkspace ? `v${activeWorkspace.version}` : $i18n.t('common.notOpen')}
        </span>
        {#if activeWorkspace}
          <span class="pill">{$i18n.t('common.requestsCount', { count: activeWorkspace.savedRequests?.length ?? 0 })}</span>
        {/if}
      </div>
    </div>

    <div class="form-grid">
      <label class="field">
        <span>{$i18n.t('workspace.path')}</span>
        <input bind:value={workspacePath} placeholder={$i18n.t('workspace.pathPlaceholder')} />
      </label>

      <label class="field">
        <span>{$i18n.t('workspace.name')}</span>
        <input bind:value={workspaceName} placeholder={$i18n.t('workspace.namePlaceholder')} />
      </label>
    </div>

    <div class="pill-row">
      <button class="ghost-button" disabled={workspacePending} on:click={runWorkspaceCreate}>
        {workspacePending ? $i18n.t('common.working') : $i18n.t('common.create')}
      </button>
      <button class="ghost-button" disabled={workspacePending} on:click={runWorkspaceOpen}>
        {workspacePending ? $i18n.t('common.working') : $i18n.t('common.open')}
      </button>
      <button class="action-button" disabled={workspacePending || !activeWorkspace} on:click={runWorkspaceSave}>
        {workspacePending ? $i18n.t('common.saving') : $i18n.t('common.save')}
      </button>
      <button class="ghost-button" disabled={workspacePending || !activeWorkspace} on:click={runWorkspaceValidate}>
        {workspacePending ? $i18n.t('common.checking') : $i18n.t('common.validate')}
      </button>
    </div>

    {#if activeWorkspace}
      <div class="table-like">
        <div class="table-row">
          <strong>{$i18n.t('workspace.manifest')}</strong>
          <span>{activeWorkspace.manifestPath}</span>
        </div>
        <div class="table-row">
          <strong>{$i18n.t('workspace.id')}</strong>
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
        <h3>{$i18n.t('endpoint.title')}</h3>
        <div class="pill-row">
          <span class="pill pill--accent">{$i18n.t('common.bindingsCount', { count: bootstrap.contract.boundMethods.length })}</span>
          <span class="pill">{contractMismatch.length === 0 ? $i18n.t('common.contractVerified') : $i18n.t('common.contractDrift')}</span>
        </div>
      </div>

      <div class="form-grid">
        <label class="field">
          <span>{$i18n.t('endpoint.target')}</span>
          <input bind:value={endpoint.target} placeholder="127.0.0.1:50051" />
        </label>

        <label class="field">
          <span>{$i18n.t('endpoint.authority')}</span>
          <input bind:value={endpoint.authority} placeholder={$i18n.t('endpoint.authorityPlaceholder')} />
        </label>

        <label class="field">
          <span>{$i18n.t('endpoint.tlsMode')}</span>
          <select bind:value={endpoint.tls.mode}>
            <option value="plaintext">{formatTLSMode('plaintext')}</option>
            <option value="system_ca">{formatTLSMode('system_ca')}</option>
            <option value="custom_ca">{formatTLSMode('custom_ca')}</option>
            <option value="mtls">{formatTLSMode('mtls')}</option>
          </select>
        </label>

        <label class="field">
          <span>{$i18n.t('endpoint.serverNameOverride')}</span>
          <input bind:value={endpoint.tls.serverNameOverride} placeholder={$i18n.t('endpoint.serverNamePlaceholder')} />
        </label>

        <label class="field">
          <span>{$i18n.t('endpoint.connectTimeout')}</span>
          <input bind:value={endpoint.connectTimeoutMs} min="1" type="number" />
        </label>

        <label class="field">
          <span>{$i18n.t('endpoint.requestTimeout')}</span>
          <input bind:value={endpoint.requestTimeoutMs} min="0" type="number" />
        </label>

        <label class="field">
          <span>{$i18n.t('endpoint.streamIdleTimeout')}</span>
          <input bind:value={endpoint.streamIdleTimeoutMs} min="0" type="number" />
        </label>

        {#if isTLSMode('custom_ca') || isTLSMode('mtls')}
          <label class="field field--span-2">
            <span>{$i18n.t('endpoint.caSecretRef')}</span>
            <input bind:value={endpoint.tls.caCert} placeholder="secret-ref:file/tls/ca.pem" />
          </label>
        {/if}

        {#if isTLSMode('mtls')}
          <label class="field">
            <span>{$i18n.t('endpoint.clientCertSecretRef')}</span>
            <input bind:value={endpoint.tls.clientCert} placeholder="secret-ref:file/tls/client.crt" />
          </label>

          <label class="field">
            <span>{$i18n.t('endpoint.clientKeySecretRef')}</span>
            <input bind:value={endpoint.tls.clientKey} placeholder="secret-ref:file/tls/client.key" />
          </label>
        {/if}
      </div>

      <label class="field">
        <span>{$i18n.t('endpoint.catalogSource')}</span>
        <select bind:value={selectedCatalogMode}>
          <option value="reflection">{$i18n.t('source.reflection')}</option>
          <option value="proto">{$i18n.t('source.proto')}</option>
        </select>
      </label>

      {#if selectedCatalogMode === 'proto'}
        <div class="stack">
          <label class="field field--textarea">
            <span>{$i18n.t('endpoint.protoDirectories')}</span>
            <textarea rows="4" bind:value={protoDirectoriesText} placeholder={$i18n.t('endpoint.protoDirectoriesPlaceholder')}></textarea>
          </label>

          <label class="field field--textarea">
            <span>{$i18n.t('endpoint.protoFiles')}</span>
            <textarea rows="4" bind:value={protoFilesText} placeholder={$i18n.t('endpoint.protoFilesPlaceholder')}></textarea>
          </label>

          <label class="field field--textarea">
            <span>{$i18n.t('endpoint.importPaths')}</span>
            <textarea rows="4" bind:value={importPathsText} placeholder={$i18n.t('endpoint.importPathsPlaceholder')}></textarea>
          </label>

          <div class="subtle">{$i18n.t('endpoint.protoNote')}</div>
        </div>
      {/if}

      <div class="pill-row">
        <button class="ghost-button" disabled={testPending || reflectionPending || protoPending || invokePending} on:click={runEndpointTest}>
          {testPending ? $i18n.t('endpoint.testPending') : $i18n.t('endpoint.runPreflight')}
        </button>
        {#if selectedCatalogMode === 'reflection'}
          <button class="action-button" disabled={testPending || reflectionPending || protoPending || invokePending} on:click={runReflectionLoad}>
            {reflectionPending ? $i18n.t('endpoint.loadReflectionPending') : $i18n.t('endpoint.loadReflection')}
          </button>
        {:else}
          <button class="action-button" disabled={testPending || reflectionPending || protoPending || invokePending} on:click={runProtoLoad}>
            {protoPending ? $i18n.t('endpoint.loadProtoPending') : $i18n.t('endpoint.loadProto')}
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
          <div class="subtle">{$i18n.t('workspace.infoDiagnostics')}</div>
        </div>
      {/if}

      {#if infoMessage}
        <div class="info-banner">{infoMessage}</div>
      {/if}
    </div>

    <div class="stack">
      <div class="card stack">
        <h3>{$i18n.t('endpoint.preflightTitle')}</h3>
        {#if !endpointTestResult}
          <div class="empty-state">{$i18n.t('endpoint.preflightEmpty')}</div>
        {:else}
          <div class="pill-row">
            <span class="pill" class:pill--accent={endpointTestResult.transportReachable}>
              {$i18n.t('endpoint.transport')} {endpointTestResult.transportReachable ? $i18n.t('endpoint.transportReachable') : $i18n.t('endpoint.transportBlocked')}
            </span>
            <span class="pill" class:pill--accent={endpointTestResult.grpcReadyProven}>
              gRPC {endpointTestResult.grpcReadyProven ? $i18n.t('endpoint.grpcReady') : $i18n.t('endpoint.grpcBlocked')}
            </span>
            <span class="pill" class:pill--accent={endpointTestResult.tlsOk || !endpointTestResult.tlsConfigured}>
              TLS {endpointTestResult.tlsConfigured ? (endpointTestResult.tlsOk ? $i18n.t('endpoint.tlsOk') : $i18n.t('endpoint.tlsFailed')) : $i18n.t('endpoint.tlsOff')}
            </span>
          </div>

          <div class="stack">
            {#each endpointTestResult.checks as check}
              <article class="diagnostic-item">
                <div class="diagnostic-item__head">
                  <strong>{formatCheckStage(check.stage)}</strong>
                  <span class="diagnostic-item__meta">{formatCheckOutcome(check.outcome)}</span>
                </div>
                <div>{formatCheckMessage(check.stage, check.outcome, check.message)}</div>
              </article>
            {/each}
          </div>
        {/if}
      </div>

      <div class="card stack">
        <div class="card__header">
          <h3>{$i18n.t('history.recentTitle')}</h3>
          <span class="pill">{historyPending ? $i18n.t('common.refreshing') : $i18n.t('common.callsCount', { count: historySummaries.length })}</span>
        </div>

        {#if historySummaries.length === 0}
          <div class="empty-state">{$i18n.t('history.empty')}</div>
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
        <h3>{$i18n.t('method.catalogTitle')}</h3>
        {#if activeCatalog}
          <div class="pill-row">
            <span class="pill pill--accent">{$i18n.t('common.servicesCount', { count: activeCatalog.services.length })}</span>
            <span class="pill">{formatCatalogSource(activeCatalog.kind)}</span>
          </div>
        {/if}
      </div>

      {#if !activeCatalog}
        <div class="empty-state">
          {#if selectedCatalogMode === 'reflection'}
            {$i18n.t('method.catalogEmptyReflection')}
          {:else}
            {$i18n.t('method.catalogEmptyProto')}
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
                <span class="badge">{$i18n.t('common.methodsCount', { count: service.methods.length })}</span>
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
                      <span>{$i18n.t('method.types.request')} <strong>{method.requestType.fullName}</strong></span>
                    </div>
                    <div class="catalog-method__types">
                      <span>{$i18n.t('method.types.response')} <strong>{method.responseType.fullName}</strong></span>
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
        <h3>{$i18n.t('request.composerTitle')}</h3>
        {#if selectedMethod}
          <span class="pill">{formatRpcType(selectedMethod.rpcType)}</span>
        {/if}
      </div>

      {#if !selectedMethod}
        <div class="empty-state">{$i18n.t('request.empty')}</div>
      {:else if !supportsStaticComposer(selectedMethod)}
        <div class="empty-state">
          <strong>{$i18n.t('request.unsupportedMethod', { method: selectedMethod.name, rpcType: formatRpcType(selectedMethod.rpcType) })}</strong>
          <div class="subtle">{$i18n.t('request.unsupported')}</div>
        </div>
      {:else}
        <div class="stack">
          <div class="table-like">
            <div class="table-row">
              <strong>{$i18n.t('method.method')}</strong>
              <span>{selectedMethod.fullName}</span>
            </div>
            <div class="table-row">
              <strong>{$i18n.t('method.requestType')}</strong>
              <span>{selectedMethod.requestType.fullName}</span>
            </div>
            <div class="table-row">
              <strong>{$i18n.t('method.responseType')}</strong>
              <span>{selectedMethod.responseType.fullName}</span>
            </div>
          </div>

          <label class="field field--textarea">
            <span>{bodyEditorLabel(selectedMethod)}</span>
            <textarea
              rows={selectedMethod.rpcType === 'client_stream' ? 18 : 14}
              value={requestBodyText}
              on:input={(event) => updateBodyDraft((event.currentTarget as HTMLTextAreaElement).value)}
              on:keydown={handleComposerKeydown}
            ></textarea>
          </label>
          {#if requestBodyError}
            <div class="inline-error">{requestBodyError}</div>
          {/if}

          <label class="field field--textarea">
            <span>{$i18n.t('request.metadataJson')}</span>
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
            <span>{$i18n.t('request.savedRequestId')}</span>
            <input bind:value={savedRequestId} placeholder={defaultRequestId(selectedMethod)} />
          </label>

          <div class="pill-row">
            <button class="ghost-button" on:click={() => selectedMethod && restoreTemplateDraft(selectedMethod)}>{$i18n.t('request.resetTemplate')}</button>
            {#if selectedMethod.rpcType === 'unary'}
              <button class="ghost-button" disabled={requestSavePending || !activeWorkspace} on:click={runRequestSave}>
                {requestSavePending ? $i18n.t('request.saveRequestPending') : $i18n.t('request.saveRequest')}
              </button>
              <button class="action-button" disabled={invokePending || reflectionPending || protoPending} on:click={runUnaryInvoke}>
                {invokePending ? $i18n.t('request.invokePending') : $i18n.t('request.invoke')}
              </button>
            {:else if selectedMethod.rpcType === 'server_stream'}
              <button
                class="action-button"
                disabled={streamPending || reflectionPending || protoPending || (streamStartResult !== null && !isTerminalState(streamState))}
                on:click={runServerStreamInvoke}
              >
                {streamPending ? $i18n.t('request.startStreamPending') : $i18n.t('request.startStream')}
              </button>
              <button
                class="ghost-button"
                disabled={cancelPending || !streamStartResult || isTerminalState(streamState)}
                on:click={runStreamCancel}
              >
                {cancelPending ? $i18n.t('request.cancelStreamPending') : $i18n.t('request.cancelStream')}
              </button>
            {:else if selectedMethod.rpcType === 'client_stream'}
              <button
                class="action-button"
                disabled={streamPending || reflectionPending || protoPending || (streamStartResult !== null && !isTerminalState(streamState))}
                on:click={runClientStreamInvoke}
              >
                {streamPending ? $i18n.t('request.startClientStreamPending') : $i18n.t('request.startClientStream')}
              </button>
              <button
                class="ghost-button"
                disabled={cancelPending || !streamStartResult || isTerminalState(streamState)}
                on:click={runStreamCancel}
              >
                {cancelPending ? $i18n.t('request.cancelStreamPending') : $i18n.t('request.cancelStream')}
              </button>
            {/if}
          </div>
        </div>
      {/if}
    </div>

    <div class="card stack">
      <div class="card__header">
        <h3>{$i18n.t('response.panelTitle')}</h3>
        {#if invokeResult}
          <span class:badge--warning={invokeResult.status.code !== 'OK'} class="badge">{invokeResult.status.code}</span>
        {/if}
      </div>

      {#if streamStartResult}
        <div class="pill-row">
          <span class="pill" class:pill--accent={streamState === 'open'}>{translateStreamStateLabel($i18n.language, streamState)}</span>
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
            <div>{translateDiagnosticMessage($i18n.language, streamError.error.code, streamError.error.message)}</div>
          </article>
        {/if}

        <div class="stream-timeline">
          {#if streamEvents.length === 0}
            <div class="empty-state">{$i18n.t('stream.emptyTimeline')}</div>
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
        <div class="empty-state">{$i18n.t('response.empty')}</div>
      {:else}
        <div class="pill-row">
          <span class="pill pill--accent">{translateStreamStateLabel($i18n.language, invokeResult.finalState)}</span>
          <span class="pill">{formatDuration(invokeResult.durationMs)}</span>
          <span class="pill">{invokeResult.callId}</span>
        </div>

        <div class="table-like">
          <div class="table-row">
            <strong>{$i18n.t('response.started')}</strong>
            <span>{formatTimestamp(invokeResult.startedAt)}</span>
          </div>
          <div class="table-row">
            <strong>{$i18n.t('response.finished')}</strong>
            <span>{formatTimestamp(invokeResult.finishedAt)}</span>
          </div>
          <div class="table-row">
            <strong>{$i18n.t('response.status')}</strong>
            <span>{invokeResult.status.code}{invokeResult.status.message ? ` — ${invokeResult.status.message}` : ''}</span>
          </div>
        </div>

        <div class="stack">
          <h4>{$i18n.t('response.body')}</h4>
          <pre class="code-block">{formatJsonValue(invokeResult.responseBody)}</pre>
        </div>

        <div class="stack">
          <h4>{$i18n.t('response.headers')}</h4>
          <pre class="code-block">{formatJsonValue(invokeResult.headers as unknown as JsonValue)}</pre>
        </div>

        <div class="stack">
          <h4>{$i18n.t('response.trailers')}</h4>
          <pre class="code-block">{formatJsonValue(invokeResult.trailers as unknown as JsonValue)}</pre>
        </div>

        {#if invokeResult.diagnostic}
          <article class="diagnostic-item">
            <div class="diagnostic-item__head">
              <strong>{invokeResult.diagnostic.code}</strong>
              <span class="diagnostic-item__meta">{invokeResult.diagnostic.category}</span>
            </div>
            <div>{translateDiagnosticMessage($i18n.language, invokeResult.diagnostic.code, invokeResult.diagnostic.message)}</div>
          </article>
        {/if}
      {/if}
    </div>
  </div>

  {#if historyDetail || historyDetailPending}
    <div class="card stack">
      <div class="card__header">
        <h3>{$i18n.t('history.detailTitle')}</h3>
        {#if historyDetail}
          <span class="pill">{historyDetail.summary.callId}</span>
        {/if}
      </div>

      {#if historyDetailPending}
        <div class="empty-state">{$i18n.t('history.loading')}</div>
      {:else if historyDetail}
        <div class="grid-cards">
          <article class="card list-block">
            <h4>{$i18n.t('history.storedSummary')}</h4>
            <div class="table-like">
              <div class="table-row">
                <strong>{$i18n.t('method.method')}</strong>
                <span>{historyDetail.summary.method}</span>
              </div>
              <div class="table-row">
                <strong>{$i18n.t('response.status')}</strong>
                <span>{historyDetail.status.code}</span>
              </div>
              <div class="table-row">
                <strong>{$i18n.t('history.artifacts')}</strong>
                <span>{historyDetail.summary.sessionLogPath}</span>
              </div>
            </div>
          </article>

          <article class="card list-block">
            <h4>{$i18n.t('history.structuredLogEvents')}</h4>
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
