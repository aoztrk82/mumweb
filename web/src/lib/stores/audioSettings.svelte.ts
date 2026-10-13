// Persistent audio preferences.
import { getSetting, setSetting } from '../settings'

export type TransmitMode = 'ptt' | 'vad'

export interface AudioPrefs {
  mode: TransmitMode
  /** KeyboardEvent.code of the push-to-talk key. */
  pttKey: string
  /** VAD threshold on the 0..1 meter scale (-60..0 dBFS). */
  vadThreshold: number
  inputDeviceId: string
  outputDeviceId: string
  noiseSuppression: boolean
}

const DEFAULTS: AudioPrefs = {
  mode: 'vad',
  pttKey: 'KeyV',
  vadThreshold: 0.45,
  inputDeviceId: '',
  outputDeviceId: '',
  noiseSuppression: true,
}

class AudioSettings {
  prefs = $state<AudioPrefs>({ ...DEFAULTS, ...getSetting<Partial<AudioPrefs>>('audio', {}) })

  update(patch: Partial<AudioPrefs>): void {
    this.prefs = { ...this.prefs, ...patch }
    setSetting('audio', this.prefs)
  }
}

export const audioSettings = new AudioSettings()
