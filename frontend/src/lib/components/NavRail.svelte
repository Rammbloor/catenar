<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { AppOverlay, AppView } from '../contracts'

  export let activeView: AppView
  export let activeOverlay: AppOverlay | null

  const dispatch = createEventDispatcher<{
    openview: AppView
    toggleoverlay: AppOverlay
  }>()

  const views: { id: AppView; title: string; copy: string }[] = [
    { id: 'home', title: 'Home', copy: 'Onboarding, slice status and shell readiness.' },
    { id: 'workspace', title: 'Workspace', copy: 'Layout regions, module boundaries and invoke contract.' },
    { id: 'session', title: 'Session', copy: 'Canonical stream states and diagnostics-aware session shell.' },
  ]

  const overlays: { id: AppOverlay; title: string }[] = [
    { id: 'history-overlay', title: 'History overlay' },
    { id: 'settings-overlay', title: 'Settings overlay' },
    { id: 'diagnostics-overlay', title: 'Diagnostics overlay' },
  ]
</script>

<aside class="nav-rail">
  <div class="nav-group">
    <p class="eyebrow">Primary flow</p>
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
    <h3>Overlays</h3>
    <div class="nav-list">
      {#each overlays as overlay}
        <button
          class:view-button--active={activeOverlay === overlay.id}
          class="view-button"
          on:click={() => dispatch('toggleoverlay', overlay.id)}
        >
          {overlay.title}
        </button>
      {/each}
    </div>
  </div>
</aside>
