import { describe, expect, it } from 'vitest'

import { DEFAULT_SETTINGS, parseStoredSettings } from './app-settings'

describe('app settings', () => {
  it('uses defaults when settings are missing or unreadable', () => {
    expect(parseStoredSettings(null)).toEqual(DEFAULT_SETTINGS)
    expect(parseStoredSettings('{nope')).toEqual(DEFAULT_SETTINGS)
  })

  it('keeps only the app-level settings that still exist', () => {
    expect(
      parseStoredSettings(
        JSON.stringify({
          autoUpdateEnabled: false,
          legacyEditorFont: 'fira-code',
          legacyEditorFontSize: 16,
          workspaceThemeEnabled: true,
        }),
      ),
    ).toEqual({ autoUpdateEnabled: false })
  })
})
