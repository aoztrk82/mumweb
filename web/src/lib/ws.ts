// WebSocket client for the gateway with automatic reconnection.
import type { ClientMessage, ServerMessage } from './protocol'

export type ConnStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting'

export interface SocketHandlers {
  /** Builds the hello message for each (re)connect. */
  hello(): Promise<ClientMessage>
  message(m: ServerMessage): void
  audio(buf: ArrayBuffer): void
  /** Called on every status change. attempt/retryIn are set while reconnecting. */
  status(s: ConnStatus, attempt: number, retryIn: number): void
  /** A new socket opened; state from a previous connection is stale. */
  opened(): void
}

const PING_INTERVAL = 20_000
const PONG_TIMEOUT = 10_000
const MAX_BACKOFF_S = 30
// Close codes after which reconnecting is pointless.
const CLOSE_NORMAL = 1000
const CLOSE_POLICY = 1008

/** Reconnect delay in seconds for the given attempt (1-based): 1, 2, 4, 8 … 30. */
export function backoffSeconds(attempt: number): number {
  return Math.min(2 ** Math.max(0, attempt - 1), MAX_BACKOFF_S)
}

export function gatewayUrl(loc: Location = window.location): string {
  const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${loc.host}/ws`
}

export class ReconnectingSocket {
  private ws: WebSocket | null = null
  private running = false
  private attempt = 0
  private retryTimer: ReturnType<typeof setTimeout> | undefined
  private pingTimer: ReturnType<typeof setInterval> | undefined
  private pongTimer: ReturnType<typeof setTimeout> | undefined
  private readonly onOnline = () => {
    // Network came back: skip the remaining backoff.
    if (this.running && this.retryTimer !== undefined) {
      clearTimeout(this.retryTimer)
      this.retryTimer = undefined
      void this.open()
    }
  }

  constructor(
    private readonly url: string,
    private readonly h: SocketHandlers,
  ) {}

  start(): void {
    if (this.running) return
    this.running = true
    this.attempt = 0
    window.addEventListener('online', this.onOnline)
    this.h.status('connecting', 0, 0)
    void this.open()
  }

  /** Stops for good (logout, rejection, kick). */
  stop(): void {
    this.running = false
    window.removeEventListener('online', this.onOnline)
    clearTimeout(this.retryTimer)
    this.retryTimer = undefined
    this.stopPing()
    const ws = this.ws
    this.ws = null
    if (ws && ws.readyState <= WebSocket.OPEN) ws.close(CLOSE_NORMAL)
    this.h.status('idle', 0, 0)
  }

  /** Marks the session as established; the next drop starts again at 1 s. */
  established(): void {
    this.attempt = 0
    this.h.status('connected', 0, 0)
  }

  send(m: ClientMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(m))
  }

  sendBinary(buf: ArrayBuffer): void {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(buf)
  }

  get isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  private async open(): Promise<void> {
    if (!this.running) return
    const hello = await this.h.hello()
    if (!this.running) return
    const ws = new WebSocket(this.url)
    ws.binaryType = 'arraybuffer'
    this.ws = ws

    ws.onopen = () => {
      if (this.ws !== ws) return
      this.h.opened()
      ws.send(JSON.stringify(hello))
      this.startPing()
    }
    ws.onmessage = (e: MessageEvent) => {
      if (this.ws !== ws) return
      if (typeof e.data !== 'string') {
        this.h.audio(e.data as ArrayBuffer)
        return
      }
      let m: ServerMessage
      try {
        m = JSON.parse(e.data) as ServerMessage
      } catch {
        return
      }
      if (m.t === 'pong') {
        clearTimeout(this.pongTimer)
        this.pongTimer = undefined
      }
      this.h.message(m)
    }
    ws.onclose = (e: CloseEvent) => {
      if (this.ws !== ws) return
      this.ws = null
      this.stopPing()
      if (!this.running) return
      if (e.code === CLOSE_POLICY) {
        // Protocol violation: retrying would fail the same way.
        this.stop()
        return
      }
      this.scheduleRetry()
    }
  }

  private scheduleRetry(): void {
    this.attempt++
    const delay = backoffSeconds(this.attempt)
    this.h.status('reconnecting', this.attempt, delay)
    this.retryTimer = setTimeout(() => {
      this.retryTimer = undefined
      void this.open()
    }, delay * 1000)
  }

  private startPing(): void {
    this.stopPing()
    this.pingTimer = setInterval(() => {
      const ws = this.ws
      if (!ws || ws.readyState !== WebSocket.OPEN) return
      ws.send(JSON.stringify({ t: 'ping', ts: Date.now() } satisfies ClientMessage))
      if (this.pongTimer === undefined) {
        this.pongTimer = setTimeout(() => {
          // Silent network death (e.g. Wi-Fi off): force a reconnect.
          // close() on a dead link may not fire onclose quickly, so detach first.
          this.pongTimer = undefined
          if (this.ws !== ws) return
          this.ws = null
          this.stopPing()
          ws.close()
          if (this.running) this.scheduleRetry()
        }, PONG_TIMEOUT)
      }
    }, PING_INTERVAL)
  }

  private stopPing(): void {
    clearInterval(this.pingTimer)
    clearTimeout(this.pongTimer)
    this.pingTimer = undefined
    this.pongTimer = undefined
  }
}
