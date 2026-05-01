<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type {
    BootstrapData,
    DiagnosticsUpdateEvent,
    ProbeAcknowledgement,
    SessionCondition,
    StreamState,
  } from '../contracts'
  import {
    i18n,
    translateDiagnosticCodeLabel,
    translateDiagnosticMessage,
    translateStreamStateLabel,
  } from '../i18n'

  export let bootstrap: BootstrapData
  export let state: StreamState
  export let conditions: SessionCondition[]
  export let lastProbe: ProbeAcknowledgement | undefined
  export let diagnostics: DiagnosticsUpdateEvent[]

  const dispatch = createEventDispatcher<{
    openworkspace: void
  }>()

  $: stateBadgeClass =
    state === 'closed' || state === 'cancelled' || state === 'error' ? 'badge badge--terminal' : state === 'open' ? 'badge badge--live' : 'badge'

  function formatProbeClassification(classification: string): string {
    const label = translateDiagnosticCodeLabel($i18n.language, classification)
    return label === classification ? classification : `${label} (${classification})`
  }
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">{$i18n.t('session.eyebrow')}</p>
    <h2 class="section-title">{$i18n.t('session.title')}</h2>
    <p class="section-copy">
      {$i18n.t('session.copy')}
    </p>
  </div>

  <div class="two-column">
    <div class="card stack">
      <div>
        <span class={stateBadgeClass}>{translateStreamStateLabel($i18n.language, state)}</span>
      </div>
      <div class="pill-row">
        {#each bootstrap.contract.streamStates as streamState}
          <span class:badge--live={streamState === state} class="pill">{translateStreamStateLabel($i18n.language, streamState)}</span>
        {/each}
      </div>
      <div class="pill-row">
        {#if conditions.length === 0}
          <span class="pill">{$i18n.t('session.noConditions')}</span>
        {:else}
          {#each conditions as condition}
            <span class="pill pill--warning">{condition}</span>
          {/each}
        {/if}
      </div>
      <button class="ghost-button" on:click={() => dispatch('openworkspace')}>{$i18n.t('session.backToWorkspace')}</button>
    </div>

    <div class="card list-block">
      <h3>{$i18n.t('session.diagnosticsProbe')}</h3>
      <p>
        {#if lastProbe}
          {$i18n.t('session.lastAcknowledgement')} <strong>{formatProbeClassification(lastProbe.classification)}</strong>
        {:else}
          {$i18n.t('session.probeNotCompleted')}
        {/if}
      </p>
      <p class="subtle">{lastProbe?.emittedAt ?? $i18n.t('session.waitingEmission')}</p>
    </div>
  </div>

  <div class="grid-cards">
    <article class="card list-block">
      <h4>{$i18n.t('session.errorTaxonomy')}</h4>
      <div class="pill-row">
        {#each bootstrap.contract.errorCategories as category}
          <span class="pill">{category}</span>
        {/each}
      </div>
    </article>
    <article class="card list-block">
      <h4>{$i18n.t('session.recentDiagnostics')}</h4>
      {#if diagnostics.length === 0}
        <div class="empty-state">{$i18n.t('diagnostics.emptyRecent')}</div>
      {:else}
        <ul>
          {#each diagnostics.slice(0, 3) as diagnostic}
            <li><strong>{diagnostic.code}</strong> — {translateDiagnosticMessage($i18n.language, diagnostic.code, diagnostic.message)}</li>
          {/each}
        </ul>
      {/if}
    </article>
  </div>
</section>
