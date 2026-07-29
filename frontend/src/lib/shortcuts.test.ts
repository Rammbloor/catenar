import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  isModifierOnlyShortcutEvent,
  isShortcutEvent,
  persistShortcutOverrides,
  shortcutFromEvent,
  SHORTCUT_DEFAULTS,
  SHORTCUT_STORAGE_KEY,
} from './shortcuts'

function createMemoryStorage(): Storage {
  const values = new Map<string, string>()
  return {
    get length() {
      return values.size
    },
    clear: () => values.clear(),
    getItem: (key: string) => values.get(key) ?? null,
    key: (index: number) => Array.from(values.keys())[index] ?? null,
    removeItem: (key: string) => {
      values.delete(key)
    },
    setItem: (key: string, value: string) => {
      values.set(key, value)
    },
  }
}

function keydown(key: string, init: Partial<KeyboardEventInit> = {}): KeyboardEvent {
  return {
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    shiftKey: false,
    key,
    ...init,
  } as KeyboardEvent
}

describe('shortcuts', () => {
  beforeEach(() => {
    vi.stubGlobal('window', {
      localStorage: createMemoryStorage(),
    })
  })

  afterEach(() => {
    window.localStorage.removeItem(SHORTCUT_STORAGE_KEY)
    vi.unstubAllGlobals()
  })

  it('matches default application shortcuts', () => {
    expect(isShortcutEvent(keydown(',', { metaKey: true }), 'window.settings')).toBe(true)
    expect(isShortcutEvent(keydown('Enter', { metaKey: true }), 'request.send')).toBe(true)
    expect(isShortcutEvent(keydown('s', { metaKey: true }), 'request.save')).toBe(true)
    expect(isShortcutEvent(keydown('F', { metaKey: true, shiftKey: true }), 'request.formatJson')).toBe(true)
    expect(isShortcutEvent(keydown('O', { metaKey: true }), 'workspace.open')).toBe(true)
    expect(isShortcutEvent(keydown('S', { metaKey: true, shiftKey: true }), 'workspace.save')).toBe(true)
    expect(isShortcutEvent(keydown('V', { metaKey: true, shiftKey: true }), 'workspace.validate')).toBe(true)
    expect(isShortcutEvent(keydown('W', { metaKey: true, altKey: true }), 'workspace.close')).toBe(true)
    expect(isShortcutEvent(keydown('1', { metaKey: true }), 'view.client')).toBe(true)
    expect(isShortcutEvent(keydown('2', { metaKey: true }), 'view.traffic')).toBe(true)
    expect(isShortcutEvent(keydown('ArrowDown'), 'settings.nextTab')).toBe(true)
    expect(isShortcutEvent(keydown('ArrowUp'), 'settings.previousTab')).toBe(true)
  })

  it('uses stored shortcut overrides', () => {
    persistShortcutOverrides({
      'window.settings': ['⌘', 'O'],
    })

    expect(isShortcutEvent(keydown('O', { metaKey: true }), 'window.settings')).toBe(true)
    expect(isShortcutEvent(keydown(',', { metaKey: true }), 'window.settings')).toBe(false)
  })

  it('formats keyboard events into display keys', () => {
    expect(shortcutFromEvent(keydown('ArrowDown'))).toEqual(['↓'])
    expect(shortcutFromEvent(keydown('F', { metaKey: true, shiftKey: true }))).toEqual(['⇧', '⌘', 'F'])
    expect(shortcutFromEvent(keydown('Backspace', { altKey: true }))).toEqual(['⌥', '⌫'])
    expect(isModifierOnlyShortcutEvent(keydown('Meta', { metaKey: true }))).toBe(true)
  })

  it('does not expose hidden connection clipboard shortcuts', () => {
    expect(SHORTCUT_DEFAULTS.some((shortcut) => shortcut.id === 'sidebar.copy')).toBe(false)
    expect(SHORTCUT_DEFAULTS.some((shortcut) => shortcut.id === 'sidebar.paste')).toBe(false)
    expect(SHORTCUT_DEFAULTS.some((shortcut) => shortcut.id === 'sidebar.duplicate')).toBe(false)
  })
})
