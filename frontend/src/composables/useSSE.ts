import { ref } from 'vue'
import type { AlertStoreEntry } from '@/types'

export interface SSEMessage {
  type: 'alert' | 'job_status' | 'connected'
  data: AlertStoreEntry | { jobName: string; status: string } | { message: string }
}

export interface SSEOptions {
  onMessage?: (message: SSEMessage) => void
  onConnect?: () => void
  onError?: (error: string) => void
}

export function useSSE(url: string = '/api/events', options: SSEOptions = {}) {
  const isConnected = ref(false)
  const lastMessage = ref<SSEMessage | null>(null)
  const error = ref<string | null>(null)
  let eventSource: EventSource | null = null

  function connect() {
    if (eventSource) {
      eventSource.close()
    }

    eventSource = new EventSource(url)

    eventSource.onopen = () => {
      isConnected.value = true
      error.value = null
      options.onConnect?.()
    }

    eventSource.onmessage = (event) => {
      try {
        const message: SSEMessage = JSON.parse(event.data)
        lastMessage.value = message
        options.onMessage?.(message)
      } catch (e) {
        console.error('Failed to parse SSE message:', e)
      }
    }

    eventSource.onerror = () => {
      isConnected.value = false
      error.value = 'SSE connection lost, attempting to reconnect...'
      options.onError?.(error.value)
      // EventSource will automatically attempt to reconnect
    }
  }

  function disconnect() {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
    isConnected.value = false
  }

  return {
    isConnected,
    lastMessage,
    error,
    connect,
    disconnect,
  }
}
