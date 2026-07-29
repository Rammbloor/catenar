<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
  import type {
    AppMetadata,
    GitHubSyncActionInput,
    GitHubSyncStatus,
    GitHubWorkspaceLinkInput,
    UpdateCheckResult,
    WorkspaceSettings,
    WorkspaceSnapshot,
  } from '../contracts'
  import { i18n, language, setLanguage, SUPPORTED_LANGUAGES, type Language } from '../i18n'
  import {
    isModifierOnlyShortcutEvent,
    isShortcutEvent,
    loadShortcutOverrides,
    persistShortcutOverrides as writeShortcutOverrides,
    shortcutFromEvent,
    shortcutKeys as resolveShortcutKeys,
    SHORTCUT_DEFAULTS,
    type ShortcutDefinition,
  } from '../shortcuts'
  import {
    appSettings,
    setAutoUpdateEnabled,
  } from '../app-settings'
  import { setTheme, SUPPORTED_THEMES, theme, type Theme } from '../theme'
  import {
    BackendResponseError,
    checkUpdates,
    getGitHubWorkspaceStatus,
    linkGitHubWorkspace,
    pullGitHubWorkspace,
    pushGitHubWorkspace,
    setGitHubWorkspaceToken,
    unlinkGitHubWorkspace,
  } from '../wails/backend'

  type SettingsTab = 'shortcuts' | 'theme' | 'workspace' | 'updates' | 'about'

  export let isOpen: boolean
  export let appVersion = '0.0.0'
  export let appMetadata: AppMetadata | undefined
  export let initialTab: SettingsTab = 'theme'
  export let workspaceSettings: WorkspaceSettings | undefined
  export let workspaceSettingsAvailable = false
  export let workspaceSettingsPending = false
  export let workspaceSettingsError = ''
  export let workspaceSettingsInfo = ''
  export let workspaceId = ''
  export let workspaceName = ''

  const DEFAULT_MAX_EVENTS_PER_CALL = 10000
  const DEFAULT_MAX_BYTES_PER_CALL = 33554432
  const SETTINGS_NAV: { tab: SettingsTab; labelKey: string; icon: string }[] = [
    { tab: 'shortcuts', labelKey: 'settings.shortcutsTab', icon: '⌨' },
    { tab: 'theme', labelKey: 'settings.themeTab', icon: '☼' },
    { tab: 'workspace', labelKey: 'settings.workspaceTab', icon: '▦' },
    { tab: 'updates', labelKey: 'settings.updatesTab', icon: '⇩' },
    { tab: 'about', labelKey: 'settings.aboutTab', icon: 'ⓘ' },
  ]

  const dispatch = createEventDispatcher<{
    close: void
    workspaceSettingsSave: WorkspaceSettings
    workspaceSnapshot: WorkspaceSnapshot
  }>()

  let activeTab: SettingsTab = 'theme'
  let updatePending = false
  let updateResult: UpdateCheckResult | null = null
  let updateError = ''
  let autoCheckStarted = false
  let activeShortcutId = ''
  let shortcutOverrides = loadShortcutOverrides()
  let wasOpen = false
  let lastInitialTab: SettingsTab = initialTab
  let workspaceRedactDefaults = true
  let workspaceCustomSecretKeys = ''
  let workspaceMaxEventsPerCall = String(DEFAULT_MAX_EVENTS_PER_CALL)
  let workspaceMaxBytesPerCall = String(DEFAULT_MAX_BYTES_PER_CALL)
  let lastWorkspaceSettingsToken = ''
  let githubRepositoryUrl = ''
  let githubBranch = 'main'
  let githubWorkspacePath = ''
  let githubAccessToken = ''
  let githubUsesHTTPS = false
  let githubStatus: GitHubSyncStatus | null = null
  let githubPending = false
  let githubError = ''
  let githubInfo = ''
  let githubStatusToken = ''

  $: if (isOpen && !wasOpen) {
    activeTab = initialTab
    lastInitialTab = initialTab
    wasOpen = true
  }

  $: if (isOpen && initialTab !== lastInitialTab) {
    activeTab = initialTab
    lastInitialTab = initialTab
    activeShortcutId = ''
  }

  $: if (!isOpen && wasOpen) {
    activeTab = 'theme'
    autoCheckStarted = false
    activeShortcutId = ''
    wasOpen = false
  }

  $: workspaceSettingsToken = JSON.stringify(workspaceSettings ?? null)
  $: githubUsesHTTPS = /^https:\/\/github\.com\//i.test(githubRepositoryUrl.trim())
  $: if (isOpen && workspaceSettingsToken !== lastWorkspaceSettingsToken) {
    syncWorkspaceSettingsDraft(workspaceSettings)
    lastWorkspaceSettingsToken = workspaceSettingsToken
  }

  $: if (isOpen && activeTab === 'workspace' && workspaceSettingsAvailable) {
    const nextToken = `${workspaceId}:${workspaceSettingsToken}`
    if (nextToken !== githubStatusToken) {
      githubStatusToken = nextToken
      syncGitHubStatus()
    }
  }

  $: if (isOpen && activeTab === 'updates' && $appSettings.autoUpdateEnabled && !autoCheckStarted) {
    autoCheckStarted = true
    void handleCheckUpdates()
  }

  function handleLanguageChange(nextLanguage: Language): void {
    setLanguage(nextLanguage)
  }

  function handleThemeChange(nextTheme: Theme): void {
    setTheme(nextTheme)
  }

  function persistShortcutOverrides(nextOverrides: Record<string, string[]>): void {
    shortcutOverrides = nextOverrides
    writeShortcutOverrides(nextOverrides)
  }

  function shortcutKeys(shortcut: ShortcutDefinition): string[] {
    return resolveShortcutKeys(shortcut.id, shortcutOverrides)
  }

  function shortcutGroups(): string[] {
    return [...new Set(SHORTCUT_DEFAULTS.map((shortcut) => shortcut.group))]
  }

  function shortcutsForGroup(group: string): ShortcutDefinition[] {
    return SHORTCUT_DEFAULTS.filter((shortcut) => shortcut.group === group)
  }

  function resetShortcut(shortcutId: string): void {
    const nextOverrides = { ...shortcutOverrides }
    delete nextOverrides[shortcutId]
    persistShortcutOverrides(nextOverrides)
    activeShortcutId = ''
  }

  function resetAllShortcuts(): void {
    persistShortcutOverrides({})
    activeShortcutId = ''
  }

  function isEditableTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) {
      return false
    }

    return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
  }

  function handleShortcutRecorderKeydown(event: KeyboardEvent): void {
    if (!activeShortcutId) {
      return
    }

    event.preventDefault()
    event.stopPropagation()

    if (event.key === 'Escape') {
      activeShortcutId = ''
      return
    }

    const keys = shortcutFromEvent(event)

    if (keys.length === 0 || isModifierOnlyShortcutEvent(event)) {
      return
    }

    persistShortcutOverrides({
      ...shortcutOverrides,
      [activeShortcutId]: keys,
    })
    activeShortcutId = ''
  }

  function selectSettingsTabByOffset(offset: 1 | -1): void {
    const currentIndex = SETTINGS_NAV.findIndex((item) => item.tab === activeTab)
    const baseIndex = currentIndex >= 0 ? currentIndex : 0
    const nextIndex = (baseIndex + offset + SETTINGS_NAV.length) % SETTINGS_NAV.length
    activeTab = SETTINGS_NAV[nextIndex].tab
  }

  function handleSettingsKeydown(event: KeyboardEvent): void {
    if (!isOpen || event.defaultPrevented) {
      return
    }

    if (activeShortcutId) {
      handleShortcutRecorderKeydown(event)
      return
    }

    if (isEditableTarget(event.target)) {
      return
    }

    if (event.key === 'Escape') {
      event.preventDefault()
      dispatch('close')
      return
    }

    if (isShortcutEvent(event, 'settings.nextTab')) {
      event.preventDefault()
      selectSettingsTabByOffset(1)
      return
    }

    if (isShortcutEvent(event, 'settings.previousTab')) {
      event.preventDefault()
      selectSettingsTabByOffset(-1)
      return
    }

  }

  function syncWorkspaceSettingsDraft(settings: WorkspaceSettings | undefined): void {
    workspaceRedactDefaults = settings?.redactDefaults ?? true
    workspaceCustomSecretKeys = (settings?.customSecretKeys ?? []).join('\n')
    workspaceMaxEventsPerCall = String(
      settings?.eventRetention?.maxEventsPerCall ?? DEFAULT_MAX_EVENTS_PER_CALL,
    )
    workspaceMaxBytesPerCall = String(
      settings?.eventRetention?.maxBytesPerCall ?? DEFAULT_MAX_BYTES_PER_CALL,
    )
  }

  function positiveInteger(value: string): number | undefined {
    const parsed = Number(value)
    return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : undefined
  }

  function workspaceSettingsDraft(): WorkspaceSettings {
    const customSecretKeys = workspaceCustomSecretKeys
      .split(/[\n,]/)
      .map((key) => key.trim())
      .filter(Boolean)

    return {
      redactDefaults: workspaceRedactDefaults,
      customSecretKeys,
      eventRetention: {
        maxEventsPerCall: positiveInteger(workspaceMaxEventsPerCall),
        maxBytesPerCall: positiveInteger(workspaceMaxBytesPerCall),
      },
    }
  }

  function handleWorkspaceSettingsSave(): void {
    dispatch('workspaceSettingsSave', workspaceSettingsDraft())
  }

  function defaultGitHubWorkspacePath(): string {
    const slug = workspaceName
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9_-]+/g, '-')
      .replace(/^-+|-+$/g, '')
    return `catenar/${slug || 'default'}`
  }

  function githubSyncDirectionHint(): string {
    if (!githubStatus?.linked) {
      return ''
    }

    if (githubStatus.localChanges && githubStatus.remoteChanges) {
      return $i18n.t('settings.githubFlowBothChanged')
    }

    if (githubStatus.remoteChanges) {
      return $i18n.t('settings.githubFlowPull')
    }

    if (githubStatus.localChanges) {
      return $i18n.t('settings.githubFlowPush')
    }

    return $i18n.t('settings.githubFlowCurrent')
  }

  async function syncGitHubStatus(showFeedback = false): Promise<void> {
    if (!workspaceSettingsAvailable) {
      githubStatus = null
      return
    }
    githubPending = true
    githubError = ''
    try {
      const status = await getGitHubWorkspaceStatus()
      githubStatus = status
      if (status.link) {
        githubRepositoryUrl = status.link.repositoryUrl
        githubBranch = status.link.branch
        githubWorkspacePath = status.link.workspacePath
      } else {
        githubRepositoryUrl = ''
        githubBranch = 'main'
        githubWorkspacePath = defaultGitHubWorkspacePath()
      }
      if (showFeedback) {
        githubInfo = $i18n.t('settings.githubStatusRefreshed')
      }
    } catch (error) {
      githubError = localizedGitHubError(error)
    } finally {
      githubPending = false
    }
  }

  async function handleGitHubLink(): Promise<void> {
    githubPending = true
    githubError = ''
    githubInfo = ''
    try {
      const input: GitHubWorkspaceLinkInput = {
        repositoryUrl: githubRepositoryUrl,
        branch: githubBranch,
        workspacePath: githubWorkspacePath || defaultGitHubWorkspacePath(),
        accessToken: githubAccessToken.trim() || undefined,
      }
      githubStatus = await linkGitHubWorkspace(input)
      githubAccessToken = ''
      githubInfo = $i18n.t('settings.githubLinked')
      githubStatusToken = `${workspaceId}:${workspaceSettingsToken}:${Date.now()}`
    } catch (error) {
      githubError = localizedGitHubError(error)
    } finally {
      githubPending = false
    }
  }

  async function handleGitHubTokenSave(): Promise<void> {
    githubPending = true
    githubError = ''
    githubInfo = ''
    try {
      githubStatus = await setGitHubWorkspaceToken({ accessToken: githubAccessToken.trim() })
      githubAccessToken = ''
      githubInfo = $i18n.t('settings.githubTokenSaved')
    } catch (error) {
      githubError = localizedGitHubError(error)
    } finally {
      githubPending = false
    }
  }

  async function handleGitHubPull(): Promise<void> {
    await handleGitHubAction('pull')
  }

  async function handleGitHubPush(): Promise<void> {
    const requestedPath = githubWorkspacePath.trim() || defaultGitHubWorkspacePath()
    if (githubStatus?.linked && githubStatus.link?.workspacePath !== requestedPath) {
      await handleGitHubLink()
      if (githubStatus?.link?.workspacePath !== requestedPath) {
        return
      }
    }
    await handleGitHubAction('push')
  }

  async function handleGitHubAction(action: 'pull' | 'push'): Promise<void> {
    const overwrite = githubStatus?.conflict || githubStatus?.initialSyncRequired
    githubPending = true
    githubError = ''
    githubInfo = ''
    const input: GitHubSyncActionInput = { overwrite: Boolean(overwrite) }
    try {
      githubStatus = action === 'pull'
        ? await pullGitHubWorkspace(input)
        : await pushGitHubWorkspace(input)
      githubInfo = $i18n.t(action === 'pull' ? 'settings.githubPulled' : 'settings.githubPushed')
      if (githubStatus.workspace) {
        dispatch('workspaceSnapshot', githubStatus.workspace)
      }
    } catch (error) {
      githubError = localizedGitHubError(error)
    } finally {
      githubPending = false
    }
  }

  async function handleGitHubUnlink(): Promise<void> {
    githubPending = true
    githubError = ''
    githubInfo = ''
    try {
      githubStatus = await unlinkGitHubWorkspace()
      githubInfo = $i18n.t('settings.githubUnlinked')
    } catch (error) {
      githubError = localizedGitHubError(error)
    } finally {
      githubPending = false
    }
  }

  function localizedGitHubError(error: unknown): string {
    if (error instanceof BackendResponseError) {
      const key = `settings.githubError.${error.code ?? ''}`
      const translated = $i18n.t(key)
      const message = translated === key ? error.backendMessage : translated
      const cause = error.details?.cause?.trim()
      if (cause && /invalid username or token|password authentication is not supported/i.test(cause)) {
        return `${$i18n.t('settings.githubAuthFailed')} ${$i18n.t('settings.githubAuthHelp')}`
      }
      return cause ? `${message} (${cause})` : message
    }
    return error instanceof Error ? error.message : $i18n.t('settings.githubGenericError')
  }

  async function handleCheckUpdates(): Promise<void> {
    updatePending = true
    updateError = ''

    try {
      updateResult = await checkUpdates()
    } catch (error) {
      updateError = localizedUpdateError(error)
    } finally {
      updatePending = false
    }
  }

  function localizedUpdateError(error: unknown): string {
    if (error instanceof BackendResponseError) {
      if (error.code === 'application.update_repo_missing') {
        return $i18n.t('settings.updateRepoMissing')
      }
      if (
        error.code === 'application.update_unavailable' ||
        error.code === 'application.update_unreachable'
      ) {
        return $i18n.t('settings.updateUnavailable')
      }
      if (
        error.code === 'application.update_decode_failed' ||
        error.code === 'application.update_version_missing'
      ) {
        return $i18n.t('settings.updateUnreadable')
      }

      return error.backendMessage
    }

    return error instanceof Error ? error.message : $i18n.t('settings.updateCheckFailed')
  }

  function platformLabel(platform: string | undefined): string {
    const normalized = (platform ?? '').toLowerCase()
    if (normalized === 'darwin' || normalized === 'macos') {
      return 'macOS'
    }
    if (normalized === 'windows' || normalized === 'win32') {
      return 'Windows'
    }
    if (normalized === 'linux') {
      return 'Linux'
    }

    return platform || 'macOS'
  }

  function openExternalUrl(url: string): void {
    BrowserOpenURL(url)
  }
</script>

<svelte:window on:keydown={handleSettingsKeydown} />

{#if isOpen}
  <button aria-label={$i18n.t('common.close')} class="overlay-backdrop" on:click={() => dispatch('close')} type="button"></button>
  <div aria-label={$i18n.t('settings.title')} class="overlay-panel overlay-panel--settings" role="dialog" aria-modal="true">
    <div class="settings-modal__header">
      <h2 class="settings-modal__title">{$i18n.t('settings.title')}</h2>
      <button class="icon-close-button" on:click={() => dispatch('close')} type="button">
        <span aria-hidden="true">×</span>
      </button>
    </div>

    <div class="settings-modal__body">
      <aside class="settings-sidebar" aria-label={$i18n.t('settings.title')}>
        {#each SETTINGS_NAV as item}
          <button
            class:settings-sidebar__button--active={activeTab === item.tab}
            class="settings-sidebar__button"
            on:click={() => (activeTab = item.tab)}
            type="button"
          >
            <span aria-hidden="true">{item.icon}</span>
            {$i18n.t(item.labelKey)}
          </button>
        {/each}
      </aside>

      <div class="settings-modal__content">
      {#if activeTab === 'theme'}
        <section class="settings-group">
          <div class="settings-group__title">
            <span class="settings-group__icon" aria-hidden="true">◔</span>
            <span>{$i18n.t('settings.appearanceTitle')}</span>
          </div>
          <div class="settings-option-grid settings-option-grid--compact">
            {#each SUPPORTED_THEMES as supportedTheme}
              <button
                class:selected-option={$theme === supportedTheme}
                class="settings-option-card"
                on:click={() => handleThemeChange(supportedTheme)}
                type="button"
              >
                <div class="settings-option-card__icon" aria-hidden="true">
                  {supportedTheme === 'dark' ? '☾' : '◌'}
                </div>
                <div class="settings-option-card__label">
                  {$i18n.t(supportedTheme === 'dark' ? 'settings.themeDark' : 'settings.themeLight')}
                </div>
              </button>
            {/each}
          </div>
        </section>

        <section class="settings-group">
          <div class="settings-group__title">
            <span class="settings-group__icon" aria-hidden="true">◍</span>
            <span>{$i18n.t('settings.languageTitle')}</span>
          </div>
          <div class="settings-option-grid settings-option-grid--compact">
            {#each SUPPORTED_LANGUAGES as supportedLanguage}
              <button
                class:selected-option={$language === supportedLanguage}
                class="settings-option-card"
                on:click={() => handleLanguageChange(supportedLanguage)}
                type="button"
              >
                <div class="settings-option-card__icon settings-option-card__icon--flag" aria-hidden="true">
                  {supportedLanguage === 'ru' ? '🇷🇺' : '🇬🇧'}
                </div>
                <div class="settings-option-card__label">
                  {supportedLanguage === 'ru' ? $i18n.t('common.languageRu') : $i18n.t('common.languageEn')}
                </div>
              </button>
            {/each}
          </div>
        </section>

      {:else if activeTab === 'workspace'}
        <section class="settings-group">
          <div class="settings-section-label">{$i18n.t('settings.workspaceRuntimeTitle')}</div>
          <p class="settings-help-copy">{$i18n.t('settings.workspaceHelp')}</p>
          <p class="settings-help-copy">{$i18n.t('settings.workspaceHelpDetail')}</p>
          {#if !workspaceSettingsAvailable}
            <div class="inline-warning" role="status">{$i18n.t('settings.workspaceSettingsUnavailable')}</div>
          {/if}
          {#if workspaceSettingsError}
            <div class="inline-error" role="alert">{workspaceSettingsError}</div>
          {/if}
          {#if workspaceSettingsInfo}
            <div class="inline-success" role="status">{workspaceSettingsInfo}</div>
          {/if}
          <label class="switch-row switch-row--settings">
            <input
              bind:checked={workspaceRedactDefaults}
              disabled={!workspaceSettingsAvailable || workspaceSettingsPending}
              type="checkbox"
            />
            {$i18n.t('settings.workspaceRedactDefaults')}
          </label>
          <label class="settings-textarea-field">
            <span>{$i18n.t('settings.workspaceCustomSecretKeys')}</span>
            <textarea
              bind:value={workspaceCustomSecretKeys}
              disabled={!workspaceSettingsAvailable || workspaceSettingsPending}
              placeholder={$i18n.t('settings.workspaceCustomSecretKeysPlaceholder')}
              rows="4"
            ></textarea>
          </label>
        </section>

        <section class="settings-group">
          <div class="settings-section-label">{$i18n.t('settings.workspaceRetentionTitle')}</div>
          <p class="settings-help-copy">{$i18n.t('settings.workspaceRetentionHelp')}</p>
          <div class="settings-control-grid">
            <label class="settings-number-field">
              <span>{$i18n.t('settings.workspaceMaxEvents')}</span>
              <input
                bind:value={workspaceMaxEventsPerCall}
                disabled={!workspaceSettingsAvailable || workspaceSettingsPending}
                min="1"
                step="1"
                type="number"
              />
            </label>
            <label class="settings-number-field">
              <span>{$i18n.t('settings.workspaceMaxBytes')}</span>
              <input
                bind:value={workspaceMaxBytesPerCall}
                disabled={!workspaceSettingsAvailable || workspaceSettingsPending}
                min="1"
                step="1"
                type="number"
              />
            </label>
          </div>
          <button
            class="settings-wide-command"
            disabled={!workspaceSettingsAvailable || workspaceSettingsPending}
            on:click={handleWorkspaceSettingsSave}
            type="button"
          >
            {workspaceSettingsPending ? $i18n.t('workspace.pending') : $i18n.t('settings.workspaceSaveSettings')}
          </button>
        </section>

        <section class="settings-group settings-github">
          <div class="settings-section-label">{$i18n.t('settings.githubTitle')}</div>
          <p class="settings-help-copy">{$i18n.t('settings.githubHelp')}</p>
          {#if githubError}
            <div class="inline-error" role="alert">{githubError}</div>
          {/if}
          {#if githubInfo}
            <div class="inline-success" role="status">{githubInfo}</div>
          {/if}

          <div class="settings-github__fields">
            <label class="settings-text-field settings-text-field--wide">
              <span>{$i18n.t('settings.githubRepository')}</span>
              <input
                bind:value={githubRepositoryUrl}
                disabled={!workspaceSettingsAvailable || githubPending || githubStatus?.linked}
                placeholder="git@github.com:owner/catenar-workspaces.git"
                spellcheck="false"
                type="text"
              />
            </label>
            <label class="settings-text-field">
              <span>{$i18n.t('settings.githubBranch')}</span>
              <input
                bind:value={githubBranch}
                disabled={!workspaceSettingsAvailable || githubPending || githubStatus?.linked}
                placeholder="main"
                spellcheck="false"
                type="text"
              />
            </label>
            <label class="settings-text-field">
              <span>{$i18n.t('settings.githubWorkspacePath')}</span>
              <input
                bind:value={githubWorkspacePath}
                disabled={!workspaceSettingsAvailable || githubPending}
                placeholder="catenar/my-project"
                spellcheck="false"
                type="text"
              />
            </label>
          </div>

          {#if githubUsesHTTPS}
            <section class="settings-github__authentication" aria-label={$i18n.t('settings.githubToken')}>
              <label class="settings-text-field">
                <span>{$i18n.t('settings.githubToken')}</span>
                <input
                  bind:value={githubAccessToken}
                  autocomplete="off"
                  disabled={!workspaceSettingsAvailable || githubPending}
                  placeholder={$i18n.t('settings.githubTokenPlaceholder')}
                  spellcheck="false"
                  type="password"
                />
                <small>{$i18n.t('settings.githubTokenHelp')}</small>
              </label>
              <div class="settings-github__authentication-actions">
                <button class="settings-inline-link" on:click={() => BrowserOpenURL('https://github.com/settings/personal-access-tokens/new')} type="button">
                  {$i18n.t('settings.githubCreateToken')}
                </button>
                {#if githubStatus?.linked}
                  <span class:settings-github__token-state--configured={githubStatus.tokenConfigured} class="settings-github__token-state">
                    {$i18n.t(githubStatus.tokenConfigured ? 'settings.githubTokenConfigured' : 'settings.githubTokenMissing')}
                  </span>
                {/if}
              </div>
            </section>
          {/if}

          {#if githubStatus?.linked}
            <div class="settings-github__status" role="status">
              <span class:settings-github__status-chip--active={githubStatus.localChanges} class="settings-github__status-chip">
                {$i18n.t(githubStatus.localChanges ? 'settings.githubLocalChanged' : 'settings.githubLocalCurrent')}
              </span>
              <span class:settings-github__status-chip--active={githubStatus.remoteChanges} class="settings-github__status-chip">
                {$i18n.t(githubStatus.remoteChanges ? 'settings.githubRemoteChanged' : 'settings.githubRemoteCurrent')}
              </span>
              {#if githubStatus.conflict || githubStatus.initialSyncRequired}
                <span class="settings-github__status-chip settings-github__status-chip--warning">
                  {$i18n.t('settings.githubDirectionRequired')}
                </span>
              {/if}
            </div>
            <p class="settings-github__flow" role="status">{githubSyncDirectionHint()}</p>
            <div class="settings-github__actions">
              {#if githubUsesHTTPS}
                <button
                  class="secondary-button"
                  disabled={githubPending || !githubAccessToken.trim()}
                  on:click={handleGitHubTokenSave}
                  type="button"
                >
                  {githubPending ? $i18n.t('workspace.pending') : $i18n.t(githubStatus.tokenConfigured ? 'settings.githubTokenReplace' : 'settings.githubTokenSave')}
                </button>
              {/if}
              <button
                class="secondary-button"
                disabled={githubPending || (!githubStatus.remoteChanges && !githubStatus.conflict)}
                on:click={handleGitHubPull}
                type="button"
              >
                {githubPending ? $i18n.t('workspace.pending') : $i18n.t('settings.githubPull')}
              </button>
              <button
                class="primary-button"
                disabled={githubPending || (!githubStatus.localChanges && !githubStatus.conflict)}
                on:click={handleGitHubPush}
                type="button"
              >
                {githubPending ? $i18n.t('workspace.pending') : $i18n.t('settings.githubPush')}
              </button>
              <button
                class="secondary-button"
                disabled={githubPending}
                on:click={() => void syncGitHubStatus(true)}
                title={$i18n.t('settings.githubRefreshHint')}
                type="button"
              >
                {$i18n.t('settings.githubRefresh')}
              </button>
              <button class="danger-text-button" disabled={githubPending} on:click={handleGitHubUnlink} type="button">
                {$i18n.t('settings.githubUnlink')}
              </button>
            </div>
          {:else}
            <button
              class="settings-wide-command"
              disabled={!workspaceSettingsAvailable || githubPending || !githubRepositoryUrl.trim()}
              on:click={handleGitHubLink}
              type="button"
            >
              {githubPending ? $i18n.t('workspace.pending') : $i18n.t('settings.githubLink')}
            </button>
          {/if}
        </section>
      {:else if activeTab === 'updates'}
        <section class="settings-group">
          <div class="settings-version-row">
            <span>{$i18n.t('settings.currentVersion')}</span>
            <code>{appVersion}</code>
          </div>
          <button class="settings-wide-command" disabled={updatePending} on:click={handleCheckUpdates} type="button">
            {updatePending ? $i18n.t('settings.updateChecking') : $i18n.t('settings.checkUpdates')}
          </button>
          <label class="switch-row switch-row--settings">
            <input
              checked={$appSettings.autoUpdateEnabled}
              type="checkbox"
              on:change={(event) => setAutoUpdateEnabled((event.currentTarget as HTMLInputElement).checked)}
            />
            {$i18n.t('settings.autoUpdate')}
          </label>

          {#if updateError}
            <div class="inline-error" role="alert">{updateError}</div>
          {/if}
          {#if updateResult}
            <div class:settings-update-result--available={updateResult.updateAvailable} class="settings-update-result" role="status">
              <strong>
                {updateResult.updateAvailable ? $i18n.t('settings.updateAvailable') : $i18n.t('settings.updateCurrent')}
              </strong>
              <span>{$i18n.t('settings.latestVersion', { version: updateResult.latestVersion })}</span>
              {#if updateResult.downloadUrl}
                <button class="settings-inline-link" on:click={() => openExternalUrl(updateResult?.downloadUrl ?? '')} type="button">
                  {$i18n.t('settings.downloadUpdate')}
                </button>
              {:else if updateResult.releaseUrl}
                <button class="settings-inline-link" on:click={() => openExternalUrl(updateResult?.releaseUrl ?? '')} type="button">
                  {$i18n.t('settings.openRelease')}
                </button>
              {/if}
            </div>
          {/if}
        </section>
      {:else if activeTab === 'about'}
        <section class="settings-about">
          <img class="settings-about__mark" src="/app-icon.png" alt="" aria-hidden="true" />
          <h3>Catenar</h3>
          <dl class="settings-about__meta">
            <div>
              <dt>{$i18n.t('settings.aboutVersion')}</dt>
              <dd>{appMetadata?.version ?? appVersion}</dd>
            </div>
            <div>
              <dt>{$i18n.t('settings.aboutPlatform')}</dt>
              <dd>{platformLabel(appMetadata?.platform)}</dd>
            </div>
          </dl>
          <div class="settings-about__contact">
            <span>{$i18n.t('settings.aboutQuestions')}</span>
            <button class="settings-about__link" on:click={() => openExternalUrl('https://t.me/Rammbloor')} type="button">
              @Rammbloor ↗
            </button>
          </div>
        </section>
      {:else}
        <section class="settings-group">
          <div class="settings-shortcut-head">
            <span>{$i18n.t('settings.shortcutsHint')}</span>
            <button class="settings-reset-button" on:click={resetAllShortcuts} type="button">
              {$i18n.t('settings.shortcutsResetAll')}
            </button>
          </div>
          <div class="settings-shortcut-sections">
            {#each shortcutGroups() as group}
              <section class="settings-shortcut-section">
                <h3>{$i18n.t(group)}</h3>
                <div class="settings-shortcut-list">
                  {#each shortcutsForGroup(group) as shortcut}
                    <div
                      class:settings-shortcut-row--recording={activeShortcutId === shortcut.id}
                      class="settings-shortcut-row"
                    >
                      <button
                        class="settings-shortcut-row__label"
                        data-action-hint={activeShortcutId === shortcut.id
                          ? $i18n.t('settings.shortcutsRecordingHint')
                          : $i18n.t('settings.shortcutsApplyHint')}
                        on:click={() => (activeShortcutId = shortcut.id)}
                        type="button"
                      >
                        {activeShortcutId === shortcut.id ? $i18n.t('settings.shortcutsRecording') : $i18n.t(shortcut.label)}
                      </button>
                      <span class="settings-shortcut-row__keys">
                        {#each shortcutKeys(shortcut) as key}
                          <kbd>{key}</kbd>
                        {/each}
                        <button
                          aria-label={$i18n.t('settings.shortcutsReset')}
                          class="settings-shortcut-row__reset"
                          on:click|stopPropagation={() => resetShortcut(shortcut.id)}
                          type="button"
                        >
                          ↻
                        </button>
                      </span>
                    </div>
                  {/each}
                </div>
              </section>
            {/each}
          </div>
        </section>
      {/if}
      </div>
    </div>
  </div>
{/if}
