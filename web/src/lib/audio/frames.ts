// Pure helpers for the capture path (used in the worklet and unit-tested).

export const FRAME_SAMPLES = 960 // 20 ms at 48 kHz

/** Collects input blocks at any sample rate into fixed 48 kHz frames. */
export class FrameAssembler {
  private frame = new Float32Array(FRAME_SAMPLES)
  private filled = 0
  private readonly step: number // input samples per output sample
  private pos = 0 // fractional read position relative to the current input block
  private prev = 0 // last input sample of the previous block (for interpolation)

  constructor(
    inputRate: number,
    private readonly emit: (frame: Float32Array<ArrayBuffer>) => void,
  ) {
    this.step = inputRate / 48000
  }

  push(block: Float32Array): void {
    if (this.step === 1) {
      let i = 0
      while (i < block.length) {
        const n = Math.min(block.length - i, FRAME_SAMPLES - this.filled)
        this.frame.set(block.subarray(i, i + n), this.filled)
        this.filled += n
        i += n
        if (this.filled === FRAME_SAMPLES) this.flush()
      }
      return
    }
    // Linear interpolation; pos may start negative (between prev and block[0]).
    while (this.pos < block.length - 1 + 1e-9) {
      const idx = Math.floor(this.pos)
      const frac = this.pos - idx
      const a = idx < 0 ? this.prev : block[idx]
      const b = block[Math.min(idx + 1, block.length - 1)]
      this.frame[this.filled++] = a + (b - a) * frac
      if (this.filled === FRAME_SAMPLES) this.flush()
      this.pos += this.step
    }
    this.prev = block[block.length - 1]
    this.pos -= block.length
  }

  private flush(): void {
    this.emit(this.frame)
    this.frame = new Float32Array(FRAME_SAMPLES)
    this.filled = 0
  }
}

/** Voice activity detection with immediate open and a hangover before closing. */
export class Vad {
  private lastAbove = -Infinity

  constructor(public hangoverMs = 300) {}

  /** Returns whether to transmit this frame. */
  update(rms: number, threshold: number, nowMs: number): boolean {
    if (rms >= threshold) this.lastAbove = nowMs
    return nowMs - this.lastAbove <= this.hangoverMs
  }

  reset(): void {
    this.lastAbove = -Infinity
  }
}

/** Opus bitrate that fits the server's per-user bandwidth limit (TCP tunnel, 20 ms packets). */
export function bitrateFor(maxBandwidth: number, preferred = 40000): number {
  // Official client: (20 + 8 + 4 + 1 + 2 + 12 [TCP] + frames) bytes * (800 / frames) bits/s, frames = 2.
  const overhead = (20 + 8 + 4 + 1 + 2 + 12 + 2) * 400
  if (!maxBandwidth) return preferred
  return Math.max(8000, Math.min(preferred, maxBandwidth - overhead))
}

/** Maps a linear RMS level to 0..1 on a -60..0 dBFS scale for meters and the VAD slider. */
export function rmsToMeter(rms: number): number {
  if (rms <= 0) return 0
  const db = 20 * Math.log10(rms)
  return Math.max(0, Math.min(1, (db + 60) / 60))
}

export function meterToRms(v: number): number {
  return 10 ** ((v * 60 - 60) / 20)
}
