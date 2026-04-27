<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type {
    BootstrapData,
    DiagnosticsUpdateEvent,
    ProbeAcknowledgement,
    SessionCondition,
    StreamState,
  } from '../contracts'

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
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">Slice 0.3</p>
    <h2 class="section-title">Canonical session state model</h2>
    <p class="section-copy">
      UI shell and Go runtime share the same stream states, terminal outcomes and diagnostics
      taxonomy before streaming transport arrives.
    </p>
  </div>

  <div class="two-column">
    <div class="card stack">
      <div>
        <span class={stateBadgeClass}>{state}</span>
      </div>
      <div class="pill-row">
        {#each bootstrap.contract.streamStates as streamState}
          <span class:badge--live={streamState === state} class="pill">{streamState}</span>
        {/each}
      </div>
      <div class="pill-row">
        {#if conditions.length === 0}
          <span class="pill">no conditions</span>
        {:else}
          {#each conditions as condition}
            <span class="pill pill--warning">{condition}</span>
          {/each}
        {/if}
      </div>
      <button class="ghost-button" on:click={() => dispatch('openworkspace')}>Back to workspace shell</button>
    </div>

    <div class="card list-block">
      <h3>Diagnostics probe</h3>
      <p>
        {#if lastProbe}
          Last event bridge acknowledgement: <strong>{lastProbe.classification}</strong>
        {:else}
          Diagnostics probe has not completed yet.
        {/if}
      </p>
      <p class="subtle">{lastProbe?.emittedAt ?? 'Waiting for runtime event emission.'}</p>
    </div>
  </div>

  <div class="grid-cards">
    <article class="card list-block">
      <h4>Error taxonomy</h4>
      <div class="pill-row">
        {#each bootstrap.contract.errorCategories as category}
          <span class="pill">{category}</span>
        {/each}
      </div>
    </article>
    <article class="card list-block">
      <h4>Recent diagnostics</h4>
      {#if diagnostics.length === 0}
        <div class="empty-state">No diagnostics yet.</div>
      {:else}
        <ul>
          {#each diagnostics.slice(0, 3) as diagnostic}
            <li><strong>{diagnostic.code}</strong> — {diagnostic.message}</li>
          {/each}
        </ul>
      {/if}
    </article>
  </div>
</section>
