import { writable } from 'svelte/store'

export const APP_SETTINGS_STORAGE_KEY = 'catenar.appSettings'
const LEGACY_APP_SETTINGS_STORAGE_KEY = 'tether.appSettings'

export interface AppSettings {
  autoUpdateEnabled: boolean
}

export const DEFAULT_SETTINGS: AppSettings = {
  autoUpdateEnabled: true,
}

function browserStorage(): Storage | undefined {
  if (typeof window === 'undefined') {
    return undefined
  }

  return window.localStorage
}

export function parseStoredSettings(value: string | null | undefined): AppSettings {
  if (!value) {
    return DEFAULT_SETTINGS
  }

  try {
    const parsed = JSON.parse(value) as Partial<AppSettings>
    return {
      autoUpdateEnabled:
        typeof parsed.autoUpdateEnabled === 'boolean'
          ? parsed.autoUpdateEnabled
          : DEFAULT_SETTINGS.autoUpdateEnabled,
    }
  } catch {
    return DEFAULT_SETTINGS
  }
}

function createAppSettingsStore() {
  const storage = browserStorage()
  const initialSettings = parseStoredSettings(
    storage?.getItem(APP_SETTINGS_STORAGE_KEY) ?? storage?.getItem(LEGACY_APP_SETTINGS_STORAGE_KEY),
  )
  const { subscribe, set } = writable<AppSettings>(initialSettings)

  function persist(nextSettings: AppSettings): void {
    browserStorage()?.setItem(APP_SETTINGS_STORAGE_KEY, JSON.stringify(nextSettings))
    set(nextSettings)
  }

  return {
    subscribe,
    update(patch: Partial<AppSettings>) {
      const current = parseStoredSettings(
        browserStorage()?.getItem(APP_SETTINGS_STORAGE_KEY) ?? browserStorage()?.getItem(LEGACY_APP_SETTINGS_STORAGE_KEY),
      )
      persist(parseStoredSettings(JSON.stringify({ ...current, ...patch })))
    },
  }
}

export const appSettings = createAppSettingsStore()

export function setAutoUpdateEnabled(autoUpdateEnabled: boolean): void {
  appSettings.update({ autoUpdateEnabled })
}
