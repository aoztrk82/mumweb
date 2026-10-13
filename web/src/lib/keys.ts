import { t } from '../i18n/index.svelte'

// Human-readable names for KeyboardEvent.code values (translated ones live in the i18n files).
const NAMES: Record<string, string> = {
  CapsLock: 'Caps Lock',
  Backquote: '`',
  Tab: 'Tab',
}

export function keyName(code: string): string {
  if (t.keys[code]) return t.keys[code]
  if (NAMES[code]) return NAMES[code]
  if (code.startsWith('Key')) return code.slice(3)
  if (code.startsWith('Digit')) return code.slice(5)
  if (code.startsWith('Numpad')) return `${t.keys.numpad} ${code.slice(6)}`
  return code
}

/** True if the event target is a text field (typing there must not trigger push-to-talk). */
export function isTyping(e: Event): boolean {
  const el = e.target as HTMLElement | null
  if (!el) return false
  return el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName)
}
