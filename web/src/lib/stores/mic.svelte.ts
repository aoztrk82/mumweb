// Microphone state for the UI.
class MicState {
  state = $state<'off' | 'on' | 'denied' | 'unavailable'>('off')
  /** Current input RMS (linear). */
  level = $state(0)
  /** DOMException name and message of the last failure (for the warning text). */
  errorName = $state('')
  errorDetail = $state('')
}

export const mic = new MicState()
