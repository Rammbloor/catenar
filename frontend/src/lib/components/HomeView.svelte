<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { BootstrapData } from '../contracts'

  export let bootstrap: BootstrapData
  export let contractMismatch: string[]

  const dispatch = createEventDispatcher<{
    openworkspace: void
    opensession: void
  }>()
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">Slice 0.1</p>
    <h2 class="section-title">Production shell is live</h2>
    <p class="section-copy">
      Wails runtime, Svelte 5 shell and diagnostics event bus are wired together before the
      transport/runtime features arrive in Epic 1.
    </p>
  </div>

  <div class="grid-cards">
    <article class="card">
      <h3>App shell</h3>
      <p>
        Four layout regions are active and the frontend is bootstrapped from Go runtime metadata,
        not hardcoded browser-only assumptions.
      </p>
    </article>
    <article class="card">
      <h3>Contract guard</h3>
      <p>
        {#if contractMismatch.length === 0}
          Frontend and backend identifiers match the v1 contract manifest.
        {:else}
          Contract drift detected and blocked by startup verification.
        {/if}
      </p>
    </article>
  </div>

  <div class="two-column">
    <div class="list-block card">
      <h4>Epic 0 slices</h4>
      <ul>
        {#each bootstrap.epicZero as slice}
          <li><strong>{slice.slice}</strong> — {slice.summary}</li>
        {/each}
      </ul>
    </div>

    <div class="list-block card">
      <h4>Primary flow</h4>
      <div class="pill-row">
        {#each bootstrap.stateModel.primaryFlow as step}
          <span class="pill">{step}</span>
        {/each}
      </div>
      <div class="pill-row">
        <button class="ghost-button" on:click={() => dispatch('openworkspace')}>Open workspace shell</button>
        <button class="ghost-button" on:click={() => dispatch('opensession')}>Open session shell</button>
      </div>
    </div>
  </div>
</section>
