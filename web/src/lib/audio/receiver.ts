// Incoming voice: one WebCodecs Opus decoder per speaker, feeding the playback worklet.
import { decodeAudioFrame, type IncomingAudio } from '../protocol'
import { session } from '../stores/session.svelte'
import { userGain } from '../stores/volumes.svelte'
import { playbackNode } from './context'
import type { PlaybackMessage } from './playback-worklet'

const SPEAKING_TIMEOUT_MS = 300
const DECODER_IDLE_MS = 30_000
const OPUS_CONFIG: AudioDecoderConfig = { codec: 'opus', sampleRate: 48000, numberOfChannels: 1 }

interface Speaker {
  decoder: AudioDecoder
  lastFrame: number
  /** Timestamp of the terminator packet, to send "end" after its PCM. */
  endTs: number | null
  volumeAdjustment: number
  speakingTimer: ReturnType<typeof setTimeout> | undefined
  idleTimer: ReturnType<typeof setTimeout> | undefined
}

const speakers = new Map<number, Speaker>()
let node: AudioWorkletNode | null = null
let supported: boolean | null = null

function post(m: PlaybackMessage, transfer: Transferable[] = []): void {
  node?.port.postMessage(m, transfer)
}

/** Checks Opus decoding support and loads the playback worklet. Returns false if unsupported. */
export async function startReceiver(): Promise<boolean> {
  if (supported === null) {
    try {
      supported = typeof AudioDecoder !== 'undefined' && !!(await AudioDecoder.isConfigSupported(OPUS_CONFIG)).supported
    } catch {
      supported = false
    }
  }
  if (!supported) return false
  node = await playbackNode()
  return true
}

function createSpeaker(id: number): Speaker {
  const s: Speaker = {
    lastFrame: -1,
    endTs: null,
    volumeAdjustment: 0,
    speakingTimer: undefined,
    idleTimer: undefined,
    decoder: null as unknown as AudioDecoder,
  }
  s.decoder = new AudioDecoder({
    output: (data) => {
      const pcm = new Float32Array(data.numberOfFrames)
      data.copyTo(pcm, { planeIndex: 0, format: 'f32-planar' })
      const ts = data.timestamp
      data.close()
      if (session.selfDeaf) return
      const adj = s.volumeAdjustment > 0 ? s.volumeAdjustment : 1
      post({ t: 'pcm', id, pcm, gain: userGain(id) * adj }, [pcm.buffer])
      if (s.endTs === ts) {
        s.endTs = null
        post({ t: 'end', id })
      }
    },
    error: (e) => {
      console.warn('opus decoder error; recreating', id, e)
      dropSpeaker(id)
    },
  })
  s.decoder.configure(OPUS_CONFIG)
  speakers.set(id, s)
  return s
}

function dropSpeaker(id: number): void {
  const s = speakers.get(id)
  if (!s) return
  clearTimeout(s.speakingTimer)
  clearTimeout(s.idleTimer)
  if (s.decoder.state !== 'closed') s.decoder.close()
  speakers.delete(id)
  session.speaking.delete(id)
}

function setSpeaking(id: number, s: Speaker, on: boolean): void {
  clearTimeout(s.speakingTimer)
  if (on) {
    session.speaking.add(id)
    s.speakingTimer = setTimeout(() => {
      session.speaking.delete(id)
      s.lastFrame = -1 // a new burst may restart numbering
    }, SPEAKING_TIMEOUT_MS)
  } else {
    session.speaking.delete(id)
  }
}

/** Handles one binary frame from the gateway. */
export function handleAudioFrame(buf: ArrayBuffer): void {
  const p = decodeAudioFrame(buf)
  if (!p || !node) return
  const s = speakers.get(p.sender) ?? createSpeaker(p.sender)
  clearTimeout(s.idleTimer)
  s.idleTimer = setTimeout(() => dropSpeaker(p.sender), DECODER_IDLE_MS)

  // Late or duplicate packet within a burst.
  if (p.frameNumber <= s.lastFrame) return
  s.lastFrame = p.frameNumber
  s.volumeAdjustment = p.volumeAdjustment

  setSpeaking(p.sender, s, !p.terminator)
  if (p.terminator) s.lastFrame = -1
  decode(p, s)
}

function decode(p: IncomingAudio, s: Speaker): void {
  if (p.opus.byteLength === 0) {
    if (p.terminator) post({ t: 'end', id: p.sender })
    return
  }
  const timestamp = p.frameNumber * 10_000 // frame numbers count 10 ms frames
  if (p.terminator) s.endTs = timestamp
  try {
    s.decoder.decode(new EncodedAudioChunk({ type: 'key', timestamp, data: p.opus }))
  } catch (e) {
    console.warn('decode failed', e)
    dropSpeaker(p.sender)
  }
}

/** Forget a speaker that left the server. */
export function removeSpeaker(id: number): void {
  dropSpeaker(id)
  post({ t: 'remove', id })
}

/** Deafened: stop all playback immediately. */
export function silenceAll(): void {
  post({ t: 'clear' })
}

export function stopReceiver(): void {
  for (const id of [...speakers.keys()]) dropSpeaker(id)
  post({ t: 'clear' })
}
