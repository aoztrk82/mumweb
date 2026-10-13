// Transient UI state shared between components.

export interface MenuTarget {
  kind: 'user' | 'channel'
  id: number
  x: number
  y: number
}

class UiState {
  /** Open context menu, if any. */
  menu = $state<MenuTarget | null>(null)
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
