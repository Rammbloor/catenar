<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { BootstrapData } from '../contracts'
  import {
    i18n,
    translateBootstrapSliceSummary,
    translateViewLabel,
  } from '../i18n'

  export let bootstrap: BootstrapData
  export let contractMismatch: string[]

  const dispatch = createEventDispatcher<{
    openworkspace: void
    opensession: void
  }>()
</script>

<section class="panel">
  <div class="stack">
    <p class="eyebrow">{$i18n.t('home.eyebrow')}</p>
    <h2 class="section-title">{$i18n.t('home.title')}</h2>
    <p class="section-copy">
      {$i18n.t('home.copy')}
    </p>
  </div>

  <div class="grid-cards">
    <article class="card">
      <h3>{$i18n.t('home.appShellTitle')}</h3>
      <p>
        {$i18n.t('home.appShellCopy')}
      </p>
    </article>
    <article class="card">
      <h3>{$i18n.t('home.contractGuardTitle')}</h3>
      <p>
        {#if contractMismatch.length === 0}
          {$i18n.t('home.contractMatch')}
        {:else}
          {$i18n.t('home.contractDrift')}
        {/if}
      </p>
    </article>
  </div>

  <div class="two-column">
    <div class="list-block card">
      <h4>{$i18n.t('home.slices')}</h4>
      <ul>
        {#each bootstrap.epicZero as slice}
          <li>
            <strong>{slice.slice}</strong>
            — {translateBootstrapSliceSummary($i18n.language, slice.slice, slice.summary)}
          </li>
        {/each}
      </ul>
    </div>

    <div class="list-block card">
      <h4>{$i18n.t('home.primaryFlow')}</h4>
      <div class="pill-row">
        {#each bootstrap.stateModel.primaryFlow as step}
          <span class="pill">{translateViewLabel($i18n.language, step)}</span>
        {/each}
      </div>
      <div class="pill-row">
        <button class="ghost-button" on:click={() => dispatch('openworkspace')}>{$i18n.t('home.openWorkspace')}</button>
        <button class="ghost-button" on:click={() => dispatch('opensession')}>{$i18n.t('home.openSession')}</button>
      </div>
    </div>
  </div>
</section>
