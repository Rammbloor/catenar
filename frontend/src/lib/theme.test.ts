import { afterEach, describe, expect, it, vi } from 'vitest'

describe('theme store', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.resetModules()
  })

  it('applies and persists the selected theme on the document root', async () => {
    const values = new Map<string, string>()
    const dataset: Record<string, string> = {}
    const style: Record<string, string> = {}

    vi.stubGlobal('window', {
      localStorage: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, value),
      },
    })
    vi.stubGlobal('document', {
      documentElement: { dataset, style },
    })

    const { setTheme, THEME_STORAGE_KEY } = await import('./theme')
    setTheme('light')

    expect(dataset.theme).toBe('light')
    expect(style.colorScheme).toBe('light')
    expect(values.get(THEME_STORAGE_KEY)).toBe('light')
  })
})
