<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { AppOverlay, AppView } from '../contracts'
  import { i18n } from '../i18n'

  export let activeView: AppView
  export let activeOverlay: AppOverlay | null

  const dispatch = createEventDispatcher<{
    openview: AppView
    toggleoverlay: AppOverlay
  }>()

  $: views = [
    { id: 'home' as const, title: $i18n.t('nav.homeTitle'), copy: $i18n.t('nav.homeCopy') },
    { id: 'workspace' as const, title: $i18n.t('nav.workspaceTitle'), copy: $i18n.t('nav.workspaceCopy') },
    { id: 'session' as const, title: $i18n.t('nav.sessionTitle'), copy: $i18n.t('nav.sessionCopy') },
  ]

  $: overlays = [
    { id: 'history-overlay' as const, title: $i18n.t('nav.historyOverlay'), copy: $i18n.t('nav.historyOverlayCopy') },
    { id: 'settings-overlay' as const, title: $i18n.t('nav.settingsOverlay'), copy: $i18n.t('nav.settingsOverlayCopy') },
    { id: 'diagnostics-overlay' as const, title: $i18n.t('nav.diagnosticsOverlay'), copy: $i18n.t('nav.diagnosticsOverlayCopy') },
  ]
</script>

<aside class="nav-rail">
  <div class="nav-group">
    <p class="eyebrow">{$i18n.t('nav.primaryFlow')}</p>
    <div class="nav-list">
      {#each views as view}
        <button
          class:view-button--active={activeView === view.id}
          class="view-button"
          on:click={() => dispatch('openview', view.id)}
        >
          <strong>{view.title}</strong>
          <div class="subtle">{view.copy}</div>
        </button>
      {/each}
    </div>
  </div>

  <div class="divider"></div>

  <div class="nav-group">
    <h3>{$i18n.t('nav.overlays')}</h3>
    <div class="nav-list">
      {#each overlays as overlay}
        <button
          class:view-button--active={activeOverlay === overlay.id}
          class="view-button"
          on:click={() => dispatch('toggleoverlay', overlay.id)}
        >
          <strong>{overlay.title}</strong>
          <div class="subtle">{overlay.copy}</div>
        </button>
      {/each}
    </div>
  </div>
</aside>
