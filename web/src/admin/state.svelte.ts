// Reactive state of the admin panel session (separate from the voice client's session store).
import { SvelteMap } from 'svelte/reactivity'
import { locale } from '../i18n/index.svelte'
import type { AclData, BanEntry, ChannelInfo, RegisteredUser, UserInfo } from '../lib/protocol'
import type { ConnStatus } from '../lib/ws'

export interface Toast {
  id: number
  kind: 'error' | 'info'
  text: string
}

class AdminState {
  status = $state<ConnStatus>('idle')
  attempt = $state(0)
  retryIn = $state(0)
  /** Fatal error shown on the login form. */
  error = $state<string | null>(null)
  /** True once the first "ready" arrived; the panel stays up while reconnecting. */
  joined = $state(false)
  ready = $state(false)
  selfSession = $state<number | null>(null)
  username = $state('')

  channels = new SvelteMap<number, ChannelInfo>()
  users = new SvelteMap<number, UserInfo>()
  channelPasswords = new SvelteMap<number, string>()

  /** null = not loaded (or the server refused). */
  bans = $state<BanEntry[] | null>(null)
  registered = $state<RegisteredUser[] | null>(null)
  /** ACL of the channel being edited. */
  acl = $state<({ channelId: number } & AclData) | null>(null)

  /** Invite links created this session (newest first); the server keeps no list of its own. */
  invites = $state<{ token: string; url: string; expiresAt: number }[]>([])

  toasts = $state<Toast[]>([])
  private nextId = 1

  self = $derived(this.selfSession === null ? undefined : this.users.get(this.selfSession))

  resetServerState(): void {
    this.ready = false
    this.selfSession = null
    this.channels.clear()
    this.users.clear()
    this.channelPasswords.clear()
    this.bans = null
    this.registered = null
    this.acl = null
  }

  toast(kind: Toast['kind'], text: string): void {
    const id = this.nextId++
    this.toasts.push({ id, kind, text })
    if (this.toasts.length > 5) this.toasts.splice(0, this.toasts.length - 5)
    setTimeout(() => this.dismiss(id), 8000)
  }

  dismiss(id: number): void {
    this.toasts = this.toasts.filter((x) => x.id !== id)
  }

  /** Display name of a registered user id (from the loaded registry, else a placeholder). */
  userName(id: number, fallback: (id: number) => string): string {
    if (id === 0) return 'SuperUser'
    return this.registered?.find((u) => u.id === id)?.name ?? fallback(id)
  }
}

export const admin = new AdminState()

/** Children of a channel sorted like Mumble: by position, then name. */
export function childChannels(parent: number): ChannelInfo[] {
  const out: ChannelInfo[] = []
  for (const c of admin.channels.values()) if (c.parent === parent) out.push(c)
  return out.sort((a, b) => a.position - b.position || a.name.localeCompare(b.name, locale()))
}

/** The channel tree flattened depth-first, with nesting depth. */
export function flatChannels(): { channel: ChannelInfo; depth: number }[] {
  const out: { channel: ChannelInfo; depth: number }[] = []
  const walk = (id: number, depth: number) => {
    const c = admin.channels.get(id)
    if (!c) return
    out.push({ channel: c, depth })
    for (const child of childChannels(id)) walk(child.id, depth + 1)
  }
  walk(0, 0)
  return out
}
