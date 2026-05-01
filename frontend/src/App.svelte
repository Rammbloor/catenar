<script lang="ts">
  import { onMount } from 'svelte'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import DiagnosticsPanel from './lib/components/DiagnosticsPanel.svelte'
  import HomeView from './lib/components/HomeView.svelte'
  import NavRail from './lib/components/NavRail.svelte'
  import SessionView from './lib/components/SessionView.svelte'
  import WorkspaceView from './lib/components/WorkspaceView.svelte'
  import type {
    AppOverlay,
    AppView,
    DiagnosticsUpdateEvent,
    StreamCompletedEvent,
    StreamStateEvent,
  } from './lib/contracts'
  import {
    i18n,
    language,
    setLanguage,
    SUPPORTED_LANGUAGES,
    translateDiagnosticCodeLabel,
    translateProductLine,
    translateStreamStateLabel,
    translateViewLabel,
    type Language,
  } from './lib/i18n'
  import { createAppShellStore } from './lib/state/app-shell'
  import { emitDiagnosticsProbe, fetchBootstrap } from './lib/wails/backend'

  const shell = createAppShellStore()

  function toggleOverlay(kind: AppOverlay): void {
    if ($shell.activeOverlay === kind) {
      shell.closeOverlay()
      return
    }

    shell.openOverlay(kind)
  }

  function isActiveView(view: AppView): boolean {
    return $shell.currentView === view
  }

  async function runProbe(): Promise<void> {
    shell.startProbe()

    try {
      const acknowledgement = await emitDiagnosticsProbe()
      shell.finishProbe(acknowledgement)
    } catch (error) {
      const message = error instanceof Error ? error.message : $i18n.t('errors.diagnosticsProbeUnexpected')
      shell.setProbeError(message)
    }
  }

  function handleLanguageChange(event: Event): void {
    const nextLanguage = (event.currentTarget as HTMLSelectElement).value as Language
    setLanguage(nextLanguage)
  }

  function formatProbeFooter(): string {
    const classification = $shell.eventProbe.lastAcknowledgement?.classification
    if (classification) {
      const label = translateDiagnosticCodeLabel($i18n.language, classification)
      return label === classification ? classification : `${label} (${classification})`
    }

    return $shell.eventProbe.error ?? $i18n.t('footer.notRun')
  }

  onMount(() => {
    const offDiagnostics = EventsOn('diagnostics:update', (payload: DiagnosticsUpdateEvent) => {
      shell.applyDiagnosticsEvent(payload)
    })
    const offStreamState = EventsOn('stream:state', (payload: StreamStateEvent) => {
      shell.setStreamState(payload.state)
      shell.setStreamConditions(payload.conditions)
    })
    const offStreamCompleted = EventsOn('stream:completed', (payload: StreamCompletedEvent) => {
      shell.setStreamState(payload.finalState)
      shell.setStreamConditions(payload.conditions)
    })

    void (async () => {
      try {
        const bootstrap = await fetchBootstrap()
        shell.hydrateBootstrap(bootstrap)
        await runProbe()
      } catch (error) {
        const message = error instanceof Error ? error.message : $i18n.t('errors.bootstrapUnexpected')
        shell.setBootstrapError(message)
      }
    })()

    return () => {
      offDiagnostics()
      offStreamState()
      offStreamCompleted()
    }
  })
</script>

<svelte:head>
  <title>{$i18n.t('app.title')}</title>
</svelte:head>

<div class="app-shell">
  <header class="topbar">
    <div class="topbar__identity">
      <div>
        <p class="eyebrow">{$i18n.t('topbar.eyebrow')}</p>
        <h1>tether</h1>
      </div>
      <p class="tagline">
        {translateProductLine($i18n.language, $shell.bootstrap?.app.productLine)}
      </p>
    </div>

    <div class="topbar__meta">
      <div class="meta-chip">
        <span>{$i18n.t('topbar.runtime')}</span>
        <strong>{$shell.bootstrap?.app.platform ?? $i18n.t('common.booting')} / {$shell.bootstrap?.app.architecture ?? $i18n.t('common.archPending')}</strong>
      </div>
      <div class="meta-chip" class:meta-chip--warning={$shell.contractMismatch.length > 0}>
        <span>{$i18n.t('topbar.contract')}</span>
        <strong>{$shell.contractMismatch.length === 0 ? $i18n.t('common.contractVerified') : $i18n.t('common.contractDrift')}</strong>
      </div>
      <label class="language-switcher">
        <span>{$i18n.t('common.language')}</span>
        <select value={$language} on:change={handleLanguageChange}>
          {#each SUPPORTED_LANGUAGES as supportedLanguage}
            <option value={supportedLanguage}>
              {supportedLanguage === 'ru' ? $i18n.t('common.languageRu') : $i18n.t('common.languageEn')}
            </option>
          {/each}
        </select>
      </label>
      <button class="action-button" on:click={runProbe} disabled={$shell.eventProbe.pending}>
        {$shell.eventProbe.pending ? $i18n.t('topbar.probePending') : $i18n.t('topbar.probe')}
      </button>
    </div>
  </header>

  <div class="shell-grid">
    <NavRail
      activeView={$shell.currentView}
      activeOverlay={$shell.activeOverlay}
      on:openview={(event) => shell.openView(event.detail)}
      on:toggleoverlay={(event) => toggleOverlay(event.detail)}
    />

    <main class="canvas">
      {#if $shell.bootstrapError}
        <section class="panel panel--critical">
          <h2>{$i18n.t('app.bootstrapFailed')}</h2>
          <p>{$shell.bootstrapError}</p>
        </section>
      {:else if !$shell.bootstrap}
        <section class="panel panel--loading">
          <p>{$i18n.t('app.loading')}</p>
        </section>
      {:else}
        {#if isActiveView('home')}
          <HomeView
            bootstrap={$shell.bootstrap}
            contractMismatch={$shell.contractMismatch}
            on:openworkspace={() => shell.openView('workspace')}
            on:opensession={() => shell.openView('session')}
          />
        {/if}

        {#if isActiveView('workspace')}
          <WorkspaceView
            bootstrap={$shell.bootstrap}
            contractMismatch={$shell.contractMismatch}
            on:opensession={() => shell.openView('session')}
          />
        {/if}

        {#if isActiveView('session')}
          <SessionView
            bootstrap={$shell.bootstrap}
            state={$shell.activeStreamState}
            conditions={$shell.streamConditions}
            lastProbe={$shell.eventProbe.lastAcknowledgement}
            diagnostics={$shell.diagnostics}
            on:openworkspace={() => shell.openView('workspace')}
          />
        {/if}
      {/if}
    </main>

    <DiagnosticsPanel
      isOpen={$shell.activeOverlay === 'diagnostics-overlay'}
      diagnostics={$shell.diagnostics}
      probe={$shell.eventProbe}
      mismatch={$shell.contractMismatch}
      on:close={() => shell.closeOverlay()}
    />
  </div>

  <footer class="status-strip">
    <div>
      <span class="status-label">{$i18n.t('common.view')}</span>
      <strong>{translateViewLabel($i18n.language, $shell.currentView)}</strong>
    </div>
    <div>
      <span class="status-label">{$i18n.t('footer.stream')}</span>
      <strong>{translateStreamStateLabel($i18n.language, $shell.activeStreamState)}</strong>
    </div>
    <div>
      <span class="status-label">{$i18n.t('common.diagnostics')}</span>
      <strong>{$shell.diagnostics.length}</strong>
    </div>
    <div>
      <span class="status-label">{$i18n.t('footer.probe')}</span>
      <strong>{formatProbeFooter()}</strong>
    </div>
  </footer>
</div>
