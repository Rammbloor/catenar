<script lang="ts">
  import { createEventDispatcher, onDestroy, tick } from 'svelte'
  import type {
    CallInvokeUnaryResult,
    CallStartStreamResult,
    CatalogMethod,
    CatalogService,
    HistoryCallSummary,
    HistoryGetResult,
    RPCType,
    StreamCompletedEvent,
    StreamErrorEvent,
    StreamEventRecord,
    StreamState,
    WorkspaceSavedRequest,
    WorkspaceSavedRequestSummary,
    WorkspaceSnapshot,
    JsonValue,
  } from '../contracts'
  import { i18n, translateStreamStateLabel } from '../i18n'
  import type { MonitorConnection } from '../monitor-flow'
  import { isShortcutEvent } from '../shortcuts'
  import { highlightJson } from '../json-highlight'
  import type { MetadataDraftRow } from '../unary-flow'
  import {
    buildMetadataFromRows,
    buildUnaryRequestSaveInput,
    cloneMetadataRows,
    createEmptyMetadataRow,
    defaultUnaryRequestId,
    formatJsonValue,
    parseJsonDraft,
    starterBodyDraft,
    unaryDraftKey,
    type MetadataValidationError,
  } from '../unary-flow'
  import {
    BackendResponseError,
    cancelStream,
    deleteSavedRequest,
    getSavedRequest,
    getHistory,
    halfCloseStream,
    invokeUnary,
    listHistory,
    saveRequest,
    sendStreamMessage,
    startStream,
  } from '../wails/backend'

  type ComposerTab = 'request' | 'schema' | 'metadata' | 'authorization'
  type ResponseTab = 'body' | 'headers' | 'trailers'
  type MonitorMode = 'client' | 'traffic'
  type JsonLintState = 'idle' | 'valid' | 'invalid'
  type ToastTone = 'success' | 'error' | 'info'
  type RepeatedMetadata = Record<string, string[]>
  type CatalogMethodListItem = {
    method: CatalogMethod
    serviceName: string
  }

  const AUTHORIZATION_METADATA_KEY = 'authorization'
  const AUTHORIZATION_METADATA_KEYS = new Set(['authorization', 'proxy-authorization'])

  export let connections: MonitorConnection[] = []
  export let selectedConnection: MonitorConnection | undefined
  export let streamState: StreamState
  export let activeStreamSessionId: string | undefined
  export let activeStreamCallId: string | undefined
  export let streamEvents: StreamEventRecord[] = []
  export let streamErrors: StreamErrorEvent[] = []
  export let lastStreamCompleted: StreamCompletedEvent | undefined
  export let savedRequests: WorkspaceSavedRequestSummary[] = []
  export let workspaceAvailable = false
  export let reconnectingConnectionId: string | null = null
  export let shortcutsEnabled = true
  export let leftSidebarVisible = true
  export let rightPanelVisible = true

  const dispatch = createEventDispatcher<{
    deleteconnection: string
    editconnection: string
    reconnectconnection: string
    renameconnection: {
      connectionId: string
      name: string
    }
    selectconnection: string
    selectmethod: {
      connectionId: string
      methodFullName: string
    }
    workspacesnapshot: WorkspaceSnapshot
  }>()

  const HISTORY_FETCH_LIMIT = 100

  let calls: HistoryCallSummary[] = []
  let connectionHistory: Record<string, HistoryCallSummary[]> = {}
  let connectionHistoryToken = ''
  let historyPending = false
  let historyLoadFailed = false
  let historyDetail: HistoryGetResult | null = null
  let historyDetailPending = false
  let selectedHistoryCallId = ''
  let historyDetailElement: HTMLElement | null = null
  let invokeResult: CallInvokeUnaryResult | null = null
  let activeStreamResult: CallStartStreamResult | null = null
  let streamActionPending = false
  let streamActionError = ''
  let streamActionInfo = ''
  let invokePending = false
  let requestSavePending = false
  let saveRequestDialogOpen = false
  let saveRequestName = ''
  let saveRequestNameError = ''
  let saveRequestNameInput: HTMLInputElement | null = null
  let savedRequestRenameTarget: WorkspaceSavedRequestSummary | null = null
  let savedRequestRenameName = ''
  let savedRequestRenameError = ''
  let savedRequestRenamePending = false
  let savedRequestRenameInput: HTMLInputElement | null = null
  let editingSavedRequest: WorkspaceSavedRequestSummary | null = null
  let appliedSavedRequest: WorkspaceSavedRequestSummary | null = null
  let pendingRequestSave: { body: JsonValue; metadata: Record<string, string> } | null = null
  let requestDeletePendingId = ''
  let activeConnectionMenuId = ''
  let savedRequestsOpen = true
  let savedRequestMenu: { request: WorkspaceSavedRequestSummary; x: number; y: number } | null = null
  let detailConnectionId = ''
  let renamingConnectionId = ''
  let renamingValue = ''
  let requestBodyText = '{}'
  let jsonEditorHighlight: HTMLPreElement | null = null
  let requestBodyError = ''
  let requestBodyLintState: JsonLintState = 'idle'
  let requestBodyLintMessage = ''
  let metadataRows: MetadataDraftRow[] = [createEmptyMetadataRow('metadata-0')]
  let metadataError = ''
  let bodyDrafts: Record<string, string> = {}
  let metadataDrafts: Record<string, MetadataDraftRow[]> = {}
  let metadataRowCounter = 1
  let hydratedDraftKey = ''
  let historyEndpointId = ''
  let activeComposerTab: ComposerTab = 'request'
  let activeResponseTab: ResponseTab = 'body'
  let actionError = ''
  let saveInfoMessage = ''
  let toast: { message: string; tone: ToastTone } | null = null
  let toastTimer: ReturnType<typeof setTimeout> | undefined
  let sidebarSearchOpen = false
  let sidebarSearchQuery = ''
  let sidebarSearchInput: HTMLInputElement | null = null
  let methodSearchQuery = ''
  let clientMethodPickerOpen = false
  let clientMethodSearchQuery = ''
  let clientSavedRequestsModalOpen = false
  let clientSavedRequestPreview: WorkspaceSavedRequest | null = null
  let clientSavedRequestPreviewId = ''
  let clientSavedRequestPreviewPending = false
  let savedRequestSearchQuery = ''
  let trafficSearchQuery = ''
  let trafficMethodFilter = 'all'
  let trafficStatusFilter = 'all'
  let activeMonitorMode: MonitorMode = 'traffic'
  let lastObservedStreamCompletedCallId = ''

  onDestroy(() => {
    if (toastTimer) {
      clearTimeout(toastTimer)
    }
  })
  $: authorizationMetadataRow = metadataRows.find(isAuthorizationMetadataRow)
  $: authorizationMetadataValue = authorizationMetadataRow?.value ?? ''
  $: regularMetadataRows = metadataRows.filter((row) => !isAuthorizationMetadataRow(row))
  $: if (saveInfoMessage) {
    showToast(saveInfoMessage, 'success')
    saveInfoMessage = ''
  }

  $: completedCalls = calls.length
  $: averageDurationMs =
    calls.length === 0
      ? 0
      : Math.round(calls.reduce((total, call) => total + (call.durationMs ?? 0), 0) / calls.length)
  $: successfulCalls = calls.filter((call) => call.grpcStatusCode === 'OK' || (!call.grpcStatusCode && call.state === 'closed')).length
  $: successRate = calls.length === 0 ? '0%' : `${Math.round((successfulCalls / calls.length) * 100)}%`
  $: trafficMethodOptions = Array.from(new Set(calls.map((call) => call.method))).sort()
  $: trafficStatusOptions = Array.from(new Set(calls.map((call) => call.grpcStatusCode || call.state))).sort()
  $: trafficCalls = calls.filter((call) => {
    const search = trafficSearchQuery.trim().toLowerCase()
    const matchesSearch = !search || call.method.toLowerCase().includes(search)
    const matchesMethod = trafficMethodFilter === 'all' || call.method === trafficMethodFilter
    const status = call.grpcStatusCode || call.state
    const matchesStatus = trafficStatusFilter === 'all' || status === trafficStatusFilter
    return matchesSearch && matchesMethod && matchesStatus
  })
  $: selectedMethod = selectedConnection
    ? findMethod(selectedConnection.services, selectedConnection.selectedMethodFullName)
    : undefined
  $: detailConnection = detailConnectionId
    ? connections.find((connection) => connection.id === detailConnectionId)
    : undefined
  $: activeDraftKey =
    selectedConnection && selectedMethod
      ? unaryDraftKey({
          endpointRef: selectedConnection.id,
          methodFullName: selectedMethod.fullName,
        })
      : ''
  $: hydrateMethodDraft(selectedConnection, selectedMethod, activeDraftKey)
  $: responseStatus = historyDetail?.status ?? invokeResult?.status
  $: responseBody = historyDetail ? historyDetail.responseBody : invokeResult?.responseBody
  $: responseHeaders = historyDetail ? historyDetail.headers : invokeResult?.headers
  $: responseTrailers = historyDetail ? historyDetail.trailers : invokeResult?.trailers
  $: responseTruncated = historyDetail?.summary.truncated ?? false
  $: responseErrorMessage = responseStatus && responseStatus.code !== 'OK' ? responseStatus.message || responseStatus.code : ''
  $: historyDetailMethod =
    historyDetail && selectedConnection ? findMethod(selectedConnection.services, historyDetail.summary.method) : undefined
  $: historyRequestMetadata = historyDetail ? requestMetadataFromHistoryDetail(historyDetail) : {}
  $: historyCustomMetadata = filterRepeatedMetadata(historyRequestMetadata, (key) => !isAuthorizationMetadataKey(key))
  $: historyAuthorizationMetadata = filterRepeatedMetadata(historyRequestMetadata, isAuthorizationMetadataKey)
  $: selectedHistoryEndpointId = selectedConnection?.id ?? ''
  $: connectionHistoryKey = connections.map((connection) => connection.id).join('|')
  $: if (connectionHistoryKey && connectionHistoryKey !== connectionHistoryToken) {
    connectionHistoryToken = connectionHistoryKey
    void refreshConnectionHistories(connections)
  }
  $: visibleConnections = filterConnections(connections, sidebarSearchQuery)
  $: visibleCatalogMethods = selectedConnection
    ? filterCatalogMethods(selectedConnection.services, methodSearchQuery)
    : []
  $: clientMethodOptions = selectedConnection ? filterCatalogMethods(selectedConnection.services, '') : []
  $: clientFilteredMethods = clientMethodOptions.filter(({ method, serviceName }) => {
    const search = clientMethodSearchQuery.trim().toLowerCase()
    return !search || [method.name, method.fullName, serviceName].some((value) => value.toLowerCase().includes(search))
  })
  $: clientSavedRequestPreviewSummary = clientSavedRequestPreviewId
    ? savedRequestsForSelectedConnection.find((request) => request.id === clientSavedRequestPreviewId)
    : undefined
  $: activeStreamSessionRef = activeStreamResult?.sessionId ?? activeStreamSessionId ?? ''
  $: activeStreamCallRef = activeStreamResult?.callId ?? activeStreamCallId ?? ''
  $: activeStreamEvents = activeStreamSessionRef
    ? streamEvents.filter((event) => event.sessionId === activeStreamSessionRef)
    : []
  $: activeStreamErrors = activeStreamSessionRef
    ? streamErrors.filter((event) => event.sessionId === activeStreamSessionRef)
    : []
  $: selectedStreamCompleted =
    activeStreamSessionRef && lastStreamCompleted?.sessionId === activeStreamSessionRef ? lastStreamCompleted : undefined
  $: activeStreamCount = selectedConnection && isLiveStreamState(streamState) ? 1 : 0
  $: metricDeltas = calculateMetricDeltas(calls, activeStreamCount)
  $: savedRequestsForSelectedConnection = selectedConnection
    ? savedRequests.filter((request) => request.endpointRef === selectedConnection.id)
    : []
  $: visibleSavedRequests = savedRequestsForSelectedConnection.filter((request) => savedRequestMatchesSearch(request, methodSearchQuery))
  $: clientVisibleSavedRequests = savedRequestsForSelectedConnection.filter((request) => {
    const search = savedRequestSearchQuery.trim().toLowerCase()
    return !search || [savedRequestMethodName(request), request.method].some((value) => value.toLowerCase().includes(search))
  })
  $: canSendStreamMessage =
    Boolean(activeStreamSessionRef) && (streamState === 'open' || streamState === 'half_closed_remote')
  $: canHalfCloseStream =
    Boolean(activeStreamSessionRef) && (streamState === 'open' || streamState === 'half_closed_remote')
  $: canCancelStream = Boolean(activeStreamSessionRef) && !isTerminalStreamState(streamState)
  $: if (lastStreamCompleted?.callId && lastStreamCompleted.callId !== lastObservedStreamCompletedCallId) {
    lastObservedStreamCompletedCallId = lastStreamCompleted.callId
    void refreshHistory()
  }
  $: if (selectedHistoryEndpointId !== historyEndpointId) {
    historyEndpointId = selectedHistoryEndpointId
    selectedHistoryCallId = ''
    historyDetail = null
    historyDetailPending = false
    trafficSearchQuery = ''
    trafficMethodFilter = 'all'
    trafficStatusFilter = 'all'
    if (historyEndpointId) {
      void refreshHistory()
    } else {
      calls = []
      historyLoadFailed = false
      historyPending = false
    }
  }

  function statusClass(status: MonitorConnection['status']): string {
    return `connection-status connection-status--${status}`
  }

  function isLiveStreamState(state: StreamState): boolean {
    return state === 'connecting' || state === 'open' || state === 'half_closed_local' || state === 'half_closed_remote'
  }

  function isTerminalStreamState(state: StreamState): boolean {
    return state === 'closed' || state === 'cancelled' || state === 'error'
  }

  function selectConnection(connectionId: string): void {
    dispatch('selectconnection', connectionId)
    activeConnectionMenuId = ''
  }

  function selectAdjacentConnection(direction: 1 | -1): void {
    if (connections.length === 0) {
      return
    }

    const selectedIndex = connections.findIndex((connection) => connection.id === selectedConnection?.id)
    const baseIndex = selectedIndex >= 0 ? selectedIndex : direction === 1 ? -1 : 0
    const nextIndex = (baseIndex + direction + connections.length) % connections.length
    selectConnection(connections[nextIndex].id)
  }

  function isEditableShortcutTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) {
      return false
    }

    return (
      target.isContentEditable ||
      target instanceof HTMLInputElement ||
      target instanceof HTMLTextAreaElement ||
      target instanceof HTMLSelectElement
    )
  }

  function filterConnections(items: MonitorConnection[], query: string): MonitorConnection[] {
    const normalizedQuery = query.trim().toLowerCase()
    if (!normalizedQuery) {
      return items
    }

    return items.filter((connection) =>
      [connection.name, connection.endpoint, connection.selectedMethodFullName ?? '']
        .some((value) => value.toLowerCase().includes(normalizedQuery)),
    )
  }

  function filterCatalogMethods(services: CatalogService[], query: string): CatalogMethodListItem[] {
    const normalizedQuery = query.trim().toLowerCase()
    const methods = services.flatMap((service) =>
      service.methods.map((method) => ({
        method,
        serviceName: service.fullName,
      })),
    )

    if (!normalizedQuery) {
      return methods
    }

    return methods.filter(({ method, serviceName }) =>
      [method.name, method.fullName, method.requestType.fullName, method.responseType.fullName, serviceName]
        .some((value) => value.toLowerCase().includes(normalizedQuery)),
    )
  }

  function openSidebarSearch(): void {
    sidebarSearchOpen = true
    void tick().then(() => sidebarSearchInput?.focus())
  }

  function closeSidebarSearch(): void {
    sidebarSearchOpen = false
    sidebarSearchQuery = ''
  }

  function handleSidebarSearchKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault()
      closeSidebarSearch()
      return
    }

    if (event.key === 'Enter' && visibleConnections[0]) {
      event.preventDefault()
      selectConnection(visibleConnections[0].id)
    }
  }

  function handleConnectionKeyboard(event: KeyboardEvent, connectionId: string): void {
    if (event.key !== 'Enter' && event.key !== ' ') {
      return
    }

    event.preventDefault()
    selectConnection(connectionId)
  }

  function toggleConnectionMenu(connectionId: string): void {
    activeConnectionMenuId = activeConnectionMenuId === connectionId ? '' : connectionId
  }

  function openConnectionDetails(connectionId: string): void {
    detailConnectionId = connectionId
    activeConnectionMenuId = ''
  }

  function closeConnectionDetails(): void {
    detailConnectionId = ''
  }

  function deleteConnection(connectionId: string): void {
    dispatch('deleteconnection', connectionId)
    activeConnectionMenuId = ''

    if (detailConnectionId === connectionId) {
      detailConnectionId = ''
    }
  }

  function reconnectConnection(connectionId: string): void {
    dispatch('reconnectconnection', connectionId)
    activeConnectionMenuId = ''
  }

  function startConnectionRename(connection: MonitorConnection): void {
    renamingConnectionId = connection.id
    renamingValue = connection.name
    activeConnectionMenuId = ''
  }

  function finishConnectionRename(connection: MonitorConnection): void {
    const nextName = renamingValue.trim()
    if (nextName && nextName !== connection.name) {
      dispatch('renameconnection', {
        connectionId: connection.id,
        name: nextName,
      })
    }

    renamingConnectionId = ''
    renamingValue = ''
  }

  function cancelConnectionRename(): void {
    renamingConnectionId = ''
    renamingValue = ''
  }

  function handleWindowPointerDown(event: PointerEvent): void {
    if (!renamingConnectionId) {
      return
    }

    const target = event.target
    if (target instanceof HTMLElement && target.closest('.connection-rename-input')) {
      return
    }

    cancelConnectionRename()
  }

  function handleRenameKeydown(event: KeyboardEvent, connection: MonitorConnection): void {
    if (event.key === 'Enter') {
      event.preventDefault()
      finishConnectionRename(connection)
    }

    if (event.key === 'Escape') {
      event.preventDefault()
      cancelConnectionRename()
    }
  }

  function handleWindowClick(event: MouseEvent): void {
    const target = event.target
    if (target instanceof HTMLElement && target.closest('.connection-item__menu, .connection-menu, .client-method-picker, .client-saved-requests-picker, .client-method-modal, .client-saved-requests-modal, .save-request-modal')) {
      return
    }

    activeConnectionMenuId = ''
    clientMethodPickerOpen = false
    clientSavedRequestsModalOpen = false
    closeSavedRequestMenu()
  }

  function requestStatusClass(call: HistoryCallSummary): string {
    if (call.truncated) {
      return 'request-status request-status--warning'
    }

    if (call.grpcStatusCode === 'OK' || (!call.grpcStatusCode && call.state === 'closed')) {
      return 'request-status request-status--ok'
    }

    if (call.state === 'error' || call.errorCategory) {
      return 'request-status request-status--error'
    }

    return 'request-status request-status--info'
  }

  function formatDuration(durationMs: number | undefined): string {
    return durationMs !== undefined && durationMs >= 0 ? `${durationMs}ms` : '0ms'
  }

  function timelineVisualWidth(durationMs: number | undefined): number {
    if (durationMs === undefined || durationMs <= 0) {
      return 0
    }
    return Math.min(100, Math.max(14, 18 + Math.round(Math.log2(durationMs + 1) * 15)))
  }

  function formatTime(value: string | undefined): string {
    if (!value) {
      return '-'
    }

    return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  function connectionSparklinePoints(connectionId: string): string {
    const history = connectionHistory[connectionId] ?? (connectionId === selectedConnection?.id ? calls : [])
    const values = history
      .slice(0, 18)
      .reverse()
      .map((call) => Math.max(0, call.durationMs ?? 0))
    if (values.length < 2) {
      return '0,27 100,27'
    }

    const min = Math.min(...values)
    const max = Math.max(...values)
    const range = Math.max(1, max - min)
    return values
      .map((value, index) => {
        const x = (index / (values.length - 1)) * 100
        const y = 27 - ((value - min) / range) * 21
        return `${x.toFixed(1)},${y.toFixed(1)}`
      })
      .join(' ')
  }

  function connectionLatency(connectionId: string): string {
    const history = connectionHistory[connectionId] ?? (connectionId === selectedConnection?.id ? calls : [])
    const latest = history[0]?.durationMs
    return latest === undefined ? '—' : `${latest}ms`
  }

  function methodDisplayName(fullName: string): string {
    return fullName.split(/[/.]/).filter(Boolean).at(-1) ?? fullName
  }

  function serviceDisplayName(fullName: string): string {
    const parts = fullName.split(/[/.]/).filter(Boolean)
    return parts.slice(0, -1).join('.') || '-'
  }

  function formatRpcType(rpcType: RPCType): string {
    return $i18n.t(`method.rpc.${rpcType}`)
  }

  function schemaFieldLabel(field: NonNullable<CatalogMethod['requestType']['fields']>[number]): string {
    const modifiers = [field.repeated ? $i18n.t('schema.repeated') : '', field.required ? $i18n.t('schema.required') : '', field.oneof ? `${$i18n.t('schema.oneof')}: ${field.oneof}` : ''].filter(Boolean)
    return modifiers.length > 0 ? `${field.type} · ${modifiers.join(' · ')}` : field.type
  }

  function formatCatalogSource(source: MonitorConnection['catalogSource']): string {
    return $i18n.t(`catalog.source.${source}`)
  }

  function methodCount(services: CatalogService[] | undefined): number {
    return services?.reduce((total, service) => total + service.methods.length, 0) ?? 0
  }

  function metadataDefaults(connection: MonitorConnection): [string, string][] {
    return Object.entries(connection.endpointCheck?.endpoint.metadataDefaults ?? {})
  }

  function calculateMetricDeltas(items: HistoryCallSummary[], activeStreams: number): Record<string, number> {
    const recent = items.slice(0, 5)
    const previous = items.slice(5, 10)
    const previousSuccessRate = successRateForCalls(previous)
    const recentSuccessRate = successRateForCalls(recent)
    const previousAverageDuration = averageDurationForCalls(previous)
    const recentAverageDuration = averageDurationForCalls(recent)

    return {
      requests: recent.length - previous.length,
      duration: previousAverageDuration === 0 && recentAverageDuration === 0 ? 0 : previousAverageDuration - recentAverageDuration,
      success: recentSuccessRate - previousSuccessRate,
      streams: activeStreams,
    }
  }

  function averageDurationForCalls(items: HistoryCallSummary[]): number {
    if (items.length === 0) {
      return 0
    }

    return Math.round(items.reduce((total, call) => total + (call.durationMs ?? 0), 0) / items.length)
  }

  function successRateForCalls(items: HistoryCallSummary[]): number {
    if (items.length === 0) {
      return 0
    }

    const successful = items.filter((call) => call.grpcStatusCode === 'OK' || (!call.grpcStatusCode && call.state === 'closed')).length
    return Math.round((successful / items.length) * 100)
  }

  function metricDeltaLabel(value: number, suffix = ''): string {
    if (value === 0) {
      return `0${suffix}`
    }

    return `${value > 0 ? '+' : ''}${value}${suffix}`
  }

  function metricDeltaClass(value: number, positiveWhenHigher = true): string {
    if (value === 0) {
      return 'metric-delta metric-delta--neutral'
    }

    const positive = positiveWhenHigher ? value > 0 : value < 0
    return positive ? 'metric-delta metric-delta--up' : 'metric-delta metric-delta--down'
  }

  function isAuthorizationMetadataKey(key: string): boolean {
    return AUTHORIZATION_METADATA_KEYS.has(key.trim().toLowerCase())
  }

  function isAuthorizationMetadataRow(row: MetadataDraftRow): boolean {
    return isAuthorizationMetadataKey(row.key)
  }

  function requestMetadataFromHistoryDetail(detail: HistoryGetResult): RepeatedMetadata {
    return detail.events.find((event) => event.kind === 'call_started')?.grpc?.metadata ?? {}
  }

  function filterRepeatedMetadata(
    metadata: RepeatedMetadata,
    predicate: (key: string) => boolean,
  ): RepeatedMetadata {
    return Object.fromEntries(Object.entries(metadata).filter(([key]) => predicate(key)))
  }

  function hasRepeatedMetadata(metadata: RepeatedMetadata): boolean {
    return Object.keys(metadata).length > 0
  }

  function findMethod(services: CatalogService[], fullName: string | undefined): CatalogMethod | undefined {
    for (const service of services) {
      const method = service.methods.find((method) => method.fullName === fullName)
      if (method) {
        return method
      }
    }

    return undefined
  }

  function selectMethod(connectionId: string, methodFullName: string): void {
    dispatch('selectmethod', {
      connectionId,
      methodFullName,
    })
  }

  function savedRequestMethodName(request: WorkspaceSavedRequestSummary): string {
	return request.name?.trim() || request.method.split(/[/.]/).filter(Boolean).at(-1) || request.method
	}

  function closeClientSavedRequestsModal(): void {
    clientSavedRequestsModalOpen = false
    clientSavedRequestPreview = null
    clientSavedRequestPreviewId = ''
    clientSavedRequestPreviewPending = false
    savedRequestSearchQuery = ''
  }

  async function previewClientSavedRequest(summary: WorkspaceSavedRequestSummary): Promise<void> {
    clientSavedRequestPreviewId = summary.id
    clientSavedRequestPreview = null
    clientSavedRequestPreviewPending = true

    try {
      const result = await getSavedRequest({ id: summary.id })
      dispatch('workspacesnapshot', result.workspace)
      if (clientSavedRequestPreviewId === summary.id) {
        clientSavedRequestPreview = result.savedRequest
      }
    } catch (error) {
      setActionError(error, $i18n.t('request.savedRequestLoadFailed'))
    } finally {
      if (clientSavedRequestPreviewId === summary.id) {
        clientSavedRequestPreviewPending = false
      }
    }
  }

  async function openClientSavedRequestsModal(): Promise<void> {
    clientSavedRequestsModalOpen = true
    savedRequestSearchQuery = ''
    const firstRequest = savedRequestsForSelectedConnection[0]
    if (firstRequest) {
      await previewClientSavedRequest(firstRequest)
    }
  }

  async function loadClientSavedRequest(summary: WorkspaceSavedRequestSummary): Promise<void> {
    await applySavedRequest(summary)
    if (editingSavedRequest?.id === summary.id) {
      closeClientSavedRequestsModal()
    }
  }

  async function deleteClientSavedRequest(summary: WorkspaceSavedRequestSummary): Promise<void> {
    const deleted = await deleteSavedRequestSummary(summary)
    if (!deleted || !clientSavedRequestsModalOpen) {
      return
    }

    await tick()
    const nextRequest = clientVisibleSavedRequests[0]
    if (nextRequest) {
      await previewClientSavedRequest(nextRequest)
    } else {
      clientSavedRequestPreview = null
      clientSavedRequestPreviewId = ''
    }
  }

	function savedRequestMatchesSearch(request: WorkspaceSavedRequestSummary, query: string): boolean {
		const normalizedQuery = query.trim().toLocaleLowerCase()
		if (!normalizedQuery) {
			return true
		}

		return [savedRequestMethodName(request), request.method, request.id]
			.some((value) => value.toLocaleLowerCase().includes(normalizedQuery))
  }

  function createSavedRequestId(method: CatalogMethod): string {
    const base = defaultUnaryRequestId(method)
    const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)
    const existingIds = new Set(savedRequestsForSelectedConnection.map((request) => request.id))
    const uniqueSuffix =
      globalThis.crypto?.randomUUID?.().replace(/-/g, '').slice(0, 10) ??
      `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`

    let candidate = `${base}-${stamp}-${uniqueSuffix}`
    let suffix = 2
    while (existingIds.has(candidate)) {
      candidate = `${base}-${stamp}-${uniqueSuffix}-${suffix}`
      suffix += 1
    }
    return candidate
  }

  function openSavedRequestMenu(event: MouseEvent, request: WorkspaceSavedRequestSummary): void {
    event.preventDefault()
    event.stopPropagation()
    const width = 178
    const height = 44
    savedRequestMenu = {
      request,
      x: Math.max(8, Math.min(event.clientX, window.innerWidth - width - 8)),
      y: Math.max(8, Math.min(event.clientY, window.innerHeight - height - 8)),
    }
  }

  function closeSavedRequestMenu(): void {
    savedRequestMenu = null
  }

  function metadataRowsFromTemplate(template: Record<string, string> | undefined): MetadataDraftRow[] {
    const rows = Object.entries(template ?? {}).map(([key, value]) => ({
      id: nextMetadataRowId(),
      key,
      value,
    }))

    return rows.length > 0 ? rows : [createEmptyMetadataRow(nextMetadataRowId())]
  }

  function bodyFromSavedRequest(request: WorkspaceSavedRequest): unknown {
    if (request.requestSpec.mode === 'single') {
      return request.requestSpec.body ?? {}
    }

    return request.requestSpec.messages?.[0]?.body ?? {}
  }

  async function applySavedRequest(summary: WorkspaceSavedRequestSummary): Promise<void> {
    if (!selectedConnection) {
      return
    }

    closeSavedRequestMenu()
    clearActionError()
    saveInfoMessage = ''

    try {
      const result = await getSavedRequest({ id: summary.id })
      dispatch('workspacesnapshot', result.workspace)
      const request = result.savedRequest
      const method = findMethod(selectedConnection.services, request.method)
      if (!method) {
        actionError = $i18n.t('errors.requestSaveLoadedMethod')
        return
      }

      const draftKey = unaryDraftKey({
        endpointRef: selectedConnection.id,
        methodFullName: request.method,
      })
      const bodyText = formatJsonValue(bodyFromSavedRequest(request))
      const rows = metadataRowsFromTemplate(request.metadataTemplate)
      bodyDrafts = {
        ...bodyDrafts,
        [draftKey]: bodyText,
      }
      metadataDrafts = {
        ...metadataDrafts,
        [draftKey]: cloneMetadataRows(rows),
      }
      selectMethod(selectedConnection.id, request.method)
      await tick()
      requestBodyText = bodyText
      metadataRows = cloneMetadataRows(rows)
      hydratedDraftKey = draftKey
      activeComposerTab = 'request'
      requestBodyError = ''
      requestBodyLintState = 'idle'
      requestBodyLintMessage = ''
      metadataError = ''
      invokeResult = null
      historyDetail = null
      selectedHistoryCallId = ''
      editingSavedRequest = summary
      appliedSavedRequest = summary
      saveInfoMessage = $i18n.t('request.savedRequestApplied', {
        name: savedRequestMethodName(summary),
      })
    } catch (error) {
      setActionError(error, $i18n.t('request.savedRequestLoadFailed'))
    }
  }

  async function deleteSavedRequestSummary(summary: WorkspaceSavedRequestSummary): Promise<boolean> {
    closeSavedRequestMenu()
    clearActionError()
    saveInfoMessage = ''
    requestDeletePendingId = summary.id

    try {
      const result = await deleteSavedRequest({ id: summary.id })
      dispatch('workspacesnapshot', result.workspace)
      if (editingSavedRequest?.id === summary.id) {
        editingSavedRequest = null
      }
      if (appliedSavedRequest?.id === summary.id) {
        appliedSavedRequest = null
      }
      saveInfoMessage = $i18n.t('request.savedRequestDeleted', {
        name: savedRequestMethodName(summary),
      })
      return true
    } catch (error) {
      setActionError(error, $i18n.t('request.savedRequestDeleteFailed'))
    } finally {
      requestDeletePendingId = ''
    }

    return false
  }

  function closeSavedRequestRenameDialog(): void {
    if (savedRequestRenamePending) {
      return
    }

    savedRequestRenameTarget = null
    savedRequestRenameName = ''
    savedRequestRenameError = ''
  }

  async function openSavedRequestRenameDialog(summary: WorkspaceSavedRequestSummary): Promise<void> {
    closeSavedRequestMenu()
    savedRequestRenameTarget = summary
    savedRequestRenameName = savedRequestMethodName(summary)
    savedRequestRenameError = ''
    await tick()
    savedRequestRenameInput?.focus()
    savedRequestRenameInput?.select()
  }

  async function confirmSavedRequestRename(): Promise<void> {
    const target = savedRequestRenameTarget
    const name = savedRequestRenameName.trim()
    if (!target) {
      return
    }
    if (!name) {
      savedRequestRenameError = $i18n.t('request.saveRequestNameRequired')
      savedRequestRenameInput?.focus()
      return
    }
    if (!workspaceAvailable) {
      actionError = $i18n.t('errors.requestSaveWorkspaceRequired')
      closeSavedRequestRenameDialog()
      return
    }

    const duplicate = savedRequests.some(
      (request) =>
        request.endpointRef === target.endpointRef &&
        request.id !== target.id &&
        request.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase(),
    )
    if (duplicate) {
      savedRequestRenameError = $i18n.t('request.saveRequestNameDuplicate', { name })
      savedRequestRenameInput?.focus()
      return
    }

    let renamed = false
    savedRequestRenamePending = true
    savedRequestRenameError = ''
    try {
      const loaded = await getSavedRequest({ id: target.id })
      const request = loaded.savedRequest
      const result = await saveRequest({
        id: request.id,
        name,
        method: request.method,
        rpcType: request.rpcType,
        endpointRef: request.endpointRef,
        environmentRef: request.environmentRef,
        metadataTemplate: request.metadataTemplate,
        callOptions: request.callOptions,
        requestSpec: request.requestSpec,
      })
      dispatch('workspacesnapshot', result.workspace)
      if (editingSavedRequest?.id === target.id) {
        editingSavedRequest = result.savedRequest
      }
      saveInfoMessage = $i18n.t('request.savedRequestRenamed', { name })
      renamed = true
    } catch (error) {
      if (error instanceof BackendResponseError && error.code === 'validation.saved_request_name_duplicate') {
        savedRequestRenameError = $i18n.t('request.saveRequestNameDuplicate', { name })
      } else {
        savedRequestRenameError = formatActionError(error, $i18n.t('request.savedRequestRenameFailed'))
      }
    } finally {
      savedRequestRenamePending = false
      if (renamed) {
        closeSavedRequestRenameDialog()
      }
    }
  }

  function nextMetadataRowId(): string {
    const id = `metadata-${metadataRowCounter}`
    metadataRowCounter += 1
    return id
  }

  function hydrateMethodDraft(
    connection: MonitorConnection | undefined,
    method: CatalogMethod | undefined,
    draftKey: string,
  ): void {
    if (!connection || !method || !draftKey) {
      if (hydratedDraftKey) {
        requestBodyText = '{}'
        metadataRows = [createEmptyMetadataRow(nextMetadataRowId())]
        requestBodyError = ''
        requestBodyLintState = 'idle'
        requestBodyLintMessage = ''
        metadataError = ''
        actionError = ''
        saveInfoMessage = ''
        invokeResult = null
        historyDetail = null
        selectedHistoryCallId = ''
        hydratedDraftKey = ''
      }
      return
    }

    if (draftKey === hydratedDraftKey) {
      return
    }

    requestBodyText = bodyDrafts[draftKey] ?? starterBodyDraft(connection, method)
    metadataRows = cloneMetadataRows(metadataDrafts[draftKey] ?? [createEmptyMetadataRow(nextMetadataRowId())])
    requestBodyError = ''
    requestBodyLintState = 'idle'
    requestBodyLintMessage = ''
    metadataError = ''
    actionError = ''
    saveInfoMessage = ''
    invokeResult = null
    historyDetail = null
    selectedHistoryCallId = ''
    hydratedDraftKey = draftKey
  }

  function updateBodyDraft(value: string): void {
    requestBodyText = value
    requestBodyError = ''
    requestBodyLintState = 'idle'
    requestBodyLintMessage = ''
    saveInfoMessage = ''

    if (activeDraftKey) {
      bodyDrafts = {
        ...bodyDrafts,
        [activeDraftKey]: value,
      }
    }
  }

  function syncJsonEditorScroll(event: Event): void {
    const editor = event.currentTarget as HTMLTextAreaElement
    if (jsonEditorHighlight) {
      jsonEditorHighlight.scrollTop = editor.scrollTop
      jsonEditorHighlight.scrollLeft = editor.scrollLeft
    }
  }

  function persistMetadataDraft(rows: MetadataDraftRow[]): void {
    if (!activeDraftKey) {
      return
    }

    metadataDrafts = {
      ...metadataDrafts,
      [activeDraftKey]: cloneMetadataRows(rows),
    }
  }

  function updateMetadataRow(rowId: string, field: 'key' | 'value', value: string): void {
    metadataRows = metadataRows.map((row) => (row.id === rowId ? { ...row, [field]: value } : row))
    metadataError = ''
    saveInfoMessage = ''
    persistMetadataDraft(metadataRows)
  }

  function addMetadataRow(): void {
    metadataRows = [...metadataRows, createEmptyMetadataRow(nextMetadataRowId())]
    saveInfoMessage = ''
    persistMetadataDraft(metadataRows)
  }

  function removeMetadataRow(rowId: string): void {
    if (metadataRows.length === 1) {
      metadataRows = [createEmptyMetadataRow(nextMetadataRowId())]
    } else {
      metadataRows = metadataRows.filter((row) => row.id !== rowId)
    }

    metadataError = ''
    saveInfoMessage = ''
    persistMetadataDraft(metadataRows)
  }

  function updateAuthorizationMetadata(value: string): void {
    const existingRow = metadataRows.find(isAuthorizationMetadataRow)
    let nextRows: MetadataDraftRow[]

    if (existingRow) {
      nextRows = value.trim()
        ? metadataRows.map((row) =>
            row.id === existingRow.id ? { ...row, key: AUTHORIZATION_METADATA_KEY, value } : row,
          )
        : metadataRows.filter((row) => row.id !== existingRow.id)
    } else if (value.trim()) {
      nextRows = [
        ...metadataRows,
        {
          id: nextMetadataRowId(),
          key: AUTHORIZATION_METADATA_KEY,
          value,
        },
      ]
    } else {
      nextRows = metadataRows
    }

    metadataRows = nextRows.length > 0 ? nextRows : [createEmptyMetadataRow(nextMetadataRowId())]
    metadataError = ''
    saveInfoMessage = ''
    persistMetadataDraft(metadataRows)
  }

  function resetBodyDraft(): void {
    if (!selectedConnection || !selectedMethod) {
      return
    }

    updateBodyDraft(starterBodyDraft(selectedConnection, selectedMethod))
    requestBodyError = ''
    requestBodyLintState = 'idle'
    requestBodyLintMessage = ''
  }

  function requestBodyValidationMessage(body: ReturnType<typeof parseJsonDraft>): string {
    if (body.ok) {
      return ''
    }

    const detail =
      body.code === 'json.number_leading_zero' ? $i18n.t('errors.jsonNumberLeadingZero') : body.message
    return `${$i18n.t('errors.requestBodyJson')} ${detail || ''}`.trim()
  }

  function lintRequestBodyDraft(showSuccessFeedback = true): boolean {
    const body = parseJsonDraft(requestBodyText)
    if (!body.ok) {
      requestBodyError = requestBodyValidationMessage(body)
      requestBodyLintState = 'invalid'
      requestBodyLintMessage = $i18n.t('request.jsonInvalid')
      showToast(requestBodyError, 'error')
      return false
    }

    requestBodyError = ''
    requestBodyLintState = 'valid'
    requestBodyLintMessage = $i18n.t('request.jsonValid')
    if (showSuccessFeedback) {
      showToast(requestBodyLintMessage, 'success')
    }
    return true
  }

  function formatRequestBodyDraft(): void {
    const body = parseJsonDraft(requestBodyText)
    if (!body.ok) {
      requestBodyError = requestBodyValidationMessage(body)
      requestBodyLintState = 'invalid'
      requestBodyLintMessage = $i18n.t('request.jsonInvalid')
      showToast(requestBodyError, 'error')
      return
    }

    updateBodyDraft(formatJsonValue(body.value))
    requestBodyError = ''
    requestBodyLintState = 'valid'
    requestBodyLintMessage = $i18n.t('request.jsonFormatted')
    showToast(requestBodyLintMessage, 'success')
  }

  function metadataValidationMessage(error: MetadataValidationError): string {
    if (error.code === 'metadata.duplicateKey') {
      return $i18n.t('metadata.duplicateKey', { key: error.key })
    }

    return $i18n.t('metadata.keyRequired')
  }

  function clearActionError(): void {
    actionError = ''
    saveInfoMessage = ''
  }

  function formatActionError(error: unknown, fallback: string): string {
    if (error instanceof BackendResponseError) {
      const field = error.details?.field?.trim()
      const detail = error.details?.cause?.trim()
      const message = error.backendMessage?.trim() || fallback
      const context = [field, detail].filter(Boolean).join(' · ')
      return context ? `${message} (${context})` : message
    }

    return error instanceof Error && error.message ? error.message : fallback
  }

  function setActionError(error: unknown, fallback: string): void {
    actionError = formatActionError(error, fallback)
    showToast(actionError, 'error')
  }

  function setStreamActionError(error: unknown, fallback: string): void {
    streamActionInfo = ''
    streamActionError = formatActionError(error, fallback)
    showToast(streamActionError, 'error')
  }

  function showToast(message: string, tone: ToastTone): void {
    if (!message.trim()) {
      return
    }

    if (toastTimer) {
      clearTimeout(toastTimer)
    }
    toast = { message, tone }
    toastTimer = setTimeout(() => {
      toast = null
      toastTimer = undefined
    }, 4200)
  }

  async function refreshHistory(): Promise<void> {
    const endpointId = historyEndpointId
    if (!endpointId) {
      calls = []
      historyLoadFailed = false
      historyPending = false
      return
    }

    historyPending = true
    historyLoadFailed = false

    try {
      const result = await listHistory({ endpointId, limit: HISTORY_FETCH_LIMIT })
      if (historyEndpointId === endpointId) {
        calls = result.calls
        connectionHistory = { ...connectionHistory, [endpointId]: result.calls }
      }
    } catch {
      if (historyEndpointId === endpointId) {
        calls = []
        historyLoadFailed = true
      }
    } finally {
      if (historyEndpointId === endpointId) {
        historyPending = false
      }
    }
  }

  async function refreshConnectionHistories(items: MonitorConnection[]): Promise<void> {
    const results = await Promise.allSettled(
      items.map(async (connection) => ({
        endpointId: connection.id,
        calls: (await listHistory({ endpointId: connection.id, limit: 32 })).calls,
      })),
    )
    const nextHistory = { ...connectionHistory }
    for (const result of results) {
      if (result.status === 'fulfilled') {
        nextHistory[result.value.endpointId] = result.value.calls
      }
    }
    connectionHistory = nextHistory
  }

  async function loadHistoryDetail(callId: string): Promise<void> {
    if (selectedHistoryCallId === callId && historyDetail?.summary.callId === callId) {
      selectedHistoryCallId = ''
      historyDetail = null
      return
    }

    selectedHistoryCallId = callId
    activeResponseTab = 'body'
    historyDetailPending = true
    clearActionError()
    await centerHistoryDetail(callId)

    try {
      historyDetail = await getHistory(callId)
    } catch (error) {
      historyDetail = null
      setActionError(error, $i18n.t('errors.historyDetail'))
    } finally {
      historyDetailPending = false
      await centerHistoryDetail(callId)
    }
  }

  async function centerHistoryDetail(callId: string): Promise<void> {
    await tick()
    if (selectedHistoryCallId !== callId) {
      return
    }
    historyDetailElement?.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'nearest' })
  }

  async function runUnaryInvoke(): Promise<void> {
    if (!selectedConnection || !selectedMethod) {
      actionError = $i18n.t('errors.selectMethodInvoke')
      return
    }

    clearActionError()
    invokeResult = null
    historyDetail = null
    selectedHistoryCallId = ''

    if (selectedMethod.rpcType !== 'unary') {
      actionError = $i18n.t('errors.unaryOnly')
      return
    }

    if (!lintRequestBodyDraft(false)) {
      return
    }

    const body = parseJsonDraft(requestBodyText)
    if (!body.ok) {
      return
    }

    const metadata = buildMetadataFromRows(metadataRows)
    if (!metadata.ok) {
      metadataError = metadataValidationMessage(metadata.error)
      return
    }
    metadataError = ''

    invokePending = true
    historyDetailPending = false
    activeResponseTab = 'body'
    try {
      invokeResult = await invokeUnary({
        catalogSource: selectedConnection.catalogSource,
        endpointId: selectedConnection.id,
        method: selectedMethod.fullName,
        metadata: metadata.value,
        body: body.value,
      })
      await refreshHistory()
    } catch (error) {
      setActionError(error, $i18n.t('errors.unaryInvoke'))
      await refreshHistory()
    } finally {
      invokePending = false
    }
  }

  async function runStreamStart(): Promise<void> {
    if (!selectedConnection || !selectedMethod) {
      streamActionError = $i18n.t('errors.selectMethodInvoke')
      streamActionInfo = ''
      return
    }

    if (selectedMethod.rpcType === 'unary') {
      streamActionError = $i18n.t('errors.streamingOnly')
      streamActionInfo = ''
      return
    }

    const metadata = buildMetadataFromRows(metadataRows)
    if (!metadata.ok) {
      metadataError = metadataValidationMessage(metadata.error)
      return
    }
    metadataError = ''
    streamActionError = ''
    streamActionInfo = ''

    let requestSpec
    if (selectedMethod.rpcType === 'server_stream') {
      const body = parseJsonDraft(requestBodyText)
      if (!body.ok) {
        requestBodyError = requestBodyValidationMessage(body)
        requestBodyLintState = 'invalid'
        requestBodyLintMessage = $i18n.t('request.jsonInvalid')
        return
      }
      requestSpec = {
        mode: 'static-sequence' as const,
        messages: [{ body: body.value }],
      }
    } else {
      requestSpec = {
        mode: 'interactive' as const,
      }
    }

    streamActionPending = true
    try {
      const result = await startStream({
        catalogSource: selectedConnection.catalogSource,
        endpointId: selectedConnection.id,
        method: selectedMethod.fullName,
        rpcType: selectedMethod.rpcType,
        metadata: metadata.value,
        requestSpec,
      })
      activeStreamResult = result
      streamActionInfo = $i18n.t('stream.started', { sessionId: result.sessionId })
      await refreshHistory()
    } catch (error) {
      setStreamActionError(error, $i18n.t('errors.streamStart'))
      await refreshHistory()
    } finally {
      streamActionPending = false
    }
  }

  async function sendActiveStreamMessage(): Promise<void> {
    if (!activeStreamSessionRef) {
      streamActionError = $i18n.t('errors.streamSessionRequired')
      streamActionInfo = ''
      return
    }

    const body = parseJsonDraft(requestBodyText)
    if (!body.ok) {
      requestBodyError = requestBodyValidationMessage(body)
      requestBodyLintState = 'invalid'
      requestBodyLintMessage = $i18n.t('request.jsonInvalid')
      return
    }

    streamActionPending = true
    streamActionError = ''
    streamActionInfo = ''
    try {
      const result = await sendStreamMessage({
        sessionId: activeStreamSessionRef,
        message: { body: body.value },
      })
      streamActionInfo = $i18n.t('stream.messageSent', { index: result.messageIndex })
    } catch (error) {
      setStreamActionError(error, $i18n.t('errors.streamSend'))
    } finally {
      streamActionPending = false
    }
  }

  async function halfCloseActiveStream(): Promise<void> {
    if (!activeStreamSessionRef) {
      streamActionError = $i18n.t('errors.streamSessionRequired')
      streamActionInfo = ''
      return
    }

    streamActionPending = true
    streamActionError = ''
    streamActionInfo = ''
    try {
      await halfCloseStream({ sessionId: activeStreamSessionRef })
      streamActionInfo = $i18n.t('stream.halfClosed')
    } catch (error) {
      setStreamActionError(error, $i18n.t('errors.streamHalfClose'))
    } finally {
      streamActionPending = false
    }
  }

  async function cancelActiveStream(): Promise<void> {
    if (!activeStreamSessionRef) {
      streamActionError = $i18n.t('errors.streamSessionRequired')
      streamActionInfo = ''
      return
    }

    streamActionPending = true
    streamActionError = ''
    streamActionInfo = ''
    try {
      await cancelStream({ sessionId: activeStreamSessionRef })
      streamActionInfo = $i18n.t('stream.cancelled')
      await refreshHistory()
    } catch (error) {
      setStreamActionError(error, $i18n.t('errors.streamCancel'))
    } finally {
      streamActionPending = false
    }
  }

  function runPrimaryInvoke(): void {
    if (selectedMethod?.rpcType === 'unary') {
      void runUnaryInvoke()
      return
    }

    void runStreamStart()
  }

  async function runRequestSave(): Promise<void> {
    if (!selectedConnection || !selectedMethod) {
      actionError = $i18n.t('errors.requestSaveLoadedMethod')
      saveInfoMessage = ''
      return
    }

    clearActionError()

    if (!workspaceAvailable) {
      actionError = $i18n.t('errors.requestSaveWorkspaceRequired')
      saveInfoMessage = ''
      return
    }

    if (selectedMethod.rpcType !== 'unary') {
      actionError = $i18n.t('errors.requestSaveUnaryOnly')
      return
    }

    if (!lintRequestBodyDraft(false)) {
      return
    }

    const body = parseJsonDraft(requestBodyText)
    if (!body.ok) {
      return
    }

    const metadata = buildMetadataFromRows(metadataRows)
    if (!metadata.ok) {
      metadataError = metadataValidationMessage(metadata.error)
      return
    }
    metadataError = ''

    pendingRequestSave = { body: body.value, metadata: metadata.value }
    saveRequestName = editingSavedRequest
      ? savedRequestMethodName(editingSavedRequest)
      : savedRequestMethodName({
          id: '',
          name: '',
          path: '',
          method: selectedMethod.fullName,
          rpcType: 'unary',
          endpointRef: selectedConnection.id,
        })
    saveRequestNameError = ''
    if (editingSavedRequest) {
      await confirmRequestSave()
      return
    }
    saveRequestDialogOpen = true
    await tick()
    saveRequestNameInput?.focus()
  }

  function closeSaveRequestDialog(): void {
    if (requestSavePending) {
      return
    }
    saveRequestDialogOpen = false
    saveRequestName = ''
    saveRequestNameError = ''
    pendingRequestSave = null
  }

  async function confirmRequestSave(): Promise<void> {
    const name = saveRequestName.trim()
    if (!name) {
      saveRequestNameError = $i18n.t('request.saveRequestNameRequired')
      saveRequestNameInput?.focus()
      return
    }
    if (!selectedConnection || !selectedMethod || !pendingRequestSave) {
      closeSaveRequestDialog()
      return
    }

    if (!workspaceAvailable) {
      actionError = $i18n.t('errors.requestSaveWorkspaceRequired')
      closeSaveRequestDialog()
      return
    }

    const duplicate = savedRequests.some(
      (request) =>
        request.endpointRef === selectedConnection.id &&
        request.id !== editingSavedRequest?.id &&
        request.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase(),
    )
    if (duplicate) {
      saveRequestNameError = $i18n.t('request.saveRequestNameDuplicate', { name })
      saveRequestNameInput?.focus()
      return
    }

    const wasEditing = Boolean(editingSavedRequest)
    requestSavePending = true
    saveRequestNameError = ''
    let saved = false
    try {
      const result = await saveRequest(
        buildUnaryRequestSaveInput({
          connection: selectedConnection,
          method: selectedMethod,
          id: editingSavedRequest?.id ?? createSavedRequestId(selectedMethod),
          name,
          body: pendingRequestSave.body,
          metadata: pendingRequestSave.metadata,
        }),
      )
      dispatch('workspacesnapshot', result.workspace)
      editingSavedRequest = result.savedRequest
      saveInfoMessage = $i18n.t(wasEditing ? 'request.saveRequestUpdated' : 'request.saveRequestSuccess', {
        name: savedRequestMethodName(result.savedRequest),
      })
      saved = true
    } catch (error) {
      if (error instanceof BackendResponseError && error.code === 'workspace.not_open') {
        actionError = $i18n.t('errors.requestSaveWorkspaceRequired')
      } else if (error instanceof BackendResponseError && error.code === 'validation.saved_request_name_duplicate') {
        saveRequestNameError = $i18n.t('request.saveRequestNameDuplicate', { name })
      } else {
        setActionError(error, $i18n.t('errors.requestSave'))
      }
    } finally {
      requestSavePending = false
      if (saved) {
        closeSaveRequestDialog()
      }
    }
  }

  function handleComposerKeydown(event: KeyboardEvent): void {
    if (isShortcutEvent(event, 'request.formatJson')) {
      event.preventDefault()
      event.stopPropagation()
      formatRequestBodyDraft()
      return
    }

    if (isShortcutEvent(event, 'request.send')) {
      event.preventDefault()
      event.stopPropagation()
      runPrimaryInvoke()
      return
    }

    if (isShortcutEvent(event, 'request.save')) {
      event.preventDefault()
      event.stopPropagation()
      void runRequestSave()
    }
  }

  function handleGlobalShortcut(event: KeyboardEvent): void {
    if (event.key === 'Escape' && savedRequestRenameTarget) {
      event.preventDefault()
      closeSavedRequestRenameDialog()
      return
    }

    if (event.key === 'Escape' && clientMethodPickerOpen) {
      event.preventDefault()
      clientMethodPickerOpen = false
      clientMethodSearchQuery = ''
      return
    }

    if (event.key === 'Escape' && clientSavedRequestsModalOpen) {
      event.preventDefault()
      closeClientSavedRequestsModal()
      return
    }

    if (!shortcutsEnabled || event.defaultPrevented) {
      return
    }

    const targetIsEditable = isEditableShortcutTarget(event.target)

    if (isShortcutEvent(event, 'view.client')) {
      event.preventDefault()
      activeMonitorMode = 'client'
      return
    }

    if (isShortcutEvent(event, 'view.traffic')) {
      event.preventDefault()
      activeMonitorMode = 'traffic'
      return
    }

    if (isShortcutEvent(event, 'request.formatJson')) {
      event.preventDefault()
      formatRequestBodyDraft()
      return
    }

    if (isShortcutEvent(event, 'request.send')) {
      event.preventDefault()
      runPrimaryInvoke()
      return
    }

    if (isShortcutEvent(event, 'request.save')) {
      event.preventDefault()
      void runRequestSave()
      return
    }

    if (isShortcutEvent(event, 'sidebar.search')) {
      event.preventDefault()
      openSidebarSearch()
      return
    }

    if (targetIsEditable) {
      return
    }

    if (isShortcutEvent(event, 'sidebar.next')) {
      event.preventDefault()
      selectAdjacentConnection(1)
      return
    }

    if (isShortcutEvent(event, 'sidebar.previous')) {
      event.preventDefault()
      selectAdjacentConnection(-1)
      return
    }

    if (selectedConnection && isShortcutEvent(event, 'sidebar.rename')) {
      event.preventDefault()
      startConnectionRename(selectedConnection)
      return
    }

    if (selectedConnection && isShortcutEvent(event, 'sidebar.delete')) {
      event.preventDefault()
      dispatch('deleteconnection', selectedConnection.id)
    }
  }
</script>

<svelte:window on:click={handleWindowClick} on:keydown={handleGlobalShortcut} on:pointerdown={handleWindowPointerDown} />

<main class="monitor-grid">
  {#if leftSidebarVisible}
  <aside class="connection-sidebar">
    <div class="connection-sidebar__head">
      <h2>{$i18n.t('connections.title')}</h2>
      <span>{$i18n.t('connections.count', { count: connections.length })}</span>
    </div>

    {#if sidebarSearchOpen}
      <div class="connection-sidebar__search">
        <input
          aria-label={$i18n.t('connections.search')}
          bind:this={sidebarSearchInput}
          bind:value={sidebarSearchQuery}
          on:keydown={handleSidebarSearchKeydown}
          placeholder={$i18n.t('connections.search')}
          type="search"
        />
        <button aria-label={$i18n.t('common.close')} on:click={closeSidebarSearch} type="button">×</button>
      </div>
    {/if}

    <div class:connection-list--with-search={sidebarSearchOpen} class="connection-list">
      {#if connections.length > 0}
        {#if visibleConnections.length > 0}
        {#each visibleConnections as connection}
          <div class:connection-item--active={selectedConnection?.id === connection.id} class="connection-item">
            <span class={statusClass(connection.status)}></span>
            <div
              aria-label={connection.name}
              class="connection-item__main"
              on:click={() => selectConnection(connection.id)}
              on:keydown={(event) => handleConnectionKeyboard(event, connection.id)}
              role="button"
              tabindex="0"
            >
              {#if renamingConnectionId === connection.id}
                <input
                  aria-label={$i18n.t('connection.rename')}
                  class="connection-rename-input"
                  bind:value={renamingValue}
                  on:blur={() => finishConnectionRename(connection)}
                  on:click={(event) => event.stopPropagation()}
                  on:keydown={(event) => handleRenameKeydown(event, connection)}
                />
              {:else}
                <strong on:dblclick={(event) => {
                  event.stopPropagation()
                  startConnectionRename(connection)
                }}>{connection.name}</strong>
              {/if}
              <span>{connection.endpoint}</span>
              <div class="connection-sparkline" aria-label={`${$i18n.t('monitor.avgResponse')}: ${connectionLatency(connection.id)}`}>
                <svg aria-hidden="true" viewBox="0 0 100 32" preserveAspectRatio="none">
                  <polyline points={connectionSparklinePoints(connection.id)}></polyline>
                </svg>
                <strong>{connectionLatency(connection.id)}</strong>
              </div>
              <small>{connection.updatedAt}</small>
            </div>
            <button
              aria-label={$i18n.t('connection.details')}
              aria-expanded={activeConnectionMenuId === connection.id}
              class="connection-item__menu"
              on:click={() => toggleConnectionMenu(connection.id)}
              type="button"
            >
              ⋮
            </button>
            {#if activeConnectionMenuId === connection.id}
              <div class="connection-menu" role="menu">
                <button on:click={() => openConnectionDetails(connection.id)} role="menuitem" type="button">
                  {$i18n.t('connection.details')}
                </button>
                <button class="connection-menu__danger" on:click={() => deleteConnection(connection.id)} role="menuitem" type="button">
                  {$i18n.t('connection.delete')}
                </button>
              </div>
            {/if}
          </div>
        {/each}
        {:else}
          <div class="empty-state empty-state--sidebar">
            <strong>{$i18n.t('connections.noSearchResults')}</strong>
            <span>{$i18n.t('connections.searchHint')}</span>
          </div>
        {/if}
      {:else}
        <div class="empty-state empty-state--sidebar">
          <strong>{$i18n.t('connections.emptyTitle')}</strong>
          <span>{$i18n.t('connections.emptyCopy')}</span>
        </div>
      {/if}
    </div>
  </aside>
  {/if}

  <slot name="sidebar-footer"></slot>

  <section class:monitor-content--sidebar-hidden={!leftSidebarVisible} class="monitor-content">
    <nav class="monitor-mode-switch" aria-label={$i18n.t('monitor.modeLabel')}>
      <button
        aria-pressed={activeMonitorMode === 'client'}
        class:monitor-mode-switch__button--active={activeMonitorMode === 'client'}
        class="monitor-mode-switch__button"
        on:click={() => (activeMonitorMode = 'client')}
        type="button"
      >
        <span aria-hidden="true">⌘</span>
        {$i18n.t('monitor.modeClient')}
      </button>
      <button
        aria-pressed={activeMonitorMode === 'traffic'}
        class:monitor-mode-switch__button--active={activeMonitorMode === 'traffic'}
        class="monitor-mode-switch__button"
        on:click={() => (activeMonitorMode = 'traffic')}
        type="button"
      >
        <span aria-hidden="true">↕</span>
        {$i18n.t('monitor.modeTraffic')}
      </button>
    </nav>

    {#if activeMonitorMode === 'traffic'}
    <div class="metric-row">
      <article class="metric-card">
        <span class="metric-icon">∿</span>
        <small class={metricDeltaClass(metricDeltas.requests)}>{metricDeltaLabel(metricDeltas.requests)}</small>
        <strong>{completedCalls}</strong>
        <span>{$i18n.t('monitor.requestsPerSecond')}</span>
      </article>
      <article class="metric-card">
        <span class="metric-icon">◷</span>
        <small class={metricDeltaClass(metricDeltas.duration)}>{metricDeltaLabel(metricDeltas.duration, 'ms')}</small>
        <strong>{averageDurationMs}ms</strong>
        <span>{$i18n.t('monitor.avgResponse')}</span>
      </article>
      <article class="metric-card">
        <span class="metric-icon">↗</span>
        <small class={metricDeltaClass(metricDeltas.success)}>{metricDeltaLabel(metricDeltas.success, '%')}</small>
        <strong>{successRate}</strong>
        <span>{$i18n.t('monitor.successRate')}</span>
      </article>
      <article class="metric-card">
        <span class="metric-icon">ϟ</span>
        <small class={metricDeltaClass(metricDeltas.streams)}>{metricDeltaLabel(metricDeltas.streams)}</small>
        <strong>{activeStreamCount}</strong>
        <span>{$i18n.t('monitor.activeStreams')}</span>
      </article>
    </div>
    {/if}

    <div class="monitor-lower-grid">
      <div class="monitor-workstack">
        {#if activeMonitorMode === 'traffic'}
        <section class="traffic-view">
          <div class="traffic-toolbar">
            <div>
              <span class="eyebrow">{$i18n.t('monitor.modeTraffic')}</span>
              <h2>{$i18n.t('monitor.requestLog')}</h2>
              <span class="traffic-toolbar__scope">
                {$i18n.t('history.connectionScope', { connection: selectedConnection?.name ?? $i18n.t('connections.emptyTitle') })}
              </span>
            </div>
            <div class="traffic-toolbar__filters">
              <select aria-label={$i18n.t('method.allMethods')} class="traffic-filter" bind:value={trafficMethodFilter}>
                <option value="all">{$i18n.t('method.allMethods')}</option>
                {#each trafficMethodOptions as method}
                  <option value={method}>{methodDisplayName(method)}</option>
                {/each}
              </select>
              <select aria-label={$i18n.t('history.allStatuses')} class="traffic-filter" bind:value={trafficStatusFilter}>
                <option value="all">{$i18n.t('history.allStatuses')}</option>
                {#each trafficStatusOptions as status}
                  <option value={status}>{status}</option>
                {/each}
              </select>
              <label aria-label={$i18n.t('catalog.search')} class="traffic-search">
                <span aria-hidden="true">⌕</span>
                <input bind:value={trafficSearchQuery} placeholder={$i18n.t('catalog.search')} type="search" />
              </label>
              <button
                aria-label={$i18n.t('history.refresh')}
                class:traffic-icon-button--loading={historyPending}
                class="traffic-icon-button"
                disabled={!historyEndpointId || historyPending}
                on:click={refreshHistory}
                title={$i18n.t('history.refreshHint')}
                type="button"
              >↻</button>
            </div>
          </div>

          <div class="traffic-layout traffic-layout--expanded-rows">
            <section class="request-log">
              <div class="traffic-metrics-line">
                {#if selectedConnection?.status === 'online'}
                  <span class="traffic-live-dot"></span>
                  <strong>{$i18n.t('monitor.live')}</strong>
                {:else}
                  <strong class="traffic-metrics-line__offline">{$i18n.t('monitor.notConnected')}</strong>
                {/if}
                <span>{completedCalls} {$i18n.t('monitor.requests')}</span>
                <span>{averageDurationMs}ms {$i18n.t('monitor.median')}</span>
                <span>{successRate} {$i18n.t('monitor.successRate')}</span>
              </div>

              <div class="request-table">
                <div class="request-table__head">
                  <span>{$i18n.t('history.status')}</span>
                  <span>{$i18n.t('method.method')}</span>
                  <span>{$i18n.t('monitor.service')}</span>
                  <span>{$i18n.t('monitor.size')}</span>
                  <span>{$i18n.t('history.duration')}</span>
                  <span>{$i18n.t('monitor.timestamp')}</span>
                  <span></span>
                </div>

                <div class="request-table__body">
                  {#if trafficCalls.length > 0}
                    {#each trafficCalls as call}
                      <button
                        aria-expanded={selectedHistoryCallId === call.callId}
                        class={selectedHistoryCallId === call.callId
                          ? 'request-table__row request-table__row--button request-table__row--selected'
                          : 'request-table__row request-table__row--button'}
                        on:click={() => loadHistoryDetail(call.callId)}
                        type="button"
                      >
                        <span class="request-status-cell">
                          <span class={requestStatusClass(call)}></span>
                          <span>{call.truncated ? $i18n.t('history.truncated') : call.grpcStatusCode || call.state}</span>
                        </span>
                        <strong>{methodDisplayName(call.method)}</strong>
                        <span class="request-table__service">{serviceDisplayName(call.method)}</span>
                        <span>{call.requestCount}/{call.responseCount}</span>
                        <span>{formatDuration(call.durationMs)}</span>
                        <span>{formatTime(call.finishedAt ?? call.startedAt)}</span>
                        <span class="request-row-expander">{selectedHistoryCallId === call.callId ? '‹' : '›'}</span>
                      </button>
                      {#if selectedHistoryCallId === call.callId}
                        <div bind:this={historyDetailElement} class="traffic-request-detail">
                          {#if historyDetailPending || historyDetail?.summary.callId !== call.callId}
                            <div class="empty-state traffic-request-detail__loading"><strong>{$i18n.t('history.loading')}</strong></div>
                          {:else}
                            <header class="traffic-request-detail__head">
                              <div>
                                <span class="eyebrow">{formatRpcType(historyDetail.summary.rpcType)}</span>
                                <h3>{methodDisplayName(historyDetail.summary.method)}</h3>
                              </div>
                              <div class="traffic-request-detail__meta">
                                <span class:response-status-badge--error={historyDetail.status.code !== 'OK'} class="response-status-badge">{historyDetail.status.code}</span>
                                <span>{formatDuration(historyDetail.summary.durationMs)}</span>
                                <span>{formatTime(historyDetail.summary.finishedAt ?? historyDetail.summary.startedAt)}</span>
                              </div>
                            </header>
                            <div class="traffic-request-detail__payloads">
                              <section>
                                <h4>{$i18n.t('request.bodyJson')}</h4>
                                <pre class="code-block traffic-request-detail__code">{@html highlightJson(formatJsonValue(historyDetail.requestBody))}</pre>
                              </section>
                              <section>
                                <h4>{$i18n.t('response.body')}</h4>
                                <pre class="code-block traffic-request-detail__code">{@html highlightJson(formatJsonValue(historyDetail.responseBody))}</pre>
                              </section>
                              {#if Object.keys(historyCustomMetadata).length > 0}
                                <section>
                                  <h4>{$i18n.t('request.tabMetadata')}</h4>
                                  <pre class="code-block traffic-request-detail__code">{@html highlightJson(formatJsonValue(historyCustomMetadata))}</pre>
                                </section>
                              {/if}
                            </div>
                            <section class="traffic-request-detail__timeline">
                              <div class="traffic-request-detail__timeline-head">
                                <h4>{$i18n.t('history.timeline')}</h4>
                                <strong>{formatDuration(historyDetail.summary.durationMs)}</strong>
                              </div>
                              <div class="traffic-timeline">
                                <div>
                                  <span>{$i18n.t('timeline.callDuration')}</span>
                                  <i><b style={`width: ${timelineVisualWidth(historyDetail.summary.durationMs)}%`}></b></i>
                                  <strong>{formatDuration(historyDetail.summary.durationMs)}</strong>
                                </div>
                              </div>
                            </section>
                          {/if}
                        </div>
                      {/if}
                    {/each}
                  {:else if calls.length > 0}
                    <div class="empty-state request-table__empty">
                      <strong>{$i18n.t('catalog.noSearchResults')}</strong>
                      <span>{$i18n.t('catalog.searchHint')}</span>
                    </div>
                  {:else}
                    <div class="empty-state request-table__empty">
                      <strong>{historyLoadFailed ? $i18n.t('history.loadFailed') : $i18n.t('history.emptyTitle')}</strong>
                      <span>{historyLoadFailed ? $i18n.t('history.loadFailedCopy') : $i18n.t('history.emptyCopy')}</span>
                    </div>
                  {/if}
                </div>
              </div>
            </section>
          </div>
        </section>
        {/if}

        {#if activeMonitorMode === 'client'}
        <section class="unary-workbench">
          <div class="client-command-bar">
            <h2>{$i18n.t('monitor.modeClient')}</h2>
            <div class="client-method-picker">
              <button
                aria-expanded={clientMethodPickerOpen}
                aria-haspopup="listbox"
                aria-label={$i18n.t('method.method')}
                class="client-method-trigger"
                disabled={!selectedConnection || clientMethodOptions.length === 0}
                on:click={() => {
                  clientMethodPickerOpen = !clientMethodPickerOpen
                  if (clientMethodPickerOpen) {
                    clientMethodSearchQuery = ''
                  }
                }}
                type="button"
              >
                <span>{selectedMethod ? methodDisplayName(selectedMethod.fullName) : $i18n.t('request.emptyTitle')}</span>
                <span aria-hidden="true" class="client-method-trigger__actions"><b>⋯</b><i>⌄</i></span>
              </button>
            </div>
            <div class="client-saved-requests-picker">
              <button
                aria-expanded={clientSavedRequestsModalOpen}
                aria-haspopup="dialog"
                aria-label={$i18n.t('request.savedRequests')}
                class="client-method-trigger client-saved-requests-trigger"
                disabled={!selectedConnection || savedRequestsForSelectedConnection.length === 0}
                on:click={() => void openClientSavedRequestsModal()}
                type="button"
              >
                <span>{$i18n.t('request.savedRequests')}</span>
                <span aria-hidden="true" class="client-method-trigger__actions"><b>⋯</b><i>⌄</i></span>
              </button>
            </div>
            <button class="primary-command client-command-bar__send" disabled={invokePending || streamActionPending || !selectedMethod} on:click={runPrimaryInvoke} type="button">
              {invokePending || streamActionPending ? $i18n.t('request.invokePending') : $i18n.t('request.invoke')} <span aria-hidden="true">⌘↵</span>
            </button>
          </div>
          <div class="panel-title-row">
            <h2>{$i18n.t('monitor.workbench')}</h2>
          </div>

          <div class="unary-workbench__body">
            <aside class="method-catalog method-catalog--workbench">
              <div class="method-catalog__head">
                <h3>{$i18n.t('catalog.title')}</h3>
                {#if selectedConnection && selectedConnection.services.length > 0}
                  <span>{visibleCatalogMethods.length}</span>
                {/if}
              </div>

              {#if selectedConnection && selectedConnection.services.length > 0}
                <div class="method-catalog__search">
                  <input
                    aria-label={$i18n.t('catalog.search')}
                    bind:value={methodSearchQuery}
                    placeholder={$i18n.t('catalog.search')}
                    type="search"
                  />
                </div>
                <div class="method-catalog__services method-catalog__services--flat">
                  {#if visibleCatalogMethods.length > 0}
                    {#each visibleCatalogMethods as { method }}
                      <button
                        class:method-service__method--active={selectedConnection.selectedMethodFullName === method.fullName}
                        class="method-service__method"
                        on:click={() => {
                          appliedSavedRequest = null
                          selectMethod(selectedConnection.id, method.fullName)
                        }}
                        type="button"
                      >
                        <span>{method.name}</span>
                      </button>
                    {/each}
                  {:else if visibleSavedRequests.length === 0}
                    <div class="empty-state method-catalog__empty">
                      <strong>{$i18n.t('catalog.noSearchResults')}</strong>
                      <span>{$i18n.t('catalog.searchHint')}</span>
                    </div>
                  {/if}
                </div>

                <section class="saved-request-list" aria-label={$i18n.t('request.savedRequests')}>
                  <button
                    aria-expanded={savedRequestsOpen}
                    class="saved-request-list__head"
                    on:click={() => (savedRequestsOpen = !savedRequestsOpen)}
                    type="button"
                  >
                    <strong>{$i18n.t('request.savedRequests')}</strong>
                    <span>
                      {methodSearchQuery.trim() ? visibleSavedRequests.length : savedRequestsForSelectedConnection.length}
                      <span class="saved-request-list__chevron">{savedRequestsOpen ? '⌄' : '›'}</span>
                    </span>
                  </button>
                  {#if savedRequestsOpen && visibleSavedRequests.length > 0}
                    <div class="saved-request-list__items">
                      {#each visibleSavedRequests as savedRequest}
                        <button
                          class="saved-request-item"
                          aria-label={$i18n.t('request.savedRequestUse')}
                          title={savedRequestMethodName(savedRequest)}
                          on:click={() => applySavedRequest(savedRequest)}
                          on:contextmenu={(event) => openSavedRequestMenu(event, savedRequest)}
                          type="button"
                        >
                          <span>{savedRequestMethodName(savedRequest)}</span>
                        </button>
                      {/each}
                    </div>
                  {:else if savedRequestsOpen}
                    <p>{$i18n.t('request.savedRequestsEmpty')}</p>
                  {/if}
                  {#if savedRequestMenu}
                    <div
                      class="saved-request-menu"
                      role="menu"
                      style={`left: ${savedRequestMenu.x}px; top: ${savedRequestMenu.y}px;`}
                      tabindex="-1"
                    >
                      <button
                        disabled={requestDeletePendingId === savedRequestMenu.request.id}
                        on:click={() => deleteSavedRequestSummary(savedRequestMenu!.request)}
                        role="menuitem"
                        type="button"
                      >
                        {$i18n.t('request.deleteSavedRequest')}
                      </button>
                    </div>
                  {/if}
                </section>
              {:else}
                <div class="empty-state method-catalog__empty">
                  <strong>{selectedConnection ? $i18n.t('catalog.emptyTitle') : $i18n.t('monitor.noConnectionTitle')}</strong>
                  <span>{selectedConnection ? $i18n.t('catalog.emptyCopy') : $i18n.t('monitor.noConnectionCopy')}</span>
                </div>
              {/if}
            </aside>

            <div class="workbench-main">
              <div class="composer-toolbar">
                <div class="composer-tabs" aria-label={$i18n.t('request.composerTitle')}>
                  <button
                    class:composer-tab--active={activeComposerTab === 'request'}
                    class="composer-tab"
                    on:click={() => (activeComposerTab = 'request')}
                    type="button"
                  >
                    ▧ {$i18n.t('request.tabRequest')}
                  </button>
                  <button
                    class:composer-tab--active={activeComposerTab === 'schema'}
                    class="composer-tab"
                    on:click={() => (activeComposerTab = 'schema')}
                    type="button"
                  >
                    ‹› {$i18n.t('request.tabSchema')}
                  </button>
                  <button
                    class:composer-tab--active={activeComposerTab === 'metadata'}
                    class="composer-tab"
                    on:click={() => (activeComposerTab = 'metadata')}
                    type="button"
                  >
                    ▤ {$i18n.t('request.tabMetadata')}
                  </button>
                  <button
                    class:composer-tab--active={activeComposerTab === 'authorization'}
                    class="composer-tab"
                    on:click={() => (activeComposerTab = 'authorization')}
                    type="button"
                  >
                    ▣ {$i18n.t('request.tabAuthorization')}
                  </button>
                </div>

                {#if appliedSavedRequest}
                  <span class="composer-saved-request-state" title={appliedSavedRequest.method}>
                    {$i18n.t('request.savedRequestInUse', { name: savedRequestMethodName(appliedSavedRequest) })}
                  </span>
                {/if}

                {#if selectedMethod}
                  <div class="composer-toolbar__actions">
                    <button class="secondary-command secondary-command--compact" on:click={resetBodyDraft} type="button">
                      {$i18n.t('request.resetTemplate')}
                    </button>
                    {#if selectedMethod.rpcType === 'unary'}
                      <button
                        class="secondary-command secondary-command--compact"
                        disabled={requestSavePending}
                        on:click={runRequestSave}
                        type="button"
                      >
                        {requestSavePending ? $i18n.t('request.saveRequestPending') : editingSavedRequest ? $i18n.t('request.saveRequestUpdate') : $i18n.t('request.saveRequest')}
                      </button>
                      <button class="primary-command primary-command--compact" disabled={invokePending} on:click={runUnaryInvoke} type="button">
                        {invokePending ? $i18n.t('request.invokePending') : $i18n.t('request.invoke')}
                      </button>
                    {:else}
                      <button class="primary-command primary-command--compact" disabled={streamActionPending} on:click={runStreamStart} type="button">
                        {streamActionPending ? $i18n.t('stream.starting') : $i18n.t('stream.start')}
                      </button>
                      <button
                        class="secondary-command secondary-command--compact"
                        disabled={streamActionPending || !canSendStreamMessage}
                        on:click={sendActiveStreamMessage}
                        type="button"
                      >
                        {$i18n.t('stream.sendMessage')}
                      </button>
                      <button
                        class="secondary-command secondary-command--compact"
                        disabled={streamActionPending || !canHalfCloseStream}
                        on:click={halfCloseActiveStream}
                        type="button"
                      >
                        {$i18n.t('stream.halfClose')}
                      </button>
                      <button
                        class="secondary-command secondary-command--compact"
                        disabled={streamActionPending || !canCancelStream}
                        on:click={cancelActiveStream}
                        type="button"
                      >
                        {$i18n.t('stream.cancel')}
                      </button>
                    {/if}
                  </div>
                {/if}
              </div>

              <div class:workbench-panels--response-hidden={!rightPanelVisible} class="workbench-panels">
                <div class="composer-panel">
                  <div class="composer-panel__head">
                    <h3>
                      {#if activeComposerTab === 'schema'}
                        {$i18n.t('request.tabSchema')}
                      {:else if activeComposerTab === 'metadata'}
                        {$i18n.t('request.metadataTitle')}
                      {:else if activeComposerTab === 'authorization'}
                        {$i18n.t('request.authorizationHeader')}
                      {:else}
                        {$i18n.t('request.bodyJson')}
                      {/if}
                    </h3>
                  </div>

                  {#if !selectedConnection || !selectedMethod}
                    <div class="empty-state composer-panel__empty">
                      <strong>{$i18n.t('request.emptyTitle')}</strong>
                      <span>{$i18n.t('request.empty')}</span>
                    </div>
                  {:else}
                    {#if activeComposerTab === 'schema'}
                      <div class="method-summary">
                        <div>
                          <span>{$i18n.t('method.method')}</span>
                          <strong>{selectedMethod.fullName}</strong>
                        </div>
                        <div>
                          <span>{$i18n.t('method.requestType')}</span>
                          <strong>{selectedMethod.requestType.fullName}</strong>
                        </div>
                        <div>
                          <span>{$i18n.t('method.responseType')}</span>
                          <strong>{selectedMethod.responseType.fullName}</strong>
                        </div>
                      </div>
                      <div class="schema-contract">
                        <section class="schema-contract__message">
                          <div class="schema-contract__head">
                            <div>
                              <span>{$i18n.t('schema.requestFields')}</span>
                              <strong>{selectedMethod.requestType.fullName}</strong>
                            </div>
                            <span>{selectedMethod.requestType.fields?.length ?? 0}</span>
                          </div>
                          {#if selectedMethod.requestType.fields?.length}
                            <div class="schema-contract__fields">
                              {#each selectedMethod.requestType.fields as field}
                                <div class="schema-contract__field">
                                  <strong>{field.jsonName || field.name}</strong>
                                  <span>{schemaFieldLabel(field)}</span>
                                  {#if field.fields?.length}
                                    <div class="schema-contract__nested">
                                      <span>{$i18n.t('schema.nestedFields')}</span>
                                      {#each field.fields as nestedField}
                                        <div>
                                          <strong>{nestedField.jsonName || nestedField.name}</strong>
                                          <span>{schemaFieldLabel(nestedField)}</span>
                                        </div>
                                      {/each}
                                    </div>
                                  {/if}
                                </div>
                              {/each}
                            </div>
                          {:else}
                            <p>{$i18n.t('schema.noFields')}</p>
                          {/if}
                        </section>
                        <section class="schema-contract__message">
                          <div class="schema-contract__head">
                            <div>
                              <span>{$i18n.t('schema.responseFields')}</span>
                              <strong>{selectedMethod.responseType.fullName}</strong>
                            </div>
                            <span>{selectedMethod.responseType.fields?.length ?? 0}</span>
                          </div>
                          {#if selectedMethod.responseType.fields?.length}
                            <div class="schema-contract__fields">
                              {#each selectedMethod.responseType.fields as field}
                                <div class="schema-contract__field">
                                  <strong>{field.jsonName || field.name}</strong>
                                  <span>{schemaFieldLabel(field)}</span>
                                  {#if field.fields?.length}
                                    <div class="schema-contract__nested">
                                      <span>{$i18n.t('schema.nestedFields')}</span>
                                      {#each field.fields as nestedField}
                                        <div>
                                          <strong>{nestedField.jsonName || nestedField.name}</strong>
                                          <span>{schemaFieldLabel(nestedField)}</span>
                                        </div>
                                      {/each}
                                    </div>
                                  {/if}
                                </div>
                              {/each}
                            </div>
                          {:else}
                            <p>{$i18n.t('schema.noFields')}</p>
                          {/if}
                        </section>
                      </div>
                    {:else if activeComposerTab === 'metadata'}
                      <div class="metadata-editor">
                        <div class="metadata-editor__head">
                          <span>{$i18n.t('request.metadataTitle')}</span>
                          <button class="secondary-command secondary-command--compact" on:click={addMetadataRow} type="button">
                            {$i18n.t('request.metadataAdd')}
                          </button>
                        </div>

                        <div class="metadata-editor__rows">
                          {#if regularMetadataRows.length === 0}
                            <div class="metadata-editor__empty">{$i18n.t('request.metadataEmpty')}</div>
                          {/if}

                          {#each regularMetadataRows as row (row.id)}
                            <div class="metadata-row">
                              <input
                                aria-label={$i18n.t('request.metadataKey')}
                                placeholder={$i18n.t('request.metadataKeyPlaceholder')}
                                value={row.key}
                                on:input={(event) => updateMetadataRow(row.id, 'key', (event.currentTarget as HTMLInputElement).value)}
                              />
                              <input
                                aria-label={$i18n.t('request.metadataValue')}
                                placeholder={$i18n.t('request.metadataValuePlaceholder')}
                                value={row.value}
                                on:input={(event) => updateMetadataRow(row.id, 'value', (event.currentTarget as HTMLInputElement).value)}
                              />
                              <button
                                aria-label={$i18n.t('request.metadataRemove')}
                                class="metadata-row__remove"
                                on:click={() => removeMetadataRow(row.id)}
                                type="button"
                              >
                                ×
                              </button>
                            </div>
                          {/each}
                        </div>
                      </div>
                    {:else if activeComposerTab === 'authorization'}
                      <div class="metadata-editor">
                        <div class="metadata-editor__head">
                          <span>{$i18n.t('request.authorizationHeader')}</span>
                        </div>
                        <label class="composer-field">
                          <span>{$i18n.t('request.authorizationValue')}</span>
                          <input
                            class="authorization-input"
                            placeholder={$i18n.t('request.authorizationPlaceholder')}
                            value={authorizationMetadataValue}
                            on:input={(event) => updateAuthorizationMetadata((event.currentTarget as HTMLInputElement).value)}
                          />
                        </label>
                      </div>
                    {:else}
                      <div class="composer-field composer-field--textarea">
                        <div class="composer-field__head">
                          <div class="json-editor-actions">
                            <button class="editor-command" on:click={formatRequestBodyDraft} type="button">
                              {$i18n.t('request.formatJson')}
                            </button>
                            <button class="editor-command" on:click={() => lintRequestBodyDraft()} type="button">
                              {$i18n.t('request.lintJson')}
                            </button>
                          </div>
                        </div>
                        <div class="json-editor-shell">
                          <pre aria-hidden="true" bind:this={jsonEditorHighlight} class="json-editor-highlight">{@html highlightJson(requestBodyText)}</pre>
                          <textarea
                            aria-label={$i18n.t('request.bodyJson')}
                            class="json-editor json-editor--syntax"
                            rows="14"
                            spellcheck="false"
                            value={requestBodyText}
                            on:input={(event) => updateBodyDraft((event.currentTarget as HTMLTextAreaElement).value)}
                            on:keydown={handleComposerKeydown}
                            on:scroll={syncJsonEditorScroll}
                          ></textarea>
                        </div>
                      </div>
                      {#if requestBodyError}
                        <div class="inline-error" role="alert">{requestBodyError}</div>
                      {/if}
                    {/if}

                    {#if metadataError}
                      <div class="inline-error" role="alert">{metadataError}</div>
                    {/if}

                    {#if actionError}
                      <div class="inline-error inline-error--panel" role="alert">{actionError}</div>
                    {/if}
                    {#if streamActionError}
                      <div class="inline-error inline-error--panel" role="alert">{streamActionError}</div>
                    {/if}
                    {#if streamActionInfo}
                      <div class="inline-success inline-success--panel" role="status">{streamActionInfo}</div>
                    {/if}

                  {/if}
                </div>

                {#if rightPanelVisible}
                <div class="response-panel">
                  <div class="composer-panel__head response-panel__head">
                    <h3>{$i18n.t('response.panelTitle')}</h3>
                    {#if responseStatus}
                      <div class="response-panel__head-actions">
                        <div class="response-panel__tabs" aria-label={$i18n.t('response.panelTitle')}>
                          <button
                            class:response-panel__tab--active={activeResponseTab === 'body'}
                            class="response-panel__tab"
                            on:click={() => (activeResponseTab = 'body')}
                            type="button"
                          >
                            {$i18n.t('response.body')}
                          </button>
                          <button
                            class:response-panel__tab--active={activeResponseTab === 'headers'}
                            class="response-panel__tab"
                            on:click={() => (activeResponseTab = 'headers')}
                            type="button"
                          >
                            {$i18n.t('response.headers')}
                          </button>
                          <button
                            class:response-panel__tab--active={activeResponseTab === 'trailers'}
                            class="response-panel__tab"
                            on:click={() => (activeResponseTab = 'trailers')}
                            type="button"
                          >
                            {$i18n.t('response.trailers')}
                          </button>
                        </div>
                        <span class:response-status-badge--error={responseStatus.code !== 'OK'} class="response-status-badge">
                          {responseStatus.code}
                        </span>
                      </div>
                    {/if}
                  </div>

                  {#if selectedMethod && selectedMethod.rpcType !== 'unary'}
                    <div class="stream-panel">
                      <div class="stream-panel__summary">
                        <div>
                          <span>{$i18n.t('stream.state')}</span>
                          <strong>{translateStreamStateLabel($i18n.language, streamState)}</strong>
                        </div>
                        <div>
                          <span>{$i18n.t('stream.session')}</span>
                          <strong>{activeStreamSessionRef ? $i18n.t('stream.latestSession') : $i18n.t('stream.notStarted')}</strong>
                        </div>
                        <div>
                          <span>{$i18n.t('stream.events')}</span>
                          <strong>{activeStreamEvents.length}</strong>
                        </div>
                      </div>

                      {#if activeStreamCallRef}
                        <div class="stream-panel__call">
                          <span>{$i18n.t('stream.call')}</span>
                          <strong>{activeStreamCallRef}</strong>
                        </div>
                      {/if}

                      {#if selectedStreamCompleted}
                        <div class="inline-success inline-success--panel" role="status">
                          {$i18n.t('stream.completed', { status: selectedStreamCompleted.status.code })}
                        </div>
                      {/if}

                      {#each activeStreamErrors as streamError}
                        <div class="inline-error inline-error--panel" role="alert">
                          {streamError.error.code ? `${streamError.error.code}: ${streamError.error.message}` : streamError.error.message}
                        </div>
                      {/each}

                      <div class="stream-timeline">
                        {#if activeStreamEvents.length > 0}
                          {#each activeStreamEvents as event}
                            <article class="stream-timeline__event">
                              <div class="stream-timeline__event-head">
                                <span>#{event.seq}</span>
                                <strong>{event.direction} · {event.kind}</strong>
                                <small>{formatTime(event.ts)}</small>
                              </div>
                              <pre class="code-block stream-timeline__code">{@html highlightJson(formatJsonValue(event.payload.preview?.json ?? {}))}</pre>
                            </article>
                          {/each}
                        {:else}
                          <div class="empty-state response-panel__empty">
                            <strong>{$i18n.t('stream.emptyTitle')}</strong>
                            <span>{$i18n.t('stream.emptyCopy')}</span>
                          </div>
                        {/if}
                      </div>
                    </div>
                  {:else if historyDetailPending}
                    <div class="empty-state response-panel__empty">
                      <strong>{$i18n.t('history.loading')}</strong>
                    </div>
                  {:else if !responseStatus}
                    <div class="empty-state response-panel__empty">
                      <strong>{$i18n.t('response.emptyTitle')}</strong>
                      <span>{$i18n.t('response.empty')}</span>
                    </div>
                  {:else}
                    {#if responseErrorMessage}
                      <div class="inline-error inline-error--panel response-error" role="alert">
                        <strong>{responseStatus?.code}</strong>
                        <span>{responseErrorMessage}</span>
                      </div>
                    {/if}
                    {#if responseTruncated}
                      <div class="inline-warning" role="status">{$i18n.t('response.truncatedWarning')}</div>
                    {/if}

                    <div class="response-viewer">
                      <section class="response-viewer__panel">
                        {#if activeResponseTab === 'headers'}
                          <pre class="code-block response-viewer__code">{@html highlightJson(formatJsonValue(responseHeaders))}</pre>
                        {:else if activeResponseTab === 'trailers'}
                          <pre class="code-block response-viewer__code">{@html highlightJson(formatJsonValue(responseTrailers))}</pre>
                        {:else}
                          <pre class="code-block response-viewer__code">{@html highlightJson(formatJsonValue(responseBody))}</pre>
                        {/if}
                      </section>
                    </div>
                  {/if}
                </div>
                {/if}
              </div>
              <div class="client-support-grid">
                <section class="client-support-card">
                  <div class="client-support-card__head">
                    <h3>{$i18n.t('history.timeline')}</h3>
                    {#if responseStatus}<span class="response-status-badge">{formatDuration(invokeResult?.durationMs ?? historyDetail?.summary.durationMs)}</span>{/if}
                  </div>
                  <div class="client-timeline">
                    <div><span>{$i18n.t('timeline.callDuration')}</span><i><b style={`width: ${timelineVisualWidth(invokeResult?.durationMs ?? historyDetail?.summary.durationMs)}%`}></b></i><strong>{responseStatus ? formatDuration(invokeResult?.durationMs ?? historyDetail?.summary.durationMs) : '—'}</strong></div>
                  </div>
                </section>
              </div>
            </div>
          </div>
        </section>
        {/if}
      </div>
    </div>
  </section>
</main>

{#if clientMethodPickerOpen}
  <button
    aria-label={$i18n.t('common.close')}
    class="overlay-backdrop overlay-backdrop--method-picker"
    on:click={() => {
      clientMethodPickerOpen = false
      clientMethodSearchQuery = ''
    }}
    type="button"
  ></button>
  <div aria-modal="true" aria-labelledby="client-method-modal-title" class="client-method-modal" role="dialog">
    <header class="client-method-modal__head">
      <div>
        <span class="eyebrow">{$i18n.t('monitor.modeClient')}</span>
        <h2 id="client-method-modal-title">{$i18n.t('method.method')}</h2>
      </div>
      <button
        aria-label={$i18n.t('common.close')}
        class="modal-close"
        on:click={() => {
          clientMethodPickerOpen = false
          clientMethodSearchQuery = ''
        }}
        type="button"
      >×</button>
    </header>
    <input
      aria-label={$i18n.t('catalog.search')}
      class="client-method-search"
      bind:value={clientMethodSearchQuery}
      placeholder={$i18n.t('catalog.search')}
      type="search"
    />
    <div class="client-method-options" role="listbox">
      {#each clientFilteredMethods as { method, serviceName } (method.fullName)}
        <button
          aria-selected={selectedMethod?.fullName === method.fullName}
          class:client-method-option--active={selectedMethod?.fullName === method.fullName}
          class="client-method-option"
          on:click={() => {
            if (selectedConnection) {
              appliedSavedRequest = null
              selectMethod(selectedConnection.id, method.fullName)
            }
            editingSavedRequest = null
            clientMethodPickerOpen = false
            clientMethodSearchQuery = ''
          }}
          role="option"
          type="button"
        >
          <strong>{method.name}</strong>
          <span>{serviceName}</span>
        </button>
      {:else}
        <div class="client-method-empty">{$i18n.t('catalog.noSearchResults')}</div>
      {/each}
    </div>
  </div>
{/if}

{#if clientSavedRequestsModalOpen}
  <button
    aria-label={$i18n.t('common.close')}
    class="overlay-backdrop overlay-backdrop--saved-requests-picker"
    on:click={closeClientSavedRequestsModal}
    type="button"
  ></button>
  <div aria-modal="true" aria-labelledby="client-saved-requests-modal-title" class="client-saved-requests-modal" role="dialog">
    <header class="client-method-modal__head">
      <div>
        <span class="eyebrow">{$i18n.t('monitor.modeClient')}</span>
        <h2 id="client-saved-requests-modal-title">{$i18n.t('request.savedRequests')}</h2>
      </div>
      <button aria-label={$i18n.t('common.close')} class="modal-close" on:click={closeClientSavedRequestsModal} type="button">×</button>
    </header>
    <input
      aria-label={$i18n.t('catalog.search')}
      class="client-method-search"
      bind:value={savedRequestSearchQuery}
      placeholder={$i18n.t('catalog.search')}
      type="search"
    />
    <div class="client-saved-requests-modal__body">
      <div class="client-saved-requests-modal__list" role="listbox">
        {#each clientVisibleSavedRequests as savedRequest (savedRequest.id)}
          <button
            aria-selected={clientSavedRequestPreviewId === savedRequest.id}
            class:client-method-option--active={clientSavedRequestPreviewId === savedRequest.id}
            class="client-method-option"
            on:click={() => void previewClientSavedRequest(savedRequest)}
            role="option"
            type="button"
          >
            <strong>{savedRequestMethodName(savedRequest)}</strong>
            <span>{methodDisplayName(savedRequest.method)}</span>
          </button>
        {:else}
          <div class="client-method-empty">{$i18n.t('request.savedRequestsEmpty')}</div>
        {/each}
      </div>
      <aside class="client-saved-request-preview">
        {#if clientSavedRequestPreviewPending}
          <div class="empty-state"><strong>{$i18n.t('history.loading')}</strong></div>
        {:else if clientSavedRequestPreview && clientSavedRequestPreviewSummary}
          <div class="client-saved-request-preview__head">
            <div>
              <strong>{savedRequestMethodName(clientSavedRequestPreviewSummary)}</strong>
              <span>{methodDisplayName(clientSavedRequestPreview.method)}</span>
            </div>
            <span class="response-status-badge">{formatRpcType(clientSavedRequestPreview.rpcType)}</span>
          </div>
          <pre class="code-block client-saved-request-preview__body">{@html highlightJson(formatJsonValue(bodyFromSavedRequest(clientSavedRequestPreview)))}</pre>
          <div class="client-saved-request-preview__actions">
            <button class="primary-command primary-command--compact" on:click={() => void loadClientSavedRequest(clientSavedRequestPreviewSummary)} type="button">
              {$i18n.t('request.savedRequestUse')}
            </button>
            <button
              aria-label={$i18n.t('request.renameSavedRequest')}
              class="secondary-command secondary-command--compact"
              on:click={() => {
                const target = clientSavedRequestPreviewSummary
                if (target) void openSavedRequestRenameDialog(target)
              }}
              type="button"
            >✎</button>
            <button
              aria-label={$i18n.t('request.deleteSavedRequest')}
              class="secondary-command secondary-command--compact client-saved-request-preview__delete"
              disabled={requestDeletePendingId === clientSavedRequestPreviewSummary.id}
              on:click={() => {
                const target = clientSavedRequestPreviewSummary
                if (target) void deleteClientSavedRequest(target)
              }}
              type="button"
            >×</button>
          </div>
        {:else}
          <div class="empty-state"><strong>{$i18n.t('request.savedRequestsEmpty')}</strong></div>
        {/if}
      </aside>
    </div>
  </div>
{/if}

{#if detailConnection}
  <button
    aria-label={$i18n.t('common.close')}
    class="overlay-backdrop overlay-backdrop--connection-detail"
    on:click={closeConnectionDetails}
    type="button"
  ></button>
  <div
    aria-label={$i18n.t('connection.details')}
    aria-modal="true"
    class="connection-detail-modal"
    role="dialog"
  >
    <header class="connection-detail-modal__head">
      <h2>{$i18n.t('connection.details')}</h2>
      <div class="connection-detail-modal__actions">
        <button
          aria-label={$i18n.t('connection.reconnect')}
          class="modal-icon-button"
          disabled={reconnectingConnectionId === detailConnection.id}
          on:click={() => reconnectConnection(detailConnection.id)}
          type="button"
        >
          ↻
        </button>
        <button
          aria-label={$i18n.t('connection.edit')}
          class="modal-icon-button"
          on:click={() => {
            dispatch('editconnection', detailConnection.id)
            closeConnectionDetails()
          }}
          type="button"
        >
          ✎
        </button>
        <button class="modal-close" aria-label={$i18n.t('common.close')} on:click={closeConnectionDetails} type="button">
          ×
        </button>
      </div>
    </header>

    <div class="connection-detail-modal__body">
      <dl class="connection-detail-list">
        <dt>{$i18n.t('monitor.serviceName')}</dt>
        <dd>{detailConnection.name}</dd>
        <dt>{$i18n.t('monitor.endpoint')}</dt>
        <dd>{detailConnection.endpoint}</dd>
        <dt>{$i18n.t('monitor.protocol')}</dt>
        <dd>{$i18n.t('monitor.protocolGrpcHttp2')}</dd>
        <dt>{$i18n.t('response.status')}</dt>
        <dd>{translateStreamStateLabel($i18n.language, streamState)}</dd>
        <dt>{$i18n.t('catalog.source')}</dt>
        <dd>{formatCatalogSource(detailConnection.catalogSource)}</dd>
        <dt>{$i18n.t('catalog.loadedMethods')}</dt>
        <dd>{methodCount(detailConnection.services)}</dd>
      </dl>

      {#if metadataDefaults(detailConnection).length > 0}
        <div class="connection-detail-metadata">
          <h3>{$i18n.t('request.metadataTitle')}</h3>
          <dl>
            {#each metadataDefaults(detailConnection) as [key, value]}
              <dt>{key}</dt>
              <dd>{value}</dd>
            {/each}
          </dl>
        </div>
      {/if}
    </div>
  </div>
{/if}

{#if savedRequestRenameTarget}
  <button
    aria-label={$i18n.t('common.close')}
    class="overlay-backdrop overlay-backdrop--save-request overlay-backdrop--saved-request-rename"
    on:click={closeSavedRequestRenameDialog}
    type="button"
  ></button>
  <div aria-modal="true" class="save-request-modal save-request-modal--saved-request-rename" role="dialog" aria-labelledby="rename-saved-request-title">
    <div class="save-request-modal__head">
      <div>
        <span class="eyebrow">{$i18n.t('request.renameSavedRequest')}</span>
        <h2 id="rename-saved-request-title">{$i18n.t('request.savedRequestRenameTitle')}</h2>
      </div>
      <button aria-label={$i18n.t('common.close')} class="modal-close" on:click={closeSavedRequestRenameDialog} type="button">×</button>
    </div>
    <p class="save-request-modal__copy">{$i18n.t('request.savedRequestRenameCopy')}</p>
    <label class="field-label" for="rename-saved-request-name">{$i18n.t('request.saveRequestName')}</label>
    <input
      id="rename-saved-request-name"
      bind:this={savedRequestRenameInput}
      bind:value={savedRequestRenameName}
      class:error={Boolean(savedRequestRenameError)}
      class="text-input"
      on:keydown={(event) => {
        if (event.key === 'Enter') {
          event.preventDefault()
          void confirmSavedRequestRename()
        }
        if (event.key === 'Escape') {
          event.preventDefault()
          closeSavedRequestRenameDialog()
        }
      }}
      placeholder={$i18n.t('request.saveRequestNamePlaceholder')}
      type="text"
    />
    {#if savedRequestRenameError}
      <div class="inline-error" role="alert">{savedRequestRenameError}</div>
    {/if}
    <div class="save-request-modal__actions">
      <button class="secondary-command" disabled={savedRequestRenamePending} on:click={closeSavedRequestRenameDialog} type="button">
        {$i18n.t('common.cancel')}
      </button>
      <button class="primary-command" disabled={savedRequestRenamePending} on:click={() => void confirmSavedRequestRename()} type="button">
        {savedRequestRenamePending ? $i18n.t('request.saveRequestPending') : $i18n.t('common.save')}
      </button>
    </div>
  </div>
{/if}

{#if saveRequestDialogOpen}
  <button
    aria-label={$i18n.t('common.close')}
    class="overlay-backdrop overlay-backdrop--save-request"
    on:click={closeSaveRequestDialog}
    type="button"
  ></button>
  <div aria-modal="true" class="save-request-modal" role="dialog" aria-labelledby="save-request-title">
    <div class="save-request-modal__head">
      <div>
        <span class="eyebrow">{$i18n.t('request.saveRequest')}</span>
        <h2 id="save-request-title">{$i18n.t('request.saveRequestDialogTitle')}</h2>
      </div>
      <button aria-label={$i18n.t('common.close')} class="modal-close" on:click={closeSaveRequestDialog} type="button">×</button>
    </div>
    <p class="save-request-modal__copy">{$i18n.t('request.saveRequestDialogCopy')}</p>
    <label class="field-label" for="save-request-name">{$i18n.t('request.saveRequestName')}</label>
    <input
      id="save-request-name"
      bind:this={saveRequestNameInput}
      bind:value={saveRequestName}
      class:error={Boolean(saveRequestNameError)}
      class="text-input"
      on:keydown={(event) => {
        if (event.key === 'Enter') {
          event.preventDefault()
          void confirmRequestSave()
        }
        if (event.key === 'Escape') {
          event.preventDefault()
          closeSaveRequestDialog()
        }
      }}
      placeholder={$i18n.t('request.saveRequestNamePlaceholder')}
      type="text"
    />
    {#if saveRequestNameError}
      <div class="inline-error" role="alert">{saveRequestNameError}</div>
    {/if}
    <div class="save-request-modal__actions">
      <button class="secondary-command" disabled={requestSavePending} on:click={closeSaveRequestDialog} type="button">
        {$i18n.t('common.cancel')}
      </button>
      <button class="primary-command" disabled={requestSavePending} on:click={() => void confirmRequestSave()} type="button">
        {requestSavePending ? $i18n.t('request.saveRequestPending') : $i18n.t('common.save')}
      </button>
    </div>
  </div>
{/if}

{#if toast}
  <div class:app-toast--error={toast.tone === 'error'} class:app-toast--info={toast.tone === 'info'} class="app-toast" role="status">
    <span>{toast.message}</span>
    <button aria-label={$i18n.t('common.close')} on:click={() => (toast = null)} type="button">×</button>
  </div>
{/if}
