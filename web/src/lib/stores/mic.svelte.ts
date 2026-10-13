// Microphone state for the UI.
class MicState {
  state = $state<'off' | 'on' | 'denied' | 'unavailable'>('off')
  /** Current input RMS (linear). */
  level = $state(0)
}

export const mic = new MicState()
