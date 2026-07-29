<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { DiagnosticsUpdateEvent, SessionCondition, StreamState } from '../contracts'
  import {
    i18n,
    translateDiagnosticCategory,
    translateDiagnosticCodeLabel,
    translateDiagnosticDetailLabel,
    translateDiagnosticLevel,
    translateDiagnosticMessage,
    translateDiagnosticNextStep,
    translateStreamStateLabel,
  } from '../i18n'

  export let isOpen = false
  export let diagnostics: DiagnosticsUpdateEvent[] = []
  export let streamState: StreamState = 'idle'
  export let streamConditions: SessionCondition[] = []
  export let pending = false
  export let error = ''
  export let info = ''

  const dispatch = createEventDispatcher<{
    close: void
    export: void
  }>()

  let selectedDiagnosticId = ''

  $: selectedDiagnostic =
    diagnostics.find((diagnostic) => diagnostic.id === selectedDiagnosticId) ?? diagnostics[0]
  $: if (isOpen && !selectedDiagnosticId && diagnostics[0]) {
    selectedDiagnosticId = diagnostics[0].id
  }
  $: if (!isOpen) {
    selectedDiagnosticId = ''
  }

  function formatTime(value: string): string {
    return new Date(value).toLocaleString([], {
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      month: '2-digit',
      second: '2-digit',
    })
  }

  function detailsEntries(details: Record<string, string> | undefined): [string, string][] {
    return Object.entries(details ?? {})
  }

  function diagnosticLabel(diagnostic: DiagnosticsUpdateEvent): string {
    return translateDiagnosticCodeLabel($i18n.language, diagnostic.code)
  }

  function diagnosticMessage(diagnostic: DiagnosticsUpdateEvent): string {
    return translateDiagnosticMessage($i18n.language, diagnostic.code, diagnostic.message)
  }

  function diagnosticNextStep(diagnostic: DiagnosticsUpdateEvent): string {
    return translateDiagnosticNextStep($i18n.language, diagnostic.code, diagnostic.nextStep)
  }

  function levelClass(level: string): string {
    return `diagnostics-event diagnostics-event--${level.toLowerCase()}`
  }
</script>

{#if isOpen}
  <button
    aria-label={$i18n.t('common.close')}
    class="overlay-backdrop"
    on:click={() => dispatch('close')}
    type="button"
  ></button>
  <section aria-modal="true" class="overlay-panel overlay-panel--diagnostics" role="dialog">
    <header class="settings-modal__header">
      <h2 class="settings-modal__title">{$i18n.t('diagnostics.panelTitle')}</h2>
      <button class="icon-close-button" on:click={() => dispatch('close')} type="button">×</button>
    </header>

    <div class="diagnostics-modal__body">
      <aside class="diagnostics-summary">
        <div class="diagnostics-state-card">
          <span>{$i18n.t('diagnostics.streamState')}</span>
          <strong>{translateStreamStateLabel($i18n.language, streamState)}</strong>
        </div>
        <div class="diagnostics-state-card">
          <span>{$i18n.t('diagnostics.conditions')}</span>
          <strong>{streamConditions.length}</strong>
        </div>

        <button
          class="secondary-command diagnostics-export"
          disabled={pending}
          on:click={() => dispatch('export')}
          type="button"
        >
          {pending ? $i18n.t('diagnostics.exporting') : $i18n.t('diagnostics.export')}
        </button>

        {#if error}
          <p class="diagnostics-message diagnostics-message--error">{error}</p>
        {:else if info}
          <p class="diagnostics-message">{info}</p>
        {/if}
      </aside>

      <div class="diagnostics-list">
        {#if diagnostics.length > 0}
          {#each diagnostics as diagnostic}
            <button
              class:diagnostics-event--active={selectedDiagnostic?.id === diagnostic.id}
              class={levelClass(diagnostic.level)}
              on:click={() => (selectedDiagnosticId = diagnostic.id)}
              type="button"
            >
              <span>{translateDiagnosticLevel($i18n.language, diagnostic.level)}</span>
              <strong>{diagnosticLabel(diagnostic)}</strong>
              <small>{formatTime(diagnostic.ts)}</small>
            </button>
          {/each}
        {:else}
          <div class="diagnostics-empty">
            <strong>{$i18n.t('diagnostics.emptyTitle')}</strong>
            <span>{$i18n.t('diagnostics.emptyCopy')}</span>
          </div>
        {/if}
      </div>

      <section class="diagnostics-detail">
        {#if selectedDiagnostic}
          <div class="diagnostics-detail__head">
            <span>{translateDiagnosticCategory($i18n.language, selectedDiagnostic.category)}</span>
            <strong>{selectedDiagnostic.source}</strong>
          </div>
          <p>{diagnosticMessage(selectedDiagnostic)}</p>
          {#if diagnosticNextStep(selectedDiagnostic)}
            <div class="diagnostics-next-step">
              <span>{$i18n.t('diagnostics.nextStep')}</span>
              <strong>{diagnosticNextStep(selectedDiagnostic)}</strong>
            </div>
          {/if}
          {#if detailsEntries(selectedDiagnostic.details).length > 0}
            <dl class="diagnostics-details-grid">
              {#each detailsEntries(selectedDiagnostic.details) as [key, value]}
                <div>
                  <dt>{translateDiagnosticDetailLabel($i18n.language, key)}</dt>
                  <dd>{value}</dd>
                </div>
              {/each}
            </dl>
          {/if}
        {:else}
          <div class="diagnostics-empty diagnostics-empty--detail">
            <strong>{$i18n.t('diagnostics.noSelectionTitle')}</strong>
            <span>{$i18n.t('diagnostics.noSelectionCopy')}</span>
          </div>
        {/if}
      </section>
    </div>
  </section>
{/if}
