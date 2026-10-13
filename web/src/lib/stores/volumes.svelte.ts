// Per-user local playback volume (0–2) and local mute, remembered by user name.
import { SvelteMap } from 'svelte/reactivity'
import { getSetting, setSetting } from '../settings'
import { session } from './session.svelte'

interface VolumePref {
  volume: number
  muted: boolean
}

const prefs = new SvelteMap<string, VolumePref>(Object.entries(getSetting<Record<string, VolumePref>>('volumes', {})))

function save(): void {
  setSetting('volumes', Object.fromEntries(prefs))
}

export function volumePref(name: string): VolumePref {
  return prefs.get(name) ?? { volume: 1, muted: false }
}

export function setVolumePref(name: string, pref: VolumePref): void {
  if (pref.volume === 1 && !pref.muted) prefs.delete(name)
  else prefs.set(name, pref)
  save()
}

/** Effective gain for a session's audio (non-reactive read for the audio path). */
export function userGain(sessionId: number): number {
  const name = session.users.get(sessionId)?.name
  if (!name) return 1
  const p = prefs.get(name)
  if (!p) return 1
  return p.muted ? 0 : p.volume
}
