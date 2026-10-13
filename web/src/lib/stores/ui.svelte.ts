// Transient UI state shared between components.
class UiState {
  /** Session whose volume panel is open in the channel tree. */
  volumeFor = $state<number | null>(null)
}

export const ui = new UiState()
