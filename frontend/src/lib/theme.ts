import { writable } from 'svelte/store'

export const THEME_STORAGE_KEY = 'catenar.theme'
const LEGACY_THEME_STORAGE_KEY = 'tether.theme'
export const SUPPORTED_THEMES = ['dark', 'light'] as const

export type Theme = (typeof SUPPORTED_THEMES)[number]

function isSupportedTheme(value: string | null | undefined): value is Theme {
  return SUPPORTED_THEMES.some((theme) => theme === value)
}

function browserStorage(): Storage | undefined {
  if (typeof window === 'undefined') {
    return undefined
  }

  return window.localStorage
}

function applyTheme(theme: Theme): void {
  if (typeof document === 'undefined') {
    return
  }

  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
}

function resolveInitialTheme(): Theme {
  const storage = browserStorage()
  const stored = storage?.getItem(THEME_STORAGE_KEY) ?? storage?.getItem(LEGACY_THEME_STORAGE_KEY)
  if (isSupportedTheme(stored)) {
    return stored
  }

  return 'dark'
}

function createThemeStore() {
  const initialTheme = resolveInitialTheme()
  applyTheme(initialTheme)

  const { subscribe, set } = writable<Theme>(initialTheme)

  return {
    subscribe,
    set(theme: Theme) {
      browserStorage()?.setItem(THEME_STORAGE_KEY, theme)
      applyTheme(theme)
      set(theme)
    },
  }
}

export const theme = createThemeStore()

export function setTheme(nextTheme: Theme): void {
  theme.set(nextTheme)
}
