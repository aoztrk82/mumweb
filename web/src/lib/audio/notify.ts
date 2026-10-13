// Short notification tones (join/leave), synthesized so no audio files are needed.
import { prefs } from '../stores/prefs.svelte'
import { session } from '../stores/session.svelte'
import { audioContext } from './context'

function tone(freqs: number[]): void {
  const ctx = audioContext()
  if (!ctx || !prefs.value.sounds || session.selfDeaf) return
  const gain = ctx.createGain()
  gain.connect(ctx.destination)
  let t = ctx.currentTime
  for (const f of freqs) {
    const osc = ctx.createOscillator()
    osc.type = 'sine'
    osc.frequency.value = f
    osc.connect(gain)
    gain.gain.setValueAtTime(0, t)
    gain.gain.linearRampToValueAtTime(0.08, t + 0.01)
    gain.gain.linearRampToValueAtTime(0, t + 0.09)
    osc.start(t)
    osc.stop(t + 0.1)
    t += 0.1
  }
}

export const playJoin = () => tone([660, 880])
export const playLeave = () => tone([880, 660])
