// Small per-browser preferences in localStorage. Every access may throw
// (private mode, blocked storage), so failures fall back to defaults.

const PREFIX = 'mumweb.'

export function getSetting<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(PREFIX + key)
    return raw === null ? fallback : (JSON.parse(raw) as T)
  } catch {
    return fallback
  }
}

export function setSetting<T>(key: string, value: T): void {
  try {
    localStorage.setItem(PREFIX + key, JSON.stringify(value))
  } catch {
    // ignore
  }
}
