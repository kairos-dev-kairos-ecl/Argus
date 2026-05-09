/**
 * WebSocket Client with Exponential Backoff Reconnection
 *
 * Features:
 * - Exponential backoff: 1s, 2s, 4s, 8s, 16s (max)
 * - Max 10 retries before permanent failure
 * - Automatic reconnect on network recovery (navigator.onOnline)
 * - Message queue if disconnected (buffer up to 100 messages)
 * - Proper cleanup (unsubscribe, close connection)
 */

export class WebSocketClient {
  private ws: WebSocket | null = null
  private url: string
  private reconnect_delay: number = 1000
  private max_reconnect_delay: number = 16000
  private retry_count: number = 0
  private max_retries: number = 10
  private message_queue: unknown[] = []
  private is_closing: boolean = false
  private message_handlers: Set<(data: unknown) => void> = new Set()
  private error_handlers: Set<(error: string) => void> = new Set()
  private online_listener: (() => void) | null = null
  private pending_reject: ((err: Error) => void) | null = null

  constructor(url: string) {
    this.url = url
  }

  /**
   * Connect to the WebSocket endpoint
   */
  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Track reject so disconnect()/onclose-during-CONNECTING can
      // settle the promise instead of leaking it forever.
      this.pending_reject = reject
      const settleResolve = () => {
        this.pending_reject = null
        resolve()
      }
      const settleReject = (err: Error) => {
        if (this.pending_reject) {
          this.pending_reject = null
          reject(err)
        }
      }

      try {
        this.ws = new WebSocket(this.url)

        this.ws.onopen = () => {
          this.retry_count = 0
          this.reconnect_delay = 1000
          this.flushMessageQueue()
          settleResolve()
        }

        this.ws.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data)
            this.message_handlers.forEach((handler) => handler(data))
          } catch (error) {
            const err = error instanceof Error ? error.message : 'Failed to parse message'
            this.error_handlers.forEach((handler) => handler(err))
          }
        }

        this.ws.onerror = () => {
          const error = 'WebSocket connection error'
          this.error_handlers.forEach((handler) => handler(error))
          // If the connect promise hasn't resolved yet, reject it so
          // awaiters don't hang.
          settleReject(new Error(error))
        }

        this.ws.onclose = () => {
          // If the close happens before onopen AND we're closing intentionally
          // (disconnect() called during CONNECTING), reject the pending promise
          // so `await client.connect()` unblocks cleanly.
          if (this.is_closing) {
            settleReject(new Error('WebSocket: disconnected before open'))
            return
          }

          if (!this.is_closing && this.retry_count < this.max_retries) {
            this.attemptReconnect()
          } else if (this.retry_count >= this.max_retries) {
            const error = 'WebSocket: max retries exceeded'
            this.error_handlers.forEach((handler) => handler(error))
            settleReject(new Error(error))
          }
        }

        // Setup network recovery listener
        this.setupNetworkListener()
      } catch (error) {
        const err = error instanceof Error ? error.message : 'WebSocket connection failed'
        settleReject(new Error(err))
      }
    })
  }

  /**
   * Disconnect from WebSocket
   */
  disconnect(): void {
    this.is_closing = true
    this.removeNetworkListener()

    // Reject any in-flight connect() promise so awaiters don't hang.
    if (this.pending_reject) {
      const reject = this.pending_reject
      this.pending_reject = null
      reject(new Error('WebSocket: disconnected before open'))
    }

    if (this.ws) {
      this.ws.close()
      this.ws = null
    }

    this.message_queue = []
    this.message_handlers.clear()
    this.error_handlers.clear()
  }

  /**
   * Send a message through the WebSocket
   */
  send(message: unknown): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      try {
        this.ws.send(JSON.stringify(message))
      } catch (error) {
        const err = error instanceof Error ? error.message : 'Failed to send message'
        this.error_handlers.forEach((handler) => handler(err))
      }
    } else {
      // Queue message if not connected
      if (this.message_queue.length < 100) {
        this.message_queue.push(message)
      }
    }
  }

  /**
   * Register a message handler
   * Returns unsubscribe function
   */
  onMessage(callback: (data: unknown) => void): () => void {
    this.message_handlers.add(callback)
    return () => {
      this.message_handlers.delete(callback)
    }
  }

  /**
   * Register an error handler
   * Returns unsubscribe function
   */
  onError(callback: (error: string) => void): () => void {
    this.error_handlers.add(callback)
    return () => {
      this.error_handlers.delete(callback)
    }
  }

  /**
   * Get connection status
   */
  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN
  }

  /**
   * Attempt reconnection with exponential backoff
   */
  private attemptReconnect(): void {
    this.retry_count++
    const delay = Math.min(
      this.reconnect_delay * Math.pow(2, this.retry_count - 1),
      this.max_reconnect_delay
    )

    setTimeout(() => {
      if (!this.is_closing) {
        this.connect().catch((error) => {
          const err = error instanceof Error ? error.message : 'Reconnection failed'
          this.error_handlers.forEach((handler) => handler(err))
        })
      }
    }, delay)
  }

  /**
   * Flush queued messages
   */
  private flushMessageQueue(): void {
    while (this.message_queue.length > 0 && this.ws?.readyState === WebSocket.OPEN) {
      const message = this.message_queue.shift()
      if (message) {
        try {
          this.ws.send(JSON.stringify(message))
        } catch (error) {
          const err = error instanceof Error ? error.message : 'Failed to flush message'
          this.error_handlers.forEach((handler) => handler(err))
          break
        }
      }
    }
  }

  /**
   * Setup network recovery listener
   */
  private setupNetworkListener(): void {
    this.online_listener = () => {
      if (!this.isConnected() && !this.is_closing) {
        this.attemptReconnect()
      }
    }
    window.addEventListener('online', this.online_listener)
  }

  /**
   * Remove network listener
   */
  private removeNetworkListener(): void {
    if (this.online_listener) {
      window.removeEventListener('online', this.online_listener)
      this.online_listener = null
    }
  }
}

/**
 * Factory function to create a WebSocket client
 */
export function createWebSocketClient(url: string): WebSocketClient {
  return new WebSocketClient(url)
}
