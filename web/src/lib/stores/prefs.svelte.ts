// General UI preferences.
import { getSetting, setSetting } from '../settings'

export type Theme = 'dark' | 'light'

interface Prefs {
  theme: Theme
  /** Short tones when someone joins/leaves our channel. */
  sounds: boolean
}

class PrefsStore {
  value = $state<Prefs>({ theme: 'dark', sounds: true, ...getSetting<Partial<Prefs>>('prefs', {}) })

  update(patch: Partial<Prefs>): void {
    this.value = { ...this.value, ...patch }
    setSetting('prefs', this.value)
    applyTheme(this.value.theme)
  }
}

export const prefs = new PrefsStore()

export function applyTheme(theme: Theme): void {
  document.documentElement.dataset.theme = theme
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'light' ? '#f5f6f8' : '#15171c')
}
