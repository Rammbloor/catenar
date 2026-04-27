<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { DiagnosticsUpdateEvent, EventProbeState } from '../contracts'

  export let isOpen: boolean
  export let diagnostics: DiagnosticsUpdateEvent[]
  export let probe: EventProbeState
  export let mismatch: string[]

  const dispatch = createEventDispatcher<{
    close: void
  }>()
</script>

<aside class="diagnostics">
  <div class="diagnostic-item__head">
    <div>
      <p class="eyebrow">Diagnostics</p>
      <h2 class="section-title">Structured status</h2>
    </div>
    {#if isOpen}
      <button class="ghost-button" on:click={() => dispatch('close')}>Hide panel</button>
    {/if}
  </div>

  <div class="stack">
    <div class="card list-block">
      <h4>Probe status</h4>
      <p>
        {#if probe.pending}
          Wails event bridge probe in progress.
        {:else if probe.error}
          {probe.error}
        {:else if probe.lastAcknowledgement}
          {probe.lastAcknowledgement.classification}
        {:else}
          Probe has not run yet.
        {/if}
      </p>
    </div>

    <div class="card list-block">
      <h4>Contract guard</h4>
      {#if mismatch.length === 0}
        <div class="empty-state">Frontend and backend manifests match.</div>
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
          <div class="empty-state">No diagnostics captured yet.</div>
        {:else}
          {#each diagnostics as diagnostic}
            <article class="diagnostic-item">
              <div class="diagnostic-item__head">
                <strong>{diagnostic.code}</strong>
                <span class="diagnostic-item__meta">{diagnostic.ts}</span>
              </div>
              <div>{diagnostic.message}</div>
              {#if diagnostic.nextStep}
                <div class="subtle">Next step: {diagnostic.nextStep}</div>
              {/if}
            </article>
          {/each}
        {/if}
      </div>
    {:else}
      <div class="card list-block">
        <h4>Overlay behavior</h4>
        <p>
          Diagnostics remain wired in the app shell, but the detailed event feed opens only when the
          diagnostics overlay is active.
        </p>
      </div>
    {/if}
  </div>
</aside>
