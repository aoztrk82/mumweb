// Microphone state for the UI.
class MicState {
  state = $state<'off' | 'on' | 'denied' | 'unavailable'>('off')
  /** Current input RMS (linear). */
  level = $state(0)
  /** DOMException name and message of the last failure (for the warning text). */
  errorName = $state('')
  errorDetail = $state('')
  /** Audio inputs the browser reports after a failure (-1 = unknown). */
  inputCount = $state(-1)
}

export const mic = new MicState()
