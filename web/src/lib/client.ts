// Connects the gateway socket to the session state. UI components call these actions.
import { tr } from '../i18n/tr'
import { unlockAudio } from './audio/context'
import { findChannelByPath, requestedChannelPath } from './channelPath'
import { playJoin, playLeave } from './audio/notify'
import { handleAudioFrame, removeSpeaker, silenceAll, startReceiver, stopReceiver } from './audio/receiver'
import { encoderSupported, setAudioSender, startMicrophone, stopMicrophone } from './audio/transmitter'
import { loadCert, saveCert } from './certStore'
import type { ServerMessage, UserInfo } from './protocol'
import { sanitizeHtml, textToHtml } from './sanitize'
import { session } from './stores/session.svelte'
import { displayName, userPref } from './stores/userPrefs.svelte'
import { gatewayUrl, ReconnectingSocket } from './ws'

let socket: ReconnectingSocket | null = null
let credentials = { username: '', password: '' }

/** Starts a session. Call from a click handler: it also unlocks audio playback. */
export function connect(username: string, password: string): void {
  unlockAudio()
  void startAudio()
  socket?.stop()
  credentials = { username, password }
  session.error = null
  session.notice = null
  session.joined = false
  session.clearChat()
  openSocket(username, password)
}

function openSocket(username: string, password: string): void {
  socket = new ReconnectingSocket(gatewayUrl(), {
    hello: async () => ({ t: 'hello', username, password, cert: await loadCert() }),
    message: handleMessage,
    audio: handleAudioFrame,
    status: (s, attempt, retryIn) => {
      session.status = s
      session.attempt = attempt
      session.retryIn = retryIn
      if (s === 'reconnecting') session.ready = false
    },
    opened: () => {
      session.resetServerState()
      queried.clear()
    },
  })
  socket.start()
}

async function startAudio(): Promise<void> {
  try {
    const [canDecode, canEncode] = await Promise.all([startReceiver(), encoderSupported()])
    session.audioUnsupported = !canDecode || !canEncode
    if (canEncode) {
      setAudioSender(sendAudioFrame)
      await startMicrophone()
    }
  } catch (e) {
    console.error('audio setup failed', e)
    session.audioUnsupported = true
  }
}

/** Reconnects with the same credentials (e.g. after changing the certificate). */
export function reconnect(): void {
  if (!credentials.username) return
  const { username, password } = credentials
  socket?.stop()
  socket = null
  session.joined = true // keep the main view while reconnecting
  openSocket(username, password)
}

export function disconnect(): void {
  socket?.stop()
  socket = null
  stopReceiver()
  stopMicrophone()
  session.joined = false
  session.resetServerState()
}

/** Ends the session with an error shown on the login screen. */
function fail(message: string): void {
  disconnect()
  session.error = message
}

export function joinChannel(channelId: number): void {
  if (session.self?.channelId === channelId) return
  socket?.send({ t: 'joinChannel', channelId })
}

/** Asks the server to create a sub-channel; the server's ACL decides (denials show in chat). */
export function createChannel(parent: number, name: string, temporary: boolean): void {
  socket?.send({ t: 'createChannel', parent, name: name.trim(), temporary })
}

const queried = new Set<number>()

/** Asks for our permissions in a channel (at most once until the server flushes them). */
export function queryPermissions(channelId: number): void {
  if (queried.has(channelId)) return
  queried.add(channelId)
  socket?.send({ t: 'queryPermissions', channelId })
}

export function moveUser(sessionId: number, channelId: number): void {
  socket?.send({ t: 'moveUser', session: sessionId, channelId })
}

/** Server-side mute/deafen/priority speaker for another user (needs MuteDeafen). */
export function setUser(sessionId: number, patch: { mute?: boolean; deaf?: boolean; prioritySpeaker?: boolean }): void {
  socket?.send({ t: 'setUser', session: sessionId, ...patch })
}

export function kickUser(sessionId: number, reason: string): void {
  socket?.send({ t: 'kick', session: sessionId, reason })
}

/** Bans by certificate only (see gateway: an IP ban would hit every web user). */
export function banUser(sessionId: number, reason: string): void {
  socket?.send({ t: 'ban', session: sessionId, reason })
}

export function userCommand(t: 'register' | 'resetComment' | 'resetTexture' | 'requestComment' | 'userStats', sessionId: number): void {
  if (t === 'userStats') session.userStats = null
  socket?.send({ t, session: sessionId })
}

export function editChannel(channelId: number, patch: { newName?: string; description?: string; position?: number; maxUsers?: number }): void {
  socket?.send({ t: 'editChannel', channelId, ...patch })
}

export function removeChannel(channelId: number): void {
  socket?.send({ t: 'removeChannel', channelId })
}

export function requestDescription(channelId: number): void {
  socket?.send({ t: 'requestDescription', channelId })
}

/** Links/unlinks channels (Mumble links are symmetric; we always edit our own channel). */
export function linkChannels(channelId: number, add: number[], remove: number[]): void {
  socket?.send({ t: 'linkChannels', channelId, add, remove })
}

export function setListening(channelId: number, on: boolean): void {
  socket?.send({ t: 'listen', channelId, on })
}

/** Sends a text message to a channel, optionally including all its sub-channels. */
export function sendChannelText(channelId: number, text: string, tree: boolean): boolean {
  const message = textToHtml(text.trim())
  if (!message) return false
  if (session.messageLength > 0 && message.length > session.messageLength) {
    session.addChat({ kind: 'error', body: tr.errors.denied.TextTooLong })
    return false
  }
  socket?.send({ t: 'text', message, channelId, tree })
  const c = session.channels.get(channelId)
  session.addChat({
    kind: 'text',
    fromSelf: true,
    target: tr.chat.channelTarget(c?.name ?? '', tree),
    body: sanitizeHtml(message),
  })
  return true
}

/** Sends a private text message to one user. */
export function sendPrivateText(sessionId: number, text: string): boolean {
  const message = textToHtml(text.trim())
  if (!message) return false
  if (session.messageLength > 0 && message.length > session.messageLength) {
    session.addChat({ kind: 'error', body: tr.errors.denied.TextTooLong })
    return false
  }
  socket?.send({ t: 'text', message, session: sessionId })
  const to = session.users.get(sessionId)
  session.addChat({
    kind: 'text',
    fromSelf: true,
    target: to ? displayName(to) : tr.chat.unknownUser,
    private: true,
    body: sanitizeHtml(message),
  })
  return true
}

export function setSelf(mute: boolean, deaf: boolean): void {
  socket?.send({ t: 'setSelf', mute, deaf })
}

export function toggleMute(): void {
  const u = session.self
  if (!u) return
  // Unmuting while deafened also undeafens (as in the desktop client).
  if (u.selfMute) setSelf(false, false)
  else setSelf(true, u.selfDeaf)
}

export function toggleDeaf(): void {
  const u = session.self
  if (!u) return
  if (u.selfDeaf) setSelf(false, false)
  else setSelf(true, true)
}

/** Sends plain text typed by the user to the current channel. */
export function sendText(text: string): boolean {
  const message = textToHtml(text.trim())
  if (!message) return false
  if (session.messageLength > 0 && message.length > session.messageLength) {
    session.addChat({ kind: 'error', body: tr.errors.denied.TextTooLong })
    return false
  }
  socket?.send({ t: 'text', message })
  // Mumble does not echo our own messages back.
  session.addChat({ kind: 'text', from: session.self?.name ?? credentials.username, fromSelf: true, body: sanitizeHtml(message) })
  return true
}

export function sendAudioFrame(buf: ArrayBuffer): void {
  socket?.sendBinary(buf)
}

function userName(sessionId: number): string {
  const u = session.users.get(sessionId)
  return u ? displayName(u) : tr.chat.unknownUser
}

function handleMessage(m: ServerMessage): void {
  switch (m.t) {
    case 'cert':
      saveCert({ pem: m.pem, key: m.key }).catch((e) => console.error('saving certificate failed', e))
      break

    case 'channel': {
      const { t: _t, ...c } = m
      session.channels.set(c.id, c)
      break
    }

    case 'channelRemove':
      session.channels.delete(m.id)
      break

    case 'user': {
      const { t: _t, ...u } = m
      const prev = session.users.get(u.session)
      session.users.set(u.session, u)
      if (u.session === session.selfSession && u.selfDeaf && !prev?.selfDeaf) silenceAll()
      if (session.ready) announceMovement(prev, u)
      break
    }

    case 'userRemove': {
      if (m.session === session.selfSession) {
        // Kicked or banned: don't reconnect.
        fail(m.ban ? tr.errors.banned(m.reason) : tr.errors.kicked(m.reason))
        return
      }
      const prev = session.users.get(m.session)
      session.users.delete(m.session)
      removeSpeaker(m.session)
      if (prev && session.ready && prev.channelId === session.self?.channelId) {
        session.addChat({ kind: 'system', body: tr.chat.left(prev.name) })
        playLeave()
      }
      break
    }

    case 'ready':
      session.selfSession = m.session
      session.rootPermissions = m.permissions
      queried.clear()
      session.welcome = m.welcome
      session.maxBandwidth = m.maxBandwidth
      session.messageLength = m.messageLength
      session.ready = true
      session.notice = null
      if (!session.joined && m.welcome) session.addChat({ kind: 'text', from: tr.chat.server, body: sanitizeHtml(m.welcome) })
      if (!session.joined) {
        // Shared channel link (#kanal=/Oyun): join that channel after the first login.
        const path = requestedChannelPath()
        const target = path ? findChannelByPath(path) : undefined
        if (target !== undefined && target !== session.users.get(m.session)?.channelId) {
          socket?.send({ t: 'joinChannel', channelId: target })
        }
      }
      if (!session.joined && session.users.get(m.session)?.userId === 0) {
        session.addChat({ kind: 'system', body: tr.chat.superUserNotice })
      }
      session.joined = true
      socket?.established()
      break

    case 'text': {
      const sender = session.users.get(m.from)
      if (sender && userPref(sender).ignoreMessages) break
      session.addChat({ kind: 'text', from: userName(m.from), private: m.private, body: sanitizeHtml(m.message) })
      break
    }

    case 'permissions':
      if (m.flush) {
        session.channelPermissions.clear()
        queried.clear()
      }
      session.channelPermissions.set(m.channelId, m.permissions)
      break

    case 'userStats': {
      const { t: _t, ...st } = m
      session.userStats = st
      break
    }

    case 'denied': {
      // "Text" denials carry a human-readable reason from the server.
      const msg = m.type === 'Text' && m.reason ? m.reason : (tr.errors.denied[m.type] ?? tr.errors.generic)
      session.addChat({ kind: 'error', body: msg })
      break
    }

    case 'reject':
      fail(tr.errors.reject[m.reason] ?? (m.message || tr.errors.reject.None))
      break

    case 'error':
      if (m.code === 'invalid_certificate') {
        fail(tr.errors.invalidCertificate)
      } else {
        // The gateway closes the socket next; the reconnect loop takes over.
        session.notice = m.code === 'mumble_unavailable' ? tr.errors.mumbleUnavailable : tr.errors.disconnected
      }
      break

    case 'pong':
      break
  }
}

/** Adds join/leave lines for users entering or leaving our channel. */
function announceMovement(prev: UserInfo | undefined, u: UserInfo): void {
  const mine = session.self?.channelId
  if (mine === undefined || u.session === session.selfSession) return
  const was = prev?.channelId === mine
  const is = u.channelId === mine
  if (!was && is) {
    session.addChat({ kind: 'system', body: tr.chat.joined(u.name) })
    playJoin()
  } else if (was && !is) {
    session.addChat({ kind: 'system', body: tr.chat.left(u.name) })
    playLeave()
  }
}
