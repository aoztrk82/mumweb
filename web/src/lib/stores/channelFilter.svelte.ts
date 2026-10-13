// Channel tree filter (desktop "filter": hide channels without users) and the
// per-channel "hide/pin when filtering" modes. Stored locally.
import { SvelteMap } from 'svelte/reactivity'
import { getSetting, setSetting } from '../settings'

export type FilterMode = 'hide' | 'pin'

class ChannelFilter {
  enabled = $state(getSetting('channelFilter', false))
  modes = new SvelteMap<number, FilterMode>(
    Object.entries(getSetting<Record<string, FilterMode>>('channelFilterModes', {})).map(([k, v]) => [Number(k), v]),
  )

  toggle(): void {
    this.enabled = !this.enabled
    setSetting('channelFilter', this.enabled)
  }

  /** Sets or clears (same mode again) a channel's filter mode. */
  setMode(channelId: number, mode: FilterMode): void {
    if (this.modes.get(channelId) === mode) this.modes.delete(channelId)
    else this.modes.set(channelId, mode)
    setSetting('channelFilterModes', Object.fromEntries(this.modes))
  }
}

export const channelFilter = new ChannelFilter()
