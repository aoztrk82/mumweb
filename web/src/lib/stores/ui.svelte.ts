// Transient UI state shared between components.

export interface MenuTarget {
  kind: 'user' | 'channel'
  id: number
  x: number
  y: number
}

export type DialogState =
  | {
      kind: 'prompt'
      title: string
      label: string
      value: string
      hint?: string
      multiline?: boolean
      confirm: string
      danger?: boolean
      /** Allow submitting an empty value (e.g. clearing a nickname, no kick reason). */
      allowEmpty?: boolean
      /** Optional checkbox shown under the input (e.g. "send to sub-channels"). */
      checkbox?: string
      onSubmit: (value: string, checked: boolean) => void
    }
  | { kind: 'editChannel'; channelId: number }
  | { kind: 'comment'; session: number }
  | { kind: 'info'; session: number }

class UiState {
  /** Open user dialog, if any. */
  dialog = $state<DialogState | null>(null)
  /** Open context menu, if any. */
  menu = $state<MenuTarget | null>(null)
  /** Parent channel of the open "create channel" dialog, if any. */
  createIn = $state<number | null>(null)
}

export const ui = new UiState()

/** Opens the context menu for a tree row (mouse, long press or keyboard menu key). */
export function openMenu(e: MouseEvent, kind: MenuTarget['kind'], id: number): void {
  e.preventDefault()
  let { clientX: x, clientY: y } = e
  if (x === 0 && y === 0) {
    // Keyboard-invoked: anchor to the row.
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    x = r.left + 24
    y = r.bottom
  }
  ui.menu = { kind, id, x, y }
}

export function closeMenu(): void {
  ui.menu = null
}
