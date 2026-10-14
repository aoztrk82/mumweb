// Reactive state of the current Mumble session (channels, users, chat, connection).
import { SvelteMap, SvelteSet } from 'svelte/reactivity'
import { locale } from '../../i18n/index.svelte'
import type { ChannelInfo, UserInfo } from '../protocol'
import type { ConnStatus } from '../ws'

export interface ChatEntry {
  id: number
  time: Date
  kind: 'text' | 'system' | 'error'
  /** Sender name for text entries. */
  from?: string
  fromSelf?: boolean
  /** Recipient of our own message ("Takma", "#Oyun"), shown as "Sen → …". */
  target?: string
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
  /** The gateway wants its web access password (set from /config or a reject). */
  accessRequired = $state(false)
  /** The gateway keeps some slots for a reserved-slot password (from /config). */
  reservedSlots = $state(false)
  /** Operator-set display name/description (MUMWEB_SERVER_NAME/_DESCRIPTION, from /config). */
  serverName = $state('')
  serverDescription = $state('')
  /** True once the first "ready" arrived; the main view stays up while reconnecting. */
  joined = $state(false)
  ready = $state(false)

  selfSession = $state<number | null>(null)
  welcome = $state('')
  maxBandwidth = $state(0)
  messageLength = $state(0)
  /** Our permissions in the root channel (from ServerSync). */
  rootPermissions = $state(0)
  /** Permissions per channel as answered by PermissionQuery. */
  channelPermissions = new SvelteMap<number, number>()
  /** Channel passwords read from the ACL (for the edit dialog), by channel. */
  channelPasswords = new SvelteMap<number, string>()
  /** Last UserStats answer, shown by the information dialog. */
  userStats = $state<import('../protocol').UserStats | null>(null)

  channels = new SvelteMap<number, ChannelInfo>()
  users = new SvelteMap<number, UserInfo>()
  chat = $state<ChatEntry[]>([])
  /** Sessions currently transmitting voice. */
  speaking = new SvelteSet<number>()
  /** Set when the browser cannot decode/encode Opus. */
  audioUnsupported = $state(false)

  private nextId = 1

  self = $derived(this.selfSession === null ? undefined : this.users.get(this.selfSession))
  selfDeaf = $derived(this.self?.selfDeaf ?? false)
  currentChannel = $derived(this.self ? this.channels.get(this.self.channelId) : undefined)

  /** Clears server state before a new connection's initial sync. */
  resetServerState(): void {
    this.ready = false
    this.selfSession = null
    this.channels.clear()
    this.users.clear()
    this.speaking.clear()
    this.channelPermissions.clear()
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
  return out.sort((a, b) => a.position - b.position || a.name.localeCompare(b.name, locale()))
}

export function channelUsers(channelId: number): UserInfo[] {
  const out: UserInfo[] = []
  for (const u of session.users.values()) if (u.channelId === channelId) out.push(u)
  return out.sort((a, b) => a.name.localeCompare(b.name, locale()))
}

/** Users listening to a channel without being in it. */
export function channelListeners(channelId: number): UserInfo[] {
  const out: UserInfo[] = []
  for (const u of session.users.values()) if (u.listening?.includes(channelId)) out.push(u)
  return out.sort((a, b) => a.name.localeCompare(b.name, locale()))
}

/** Whether a channel or any of its sub-channels has users (or listeners) in it. */
export function subtreeHasUsers(channelId: number): boolean {
  for (const u of session.users.values()) {
    if (u.channelId === channelId || u.listening?.includes(channelId)) return true
  }
  for (const c of session.channels.values()) {
    if (c.parent === channelId && subtreeHasUsers(c.id)) return true
  }
  return false
}
