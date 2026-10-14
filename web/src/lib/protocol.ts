// Gateway <-> browser WebSocket protocol (see MUMWEB_PLAN.md section 6).

export interface CertPair {
  pem: string
  key: string
}

// --- browser -> gateway ---

export type ClientMessage =
  | {
      t: 'hello'
      username: string
      password: string
      accessPassword: string
      cert: CertPair | null
      tokens: string[]
      /** 'admin' for the /admin panel (needs the gateway's admin password). */
      role?: 'admin'
      adminPassword?: string
    }
  | { t: 'setTokens'; tokens: string[] }
  | { t: 'setChannelPassword'; channelId: number; channelPassword: string }
  | { t: 'queryChannelPassword'; channelId: number }
  | { t: 'joinChannel'; channelId: number }
  | { t: 'createChannel'; parent: number; name: string; temporary: boolean }
  | { t: 'setSelf'; mute?: boolean; deaf?: boolean }
  | { t: 'text'; message: string; session?: number; channelId?: number; tree?: boolean }
  | { t: 'editChannel'; channelId: number; newName?: string; description?: string; position?: number; maxUsers?: number }
  | { t: 'removeChannel' | 'requestDescription'; channelId: number }
  | { t: 'linkChannels'; channelId: number; add?: number[]; remove?: number[] }
  | { t: 'listen'; channelId: number; on: boolean }
  | { t: 'ping'; ts: number }
  | { t: 'queryPermissions'; channelId: number }
  | { t: 'moveUser'; session: number; channelId: number }
  | { t: 'setUser'; session: number; mute?: boolean; deaf?: boolean; prioritySpeaker?: boolean }
  | { t: 'kick' | 'ban'; session: number; reason: string }
  | { t: 'register' | 'resetComment' | 'resetTexture' | 'requestComment' | 'userStats'; session: number }
  // admin sessions only
  | { t: 'adminBanList' | 'adminUserList' }
  | { t: 'adminSetBanList'; bans: BanEntry[] }
  | { t: 'adminUpdateUsers'; userUpdates: { id: number; name: string | null }[] }
  | { t: 'adminACL'; channelId: number }
  | { t: 'adminSetACL'; channelId: number; acl: AclData }

// --- admin panel payloads ---

export interface BanEntry {
  /** IP address; "100::" with mask 128 marks a certificate-only ban. */
  address: string
  mask: number
  name?: string
  hash?: string
  reason?: string
  start?: string
  /** Seconds, 0 = permanent. */
  duration: number
}

export interface RegisteredUser {
  id: number
  name: string
  lastSeen?: string
  lastChannel?: number
}

export interface AclEntry {
  applyHere: boolean
  applySubs: boolean
  /** Defined in a parent channel (read only). */
  inherited: boolean
  userId?: number
  group?: string
  grant: number
  deny: number
}

export interface AclGroup {
  name: string
  inherited: boolean
  inherit: boolean
  inheritable: boolean
  add: number[]
  remove: number[]
  inheritedMembers: number[]
}

export interface AclData {
  inheritAcls: boolean
  acls: AclEntry[]
  groups: AclGroup[]
}

// --- gateway -> browser ---

export interface ChannelInfo {
  id: number
  parent: number | null
  name: string
  position: number
  description: string
  /** A description exists (possibly not loaded yet: request it with requestDescription). */
  hasDescription: boolean
  temporary: boolean
  /** 0 = no limit. */
  maxUsers: number
  links: number[]
  /** Entering needs a password (access token). */
  restricted: boolean
  /** Whether we may enter with our current tokens. */
  canEnter: boolean
}

export interface UserInfo {
  session: number
  name: string
  channelId: number
  mute: boolean
  deaf: boolean
  suppress: boolean
  selfMute: boolean
  selfDeaf: boolean
  prioritySpeaker: boolean
  recording: boolean
  registered: boolean
  /** Registration id when registered; 0 = SuperUser. */
  userId?: number
  comment?: string
  hasComment: boolean
  hasTexture: boolean
  /** Certificate hash (public in Mumble), used to key local preferences. */
  hash?: string
  /** Channels this user listens to without being in them. */
  listening: number[]
}

export interface PacketStats {
  good: number
  late: number
  lost: number
  resync: number
}

export interface UserStats {
  session: number
  release?: string
  os?: string
  osVersion?: string
  version?: string
  onlineSecs?: number
  idleSecs?: number
  address?: string
  bandwidth?: number
  tcpPingAvg: number
  udpPingAvg: number
  tcpPackets: number
  udpPackets: number
  opus: boolean
  strongCertificate: boolean
  certificates: number
  fromClient?: PacketStats
  fromServer?: PacketStats
}

export type ServerMessage =
  | ({ t: 'cert' } & CertPair)
  | { t: 'ready'; session: number; permissions: number; welcome: string; maxBandwidth: number; allowHtml: boolean; messageLength: number }
  | ({ t: 'channel' } & ChannelInfo)
  | { t: 'channelRemove'; id: number }
  | ({ t: 'user' } & UserInfo)
  | { t: 'userRemove'; session: number; actor?: number; reason: string; ban?: boolean }
  | { t: 'text'; from: number; channelIds: number[]; private?: boolean; message: string }
  | { t: 'denied'; type: string; reason: string; permission?: number; channelId: number; session?: number }
  | { t: 'channelPassword'; channelId: number; password: string }
  | { t: 'reject'; reason: string; message: string }
  | { t: 'pong'; ts: number }
  | { t: 'permissions'; channelId: number; permissions: number; flush: boolean }
  | ({ t: 'userStats' } & UserStats)
  | { t: 'banList'; bans: BanEntry[] }
  | { t: 'userList'; users: RegisteredUser[] }
  | ({ t: 'acl'; channelId: number } & AclData)
  | { t: 'adminBusy' }
  | { t: 'error'; code: 'invalid_certificate' | 'mumble_unavailable' | 'disconnected'; message: string }

// --- binary audio frames (little endian) ---

export const FRAME_AUDIO = 0x01
export const FLAG_TERMINATOR = 0x01
const IN_HEADER = 18 // gateway -> browser
const OUT_HEADER = 10 // browser -> gateway

export interface IncomingAudio {
  sender: number
  frameNumber: number
  volumeAdjustment: number // 0 = none
  terminator: boolean
  opus: Uint8Array
}

/** Decodes a gateway -> browser audio frame, or returns null if malformed. */
export function decodeAudioFrame(buf: ArrayBuffer): IncomingAudio | null {
  if (buf.byteLength < IN_HEADER) return null
  const v = new DataView(buf)
  if (v.getUint8(0) !== FRAME_AUDIO) return null
  return {
    terminator: (v.getUint8(1) & FLAG_TERMINATOR) !== 0,
    sender: v.getUint32(2, true),
    // Frame numbers stay far below 2^53 (20 ms frames), so a JS number is exact.
    frameNumber: Number(v.getBigUint64(6, true)),
    volumeAdjustment: v.getFloat32(14, true),
    opus: new Uint8Array(buf, IN_HEADER),
  }
}

/** Encodes a browser -> gateway audio frame. */
export function encodeAudioFrame(frameNumber: number, opus: Uint8Array, terminator: boolean): ArrayBuffer {
  const buf = new ArrayBuffer(OUT_HEADER + opus.byteLength)
  const v = new DataView(buf)
  v.setUint8(0, FRAME_AUDIO)
  v.setUint8(1, terminator ? FLAG_TERMINATOR : 0)
  v.setBigUint64(2, BigInt(frameNumber), true)
  new Uint8Array(buf, OUT_HEADER).set(opus)
  return buf
}
