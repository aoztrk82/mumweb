// Shared AudioContext. Must be created from a user gesture (login click) to satisfy autoplay rules.
import { audioSettings } from '../stores/audioSettings.svelte'
import playbackWorkletUrl from './playback-worklet.ts?worker&url'

let ctx: AudioContext | null = null
let playback: Promise<AudioWorkletNode> | null = null

/** Creates/resumes the context. Call synchronously inside a click handler. */
export function unlockAudio(): AudioContext {
  if (!ctx) {
    ctx = new AudioContext({ sampleRate: 48000, latencyHint: 'interactive' })
    const sink = audioSettings.prefs.outputDeviceId
    if (sink) void setOutputDevice(sink)
  }
  if (ctx.state === 'suspended') void ctx.resume()
  return ctx
}

type SinkContext = AudioContext & { setSinkId(id: string): Promise<void> }

/** Whether the browser can route WebAudio output to a chosen device (Chrome/Edge). */
export const canSelectOutput = typeof AudioContext !== 'undefined' && 'setSinkId' in AudioContext.prototype

export async function setOutputDevice(deviceId: string): Promise<void> {
  if (!ctx || !canSelectOutput) return
  try {
    await (ctx as SinkContext).setSinkId(deviceId)
  } catch (e) {
    console.warn('cannot switch output device', e)
  }
}

export function audioContext(): AudioContext | null {
  return ctx
}

/** The mixing playback node, connected to the speakers. */
export function playbackNode(): Promise<AudioWorkletNode> {
  if (!playback) {
    const c = unlockAudio()
    playback = c.audioWorklet.addModule(playbackWorkletUrl).then(() => {
      const node = new AudioWorkletNode(c, 'mumweb-playback', {
        numberOfInputs: 0,
        numberOfOutputs: 1,
        outputChannelCount: [2],
      })
      node.connect(c.destination)
      return node
    })
    playback.catch(() => (playback = null))
  }
  return playback
}
