// Outgoing voice: microphone -> capture worklet -> PTT/VAD gate -> Opus encoder -> gateway.
import { encodeAudioFrame } from '../protocol'
import { audioSettings } from '../stores/audioSettings.svelte'
import { session } from '../stores/session.svelte'
import { mic } from '../stores/mic.svelte'
import captureWorkletUrl from './capture-worklet.ts?worker&url'
import type { CaptureFrame } from './capture-worklet'
import { unlockAudio } from './context'
import { bitrateFor, FRAME_SAMPLES, meterToRms, Vad } from './frames'

let send: (buf: ArrayBuffer) => void = () => {}
let stream: MediaStream | null = null
let captureCtx: AudioContext | null = null
let source: MediaStreamAudioSourceNode | null = null
let worklet: AudioWorkletNode | null = null
let encoder: AudioEncoder | null = null
let encoderBitrate = 0

let frameNumber = 0
let transmitting = false
let pttHeld = false
const vad = new Vad(300)
/**
 * While a transmission is being closed, encoder output is held back until flush()
 * completes, so the very last packet (flush also emits the encoder's lookahead tail)
 * can carry the terminator flag.
 */
let holding: Uint8Array[] | null = null

export function setAudioSender(fn: (buf: ArrayBuffer) => void): void {
  send = fn
}

function encoderConfig(bitrate: number): AudioEncoderConfig {
  return {
    codec: 'opus',
    sampleRate: 48000,
    numberOfChannels: 1,
    bitrate,
    opus: { frameDuration: 20000, complexity: 9, usedtx: false },
  } as AudioEncoderConfig
}

/** Checks Opus encoding support. */
export async function encoderSupported(): Promise<boolean> {
  try {
    return typeof AudioEncoder !== 'undefined' && !!(await AudioEncoder.isConfigSupported(encoderConfig(40000))).supported
  } catch {
    return false
  }
}

/**
 * Opens the microphone. Call from a user gesture. Returns false if access was
 * denied or audio input is unavailable (the session continues listen-only).
 */
export async function startMicrophone(): Promise<boolean> {
  const p = audioSettings.prefs
  if (!navigator.mediaDevices?.getUserMedia) {
    return micFailed('SecurityError', 'navigator.mediaDevices is unavailable (not a secure context?)')
  }
  const device = p.inputDeviceId ? { deviceId: { exact: p.inputDeviceId } } : {}
  try {
    stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        ...device,
        echoCancellation: true,
        noiseSuppression: p.noiseSuppression,
        autoGainControl: true,
        channelCount: 1,
      },
    })
  } catch (e) {
    const err = e as DOMException
    // A stored device may have disappeared: retry with the default one.
    if (p.inputDeviceId && (err.name === 'OverconstrainedError' || err.name === 'NotFoundError')) {
      audioSettings.update({ inputDeviceId: '' })
      return startMicrophone()
    }
    if (err.name === 'NotAllowedError' || err.name === 'SecurityError') return micFailed(err.name, err.message)
    // Some devices/drivers (seen with Firefox on Windows) refuse to open with
    // processing constraints; try once more with a plain request.
    console.warn('microphone open failed, retrying without constraints', err)
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: p.inputDeviceId ? device : true })
    } catch (e2) {
      const err2 = e2 as DOMException
      return micFailed(err2.name, err2.message)
    }
  }
  try {
    await connectGraph(stream)
  } catch (e) {
    console.error('capture setup failed', e)
    stopMicrophone()
    const err = e as Error
    return micFailed('CaptureSetupError', `${err.name}: ${err.message}`)
  }
  mic.state = 'on'
  mic.errorName = ''
  mic.errorDetail = ''
  mic.inputCount = -1
  return true
}

function micFailed(name: string, detail: string): false {
  console.warn('microphone unavailable', name, detail)
  mic.state = name === 'NotAllowedError' ? 'denied' : 'unavailable'
  mic.errorName = name
  mic.errorDetail = detail
  mic.inputCount = -1
  // Diagnostic for the warning: how many microphones does the browser see at all?
  navigator.mediaDevices
    ?.enumerateDevices()
    .then((all) => (mic.inputCount = all.filter((d) => d.kind === 'audioinput').length))
    .catch(() => {})
  return false
}

async function connectGraph(s: MediaStream): Promise<void> {
  const ctx = unlockAudio()
  try {
    source = ctx.createMediaStreamSource(s)
    captureCtx = ctx
  } catch (e) {
    // Firefox cannot connect a stream whose rate differs from the context's;
    // capture in a context at the device rate and resample in the worklet.
    console.info('using a separate capture context', e)
    captureCtx = new AudioContext()
    source = captureCtx.createMediaStreamSource(s)
  }
  await captureCtx.audioWorklet.addModule(captureWorkletUrl)
  worklet = new AudioWorkletNode(captureCtx, 'mumweb-capture', { numberOfInputs: 1, numberOfOutputs: 0, channelCount: 1, channelCountMode: 'explicit' })
  worklet.port.onmessage = (e: MessageEvent<CaptureFrame>) => onFrame(e.data)
  source.connect(worklet)
}

export function stopMicrophone(): void {
  endTransmission()
  worklet?.port.close()
  worklet?.disconnect()
  source?.disconnect()
  stream?.getTracks().forEach((t) => t.stop())
  if (captureCtx && captureCtx !== unlockAudio()) void captureCtx.close()
  if (encoder && encoder.state !== 'closed') encoder.close()
  worklet = null
  source = null
  stream = null
  captureCtx = null
  encoder = null
  mic.level = 0
  if (mic.state === 'on') mic.state = 'off'
}

/** Re-opens the microphone after a device or processing change. */
export async function restartMicrophone(): Promise<boolean> {
  if (mic.state !== 'on') return false
  stopMicrophone()
  return startMicrophone()
}

export function setPushToTalk(held: boolean): void {
  pttHeld = held
}

function ensureEncoder(): AudioEncoder {
  const bitrate = bitrateFor(session.maxBandwidth)
  if (encoder && encoder.state !== 'closed' && bitrate === encoderBitrate) return encoder
  if (encoder && encoder.state !== 'closed') encoder.close()
  encoderBitrate = bitrate
  encoder = new AudioEncoder({
    output: (chunk) => {
      const data = new Uint8Array(chunk.byteLength)
      chunk.copyTo(data)
      if (holding) holding.push(data)
      else sendPacket(data, false)
    },
    error: (e) => {
      console.error('opus encoder error', e)
      encoder = null
    },
  })
  encoder.configure(encoderConfig(bitrate))
  return encoder
}

/** Whether the user wants to talk right now (ignoring mute state). */
function wantsToTalk(rms: number): boolean {
  const p = audioSettings.prefs
  if (p.mode === 'ptt') return pttHeld
  return vad.update(rms, meterToRms(p.vadThreshold), performance.now())
}

function canTalk(): boolean {
  const u = session.self
  return session.ready && !!u && !u.selfMute && !u.selfDeaf && !u.mute && !u.deaf && !u.suppress
}

function onFrame(f: CaptureFrame): void {
  mic.level = f.rms
  const talk = wantsToTalk(f.rms) && canTalk()
  if (talk) {
    transmitting = true
    encodeFrame(f.pcm)
  } else if (transmitting) {
    // Send this frame too, flagged as the end of the transmission.
    encodeFrame(f.pcm, true)
    endTransmission()
  }
  setSelfSpeaking(transmitting)
}

function sendPacket(opus: Uint8Array, terminator: boolean): void {
  send(encodeAudioFrame(frameNumber, opus, terminator))
  // Frame numbers count 10 ms frames (official client), i.e. +2 per 20 ms packet.
  frameNumber += 2
}

/** Flushes the encoder and sends the held-back tail with the terminator on the last packet. */
function closeTransmission(enc: AudioEncoder): void {
  const held: Uint8Array[] = []
  holding = held
  enc
    .flush()
    .catch((e) => console.warn('encoder flush failed', e))
    .finally(() => {
      if (holding === held) holding = null
      held.forEach((p, i) => sendPacket(p, i === held.length - 1))
      // Nothing came out (should not happen): still end the stream for listeners.
      if (held.length === 0) sendPacket(new Uint8Array(0), true)
    })
}

let captureTs = 0
function encodeFrame(pcm: Float32Array<ArrayBuffer>, last = false): void {
  const enc = ensureEncoder()
  const timestamp = captureTs
  captureTs += 20000
  const data = new AudioData({
    format: 'f32-planar',
    sampleRate: 48000,
    numberOfChannels: 1,
    numberOfFrames: FRAME_SAMPLES,
    timestamp,
    data: pcm,
  })
  enc.encode(data)
  data.close()
  if (last) closeTransmission(enc)
}

function endTransmission(): void {
  transmitting = false
  vad.reset()
  setSelfSpeaking(false)
}

function setSelfSpeaking(on: boolean): void {
  const id = session.selfSession
  if (id === null) return
  if (on) session.speaking.add(id)
  else session.speaking.delete(id)
}

/** Lists audio devices (labels are only available after microphone permission). */
export async function listDevices(): Promise<{ inputs: MediaDeviceInfo[]; outputs: MediaDeviceInfo[] }> {
  const all = await navigator.mediaDevices.enumerateDevices()
  return {
    inputs: all.filter((d) => d.kind === 'audioinput'),
    outputs: all.filter((d) => d.kind === 'audiooutput'),
  }
}
