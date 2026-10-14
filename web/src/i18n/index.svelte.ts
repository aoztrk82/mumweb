// UI language. The choice is stored in localStorage; without a stored choice the
// browser language decides, with Turkish as the fallback.
import { getSetting, setSetting } from '../lib/settings'
import { ar } from './ar'
import { de } from './de'
import { en } from './en'
import { es } from './es'
import { it } from './it'
import { ru } from './ru'
import { tr, type Translations } from './tr'

export type Lang = 'en' | 'tr' | 'ar' | 'de' | 'es' | 'it' | 'ru'

/** Right-to-left languages: the only ones that flip `<html dir>`. */
const RTL = new Set<Lang>(['ar'])

export const LANGS: Lang[] = ['en', 'tr', 'ar', 'de', 'es', 'it', 'ru']
const DICTS: Record<Lang, Translations> = { en, tr, ar, de, es, it, ru }
const FALLBACK: Lang = 'tr'

function detect(): Lang {
  const saved = getSetting<string>('lang', '')
  if ((LANGS as string[]).includes(saved)) return saved as Lang
  const prefs = typeof navigator === 'undefined' ? [] : (navigator.languages?.length ? navigator.languages : [navigator.language])
  for (const p of prefs) {
    const base = (p ?? '').toLowerCase().split('-')[0]
    if ((LANGS as string[]).includes(base)) return base as Lang
  }
  return FALLBACK
}

class LangStore {
  current = $state<Lang>(detect())

  set(lang: Lang): void {
    this.current = lang
    setSetting('lang', lang)
    apply(lang)
  }
}

export const language = new LangStore()

/** Locale tag for Intl and collation. */
export function locale(): string {
  return language.current === 'en' ? 'en-US' : 'tr-TR'
}

/**
 * The active strings. Reading a property reads the reactive language, so templates
 * and effects update when the language changes.
 */
export const t: Translations = new Proxy({} as Translations, {
  get: (_, key) => DICTS[language.current][key as keyof Translations],
})

export function apply(lang: Lang): void {
  document.documentElement.lang = lang
  document.documentElement.dir = RTL.has(lang) ? 'rtl' : 'ltr'
  document.querySelector('meta[name="description"]')?.setAttribute('content', DICTS[lang].appDescription)
}
