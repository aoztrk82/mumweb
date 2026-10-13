// Local, per-browser preferences about other users: nickname, ignored messages,
// friends. Keyed by certificate hash when known (stable across renames), else by name.
import { SvelteMap } from 'svelte/reactivity'
import type { UserInfo } from '../protocol'
import { getSetting, setSetting } from '../settings'

export interface UserPref {
  nickname?: string
  ignoreMessages?: boolean
  /** Name the user had when added as a friend. */
  friend?: string
}

const store = new SvelteMap<string, UserPref>(Object.entries(getSetting<Record<string, UserPref>>('userPrefs', {})))

export function prefKey(u: Pick<UserInfo, 'hash' | 'name'>): string {
  return u.hash ? `h:${u.hash}` : `n:${u.name}`
}

export function userPref(u: Pick<UserInfo, 'hash' | 'name'>): UserPref {
  return store.get(prefKey(u)) ?? {}
}

export function updateUserPref(u: Pick<UserInfo, 'hash' | 'name'>, patch: Partial<UserPref>): void {
  const next: UserPref = { ...userPref(u), ...patch }
  for (const k of Object.keys(next) as (keyof UserPref)[]) if (next[k] === undefined || next[k] === '' || next[k] === false) delete next[k]
  if (Object.keys(next).length === 0) store.delete(prefKey(u))
  else store.set(prefKey(u), next)
  setSetting('userPrefs', Object.fromEntries(store))
}

/** Name to show: the local nickname if set. */
export function displayName(u: Pick<UserInfo, 'hash' | 'name'>): string {
  return userPref(u).nickname || u.name
}
