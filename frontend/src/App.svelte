<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { EventsOn, WindowIsMaximised, WindowMaximise, WindowToggleMaximise, WindowUnmaximise } from '../wailsjs/runtime/runtime'
  import MonitorView from './lib/components/MonitorView.svelte'
  import NewConnectionOverlay, { type NewConnectionDraft } from './lib/components/NewConnectionOverlay.svelte'
  import SettingsOverlay from './lib/components/SettingsOverlay.svelte'
  import type {
    EndpointPreset,
    StreamCompletedEvent,
    StreamErrorEvent,
    StreamEventRecord,
    StreamStateEvent,
    WorkspaceResult,
    WorkspaceSettings,
    WorkspaceSnapshot,
  } from './lib/contracts'
  import { i18n, translateProductLine } from './lib/i18n'
  import {
    buildEndpointPreset,
    createMonitorConnection,
    createWorkspaceFallbackConnection,
    endpointReadinessError,
    endpointStatusFromTest,
    monitorConnectionDraft,
    workspaceDraftFromConnections,
    workspaceEndpointDraft,
    type MonitorConnection as MonitorConnectionModel,
  } from './lib/monitor-flow'
  import { createAppShellStore } from './lib/state/app-shell'
  import {
    BackendResponseError,
    closeWorkspace,
    deleteWorkspace,
    createWorkspace,
    fetchBootstrap,
    getActiveWorkspace,
    loadCatalogFromProtoSources,
    loadCatalogFromReflection,
    openWorkspace,
    saveWorkspace,
    selectWorkspaceCreatePath,
    selectWorkspaceOpenPath,
    testEndpoint,
    validateWorkspace,
  } from './lib/wails/backend'
  import { isShortcutEvent } from './lib/shortcuts'
  import { loadRuntimeSession, saveRuntimeSession } from './lib/runtime-session'

  type ActiveModal = 'new-connection' | 'edit-connection' | 'settings' | null
  type SettingsModalTab = 'shortcuts' | 'theme' | 'workspace' | 'updates' | 'about'

  const shell = createAppShellStore()

  let activeModal: ActiveModal = null
  let selectedConnectionId: string | null = null
  let connections: MonitorConnectionModel[] = []
  let connectionPending = false
  let connectionError: string | null = null
  let editingConnectionId: string | null = null
  let reconnectingConnectionId: string | null = null
  let workspaceSnapshot: WorkspaceSnapshot | null = null
  let workspaceMenuOpen = false
  let workspacePending = false
  let workspaceError = ''
  let workspaceInfo = ''
  let workspaceToast: { message: string; tone: 'success' | 'error' } | null = null
  let workspaceToastTimer: ReturnType<typeof setTimeout> | undefined
  let settingsInitialTab: SettingsModalTab = 'theme'
  let leftSidebarVisible = true
  let rightPanelVisible = true
  let runtimeSessionReady = false

  onDestroy(() => {
    if (workspaceToastTimer) {
      clearTimeout(workspaceToastTimer)
    }
  })

  $: selectedConnection =
    connections.find((connection) => connection.id === selectedConnectionId) ?? connections[0]
  $: editingConnection =
    editingConnectionId ? connections.find((connection) => connection.id === editingConnectionId) ?? null : null
  $: editingConnectionDraft = editingConnection ? (monitorConnectionDraft(editingConnection) as NewConnectionDraft) : null

  $: if (typeof document !== 'undefined') {
    document.body.style.overflow = activeModal ? 'hidden' : ''
  }
  $: if (runtimeSessionReady) {
    saveRuntimeSession({
      workspaceManifestPath: isWorkspaceOpen(workspaceSnapshot) ? workspaceSnapshot.manifestPath : undefined,
      selectedConnectionId: selectedConnectionId ?? undefined,
      connections: isWorkspaceOpen(workspaceSnapshot) ? [] : connections.map(monitorConnectionDraft),
    })
  }

  function openModal(modal: ActiveModal): void {
    workspaceMenuOpen = false
    if (modal === 'new-connection') {
      connectionError = null
      editingConnectionId = null
    }

    activeModal = modal
  }

  function showWorkspaceToast(message: string, tone: 'success' | 'error'): void {
    if (workspaceToastTimer) {
      clearTimeout(workspaceToastTimer)
    }
    workspaceToast = { message, tone }
    workspaceToastTimer = setTimeout(() => {
      workspaceToast = null
      workspaceToastTimer = undefined
    }, 4200)
  }

  function openConnectionEditor(connectionId: string): void {
    connectionError = null
    editingConnectionId = connectionId
    activeModal = 'edit-connection'
  }

  function openSettings(tab: SettingsModalTab = 'theme'): void {
    workspaceMenuOpen = false
    settingsInitialTab = tab
    activeModal = 'settings'
  }

  function closeModal(): void {
    activeModal = null
    editingConnectionId = null
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

  function selectedConnectionIndex(): number {
    return connections.findIndex((connection) => connection.id === selectedConnection?.id)
  }

  function selectConnectionByIndex(index: number): void {
    const nextConnection = connections[index]
    if (nextConnection) {
      selectedConnectionId = nextConnection.id
    }
  }

  function selectAdjacentConnection(direction: 1 | -1): void {
    if (connections.length === 0) {
      return
    }

    const currentIndex = selectedConnectionIndex()
    const baseIndex = currentIndex >= 0 ? currentIndex : direction === 1 ? -1 : 0
    const nextIndex = (baseIndex + direction + connections.length) % connections.length
    selectConnectionByIndex(nextIndex)
  }

  function replaceConnections(nextConnections: MonitorConnectionModel[], nextSelectedId?: string | null): void {
    connections = nextConnections
    if (nextSelectedId !== undefined) {
      selectedConnectionId = nextSelectedId
    }
    void persistWorkspaceConnections(nextConnections)
  }

  function closeConnectionById(connectionId: string): void {
    const connectionIndex = connections.findIndex((connection) => connection.id === connectionId)
    if (connectionIndex < 0) {
      return
    }

    const nextConnections = connections.filter((currentConnection) => currentConnection.id !== connectionId)

    const nextSelectedId =
      selectedConnectionId === connectionId ? nextConnections[Math.min(connectionIndex, nextConnections.length - 1)]?.id ?? null : selectedConnectionId
    if (workspaceSnapshot && nextConnections.length === 0) {
      connections = nextConnections
      selectedConnectionId = null
      void persistWorkspaceConnections(nextConnections).finally(() => closeActiveWorkspace(false))
      return
    }

    replaceConnections(nextConnections, nextSelectedId)
  }

  function handleGlobalShortcut(event: KeyboardEvent): void {
    if (event.defaultPrevented) {
      return
    }

    if (isShortcutEvent(event, 'window.settings')) {
      event.preventDefault()
      activeModal === 'settings' ? closeModal() : openSettings('theme')
      return
    }

    if (isShortcutEvent(event, 'window.help')) {
      event.preventDefault()
      openSettings('shortcuts')
      return
    }

    if (activeModal) {
      if (event.key === 'Escape') {
        event.preventDefault()
        closeModal()
      }
      return
    }

    if (isShortcutEvent(event, 'workspace.open')) {
      event.preventDefault()
      void handleWorkspaceOpen()
      return
    }

    if (isShortcutEvent(event, 'workspace.save')) {
      event.preventDefault()
      void handleWorkspaceSave()
      return
    }

    if (isShortcutEvent(event, 'workspace.validate')) {
      event.preventDefault()
      void handleWorkspaceValidate()
      return
    }

    if (isShortcutEvent(event, 'workspace.close')) {
      event.preventDefault()
      void handleWorkspaceClose()
      return
    }

    if (isShortcutEvent(event, 'window.new')) {
      event.preventDefault()
      openModal('new-connection')
      return
    }

    if (isShortcutEvent(event, 'interface.left')) {
      event.preventDefault()
      leftSidebarVisible = !leftSidebarVisible
      return
    }

    if (isShortcutEvent(event, 'interface.right')) {
      event.preventDefault()
      rightPanelVisible = !rightPanelVisible
      return
    }

    if (isEditableShortcutTarget(event.target)) {
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

    if (selectedConnection && isShortcutEvent(event, 'sidebar.delete')) {
      event.preventDefault()
      closeConnectionById(selectedConnection.id)
    }
  }

  function handleWindowClick(event: MouseEvent): void {
    const target = event.target
    if (target instanceof HTMLElement && target.closest('.workspace-menu-host, .workspace-menu')) {
      return
    }

    workspaceMenuOpen = false
  }

  function workspaceDisplayName(): string {
    if (isWorkspaceOpen(workspaceSnapshot)) {
      return workspaceSnapshot.name.trim() || workspaceNameFromPath(workspaceSnapshot.manifestPath)
    }
    return $i18n.t('workspace.transient')
  }

  function isWorkspaceOpen(snapshot: WorkspaceSnapshot | null): snapshot is WorkspaceSnapshot {
    return Boolean(snapshot?.id && snapshot.manifestPath)
  }

  function applyWorkspaceSnapshot(snapshot: WorkspaceSnapshot): void {
    workspaceSnapshot = isWorkspaceOpen(snapshot) ? snapshot : null
  }

  function workspaceNameFromPath(path: string): string {
    const parts = path.split(/[\\/]/).filter(Boolean)
    const filename = parts.at(-1) ?? 'workspace'
    const parent = parts.length > 1 ? parts.at(-2) : undefined

    if (/^workspace\.ya?ml$/i.test(filename) && parent) {
      return parent
    }

    return filename.replace(/\.ya?ml$/i, '') || 'workspace'
  }

  function workspaceDraft(name = workspaceSnapshot?.name) {
    return workspaceDraftFromConnections({
      name,
      connections,
      settings: workspaceSnapshot?.settings,
    })
  }

  function setWorkspaceStatusFromResult(result: WorkspaceResult, message: string): void {
    workspaceSnapshot = result.workspace
    workspaceError = ''
    workspaceInfo = result.issues?.length
      ? $i18n.t('workspace.savedWithIssues', { count: result.issues.length })
      : message
  }

  function workspaceErrorMessage(error: unknown, fallback: string): string {
    if (error instanceof BackendResponseError) {
      return error.code ? `${error.code}: ${error.backendMessage}` : error.backendMessage
    }

    return error instanceof Error ? error.message : fallback
  }

  async function persistWorkspaceConnections(nextConnections: MonitorConnectionModel[]): Promise<void> {
    if (!workspaceSnapshot) {
      return
    }

    try {
      const result = await saveWorkspace(
        workspaceDraftFromConnections({
          name: workspaceSnapshot.name,
          connections: nextConnections,
          settings: workspaceSnapshot.settings,
        }),
      )
      workspaceSnapshot = result.workspace
      workspaceError = ''
      workspaceInfo = result.issues?.length ? $i18n.t('workspace.savedWithIssues', { count: result.issues.length }) : ''
    } catch (error) {
      workspaceError = workspaceErrorMessage(error, $i18n.t('workspace.saveFailed'))
      workspaceInfo = ''
    }
  }

  async function loadWorkspaceConnection(
    workspace: WorkspaceSnapshot,
    endpoint: EndpointPreset,
  ): Promise<MonitorConnectionModel> {
    const workspaceSources = {
      protoSources: workspace.protoSources ?? [],
      importPaths: workspace.importPaths ?? [],
    }
    const draft = workspaceEndpointDraft(endpoint, workspaceSources)

    try {
      const endpointCheck = await testEndpoint({ endpoint })
      const readinessError = endpointReadinessError(endpointCheck)
      if (readinessError) {
        return createWorkspaceFallbackConnection({
          endpoint: endpointCheck.endpoint,
          workspace: workspaceSources,
          status: endpointStatusFromTest(endpointCheck),
          endpointCheck,
        })
      }

      const catalog =
        draft.catalogSource === 'proto'
          ? await loadCatalogFromProtoSources({
              endpoint: endpointCheck.endpoint,
              protoSources: draft.protoSources,
              importPaths: draft.importPaths,
            })
          : await loadCatalogFromReflection({ endpoint: endpointCheck.endpoint })

      return createMonitorConnection(draft, endpointCheck, catalog)
    } catch {
      return createWorkspaceFallbackConnection({
        endpoint,
        workspace: workspaceSources,
        status: 'offline',
      })
    }
  }

  async function hydrateWorkspaceConnections(workspace: WorkspaceSnapshot): Promise<void> {
    const nextConnections: MonitorConnectionModel[] = []
    let degradedCount = 0

    for (const endpoint of workspace.endpoints) {
      const connection = await loadWorkspaceConnection(workspace, endpoint)
      if (connection.status !== 'online' || connection.services.length === 0) {
        degradedCount += 1
      }
      nextConnections.push(connection)
    }

    connections = nextConnections
    selectedConnectionId = nextConnections[0]?.id ?? null

    if (degradedCount > 0) {
      workspaceInfo = $i18n.t('workspace.openedWithDegradedConnections', { count: degradedCount })
    }
  }

  async function handleSyncedWorkspace(workspace: WorkspaceSnapshot): Promise<void> {
    workspaceSnapshot = workspace
    workspaceError = ''
    workspaceInfo = ''
    await hydrateWorkspaceConnections(workspace)
  }

  async function handleWorkspaceCreate(): Promise<void> {
    workspaceMenuOpen = false
    workspaceError = ''
    workspaceInfo = ''

    const path = await selectWorkspaceCreatePath()
    if (!path) {
      return
    }

    workspacePending = true
    try {
      const result = await createWorkspace({
        path,
        ...workspaceDraft(workspaceSnapshot?.name ?? workspaceNameFromPath(path)),
      })
      setWorkspaceStatusFromResult(result, $i18n.t('workspace.created'))
    } catch (error) {
      workspaceError = workspaceErrorMessage(error, $i18n.t('workspace.createFailed'))
    } finally {
      workspacePending = false
    }
  }

  async function handleWorkspaceOpen(): Promise<void> {
    workspaceMenuOpen = false
    workspaceError = ''
    workspaceInfo = ''

    const path = await selectWorkspaceOpenPath()
    if (!path) {
      return
    }

    workspacePending = true
    try {
      const result = await openWorkspace(path)
      workspaceSnapshot = result.workspace
      workspaceError = ''
      workspaceInfo = result.issues?.length
        ? $i18n.t('workspace.savedWithIssues', { count: result.issues.length })
        : ''
      await hydrateWorkspaceConnections(result.workspace)
    } catch (error) {
      workspaceError = workspaceErrorMessage(error, $i18n.t('workspace.openFailed'))
    } finally {
      workspacePending = false
    }
  }

  async function handleWorkspaceSave(): Promise<void> {
    workspaceMenuOpen = false
    workspaceError = ''
    workspaceInfo = ''

    if (!workspaceSnapshot) {
      workspaceError = $i18n.t('workspace.openRequired')
      return
    }

    workspacePending = true
    try {
      const result = await saveWorkspace(workspaceDraft())
      setWorkspaceStatusFromResult(result, $i18n.t('workspace.saved'))
    } catch (error) {
      workspaceError = workspaceErrorMessage(error, $i18n.t('workspace.saveFailed'))
    } finally {
      workspacePending = false
    }
  }

  async function handleWorkspaceClose(): Promise<void> {
    workspaceMenuOpen = false
    workspaceError = ''
    workspaceInfo = ''

    if (!workspaceSnapshot) {
      return
    }

    workspacePending = true
    try {
      await closeActiveWorkspace(true)
    } catch (error) {
      workspaceError = workspaceErrorMessage(error, $i18n.t('workspace.closeFailed'))
    } finally {
      workspacePending = false
    }
  }

  async function handleWorkspaceDelete(): Promise<void> {
    workspaceMenuOpen = false
    workspaceError = ''
    workspaceInfo = ''

    if (!workspaceSnapshot) {
      return
    }
    if (!globalThis.confirm($i18n.t('workspace.deleteConfirm', { name: workspaceSnapshot.name }))) {
      return
    }

    workspacePending = true
    try {
      await deleteWorkspace()
      workspaceSnapshot = null
      connections = []
      selectedConnectionId = null
      workspaceInfo = $i18n.t('workspace.deleted')
      showWorkspaceToast(workspaceInfo, 'success')
    } catch (error) {
      workspaceError = workspaceErrorMessage(error, $i18n.t('workspace.deleteFailed'))
      showWorkspaceToast(workspaceError, 'error')
    } finally {
      workspacePending = false
    }
  }

  async function closeActiveWorkspace(clearConnections: boolean): Promise<void> {
    await closeWorkspace()
    workspaceSnapshot = null
    workspaceError = ''
    workspaceInfo = ''
    if (clearConnections) {
      connections = []
      selectedConnectionId = null
    }
  }

  async function handleWorkspaceValidate(): Promise<void> {
    workspaceMenuOpen = false
    workspaceError = ''
    workspaceInfo = ''

    if (!workspaceSnapshot) {
      workspaceError = $i18n.t('workspace.openRequired')
      return
    }

    workspacePending = true
    try {
      const result = await validateWorkspace(workspaceDraft())
      if (result.workspace) {
        workspaceSnapshot = result.workspace
      }
      workspaceInfo =
        result.issues.length > 0
          ? $i18n.t('workspace.validationIssues', { count: result.issues.length })
          : $i18n.t('workspace.validationOk')
      showWorkspaceToast(workspaceInfo, result.issues.length > 0 ? 'error' : 'success')
    } catch (error) {
      workspaceError = workspaceErrorMessage(error, $i18n.t('workspace.validateFailed'))
      showWorkspaceToast(workspaceError, 'error')
    } finally {
      workspacePending = false
    }
  }

  async function handleWorkspaceSettingsSave(event: CustomEvent<WorkspaceSettings>): Promise<void> {
    workspaceError = ''
    workspaceInfo = ''

    if (!workspaceSnapshot) {
      workspaceError = $i18n.t('workspace.openRequired')
      return
    }

    workspacePending = true
    try {
      const result = await saveWorkspace(
        workspaceDraftFromConnections({
          name: workspaceSnapshot.name,
          connections,
          settings: event.detail,
        }),
      )
      setWorkspaceStatusFromResult(result, $i18n.t('workspace.settingsSaved'))
    } catch (error) {
      workspaceError = workspaceErrorMessage(error, $i18n.t('workspace.settingsSaveFailed'))
    } finally {
      workspacePending = false
    }
  }

  async function handleShellDoubleClick(event: MouseEvent): Promise<void> {
    const target = event.target as HTMLElement | null
    if (
      target?.closest(
        'button, a, input, textarea, select, [role="menu"], .overlay-panel, .workspace-menu, .json-editor, .code-block',
      )
    ) {
      return
    }

    try {
      if (await WindowIsMaximised()) {
        WindowUnmaximise()
      } else {
        WindowMaximise()
      }
    } catch {
      WindowToggleMaximise()
    }
  }

  async function restoreRuntimeSession(bootstrapWorkspace?: WorkspaceSnapshot): Promise<void> {
    const session = loadRuntimeSession()

    const activeWorkspace = await waitForActiveWorkspace(bootstrapWorkspace)
    if (activeWorkspace) {
      workspaceSnapshot = activeWorkspace
      await hydrateWorkspaceConnections(activeWorkspace)
      if (session.selectedConnectionId && connections.some((connection) => connection.id === session.selectedConnectionId)) {
        selectedConnectionId = session.selectedConnectionId
      }
      return
    }

    if (session.workspaceManifestPath) {
      try {
        const result = await openWorkspace(session.workspaceManifestPath)
        workspaceSnapshot = result.workspace
        await hydrateWorkspaceConnections(result.workspace)
        if (session.selectedConnectionId && connections.some((connection) => connection.id === session.selectedConnectionId)) {
          selectedConnectionId = session.selectedConnectionId
        }
        return
      } catch {
        // A moved or deleted workspace must not prevent restoring standalone connections.
      }
    }

    const restored: MonitorConnectionModel[] = []
    for (const savedDraft of session.connections) {
      try {
        restored.push(await loadConnectionFromDraft(savedDraft as NewConnectionDraft))
      } catch {
        const endpoint = buildEndpointPreset(savedDraft)
        restored.push(
          createWorkspaceFallbackConnection({
            endpoint,
            workspace: { protoSources: savedDraft.protoSources, importPaths: savedDraft.importPaths ?? [] },
            status: 'offline',
          }),
        )
      }
    }
    connections = restored
    selectedConnectionId =
      (session.selectedConnectionId && restored.some((connection) => connection.id === session.selectedConnectionId)
        ? session.selectedConnectionId
        : restored[0]?.id) ?? null
  }

  async function loadConnectionFromDraft(draft: NewConnectionDraft): Promise<MonitorConnectionModel> {
      const endpoint = buildEndpointPreset(draft)
      const endpointCheck = await testEndpoint({ endpoint })
      const readinessError = endpointReadinessError(endpointCheck)
      if (readinessError) {
        throw new Error(readinessError)
      }

      const catalog =
        draft.catalogSource === 'proto'
          ? await loadCatalogFromProtoSources({
              endpoint: endpointCheck.endpoint,
              protoSources: draft.protoSources,
              importPaths: draft.importPaths,
            })
          : await loadCatalogFromReflection({ endpoint: endpointCheck.endpoint })

      return createMonitorConnection(draft, endpointCheck, catalog)
  }

  async function handleConnectionCreated(event: CustomEvent<NewConnectionDraft>): Promise<void> {
    const draft = event.detail
    connectionPending = true
    connectionError = null

    try {
      const nextConnection = await loadConnectionFromDraft(draft)
      const nextConnections = [nextConnection, ...connections.filter((connection) => connection.id !== nextConnection.id)]
      connections = nextConnections
      selectedConnectionId = nextConnection.id
      closeModal()
      void persistWorkspaceConnections(nextConnections)
    } catch (error) {
      connectionError =
        error instanceof BackendResponseError
          ? error.backendMessage
          : error instanceof Error
            ? error.message
            : $i18n.t('connection.createFailed')
    } finally {
      connectionPending = false
    }
  }

  /**
   * The macOS application bridge becomes available slightly after the first
   * frontend render. The backend can already have restored a workspace at
   * that point, so retry the authoritative runtime state before falling back
   * to browser-only session data.
  */
  async function waitForActiveWorkspace(bootstrapWorkspace?: WorkspaceSnapshot): Promise<WorkspaceSnapshot | null> {
    const usableBootstrap = bootstrapWorkspace ?? null
    if (isWorkspaceOpen(usableBootstrap)) {
      return usableBootstrap
    }

    for (const delay of [0, 80, 220, 500]) {
      if (delay > 0) {
        await new Promise<void>((resolve) => window.setTimeout(resolve, delay))
      }

      try {
        const activeWorkspace = await getActiveWorkspace()
        if (isWorkspaceOpen(activeWorkspace)) {
          return activeWorkspace
        }
      } catch {
        // The bridge may not have finished binding yet; the next retry is safe.
      }
    }

    return null
  }

  async function handleConnectionUpdated(event: CustomEvent<NewConnectionDraft>): Promise<void> {
    const existingConnection = editingConnection
    if (!existingConnection) {
      connectionError = $i18n.t('connection.editMissing')
      return
    }

    const draft: NewConnectionDraft = {
      ...event.detail,
      id: existingConnection.id,
    }
    connectionPending = true
    connectionError = null

    try {
      const loadedConnection = await loadConnectionFromDraft(draft)
      const selectedMethodFullName =
        findConnectionMethod(loadedConnection, existingConnection.selectedMethodFullName)
          ? existingConnection.selectedMethodFullName
          : loadedConnection.selectedMethodFullName
      const nextConnection: MonitorConnectionModel = {
        ...loadedConnection,
        id: existingConnection.id,
        selectedMethodFullName,
        endpointCheck: loadedConnection.endpointCheck
          ? {
              ...loadedConnection.endpointCheck,
              endpoint: {
                ...loadedConnection.endpointCheck.endpoint,
                id: existingConnection.id,
              },
            }
          : loadedConnection.endpointCheck,
      }
      const nextConnections = connections.map((connection) =>
        connection.id === existingConnection.id ? nextConnection : connection,
      )
      connections = nextConnections
      selectedConnectionId = nextConnection.id
      closeModal()
      void persistWorkspaceConnections(nextConnections)
    } catch (error) {
      connectionError =
        error instanceof BackendResponseError
          ? error.backendMessage
          : error instanceof Error
            ? error.message
            : $i18n.t('connection.updateFailed')
    } finally {
      connectionPending = false
    }
  }

  function findConnectionMethod(connection: MonitorConnectionModel, methodFullName: string | undefined): boolean {
    if (!methodFullName) {
      return false
    }

    return connection.services.some((service) => service.methods.some((method) => method.fullName === methodFullName))
  }

  function handleMethodSelected(event: CustomEvent<{ connectionId: string; methodFullName: string }>): void {
    connections = connections.map((connection) =>
      connection.id === event.detail.connectionId
        ? { ...connection, selectedMethodFullName: event.detail.methodFullName }
        : connection,
    )
  }

  function handleConnectionDeleted(event: CustomEvent<string>): void {
    closeConnectionById(event.detail)
  }

  function handleConnectionRenamed(event: CustomEvent<{ connectionId: string; name: string }>): void {
    const nextConnections = connections.map((connection) =>
      connection.id === event.detail.connectionId ? { ...connection, name: event.detail.name } : connection,
    )
    connections = nextConnections
    void persistWorkspaceConnections(nextConnections)
  }

  async function handleConnectionReconnect(event: CustomEvent<string>): Promise<void> {
    const connection = connections.find((connection) => connection.id === event.detail)
    if (!connection) {
      return
    }

    reconnectingConnectionId = connection.id

    try {
      const endpoint =
        connection.endpointCheck?.endpoint ??
        buildEndpointPreset({
          name: connection.name,
          endpoint: connection.endpoint,
          tlsEnabled: false,
          catalogSource: connection.catalogSource,
          protoSources: [],
        })
      const endpointCheck = await testEndpoint({ endpoint })
      const updatedAt = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

      connections = connections.map((currentConnection) =>
        currentConnection.id === connection.id
          ? {
              ...currentConnection,
              status: endpointStatusFromTest(endpointCheck),
              updatedAt,
              endpointCheck,
            }
          : currentConnection,
      )
    } catch {
      connections = connections.map((currentConnection) =>
        currentConnection.id === connection.id
          ? {
              ...currentConnection,
              status: 'offline',
              updatedAt: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
            }
          : currentConnection,
      )
    } finally {
      reconnectingConnectionId = null
    }
  }

  onMount(() => {
    const hasWailsRuntime = Boolean((window as unknown as { runtime?: unknown }).runtime)
    const offStreamState = hasWailsRuntime
      ? EventsOn('stream:state', (payload: StreamStateEvent) => shell.applyStreamStateEvent(payload))
      : () => undefined
    const offStreamEvent = hasWailsRuntime
      ? EventsOn('stream:event', (payload: StreamEventRecord) => shell.applyStreamEvent(payload))
      : () => undefined
    const offStreamError = hasWailsRuntime
      ? EventsOn('stream:error', (payload: StreamErrorEvent) => shell.applyStreamError(payload))
      : () => undefined
    const offStreamCompleted = hasWailsRuntime
      ? EventsOn('stream:completed', (payload: StreamCompletedEvent) => shell.applyStreamCompleted(payload))
      : () => undefined

    void (async () => {
      let bootstrapWorkspace: WorkspaceSnapshot | undefined
      workspacePending = true
      try {
        try {
          const bootstrap = await fetchBootstrap()
          shell.hydrateBootstrap(bootstrap)
          bootstrapWorkspace = bootstrap.workspace
        } catch (error) {
          const message = error instanceof Error ? error.message : $i18n.t('errors.bootstrapUnexpected')
          shell.setBootstrapError(message)
        }
        try {
          await restoreRuntimeSession(bootstrapWorkspace)
        } catch (error) {
          const message = error instanceof Error ? error.message : $i18n.t('errors.bootstrapUnexpected')
          shell.setBootstrapError(message)
        }
      } finally {
        workspacePending = false
        runtimeSessionReady = true
      }
    })()

    return () => {
      document.body.style.overflow = ''
      offStreamState()
      offStreamEvent()
      offStreamError()
      offStreamCompleted()
    }
  })
</script>

<svelte:window on:click={handleWindowClick} on:keydown={handleGlobalShortcut} />

<svelte:head>
  <title>{$i18n.t('app.title')}</title>
</svelte:head>

<div
  class:monitor-shell--sidebar-hidden={!leftSidebarVisible}
  class="monitor-shell"
  on:dblclick={handleShellDoubleClick}
  aria-label={$i18n.t('app.title')}
  role="region"
>
  {@html `<!--
    THESIS: Catenar makes fast gRPC work legible at a glance; it is a focused desktop instrument, not a landing page or generic dashboard.
    OWN-WORLD: smoked graphite glass in dark mode and pearl-satin glass in light mode; restrained ion-lilac signals establish selection and speed, with muted coral reserved for failure states. Blue and green chrome are intentionally absent.
    STORY: connect a service, load its contract, compose a call and read its outcome without losing operational context.
    FIRST VIEWPORT: a left connection rail anchors the session; the client workbench or trace starts immediately beside it, with first-run guidance occupying the otherwise empty work surface.
    FORM: Graphite Flux Glass, candidate 6, seed 02e5cd40.
    FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md
  -->`}
  <header class="monitor-header">
    <div class="monitor-brand">
      <img class="monitor-brand__mark" src="/app-icon.png" alt="" aria-hidden="true" />
      <div>
        <h1>{$i18n.t('monitor.title')}</h1>
        <p>{translateProductLine($i18n.language, $shell.bootstrap?.app.productLine)}</p>
      </div>
    </div>

  </header>

  <MonitorView
    connections={connections}
    selectedConnection={selectedConnection}
    streamState={$shell.activeStreamState}
    activeStreamSessionId={$shell.activeStreamSessionId}
    activeStreamCallId={$shell.activeStreamCallId}
    streamEvents={$shell.streamEvents}
    streamErrors={$shell.streamErrors}
    lastStreamCompleted={$shell.lastStreamCompleted}
    savedRequests={isWorkspaceOpen(workspaceSnapshot) ? workspaceSnapshot.savedRequests ?? [] : []}
    workspaceAvailable={isWorkspaceOpen(workspaceSnapshot)}
    reconnectingConnectionId={reconnectingConnectionId}
    shortcutsEnabled={activeModal === null}
    leftSidebarVisible={leftSidebarVisible}
    rightPanelVisible={rightPanelVisible}
    on:deleteconnection={handleConnectionDeleted}
    on:editconnection={(event) => openConnectionEditor(event.detail)}
    on:newconnection={() => openModal('new-connection')}
    on:renameconnection={handleConnectionRenamed}
    on:reconnectconnection={handleConnectionReconnect}
    on:selectconnection={(event) => (selectedConnectionId = event.detail)}
    on:selectmethod={handleMethodSelected}
    on:workspacesnapshot={(event) => applyWorkspaceSnapshot(event.detail)}
  >
    <aside class="sidebar-footer" aria-label={$i18n.t('monitor.actions')} slot="sidebar-footer">
    <div class="monitor-actions">
      <div class="workspace-menu-host">
        <button
          aria-expanded={workspaceMenuOpen}
          aria-label={$i18n.t('workspace.menu')}
          class:workspace-chip--active={isWorkspaceOpen(workspaceSnapshot)}
          class="workspace-chip"
          disabled={workspacePending}
          on:click={() => (workspaceMenuOpen = !workspaceMenuOpen)}
          type="button"
        >
          <span aria-hidden="true">▣</span>
          <span>{workspacePending ? $i18n.t('workspace.pending') : workspaceDisplayName()}</span>
          <span aria-hidden="true">⌃</span>
        </button>
        {#if workspaceMenuOpen}
          <div class="workspace-menu" role="menu">
            <button on:click={handleWorkspaceCreate} role="menuitem" type="button">
              {$i18n.t('workspace.create')}
            </button>
            <button on:click={handleWorkspaceOpen} role="menuitem" type="button">
              {$i18n.t('workspace.open')}
            </button>
            {#if isWorkspaceOpen(workspaceSnapshot)}
            <button disabled={!workspaceSnapshot} on:click={handleWorkspaceSave} role="menuitem" type="button">
              {$i18n.t('workspace.save')}
            </button>
            <button disabled={!workspaceSnapshot} on:click={handleWorkspaceValidate} role="menuitem" title={$i18n.t('workspace.validateHelp')} type="button">
              {$i18n.t('workspace.validate')}
            </button>
            <button disabled={!workspaceSnapshot} on:click={handleWorkspaceClose} role="menuitem" type="button">
              {$i18n.t('workspace.close')}
            </button>
            <button class="workspace-menu__delete" disabled={!workspaceSnapshot} on:click={handleWorkspaceDelete} role="menuitem" type="button">
              {$i18n.t('workspace.delete')}
            </button>
            {/if}
            {#if workspaceError}
              <div class:workspace-menu__status--error={Boolean(workspaceError)} class="workspace-menu__status">
                {workspaceError}
              </div>
            {/if}
          </div>
        {/if}
      </div>
      <button class="primary-command" on:click={() => openModal('new-connection')} type="button">
        <span aria-hidden="true">+</span>
        {$i18n.t('connection.new')}
      </button>
      <button class="icon-command" aria-label={$i18n.t('settings.title')} on:click={() => openSettings('theme')} type="button">
        <span aria-hidden="true">⚙</span>
      </button>
    </div>
    </aside>
  </MonitorView>

  {#if workspaceToast}
    <div class:app-toast--error={workspaceToast.tone === 'error'} class="app-toast" role="status">
      <span>{workspaceToast.message}</span>
      <button aria-label={$i18n.t('common.close')} on:click={() => (workspaceToast = null)} type="button">×</button>
    </div>
  {/if}

  <NewConnectionOverlay
    isOpen={activeModal === 'new-connection' || activeModal === 'edit-connection'}
    mode={activeModal === 'edit-connection' ? 'edit' : 'create'}
    initialDraft={activeModal === 'edit-connection' ? editingConnectionDraft : null}
    pending={connectionPending}
    error={connectionError}
    on:close={closeModal}
    on:create={handleConnectionCreated}
    on:update={handleConnectionUpdated}
  />

  <SettingsOverlay
    isOpen={activeModal === 'settings'}
    initialTab={settingsInitialTab}
    appVersion={$shell.bootstrap?.app.version ?? '0.0.0'}
    appMetadata={$shell.bootstrap?.app}
    workspaceSettings={isWorkspaceOpen(workspaceSnapshot) ? workspaceSnapshot.settings : undefined}
    workspaceSettingsAvailable={isWorkspaceOpen(workspaceSnapshot)}
    workspaceSettingsPending={workspacePending}
    workspaceSettingsError={workspaceError}
    workspaceSettingsInfo={workspaceInfo}
    workspaceId={isWorkspaceOpen(workspaceSnapshot) ? workspaceSnapshot.id : ''}
    workspaceName={isWorkspaceOpen(workspaceSnapshot) ? workspaceSnapshot.name : ''}
    on:close={closeModal}
    on:workspaceSnapshot={(event) => void handleSyncedWorkspace(event.detail)}
    on:workspaceSettingsSave={handleWorkspaceSettingsSave}
  />
</div>
