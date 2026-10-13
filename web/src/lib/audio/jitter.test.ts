import { describe, expect, it } from 'vitest'
import { JitterBuffer, MAX_BUFFERED, PREFILL } from './jitter'

const frame = (v: number, n = 960) => new Float32Array(n).fill(v)
const render = (jb: JitterBuffer, n = 128, gain = 1, step = 1) => {
  const out = new Float32Array(n)
  jb.mixInto(out, gain, step)
  return out
}

describe('JitterBuffer', () => {
  it('waits for 60 ms prefill before playing', () => {
    const jb = new JitterBuffer()
    jb.push(frame(0.5))
    jb.push(frame(0.5))
    expect(render(jb)[0]).toBe(0)
    expect(jb.isPlaying).toBe(false)
    jb.push(frame(0.5)) // 2880 samples = PREFILL
    expect(jb.buffered).toBe(PREFILL)
    expect(render(jb)[0]).toBe(0.5)
    expect(jb.buffered).toBe(PREFILL - 128)
  })

  it('applies gain and mixes additively', () => {
    const jb = new JitterBuffer()
    for (let i = 0; i < 3; i++) jb.push(frame(0.25))
    const out = new Float32Array(4).fill(0.1)
    jb.mixInto(out, 2, 1)
    expect(out[0]).toBeCloseTo(0.6)
  })

  it('drops the oldest audio beyond 200 ms', () => {
    const jb = new JitterBuffer()
    for (let i = 0; i < 20; i++) jb.push(frame(i))
    expect(jb.buffered).toBeLessThanOrEqual(MAX_BUFFERED)
    // The newest frames survive.
    expect(render(jb, 1)[0]).toBe(20 - MAX_BUFFERED / 960)
  })

  it('plays a short burst after terminator without full prefill, then finishes', () => {
    const jb = new JitterBuffer()
    jb.push(frame(0.3, 256))
    jb.drain()
    const out = render(jb, 300)
    expect(out[0]).toBeCloseTo(0.3)
    expect(out[255]).toBeCloseTo(0.3)
    expect(out[256]).toBe(0)
    expect(jb.finished).toBe(true)
  })

  it('stops on underrun and re-buffers', () => {
    const jb = new JitterBuffer()
    for (let i = 0; i < 3; i++) jb.push(frame(0.1))
    render(jb, PREFILL + 10)
    expect(jb.isPlaying).toBe(false)
    expect(jb.finished).toBe(false)
    jb.push(frame(0.1))
    expect(render(jb)[0]).toBe(0) // waiting for prefill again
  })

  it('resamples when the context is not 48 kHz', () => {
    const jb = new JitterBuffer()
    const ramp = new Float32Array(PREFILL).map((_, i) => i)
    jb.push(ramp)
    const out = render(jb, 4, 1, 48000 / 44100)
    expect(out[1]).toBeCloseTo(48000 / 44100)
    expect(out[3]).toBeCloseTo((3 * 48000) / 44100)
  })
})
