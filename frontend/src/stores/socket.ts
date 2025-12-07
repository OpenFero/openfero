import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { AlertStoreEntry, JobInfo } from '@/types'

export interface WSMessage {
  type: 'alert' | 'operarius_update' | 'connected'
  data: AlertStoreEntry | JobInfo | { message: string }
}

export const useSocketStore = defineStore('socket', () => {
  const isConnected = ref(false)
  const isPaused = ref(false)
  const error = ref<string | null>(null)
  let socket: WebSocket | null = null
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null

  // Event listeners
  const listeners = new Set<(message: WSMessage) => void>()

  function getWebSocketUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    return `${protocol}//${host}/api/ws`
  }

  function connect() {
    if (socket?.readyState === WebSocket.OPEN) return

    isPaused.value = false
    const url = getWebSocketUrl()
    socket = new WebSocket(url)

    socket.onopen = () => {
      isConnected.value = true
      error.value = null
      console.log('WebSocket connected')
    }

    socket.onclose = () => {
      isConnected.value = false
      console.log('WebSocket disconnected')
      if (!isPaused.value) {
        scheduleReconnect()
      }
    }

    socket.onerror = (e) => {
      console.error('WebSocket error:', e)
      error.value = 'Connection error'
    }

    socket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as WSMessage
        listeners.forEach((listener) => {
          listener(message)
        })
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e)
      }
    }
  }

  function disconnect() {
    isPaused.value = true
    if (socket) {
      // Remove listeners to prevent race conditions with new connections
      socket.onclose = null
      socket.onmessage = null
      socket.onerror = null
      socket.close()
      socket = null
    }
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
    isConnected.value = false
  }

  function toggleConnection() {
    console.log('SocketStore: toggleConnection called. isConnected:', isConnected.value)
    if (isConnected.value) {
      disconnect()
    } else {
      connect()
    }
  }

  function scheduleReconnect() {
    if (isPaused.value) return
    if (reconnectTimeout) clearTimeout(reconnectTimeout)
    reconnectTimeout = setTimeout(() => {
      connect()
    }, 3000)
  }

  function addListener(callback: (message: WSMessage) => void) {
    listeners.add(callback)
    return () => listeners.delete(callback)
  }

  return {
    isConnected,
    isPaused,
    error,
    connect,
    disconnect,
    toggleConnection,
    addListener,
  }
})
