// Jitter buffer for one speaker's decoded 48 kHz PCM. Pure logic, used inside the
// playback worklet and unit-tested directly.

export const PREFILL = 2880 // 60 ms at 48 kHz before playback starts
export const MAX_BUFFERED = 9600 // 200 ms; older audio is dropped beyond this

export class JitterBuffer {
  private chunks: Float32Array[] = []
  /** Read position inside chunks[0] (fractional when resampling). */
  private pos = 0
  private size = 0 // total samples in chunks
  private playing = false
  private draining = false
  finished = false

  /** Samples waiting to be played. */
  get buffered(): number {
    return this.size - Math.floor(this.pos)
  }

  get isPlaying(): boolean {
    return this.playing
  }

  push(pcm: Float32Array): void {
    if (pcm.length === 0) return
    this.chunks.push(pcm)
    this.size += pcm.length
    this.draining = false
    this.finished = false
    // Too far behind: drop whole chunks from the front.
    while (this.buffered > MAX_BUFFERED && this.chunks.length > 1) {
      const dropped = this.chunks.shift()!
      this.size -= dropped.length
      this.pos = 0
    }
  }

  /** End of transmission: play out what is left without waiting for prefill. */
  drain(): void {
    this.draining = true
    if (this.buffered <= 0) this.finished = true
  }

  /** Adds up to out.length samples (times gain) into out, advancing by step input samples per output sample. */
  mixInto(out: Float32Array, gain: number, step = 1): void {
    if (!this.playing) {
      if (this.buffered >= PREFILL || (this.draining && this.buffered > 0)) this.playing = true
      else return
    }
    for (let i = 0; i < out.length; i++) {
      if (this.chunks.length === 0) {
        // Underrun: wait for the buffer to refill (or stop if the stream ended).
        this.playing = false
        if (this.draining) this.finished = true
        return
      }
      const chunk = this.chunks[0]
      const idx = Math.floor(this.pos)
      const frac = this.pos - idx
      let v = chunk[idx]
      if (frac > 0) {
        const next = idx + 1 < chunk.length ? chunk[idx + 1] : (this.chunks[1]?.[0] ?? v)
        v += (next - v) * frac
      }
      out[i] += v * gain
      this.pos += step
      while (this.chunks.length > 0 && this.pos >= this.chunks[0].length) {
        this.pos -= this.chunks[0].length
        this.size -= this.chunks[0].length
        this.chunks.shift()
      }
    }
  }
}
