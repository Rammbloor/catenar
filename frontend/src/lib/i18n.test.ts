import { describe, expect, it } from 'vitest'

import {
  LANGUAGE_STORAGE_KEY,
  detectDefaultLanguage,
  resolveInitialLanguage,
  translate,
  translateBootstrapSliceSummary,
  translateDiagnosticMessage,
  translateProductLine,
  translateStreamStateLabel,
  translateViewLabel,
  translateFromDictionaries,
  type LanguageStorage,
} from './i18n'

class MemoryStorage implements LanguageStorage {
  private readonly values = new Map<string, string>()

  getItem(key: string): string | null {
    return this.values.get(key) ?? null
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value)
  }
}

describe('i18n language resolution', () => {
  it('uses Russian by default when the browser locale starts with ru', () => {
    expect(detectDefaultLanguage('ru-RU')).toBe('ru')
    expect(resolveInitialLanguage({ locale: 'ru' })).toBe('ru')
  })

  it('uses English by default for non-Russian browser locales', () => {
    expect(detectDefaultLanguage('en-US')).toBe('en')
    expect(resolveInitialLanguage({ locale: 'fr-FR' })).toBe('en')
  })

  it('prefers the stored language when it is supported', () => {
    const storage = new MemoryStorage()
    storage.setItem(LANGUAGE_STORAGE_KEY, 'ru')

    expect(resolveInitialLanguage({ storage, locale: 'en-US' })).toBe('ru')
  })

  it('ignores unsupported stored language values', () => {
    const storage = new MemoryStorage()
    storage.setItem(LANGUAGE_STORAGE_KEY, 'de')

    expect(resolveInitialLanguage({ storage, locale: 'ru-RU' })).toBe('ru')
  })

  it('localizes interactive client-stream controls', () => {
    expect(translate('en', 'request.clientStreamModeInteractive')).toBe('Interactive')
    expect(translate('ru', 'stream.halfCloseRequested', { callId: 'call_123' })).toContain('call_123')
  })

  it('localizes bidirectional streaming controls', () => {
    expect(translate('en', 'request.startBidiStream')).toBe('Start bidi stream')
    expect(translate('ru', 'request.bidiMessageJson')).toBe('JSON сообщения bidi')
    expect(translate('en', 'stream.halfClosedLocalReceiving')).toContain('incoming responses')
    expect(
      translateDiagnosticMessage('ru', 'application.stream_send_unavailable', 'raw backend copy'),
    ).toBe('Поток больше не открыт для отправки.')
  })

  it('localizes windowed timeline and active-stream guard copy', () => {
    expect(translate('en', 'timeline.windowCount', { count: 200, total: 1000 })).toBe('200/1000 rendered')
    expect(translate('ru', 'stream.contextLocked', { callId: 'call_123' })).toContain('call_123')
    expect(translate('ru', 'timeline.jumpToLive')).toBe('К live-хвосту')
  })
})

describe('translateFromDictionaries', () => {
  it('falls back to English and then to the key itself', () => {
    const dictionaries = {
      en: {
        'known.onlyEnglish': 'English fallback {value}',
      },
      ru: {},
    }

    expect(translateFromDictionaries('ru', dictionaries, 'known.onlyEnglish', { value: 7 })).toBe(
      'English fallback 7',
    )
    expect(translateFromDictionaries('ru', dictionaries, 'missing.key')).toBe('missing.key')
  })
})

describe('stable id presentation helpers', () => {
  it('maps backend bootstrap ids without showing raw English copy in Russian', () => {
    expect(translateProductLine('ru', 'Desktop-first gRPC debugging workspace')).toBe(
      'Локальное рабочее место для отладки gRPC',
    )
    expect(translateViewLabel('ru', 'workspace')).toBe('Рабочая область')
    expect(
      translateBootstrapSliceSummary(
        'ru',
        '0.1',
        'Wails shell, Svelte app frame, runtime binding and diagnostics event round-trip.',
      ),
    ).toContain('Оболочка Wails')
  })

  it('keeps machine values available while adding localized human labels', () => {
    expect(translateStreamStateLabel('ru', 'half_closed_local')).toBe('локальная отправка закрыта')
    expect(translateDiagnosticMessage('ru', 'application.runtime_ready', 'raw backend copy')).toBe(
      'Мост событий между frontend и backend успешно проверен.',
    )
    expect(translateDiagnosticMessage('ru', 'grpc_status.invalid_argument', 'INVALID_ARGUMENT')).toBe(
      'INVALID_ARGUMENT',
    )
  })
})
