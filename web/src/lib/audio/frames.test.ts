import { describe, expect, it } from 'vitest'
import { bitrateFor, FRAME_SAMPLES, FrameAssembler, meterToRms, rmsToMeter, Vad } from './frames'

describe('FrameAssembler', () => {
  it('cuts 128-sample blocks into 960-sample frames at 48 kHz', () => {
    const frames: Float32Array[] = []
    const fa = new FrameAssembler(48000, (f) => frames.push(f))
    let n = 0
    for (let b = 0; b < 30; b++) fa.push(new Float32Array(128).map(() => n++))
    expect(frames).toHaveLength(Math.floor((30 * 128) / FRAME_SAMPLES))
    expect(frames[1][0]).toBe(960)
    expect(frames[3][959]).toBe(3839)
  })

  it('resamples 44.1 kHz input to 48 kHz', () => {
    const frames: Float32Array[] = []
    const fa = new FrameAssembler(44100, (f) => frames.push(f))
    // 1 s of a ramp in 128-sample blocks -> ~48000 output samples = 50 frames.
    let n = 0
    for (let b = 0; b < 44100 / 128; b++) fa.push(new Float32Array(128).map(() => n++ / 44100))
    expect(frames.length).toBeGreaterThanOrEqual(49)
    // Output sample k corresponds to input time k/48000 s, i.e. the ramp value k/48000.
    const k = 10 * FRAME_SAMPLES + 123
    expect(frames[10][123]).toBeCloseTo(k / 48000, 4)
  })
})

describe('Vad', () => {
  it('opens immediately and closes after the hangover', () => {
    const v = new Vad(300)
    expect(v.update(0.001, 0.01, 0)).toBe(false)
    expect(v.update(0.05, 0.01, 20)).toBe(true)
    expect(v.update(0.001, 0.01, 300)).toBe(true)
    expect(v.update(0.001, 0.01, 321)).toBe(false)
  })
})

describe('bitrateFor', () => {
  it('fits the server limit including TCP overhead', () => {
    expect(bitrateFor(558000)).toBe(40000)
    expect(bitrateFor(50000)).toBe(50000 - 19600)
    expect(bitrateFor(20000)).toBe(8000)
    expect(bitrateFor(0)).toBe(40000)
  })
})

describe('meter scale', () => {
  it('maps -60..0 dBFS to 0..1 and back', () => {
    expect(rmsToMeter(1)).toBe(1)
    expect(rmsToMeter(0.001)).toBeCloseTo(0)
    expect(rmsToMeter(meterToRms(0.45))).toBeCloseTo(0.45)
  })
})
