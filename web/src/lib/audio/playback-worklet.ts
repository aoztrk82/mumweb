// AudioWorklet: per-user jitter buffers and mixing of decoded 48 kHz mono PCM.
// Loaded with audioWorklet.addModule(); runs in AudioWorkletGlobalScope.
import { JitterBuffer } from './jitter'

declare const sampleRate: number
declare function registerProcessor(name: string, ctor: unknown): void
declare class AudioWorkletProcessor {
  readonly port: MessagePort
}

export type PlaybackMessage =
  | { t: 'pcm'; id: number; pcm: Float32Array; gain: number }
  | { t: 'end'; id: number }
  | { t: 'remove'; id: number }
  | { t: 'clear' }

class PlaybackProcessor extends AudioWorkletProcessor {
  private streams = new Map<number, JitterBuffer>()
  private gains = new Map<number, number>()
  // Input is always 48 kHz; the context may run at another rate.
  private readonly step = 48000 / sampleRate

  constructor() {
    super()
    this.port.onmessage = (e: MessageEvent<PlaybackMessage>) => {
      const m = e.data
      switch (m.t) {
        case 'pcm': {
          let s = this.streams.get(m.id)
          if (!s) {
            s = new JitterBuffer()
            this.streams.set(m.id, s)
          }
          this.gains.set(m.id, m.gain)
          s.push(m.pcm)
          break
        }
        case 'end':
          this.streams.get(m.id)?.drain()
          break
        case 'remove':
          this.streams.delete(m.id)
          this.gains.delete(m.id)
          break
        case 'clear':
          this.streams.clear()
          break
      }
    }
  }

  process(_inputs: Float32Array[][], outputs: Float32Array[][]): boolean {
    const out = outputs[0]
    const left = out[0]
    if (!left) return true
    left.fill(0)
    for (const [id, s] of this.streams) {
      s.mixInto(left, this.gains.get(id) ?? 1, this.step)
      if (s.finished) this.streams.delete(id)
    }
    for (let i = 0; i < left.length; i++) {
      const v = left[i]
      left[i] = v > 1 ? 1 : v < -1 ? -1 : v
    }
    for (let c = 1; c < out.length; c++) out[c].set(left)
    return true
  }
}

registerProcessor('mumweb-playback', PlaybackProcessor)
