// Gateway <-> browser WebSocket protocol (see MUMWEB_PLAN.md section 6).

export interface CertPair {
  pem: string
  key: string
}

// --- browser -> gateway ---

export type ClientMessage =
  | { t: 'hello'; username: string; password: string; cert: CertPair | null }
  | { t: 'joinChannel'; channelId: number }
  | { t: 'setSelf'; mute?: boolean; deaf?: boolean }
  | { t: 'text'; message: string }
  | { t: 'ping'; ts: number }

// --- gateway -> browser ---

export interface ChannelInfo {
  id: number
  parent: number | null
  name: string
  position: number
  description: string
  temporary: boolean
  links: number[]
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
  comment?: string
}

export type ServerMessage =
  | ({ t: 'cert' } & CertPair)
  | { t: 'ready'; session: number; welcome: string; maxBandwidth: number; allowHtml: boolean; messageLength: number }
  | ({ t: 'channel' } & ChannelInfo)
  | { t: 'channelRemove'; id: number }
  | ({ t: 'user' } & UserInfo)
  | { t: 'userRemove'; session: number; actor?: number; reason: string; ban?: boolean }
  | { t: 'text'; from: number; channelIds: number[]; private?: boolean; message: string }
  | { t: 'denied'; type: string; reason: string }
  | { t: 'reject'; reason: string; message: string }
  | { t: 'pong'; ts: number }
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
