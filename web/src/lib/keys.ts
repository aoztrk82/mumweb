// Human-readable names for KeyboardEvent.code values.
const NAMES: Record<string, string> = {
  Space: 'Boşluk',
  ControlLeft: 'Sol Ctrl',
  ControlRight: 'Sağ Ctrl',
  ShiftLeft: 'Sol Shift',
  ShiftRight: 'Sağ Shift',
  AltLeft: 'Sol Alt',
  AltRight: 'Sağ Alt (AltGr)',
  CapsLock: 'Caps Lock',
  Backquote: '"',
  Tab: 'Tab',
}

export function keyName(code: string): string {
  if (NAMES[code]) return NAMES[code]
  if (code.startsWith('Key')) return code.slice(3)
  if (code.startsWith('Digit')) return code.slice(5)
  if (code.startsWith('Numpad')) return 'Num ' + code.slice(6)
  return code
}

/** True if the event target is a text field (typing there must not trigger push-to-talk). */
export function isTyping(e: Event): boolean {
  const el = e.target as HTMLElement | null
  if (!el) return false
  return el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName)
}
