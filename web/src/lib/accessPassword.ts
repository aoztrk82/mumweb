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

export interface LoginConfig {
  accessPassword: boolean
  reservedSlots: boolean
  /** Invite links are switched on (MUMWEB_INVITE_SECRET); the admin panel may mint them. */
  invite: boolean
  /** Operator-set display name/description (MUMWEB_SERVER_NAME/_DESCRIPTION), empty when unset. */
  serverName: string
  serverDescription: string
}

const EMPTY_CONFIG: LoginConfig = { accessPassword: false, reservedSlots: false, invite: false, serverName: '', serverDescription: '' }

/** Asks the gateway which login fields it wants and how it wants to be displayed. Failures mean "none". */
export async function fetchLoginConfig(): Promise<LoginConfig> {
  try {
    const res = await fetch('/config', { cache: 'no-store' })
    if (!res.ok) return EMPTY_CONFIG
    const cfg = (await res.json()) as Partial<LoginConfig>
    return {
      accessPassword: cfg.accessPassword === true,
      reservedSlots: cfg.reservedSlots === true,
      invite: cfg.invite === true,
      serverName: cfg.serverName ?? '',
      serverDescription: cfg.serverDescription ?? '',
    }
  } catch {
    return EMPTY_CONFIG
  }
}
