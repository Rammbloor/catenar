<script lang="ts">
  import { onMount } from 'svelte'
  import { EventsOff, EventsOn } from '../wailsjs/runtime/runtime'
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
      const message = error instanceof Error ? error.message : 'Unexpected diagnostics probe failure.'
      shell.setProbeError(message)
    }
  }

  onMount(() => {
    EventsOn('diagnostics:update', (payload: DiagnosticsUpdateEvent) => {
      shell.applyDiagnosticsEvent(payload)
    })
    EventsOn('stream:state', (payload: StreamStateEvent) => {
      shell.setStreamState(payload.state)
      shell.setStreamConditions(payload.conditions)
    })
    EventsOn('stream:completed', (payload: StreamCompletedEvent) => {
      shell.setStreamState(payload.finalState)
      shell.setStreamConditions(payload.conditions)
    })

    void (async () => {
      try {
        const bootstrap = await fetchBootstrap()
        shell.hydrateBootstrap(bootstrap)
        await runProbe()
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to bootstrap the application shell.'
        shell.setBootstrapError(message)
      }
    })()

    return () => {
      EventsOff('diagnostics:update', 'stream:state', 'stream:completed')
    }
  })
</script>

<svelte:head>
  <title>tether</title>
</svelte:head>

<div class="app-shell">
  <header class="topbar">
    <div class="topbar__identity">
      <div>
        <p class="eyebrow">Epic 0 Foundations</p>
        <h1>tether</h1>
      </div>
      <p class="tagline">
        {$shell.bootstrap?.app.productLine ?? 'Desktop-first gRPC debugging workspace'}
      </p>
    </div>

    <div class="topbar__meta">
      <div class="meta-chip">
        <span>runtime</span>
        <strong>{$shell.bootstrap?.app.platform ?? 'booting'} / {$shell.bootstrap?.app.architecture ?? '...'}</strong>
      </div>
      <div class="meta-chip" class:meta-chip--warning={$shell.contractMismatch.length > 0}>
        <span>contract</span>
        <strong>{$shell.contractMismatch.length === 0 ? 'verified' : 'mismatch'}</strong>
      </div>
      <button class="action-button" on:click={runProbe} disabled={$shell.eventProbe.pending}>
        {$shell.eventProbe.pending ? 'Probing event bridge…' : 'Probe event bridge'}
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
          <h2>Bootstrap failed</h2>
          <p>{$shell.bootstrapError}</p>
        </section>
      {:else if !$shell.bootstrap}
        <section class="panel panel--loading">
          <p>Loading the Wails runtime contract and app shell…</p>
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
      <span class="status-label">view</span>
      <strong>{$shell.currentView}</strong>
    </div>
    <div>
      <span class="status-label">stream</span>
      <strong>{$shell.activeStreamState}</strong>
    </div>
    <div>
      <span class="status-label">diagnostics</span>
      <strong>{$shell.diagnostics.length}</strong>
    </div>
    <div>
      <span class="status-label">probe</span>
      <strong>
        {$shell.eventProbe.lastAcknowledgement?.classification ?? $shell.eventProbe.error ?? 'not-run'}
      </strong>
    </div>
  </footer>
</div>
