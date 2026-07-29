export type ShortcutDefinition = {
  id: string
  group: string
  label: string
  keys: string[]
}

export const SHORTCUT_STORAGE_KEY = 'catenar.shortcuts'
const LEGACY_SHORTCUT_STORAGE_KEY = 'tether.shortcuts'

export const SHORTCUT_DEFAULTS: ShortcutDefinition[] = [
  { id: 'view.client', group: 'settings.shortcuts.groupViews', label: 'settings.shortcutViewClient', keys: ['⌘', '1'] },
  { id: 'view.traffic', group: 'settings.shortcuts.groupViews', label: 'settings.shortcutViewTraffic', keys: ['⌘', '2'] },
  { id: 'sidebar.search', group: 'settings.shortcuts.groupSidebar', label: 'settings.shortcutSearchSidebar', keys: ['⌘', 'F'] },
  { id: 'sidebar.next', group: 'settings.shortcuts.groupSidebar', label: 'settings.shortcutNextItem', keys: ['↓'] },
  { id: 'sidebar.previous', group: 'settings.shortcuts.groupSidebar', label: 'settings.shortcutPreviousItem', keys: ['↑'] },
  { id: 'sidebar.rename', group: 'settings.shortcuts.groupSidebar', label: 'settings.shortcutRenameItem', keys: ['⌘', 'E'] },
  { id: 'sidebar.delete', group: 'settings.shortcuts.groupSidebar', label: 'settings.shortcutDeleteItem', keys: ['⌫'] },
  { id: 'request.send', group: 'settings.shortcuts.groupRequest', label: 'settings.shortcutInvoke', keys: ['⌘', '↵'] },
  { id: 'request.save', group: 'settings.shortcuts.groupRequest', label: 'settings.shortcutSaveRequest', keys: ['⌘', 'S'] },
  { id: 'request.formatJson', group: 'settings.shortcuts.groupRequest', label: 'settings.shortcutFormatJson', keys: ['⇧', '⌘', 'F'] },
  { id: 'workspace.open', group: 'settings.shortcuts.groupWorkspace', label: 'settings.shortcutOpenWorkspace', keys: ['⌘', 'O'] },
  { id: 'workspace.save', group: 'settings.shortcuts.groupWorkspace', label: 'settings.shortcutSaveWorkspace', keys: ['⇧', '⌘', 'S'] },
  { id: 'workspace.validate', group: 'settings.shortcuts.groupWorkspace', label: 'settings.shortcutValidateWorkspace', keys: ['⇧', '⌘', 'V'] },
  { id: 'workspace.close', group: 'settings.shortcuts.groupWorkspace', label: 'settings.shortcutCloseWorkspace', keys: ['⌥', '⌘', 'W'] },
  { id: 'window.new', group: 'settings.shortcuts.groupWindows', label: 'settings.shortcutNew', keys: ['⌘', 'N'] },
  { id: 'window.settings', group: 'settings.shortcuts.groupWindows', label: 'settings.title', keys: ['⌘', ','] },
  { id: 'window.help', group: 'settings.shortcuts.groupWindows', label: 'settings.shortcutHelp', keys: ['⌘', '/'] },
  { id: 'settings.nextTab', group: 'settings.shortcuts.groupSettings', label: 'settings.shortcutNextSettingsTab', keys: ['↓'] },
  { id: 'settings.previousTab', group: 'settings.shortcuts.groupSettings', label: 'settings.shortcutPreviousSettingsTab', keys: ['↑'] },
  { id: 'interface.left', group: 'settings.shortcuts.groupInterface', label: 'settings.shortcutToggleLeftSidebar', keys: ['⌘', '\\'] },
  { id: 'interface.right', group: 'settings.shortcuts.groupInterface', label: 'settings.shortcutToggleRightSidebar', keys: ['⌥', '⌘', '\\'] },
]

const MODIFIER_KEYS = ['Control', 'Meta', 'Alt', 'Shift']

export function loadShortcutOverrides(): Record<string, string[]> {
  if (typeof window === 'undefined') {
    return {}
  }

  try {
    const stored = window.localStorage.getItem(SHORTCUT_STORAGE_KEY) ?? window.localStorage.getItem(LEGACY_SHORTCUT_STORAGE_KEY)
    const parsed = JSON.parse(stored ?? '{}') as Record<string, string[]>
    return Object.fromEntries(Object.entries(parsed).filter(([, keys]) => Array.isArray(keys) && keys.length > 0))
  } catch {
    return {}
  }
}

export function persistShortcutOverrides(nextOverrides: Record<string, string[]>): void {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(SHORTCUT_STORAGE_KEY, JSON.stringify(nextOverrides))
  }
}

export function shortcutDefaultsById(): Record<string, string[]> {
  return Object.fromEntries(SHORTCUT_DEFAULTS.map((shortcut) => [shortcut.id, shortcut.keys]))
}

export function shortcutKeys(shortcutId: string, overrides = loadShortcutOverrides()): string[] {
  return overrides[shortcutId] ?? shortcutDefaultsById()[shortcutId] ?? []
}

export function formatShortcutKey(eventKey: string): string {
  if (eventKey === ' ') {
    return 'Space'
  }
  if (eventKey === 'Escape') {
    return 'Esc'
  }
  if (eventKey === 'Enter') {
    return '↵'
  }
  if (eventKey === 'Backspace') {
    return '⌫'
  }
  if (eventKey === 'ArrowDown') {
    return '↓'
  }
  if (eventKey === 'ArrowUp') {
    return '↑'
  }
  if (eventKey === 'ArrowRight') {
    return '→'
  }
  if (eventKey === 'ArrowLeft') {
    return '←'
  }
  return eventKey.length === 1 ? eventKey.toUpperCase() : eventKey
}

export function shortcutFromEvent(event: KeyboardEvent): string[] {
  return [
    event.ctrlKey ? 'Ctrl' : '',
    event.altKey ? '⌥' : '',
    event.shiftKey ? '⇧' : '',
    event.metaKey ? '⌘' : '',
    formatShortcutKey(event.key),
  ].filter((key, index, array) => key && array.indexOf(key) === index)
}

export function isModifierOnlyShortcutEvent(event: KeyboardEvent): boolean {
  return MODIFIER_KEYS.includes(event.key)
}

export function isShortcutEvent(event: KeyboardEvent, shortcutId: string): boolean {
  if (isModifierOnlyShortcutEvent(event)) {
    return false
  }

  const actual = shortcutFromEvent(event)
  const expected = shortcutKeys(shortcutId)

  return actual.length === expected.length && actual.every((key, index) => key === expected[index])
}
