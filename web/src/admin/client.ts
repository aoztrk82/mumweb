// Gateway connection of the admin panel. Unlike lib/client.ts it never starts audio or the
// microphone, and it keeps its own Mumble identity (certificate) so the same browser can run
// the voice client and the panel at once.
import { t } from '../i18n/index.svelte'
import { ADMIN_KEY, loadCert, saveCert } from '../lib/certStore'
import type { AclData, BanEntry, ClientMessage, ServerMessage } from '../lib/protocol'
import { gatewayUrl, ReconnectingSocket } from '../lib/ws'
import { admin } from './state.svelte'

let socket: ReconnectingSocket | null = null
let credentials = { username: '', password: '', adminPassword: '' }

export function connect(username: string, password: string, adminPassword: string): void {
  socket?.stop()
  credentials = { username, password, adminPassword }
  admin.error = null
  admin.joined = false
  admin.username = username
  openSocket()
}

function openSocket(): void {
  const { username, password, adminPassword } = credentials
  socket = new ReconnectingSocket(gatewayUrl(), {
    hello: async () => ({
      t: 'hello',
      role: 'admin',
      username,
      password,
      adminPassword,
      accessPassword: '',
      cert: await loadCert(ADMIN_KEY),
      tokens: [],
    }),
    message: handleMessage,
    audio: () => {}, // the gateway sends none to admin sessions
    status: (s, attempt, retryIn) => {
      admin.status = s
      admin.attempt = attempt
      admin.retryIn = retryIn
      if (s === 'reconnecting') admin.ready = false
    },
    opened: () => admin.resetServerState(),
  })
  socket.start()
}

export function disconnect(): void {
  socket?.stop()
  socket = null
  admin.joined = false
  admin.resetServerState()
}

function fail(message: string): void {
  disconnect()
  admin.error = message
}

function send(m: ClientMessage): void {
  socket?.send(m)
}

// --- queries ---

export const loadBans = (): void => send({ t: 'adminBanList' })
export const loadRegistered = (): void => send({ t: 'adminUserList' })
export const loadAcl = (channelId: number): void => send({ t: 'adminACL', channelId })

// --- writes (the UI refreshes the matching list afterwards) ---

export const saveBans = (bans: BanEntry[]): void => send({ t: 'adminSetBanList', bans })
export const updateRegistered = (userUpdates: { id: number; name: string | null }[]): void => send({ t: 'adminUpdateUsers', userUpdates })
export const saveAcl = (channelId: number, acl: AclData): void => send({ t: 'adminSetACL', channelId, acl })
export const createInvite = (ttlSeconds: number): void => send({ t: 'adminCreateInvite', inviteTtlSeconds: ttlSeconds })

// --- existing session commands, reused from the voice protocol ---

export const kickUser = (session: number, reason: string): void => send({ t: 'kick', session, reason })
export const banUser = (session: number, reason: string): void => send({ t: 'ban', session, reason })
export const setUser = (session: number, patch: { mute?: boolean; deaf?: boolean }): void => send({ t: 'setUser', session, ...patch })
export const registerUser = (session: number): void => send({ t: 'register', session })
export const createChannel = (parent: number, name: string): void => send({ t: 'createChannel', parent, name: name.trim(), temporary: false })
export const renameChannel = (channelId: number, newName: string): void => send({ t: 'editChannel', channelId, newName: newName.trim() })
export const removeChannel = (channelId: number): void => send({ t: 'removeChannel', channelId })
export const queryChannelPassword = (channelId: number): void => {
  admin.channelPasswords.delete(channelId)
  send({ t: 'queryChannelPassword', channelId })
}
export const setChannelPassword = (channelId: number, channelPassword: string): void => send({ t: 'setChannelPassword', channelId, channelPassword })

function handleMessage(m: ServerMessage): void {
  switch (m.t) {
    case 'cert':
      saveCert({ pem: m.pem, key: m.key }, ADMIN_KEY).catch((e) => console.error('saving certificate failed', e))
      break

    case 'channel': {
      const { t: _t, ...c } = m
      admin.channels.set(c.id, c)
      break
    }
    case 'channelRemove':
      admin.channels.delete(m.id)
      break

    case 'user': {
      const { t: _t, ...u } = m
      admin.users.set(u.session, u)
      break
    }
    case 'userRemove':
      if (m.session === admin.selfSession) {
        fail(m.ban ? t.errors.banned(m.reason) : t.errors.kicked(m.reason))
        return
      }
      admin.users.delete(m.session)
      break

    case 'ready':
      admin.selfSession = m.session
      admin.ready = true
      admin.joined = true
      socket?.established()
      break

    case 'channelPassword':
      admin.channelPasswords.set(m.channelId, m.password)
      break

    case 'banList':
      admin.bans = m.bans
      break
    case 'userList':
      admin.registered = m.users
      break
    case 'acl': {
      const { t: _t, ...a } = m
      admin.acl = a
      break
    }

    case 'adminBusy':
      admin.toast('error', t.admin.busy)
      break

    case 'inviteCreated':
      admin.invites = [{ token: m.token, url: `${location.origin}/?invite=${m.token}`, expiresAt: m.expiresAt }, ...admin.invites]
      break

    case 'denied': {
      const msg = m.type === 'Text' && m.reason ? m.reason : (t.errors.denied[m.type] ?? t.errors.generic)
      admin.toast('error', msg)
      break
    }

    case 'reject':
      fail(t.errors.reject[m.reason] ?? (m.message || t.errors.reject.None))
      break

    case 'error':
      if (m.code === 'invalid_certificate') fail(t.errors.invalidCertificate)
      // Otherwise the gateway closes the socket and the reconnect loop takes over.
      break

    case 'text':
    case 'permissions':
    case 'userStats':
    case 'pong':
      break
  }
}
