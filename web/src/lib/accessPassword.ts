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

/** Asks the gateway whether it requires an access password. Failures mean "no". */
export async function fetchAccessRequired(): Promise<boolean> {
  try {
    const res = await fetch('/config', { cache: 'no-store' })
    if (!res.ok) return false
    const cfg = (await res.json()) as { accessPassword?: boolean }
    return cfg.accessPassword === true
  } catch {
    return false
  }
}
