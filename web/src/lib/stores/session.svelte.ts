// Reactive state of the current Mumble session (channels, users, chat, connection).
import { SvelteMap } from 'svelte/reactivity'
import type { ChannelInfo, UserInfo } from '../protocol'
import type { ConnStatus } from '../ws'

export interface ChatEntry {
  id: number
  time: Date
  kind: 'text' | 'system' | 'error'
  /** Sender name for text entries. */
  from?: string
  fromSelf?: boolean
  private?: boolean
  /** Sanitized HTML for text entries, plain text otherwise. */
  body: string
}

const MAX_CHAT = 500

class SessionState {
  status = $state<ConnStatus>('idle')
  attempt = $state(0)
  retryIn = $state(0)
  /** Fatal error shown on the login screen. */
  error = $state<string | null>(null)
  /** Transient warning shown in the status bar (cleared on reconnect). */
  notice = $state<string | null>(null)
  /** True once the first "ready" arrived; the main view stays up while reconnecting. */
  joined = $state(false)
  ready = $state(false)

  selfSession = $state<number | null>(null)
  welcome = $state('')
  maxBandwidth = $state(0)
  messageLength = $state(0)

  channels = new SvelteMap<number, ChannelInfo>()
  users = new SvelteMap<number, UserInfo>()
  chat = $state<ChatEntry[]>([])

  private nextId = 1

  self = $derived(this.selfSession === null ? undefined : this.users.get(this.selfSession))
  currentChannel = $derived(this.self ? this.channels.get(this.self.channelId) : undefined)

  /** Clears server state before a new connection's initial sync. */
  resetServerState(): void {
    this.ready = false
    this.selfSession = null
    this.channels.clear()
    this.users.clear()
  }

  addChat(e: Omit<ChatEntry, 'id' | 'time'>): void {
    this.chat.push({ ...e, id: this.nextId++, time: new Date() })
    if (this.chat.length > MAX_CHAT) this.chat.splice(0, this.chat.length - MAX_CHAT)
  }

  clearChat(): void {
    this.chat = []
  }
}

export const session = new SessionState()

/** Children of a channel sorted like Mumble: by position, then name. */
export function childChannels(parent: number): ChannelInfo[] {
  const out: ChannelInfo[] = []
  for (const c of session.channels.values()) if (c.parent === parent) out.push(c)
  return out.sort((a, b) => a.position - b.position || a.name.localeCompare(b.name, 'tr'))
}

export function channelUsers(channelId: number): UserInfo[] {
  const out: UserInfo[] = []
  for (const u of session.users.values()) if (u.channelId === channelId) out.push(u)
  return out.sort((a, b) => a.name.localeCompare(b.name, 'tr'))
}
