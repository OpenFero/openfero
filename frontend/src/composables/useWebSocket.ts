import { ref, onUnmounted } from 'vue'
import type { AlertStoreEntry } from '@/types'

export interface WSMessage {
  type: 'alert' | 'job_status' | 'connected'
  data: AlertStoreEntry | { jobName: string; status: string } | { message: string }
}

export interface WSOptions {
  onMessage?: (message: WSMessage) => void
  onConnect?: () => void
  onDisconnect?: () => void
  onError?: (error: Event) => void
  reconnectInterval?: number
  maxReconnectAttempts?: number
}

export function useWebSocket(path: string = '/api/ws', options: WSOptions = {}) {
  const isConnected = ref(false)
  const lastMessage = ref<WSMessage | null>(null)
  const error = ref<string | null>(null)

  let socket: WebSocket | null = null
  let reconnectAttempts = 0
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  let isManualDisconnect = false

  const { reconnectInterval = 3000, maxReconnectAttempts = 10 } = options

  function getWebSocketUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    return `${protocol}//${host}${path}`
  }

  function connect() {
    if (socket?.readyState === WebSocket.OPEN) {
      return
    }

    isManualDisconnect = false
    const url = getWebSocketUrl()

    try {
      socket = new WebSocket(url)

      socket.onopen = () => {
        isConnected.value = true
        error.value = null
        reconnectAttempts = 0
        console.log('WebSocket connected')
        options.onConnect?.()
      }

      socket.onmessage = (event) => {
        try {
          // Handle multiple messages separated by newlines
          const messages = event.data.split('\n').filter((m: string) => m.trim())
          for (const msgStr of messages) {
            const message: WSMessage = JSON.parse(msgStr)
            lastMessage.value = message
            options.onMessage?.(message)
          }
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e, event.data)
        }
      }

      socket.onclose = (event) => {
        isConnected.value = false
        console.log('WebSocket disconnected', event.code, event.reason)
        options.onDisconnect?.()

        // Attempt to reconnect if not manually disconnected
        if (!isManualDisconnect && reconnectAttempts < maxReconnectAttempts) {
          reconnectAttempts++
          error.value = `Connection lost. Reconnecting... (${reconnectAttempts}/${maxReconnectAttempts})`
          console.log(`Reconnecting in ${reconnectInterval}ms (attempt ${reconnectAttempts})`)

          reconnectTimeout = setTimeout(() => {
            connect()
          }, reconnectInterval)
        } else if (reconnectAttempts >= maxReconnectAttempts) {
          error.value = 'Connection lost. Max reconnection attempts reached.'
        }
      }

      socket.onerror = (event) => {
        console.error('WebSocket error:', event)
        error.value = 'WebSocket connection error'
        options.onError?.(event)
      }
    } catch (e) {
      console.error('Failed to create WebSocket:', e)
      error.value = 'Failed to establish WebSocket connection'
    }
  }

  function disconnect() {
    isManualDisconnect = true

    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }

    if (socket) {
      socket.close(1000, 'Client disconnect')
      socket = null
    }

    isConnected.value = false
    reconnectAttempts = 0
  }

  function send(message: unknown) {
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(message))
    } else {
      console.warn('WebSocket is not connected')
    }
  }

  // Cleanup on component unmount
  onUnmounted(() => {
    disconnect()
  })

  return {
    isConnected,
    lastMessage,
    error,
    connect,
    disconnect,
    send,
  }
}
