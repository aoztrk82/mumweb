// AudioWorklet: turns microphone input into 20 ms (960 sample) mono 48 kHz frames
// and reports their RMS level. Resamples linearly if the context is not 48 kHz.
import { FRAME_SAMPLES, FrameAssembler } from './frames'

declare const sampleRate: number
declare function registerProcessor(name: string, ctor: unknown): void
declare class AudioWorkletProcessor {
  readonly port: MessagePort
}

export interface CaptureFrame {
  pcm: Float32Array<ArrayBuffer> // FRAME_SAMPLES samples at 48 kHz
  rms: number
}

class CaptureProcessor extends AudioWorkletProcessor {
  private assembler = new FrameAssembler(sampleRate, (pcm) => {
    let sum = 0
    for (let i = 0; i < FRAME_SAMPLES; i++) sum += pcm[i] * pcm[i]
    const frame: CaptureFrame = { pcm, rms: Math.sqrt(sum / FRAME_SAMPLES) }
    this.port.postMessage(frame, [pcm.buffer])
  })

  process(inputs: Float32Array[][]): boolean {
    const input = inputs[0]
    if (input && input.length > 0) {
      // Downmix to mono.
      if (input.length === 1) {
        this.assembler.push(input[0])
      } else {
        const mono = new Float32Array(input[0].length)
        for (const ch of input) for (let i = 0; i < mono.length; i++) mono[i] += ch[i] / input.length
        this.assembler.push(mono)
      }
    }
    return true
  }
}

registerProcessor('mumweb-capture', CaptureProcessor)
