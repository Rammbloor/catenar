<script lang="ts" context="module">
  import type { ProtoSource, TLSMode } from '../contracts'

  export interface NewConnectionDraft {
    id?: string
    name: string
    endpoint: string
    tlsMode: TLSMode
    authority?: string
    serverNameOverride: string
    caCertRef: string
    clientCertRef: string
    clientKeyRef: string
    connectTimeoutMs?: number
    requestTimeoutMs?: number
    streamIdleTimeoutMs?: number
    metadataDefaults?: Record<string, string>
    catalogSource: 'reflection' | 'proto'
    protoSources: ProtoSource[]
    importPaths: string[]
  }
</script>

<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { MaterialKind } from '../contracts'
  import { i18n } from '../i18n'
  import { protoSourceLabel } from '../monitor-flow'
  import { registerMaterialFile, selectMaterialFile, selectProtoDirectory, selectProtoFiles } from '../wails/backend'

  export let isOpen: boolean
  export let pending = false
  export let error: string | null = null
  export let mode: 'create' | 'edit' = 'create'
  export let initialDraft: NewConnectionDraft | null = null

  const dispatch = createEventDispatcher<{
    close: void
    create: NewConnectionDraft
    update: NewConnectionDraft
  }>()

  let connectionId: string | undefined
  let name = ''
  let endpoint = 'localhost:50051'
  let authority = ''
  let tlsEnabled = false
  let tlsMode: TLSMode = 'system_ca'
  let serverNameOverride = ''
  let caCertRef = ''
  let clientCertRef = ''
  let clientKeyRef = ''
  let caCertLabel = ''
  let clientCertLabel = ''
  let clientKeyLabel = ''
  let catalogSource: 'reflection' | 'proto' = 'reflection'
  let protoSources: ProtoSource[] = []
  let importPaths: string[] = []
  let connectTimeoutMs = '5000'
  let requestTimeoutMs = '30000'
  let streamIdleTimeoutMs = '120000'
  let metadataDefaultsText = ''
  let pickerError: string | null = null
  let draftToken = ''

  $: effectiveTLSMode = tlsEnabled ? tlsMode : 'plaintext'
  $: needsCustomCA = effectiveTLSMode === 'custom_ca' || effectiveTLSMode === 'mtls'
  $: needsClientMaterial = effectiveTLSMode === 'mtls'
  $: tlsReady = (!needsCustomCA || caCertRef.length > 0) && (!needsClientMaterial || (clientCertRef.length > 0 && clientKeyRef.length > 0))
  $: metadataDefaults = parseMetadataDefaults(metadataDefaultsText)
  $: metadataDefaultsError = metadataDefaults.ok ? '' : metadataDefaults.message
  $: canCreate =
    endpoint.trim().length > 0 &&
    tlsReady &&
    metadataDefaults.ok &&
    (catalogSource === 'reflection' || protoSources.length > 0)
  $: visibleError = error ?? pickerError
  $: currentDraftToken = `${isOpen}:${mode}:${JSON.stringify(initialDraft ?? null)}`
  $: if (currentDraftToken !== draftToken) {
    hydrateDraft()
    draftToken = currentDraftToken
  }

  function hydrateDraft(): void {
    if (!isOpen) {
      pickerError = null
      return
    }

    const draft = initialDraft
    connectionId = draft?.id
    name = draft?.name ?? ''
    endpoint = draft?.endpoint ?? 'localhost:50051'
    authority = draft?.authority ?? ''
    tlsEnabled = (draft?.tlsMode ?? 'plaintext') !== 'plaintext'
    tlsMode = draft?.tlsMode && draft.tlsMode !== 'plaintext' ? draft.tlsMode : 'system_ca'
    serverNameOverride = draft?.serverNameOverride ?? ''
    caCertRef = draft?.caCertRef ?? ''
    clientCertRef = draft?.clientCertRef ?? ''
    clientKeyRef = draft?.clientKeyRef ?? ''
    caCertLabel = draft?.caCertRef ?? ''
    clientCertLabel = draft?.clientCertRef ?? ''
    clientKeyLabel = draft?.clientKeyRef ?? ''
    catalogSource = draft?.catalogSource ?? 'reflection'
    protoSources = draft?.protoSources ? [...draft.protoSources] : []
    importPaths = draft?.importPaths ? [...draft.importPaths] : []
    connectTimeoutMs = String(draft?.connectTimeoutMs ?? 5000)
    requestTimeoutMs = String(draft?.requestTimeoutMs ?? 30000)
    streamIdleTimeoutMs = String(draft?.streamIdleTimeoutMs ?? 120000)
    metadataDefaultsText = formatMetadataDefaults(draft?.metadataDefaults)
    pickerError = null
  }

  async function chooseProtoFiles(): Promise<void> {
    pickerError = null

    try {
      const paths = await selectProtoFiles()
      if (paths.length === 0) {
        return
      }

      const nextSources = paths.map((path) => ({ type: 'file' as const, path }))
      protoSources = [...protoSources.filter((source) => source.type !== 'file'), ...nextSources]
      catalogSource = 'proto'
    } catch (error) {
      pickerError = error instanceof Error ? error.message : $i18n.t('connection.protoPickerFailed')
    }
  }

  async function chooseProtoDirectory(): Promise<void> {
    pickerError = null

    try {
      const path = await selectProtoDirectory()
      if (!path) {
        return
      }

      protoSources = [...protoSources.filter((source) => source.path !== path), { type: 'directory', path }]
      importPaths = [...new Set([...importPaths, path])]
      catalogSource = 'proto'
    } catch (error) {
      pickerError = error instanceof Error ? error.message : $i18n.t('connection.protoPickerFailed')
    }
  }

  function removeProtoSource(path: string): void {
    protoSources = protoSources.filter((source) => source.path !== path)
    importPaths = importPaths.filter((importPath) => importPath !== path)
  }

  function materialNamespace(): string {
    const source = `${name.trim() || 'endpoint'}-${endpoint.trim() || 'target'}`
    const slug = source.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 38)
    let hash = 0
    for (const char of source) {
      hash = (hash * 31 + char.charCodeAt(0)) >>> 0
    }

    return `${slug || 'endpoint'}-${hash.toString(16)}`
  }

  function fileLabel(path: string, ref: string): string {
    return path.split(/[\\/]/).filter(Boolean).at(-1) ?? ref
  }

  async function chooseMaterialFile(kind: MaterialKind): Promise<void> {
    pickerError = null

    try {
      const path = await selectMaterialFile()
      if (!path) {
        return
      }

      const keyByKind: Record<MaterialKind, string> = {
        tls_ca: 'ca',
        tls_client_cert: 'client-cert',
        tls_client_key: 'client-key',
        metadata: 'metadata',
      }
      const result = await registerMaterialFile({
        namespace: materialNamespace(),
        key: keyByKind[kind],
        path,
        kind,
      })
      const label = fileLabel(path, result.ref)

      if (kind === 'tls_ca') {
        caCertRef = result.ref
        caCertLabel = label
      } else if (kind === 'tls_client_cert') {
        clientCertRef = result.ref
        clientCertLabel = label
      } else if (kind === 'tls_client_key') {
        clientKeyRef = result.ref
        clientKeyLabel = label
      }
    } catch (error) {
      pickerError = error instanceof Error ? error.message : $i18n.t('connection.materialPickerFailed')
    }
  }

  function positiveInteger(value: string): number | undefined {
    const parsed = Number(value)
    return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : undefined
  }

  function formatMetadataDefaults(values: Record<string, string> | undefined): string {
    return Object.entries(values ?? {})
      .map(([key, value]) => `${key}: ${value}`)
      .join('\n')
  }

  function parseMetadataDefaults(value: string): { ok: true; value: Record<string, string> } | { ok: false; message: string } {
    const result: Record<string, string> = {}
    const lines = value.split('\n')

    for (const line of lines) {
      const trimmed = line.trim()
      if (!trimmed) {
        continue
      }

      const separatorIndex = trimmed.includes(':') ? trimmed.indexOf(':') : trimmed.indexOf('=')
      if (separatorIndex <= 0) {
        return { ok: false, message: $i18n.t('connection.metadataDefaultsInvalid') }
      }

      const key = trimmed.slice(0, separatorIndex).trim()
      const metadataValue = trimmed.slice(separatorIndex + 1).trim()
      if (!key || !metadataValue) {
        return { ok: false, message: $i18n.t('connection.metadataDefaultsInvalid') }
      }
      result[key] = metadataValue
    }

    return { ok: true, value: result }
  }

  function submitConnection(): void {
    if (!canCreate || pending) {
      return
    }

    const payload = {
      id: connectionId,
      name: name.trim(),
      endpoint: endpoint.trim(),
      tlsMode: effectiveTLSMode as TLSMode,
      authority: authority.trim(),
      serverNameOverride: serverNameOverride.trim(),
      caCertRef,
      clientCertRef,
      clientKeyRef,
      connectTimeoutMs: positiveInteger(connectTimeoutMs),
      requestTimeoutMs: positiveInteger(requestTimeoutMs),
      streamIdleTimeoutMs: positiveInteger(streamIdleTimeoutMs),
      metadataDefaults: metadataDefaults.ok ? metadataDefaults.value : undefined,
      catalogSource,
      protoSources,
      importPaths,
    }

    dispatch(mode === 'edit' ? 'update' : 'create', payload)
  }
</script>

{#if isOpen}
  <button aria-label={$i18n.t('common.close')} class="fullscreen-backdrop" on:click={() => dispatch('close')} type="button"></button>
  <div
    aria-label={$i18n.t(mode === 'edit' ? 'connection.editTitle' : 'connection.new')}
    class="fullscreen-modal fullscreen-modal--connection"
    role="dialog"
    aria-modal="true"
  >
    <header class="fullscreen-modal__head">
      <h2>{$i18n.t(mode === 'edit' ? 'connection.editTitle' : 'connection.new')}</h2>
      <button class="modal-close" aria-label={$i18n.t('common.close')} on:click={() => dispatch('close')} type="button">×</button>
    </header>

    <div class="new-connection-body">
      <label class="modal-field">
        <span>{$i18n.t('connection.serviceName')}</span>
        <input bind:value={name} placeholder={$i18n.t('connection.serviceNamePlaceholder')} />
      </label>

      <label class="modal-field">
        <span>Endpoint</span>
        <input bind:value={endpoint} placeholder="localhost:50051" />
      </label>

      <label class="modal-field">
        <span>{$i18n.t('connection.authority')}</span>
        <input bind:value={authority} placeholder="api.example.com" />
      </label>

      <div class="modal-field-grid modal-field-grid--three">
        <label class="modal-field modal-field--compact">
          <span>{$i18n.t('connection.connectTimeoutMs')}</span>
          <input bind:value={connectTimeoutMs} min="1" step="1" type="number" />
        </label>
        <label class="modal-field modal-field--compact">
          <span>{$i18n.t('connection.requestTimeoutMs')}</span>
          <input bind:value={requestTimeoutMs} min="1" step="1" type="number" />
        </label>
        <label class="modal-field modal-field--compact">
          <span>{$i18n.t('connection.streamIdleTimeoutMs')}</span>
          <input bind:value={streamIdleTimeoutMs} min="1" step="1" type="number" />
        </label>
      </div>

      <label class="switch-row">
        <input bind:checked={tlsEnabled} type="checkbox" />
        <span>{$i18n.t('connection.useTls')}</span>
      </label>

      {#if tlsEnabled}
        <div class="tls-settings-panel">
          <label class="modal-field modal-field--compact">
            <span>{$i18n.t('connection.tlsMode')}</span>
            <select bind:value={tlsMode}>
              <option value="system_ca">{$i18n.t('connection.tlsSystemCA')}</option>
              <option value="custom_ca">{$i18n.t('connection.tlsCustomCA')}</option>
              <option value="mtls">{$i18n.t('connection.tlsMTLS')}</option>
            </select>
          </label>

          <label class="modal-field modal-field--compact">
            <span>{$i18n.t('connection.serverNameOverride')}</span>
            <input bind:value={serverNameOverride} placeholder="api.example.com" />
          </label>

          {#if needsCustomCA}
            <div class="material-field">
              <span>{$i18n.t('connection.caCert')}</span>
              <strong>{caCertLabel || caCertRef || $i18n.t('connection.materialNotSelected')}</strong>
              <button class="secondary-command" disabled={pending} on:click={() => chooseMaterialFile('tls_ca')} type="button">
                {$i18n.t('connection.browse')}
              </button>
            </div>
          {/if}

          {#if needsClientMaterial}
            <div class="material-field">
              <span>{$i18n.t('connection.clientCert')}</span>
              <strong>{clientCertLabel || clientCertRef || $i18n.t('connection.materialNotSelected')}</strong>
              <button class="secondary-command" disabled={pending} on:click={() => chooseMaterialFile('tls_client_cert')} type="button">
                {$i18n.t('connection.browse')}
              </button>
            </div>
            <div class="material-field">
              <span>{$i18n.t('connection.clientKey')}</span>
              <strong>{clientKeyLabel || clientKeyRef || $i18n.t('connection.materialNotSelected')}</strong>
              <button class="secondary-command" disabled={pending} on:click={() => chooseMaterialFile('tls_client_key')} type="button">
                {$i18n.t('connection.browse')}
              </button>
            </div>
          {/if}
        </div>
      {/if}

      <div class="catalog-tabs">
        <button class:selected-tab={catalogSource === 'reflection'} on:click={() => (catalogSource = 'reflection')} type="button">Reflection</button>
        <button class:selected-tab={catalogSource === 'proto'} on:click={() => (catalogSource = 'proto')} type="button">Proto files</button>
      </div>

      {#if catalogSource === 'proto'}
        <div class="proto-dropzone">
          <span class="proto-dropzone__icon" aria-hidden="true">⌘</span>
          <strong>{$i18n.t('connection.uploadProto')}</strong>
          <span>{$i18n.t('connection.dragProto')}</span>
          <div class="proto-picker-actions">
            <button class="secondary-command" disabled={pending} on:click={chooseProtoFiles} type="button">
              {$i18n.t('connection.chooseProtoFiles')}
            </button>
            <button class="secondary-command" disabled={pending} on:click={chooseProtoDirectory} type="button">
              {$i18n.t('connection.chooseProtoDirectory')}
            </button>
          </div>
        </div>

        <div class="proto-file-state">
          {#if protoSources.length === 0}
            {$i18n.t('connection.noFiles')}
          {:else}
            {#each protoSources as source}
              <button class="proto-file-chip" on:click={() => removeProtoSource(source.path)} type="button">
                {protoSourceLabel(source)}
              </button>
            {/each}
          {/if}
        </div>
      {:else}
        <div class="reflection-hint">
          {$i18n.t('connection.reflectionHint')}
        </div>
      {/if}

      <label class="modal-field modal-field--textarea">
        <span>{$i18n.t('connection.metadataDefaults')}</span>
        <textarea bind:value={metadataDefaultsText} placeholder={$i18n.t('connection.metadataDefaultsPlaceholder')} rows="3"></textarea>
      </label>

      {#if visibleError || metadataDefaultsError}
        <div class="connection-error" role="alert">{visibleError || metadataDefaultsError}</div>
      {/if}
    </div>

    <footer class="fullscreen-modal__foot">
      <button class="secondary-command" disabled={pending} on:click={() => dispatch('close')} type="button">{$i18n.t('common.cancel')}</button>
      <button class="primary-command" disabled={!canCreate || pending} on:click={submitConnection} type="button">
        {pending
          ? $i18n.t('connection.connecting')
          : $i18n.t(mode === 'edit' ? 'connection.saveChanges' : 'connection.create')}
      </button>
    </footer>
  </div>
{/if}
