<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { DiagnosticsUpdateEvent, EventProbeState } from '../contracts'
  import { getDiagnosticContextRows } from '../diagnostics'
  import {
    i18n,
    translateDiagnosticCodeLabel,
    translateDiagnosticMessage,
    translateDiagnosticNextStep,
  } from '../i18n'

  export let isOpen: boolean
  export let diagnostics: DiagnosticsUpdateEvent[]
  export let probe: EventProbeState
  export let mismatch: string[]

  const dispatch = createEventDispatcher<{
    close: void
  }>()

  function formatProbeClassification(classification: string): string {
    const label = translateDiagnosticCodeLabel($i18n.language, classification)
    return label === classification ? classification : `${label} (${classification})`
  }
</script>

<aside class="diagnostics">
  <div class="diagnostic-item__head">
    <div>
      <p class="eyebrow">{$i18n.t('diagnostics.title')}</p>
      <h2 class="section-title">{$i18n.t('diagnostics.structuredStatus')}</h2>
    </div>
    {#if isOpen}
      <button class="ghost-button" on:click={() => dispatch('close')}>{$i18n.t('common.hide')}</button>
    {/if}
  </div>

  <div class="stack">
    <div class="card list-block">
      <h4>{$i18n.t('diagnostics.probeStatus')}</h4>
      <p>
        {#if probe.pending}
          {$i18n.t('diagnostics.probeInProgress')}
        {:else if probe.error}
          {probe.error}
        {:else if probe.lastAcknowledgement}
          {formatProbeClassification(probe.lastAcknowledgement.classification)}
        {:else}
          {$i18n.t('diagnostics.probeNotRun')}
        {/if}
      </p>
    </div>

    <div class="card list-block">
      <h4>{$i18n.t('diagnostics.contractGuard')}</h4>
      {#if mismatch.length === 0}
        <div class="empty-state">{$i18n.t('diagnostics.manifestsMatch')}</div>
      {:else}
        <ul>
          {#each mismatch as issue}
            <li>{issue}</li>
          {/each}
        </ul>
      {/if}
    </div>

    {#if isOpen}
      <div class="stack">
        {#if diagnostics.length === 0}
          <div class="empty-state">{$i18n.t('diagnostics.emptyCaptured')}</div>
        {:else}
          {#each diagnostics as diagnostic}
            <article class="diagnostic-item">
              <div class="diagnostic-item__head">
                <strong>{diagnostic.code}</strong>
                <span class="diagnostic-item__meta">{diagnostic.ts}</span>
              </div>
              <div>{translateDiagnosticMessage($i18n.language, diagnostic.code, diagnostic.message)}</div>
              {#if diagnostic.nextStep}
                <div class="subtle">
                  {$i18n.t('diagnostics.nextStep', {
                    step: translateDiagnosticNextStep($i18n.language, diagnostic.code, diagnostic.nextStep),
                  })}
                </div>
              {/if}
              {#if diagnostic.details}
                <div class="table-like">
                  {#each getDiagnosticContextRows(diagnostic.details, $i18n.language) as detail}
                    <div class="table-row">
                      <strong>{detail.label}</strong>
                      <span>{detail.value}</span>
                    </div>
                  {/each}
                </div>
              {/if}
            </article>
          {/each}
        {/if}
      </div>
    {:else}
      <div class="card list-block">
        <h4>{$i18n.t('diagnostics.overlayTitle')}</h4>
        <p>
          {$i18n.t('diagnostics.overlayCopy')}
        </p>
      </div>
    {/if}
  </div>
</aside>
