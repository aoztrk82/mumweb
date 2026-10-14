// The web access password (MUMWEB_ACCESS_PASSWORD) and whether this browser remembers it.
// It only gates this web client; it is not the Mumble server password.
import { getSetting, setSetting } from './settings'

const KEY = 'accessPassword'

export const accessPassword = {
  /** The remembered password, or '' when none is stored. */
  saved(): string {
    return getSetting<string>(KEY, '')
  },
  save(value: string): void {
    setSetting(KEY, value)
  },
  forget(): void {
    setSetting(KEY, '')
  },
}

/** The reserved-slot password (MUMWEB_RESERVED_PASSWORD) and whether this browser remembers it. */
export const reservedPassword = {
  saved(): string {
    return getSetting<string>('reservedPassword', '')
  },
  save(value: string): void {
    setSetting('reservedPassword', value)
  },
  forget(): void {
    setSetting('reservedPassword', '')
  },
}

/** Asks the gateway which login fields it wants. Failures mean "none". */
export async function fetchLoginConfig(): Promise<{ accessPassword: boolean; reservedSlots: boolean }> {
  try {
    const res = await fetch('/config', { cache: 'no-store' })
    if (!res.ok) return { accessPassword: false, reservedSlots: false }
    const cfg = (await res.json()) as { accessPassword?: boolean; reservedSlots?: boolean }
    return { accessPassword: cfg.accessPassword === true, reservedSlots: cfg.reservedSlots === true }
  } catch {
    return { accessPassword: false, reservedSlots: false }
  }
}
