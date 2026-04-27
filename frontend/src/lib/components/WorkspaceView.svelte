<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { BootstrapData } from '../contracts'

  export let bootstrap: BootstrapData
  export let contractMismatch: string[]

  const dispatch = createEventDispatcher<{
    opensession: void
  }>()
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">Slice 0.2</p>
    <h2 class="section-title">Runtime contract and module boundaries</h2>
    <p class="section-copy">
      The shell already exposes the future MVP binding/event surface, typed invoke DTOs and module
      ownership that later epics will fill with transport logic.
    </p>
  </div>

  <div class="two-column">
    <div class="card table-like">
      <h3>Layout regions</h3>
      {#each bootstrap.layout.regions as region}
        <div class="table-row">
          <strong>{region.id}</strong>
          <div>{region.purpose}</div>
        </div>
      {/each}
    </div>

    <div class="card list-block">
      <h3>Contract status</h3>
      <p>{contractMismatch.length === 0 ? 'No drift between frontend and backend manifests.' : 'Contract mismatch detected.'}</p>
      <div class="pill-row">
        <span class="pill pill--accent">{bootstrap.contract.boundMethods.length} bindings</span>
        <span class="pill">{bootstrap.contract.eventNames.length} live events</span>
        <span class="pill">{bootstrap.contract.modules.length} modules</span>
      </div>
      <button class="ghost-button" on:click={() => dispatch('opensession')}>Inspect session shell</button>
    </div>
  </div>

  <div class="grid-cards">
    {#each bootstrap.contract.modules as module}
      <article class="card">
        <h4>{module.name}</h4>
        <p>{module.responsibility}</p>
      </article>
    {/each}
  </div>
</section>
