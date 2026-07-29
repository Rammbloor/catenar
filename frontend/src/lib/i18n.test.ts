import { describe, expect, it } from 'vitest'

import {
  LANGUAGE_STORAGE_KEY,
  detectDefaultLanguage,
  resolveInitialLanguage,
  translate,
  translateDiagnosticCategory,
  translateDiagnosticDetailLabel,
  translateDiagnosticMessage,
  translateDiagnosticNextStep,
  translateProductLine,
  translateStreamStateLabel,
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

  it('localizes the monitor shell copy', () => {
    expect(translate('en', 'monitor.title')).toBe('Catenar')
    expect(translate('ru', 'monitor.title')).toBe('Catenar')
    expect(translate('ru', 'connection.new')).toBe('Новое соединение')
  })

  it('localizes settings choices', () => {
    expect(translate('en', 'settings.themeLight')).toBe('Light Theme')
    expect(translate('en', 'common.languageEn')).toBe('English')
    expect(translate('ru', 'settings.themeDark')).toBe('Темная тема')
    expect(translate('en', 'settings.updatesTab')).toBe('Updates')
    expect(translate('ru', 'settings.updatesTab')).toBe('Обновления')
    expect(translate('en', 'settings.shortcutsTab')).toBe('Keyboard shortcuts')
    expect(translate('ru', 'settings.shortcutsTab')).toBe('Горячие клавиши')
    expect(translate('ru', 'common.languageEn')).toBe('Английский')
    expect(translate('ru', 'settings.checkUpdates')).toBe('Проверить обновления')
    expect(translate('ru', 'settings.shortcutCloseWorkspace')).toBe('Закрыть рабочую область')
    expect(translate('ru', 'response.trailers')).toBe('Финальные метаданные')
  })

  it('localizes stream state labels used by the monitor details', () => {
    expect(translateStreamStateLabel('en', 'half_closed_local')).toBe('half-closed local')
    expect(translateStreamStateLabel('ru', 'half_closed_local')).toBe('локальная отправка закрыта')
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
  it('keeps product copy user-facing when backend bootstrap copy is technical', () => {
    expect(translateProductLine('ru', 'legacy product line')).toBe(
      'gRPC-клиент и инспектор трафика',
    )
  })

  it('keeps machine stream values available while adding localized human labels', () => {
    expect(translateStreamStateLabel('ru', 'half_closed_local')).toBe('локальная отправка закрыта')
    expect(translateStreamStateLabel('ru', 'unknown_state')).toBe('unknown_state')
  })

  it('localizes backend diagnostics by stable codes instead of raw backend copy', () => {
    expect(
      translateDiagnosticMessage(
        'ru',
        'application.runtime_ready',
        'Frontend/backend event bridge verified successfully.',
      ),
    ).toBe('Канал событий интерфейса и бэкенда успешно проверен.')
    expect(
      translateDiagnosticNextStep(
        'ru',
        'application.runtime_ready',
        'Proceed with Epic 1 runtime features on top of the validated shell.',
      ),
    ).toBe('Продолжайте работу с текущим соединением.')
    expect(translateDiagnosticCategory('ru', 'application')).toBe('Приложение')
    expect(translateDiagnosticDetailLabel('ru', 'eventName')).toBe('Событие')
  })
})
