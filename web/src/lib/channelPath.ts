// Channel paths ("/Oyun/Takım 1") for shareable links that join a channel after login.
import { session } from './stores/session.svelte'

export function channelPath(channelId: number): string {
  const names: string[] = []
  let c = session.channels.get(channelId)
  while (c && c.parent !== null) {
    names.unshift(c.name)
    c = session.channels.get(c.parent)
  }
  return '/' + names.join('/')
}

export function findChannelByPath(path: string): number | undefined {
  let current = 0
  for (const name of path.split('/').filter(Boolean)) {
    const next = [...session.channels.values()].find((c) => c.parent === current && c.name === name)
    if (!next) return undefined
    current = next.id
  }
  return current
}

const HASH_KEY = 'kanal'

/** Web link that opens mumweb and joins this channel after login. */
export function channelUrl(channelId: number): string {
  return `${location.origin}${location.pathname}#${HASH_KEY}=${encodeURIComponent(channelPath(channelId))}`
}

/** Channel requested by the page URL (#kanal=/Oyun), if any. */
export function requestedChannelPath(): string | null {
  const params = new URLSearchParams(location.hash.slice(1))
  return params.get(HASH_KEY)
}
