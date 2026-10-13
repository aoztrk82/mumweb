// UI language: English or Turkish. The choice is stored in localStorage; without
// a stored choice the browser language decides, with Turkish as the fallback.
import { getSetting, setSetting } from '../lib/settings'
import { en } from './en'
import { tr, type Translations } from './tr'

export type Lang = 'en' | 'tr'

export const LANGS: Lang[] = ['en', 'tr']
const DICTS: Record<Lang, Translations> = { en, tr }
const FALLBACK: Lang = 'tr'

function detect(): Lang {
  const saved = getSetting<string>('lang', '')
  if (saved === 'en' || saved === 'tr') return saved
  const prefs = typeof navigator === 'undefined' ? [] : (navigator.languages?.length ? navigator.languages : [navigator.language])
  for (const p of prefs) {
    const base = (p ?? '').toLowerCase().split('-')[0]
    if (base === 'en' || base === 'tr') return base
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
  document.querySelector('meta[name="description"]')?.setAttribute('content', DICTS[lang].appDescription)
}
