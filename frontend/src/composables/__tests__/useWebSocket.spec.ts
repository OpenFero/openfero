import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// We need to mock onUnmounted before importing the composable
vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return {
    ...actual,
    onUnmounted: vi.fn(),
  }
})

import { useWebSocket } from '../useWebSocket'

// Store instances for test access
let mockInstances: MockWebSocket[] = []

// Mock WebSocket class
class MockWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  readonly CONNECTING = 0
  readonly OPEN = 1
  readonly CLOSING = 2
  readonly CLOSED = 3

  url: string
  readyState: number = MockWebSocket.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  close = vi.fn()
  send = vi.fn()

  constructor(url: string) {
    this.url = url
    mockInstances.push(this)
  }
}

describe('useWebSocket', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockInstances = []
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  // Helper to get the latest mock instance
  function getLatestMock(): MockWebSocket {
    const mock = mockInstances[mockInstances.length - 1]
    if (!mock) {
      throw new Error('No WebSocket mock instance available')
    }
    return mock
  }

  // Helper to simulate successful connection
  function simulateOpen() {
    const mock = getLatestMock()
    mock.readyState = MockWebSocket.OPEN
    mock.onopen?.(new Event('open'))
  }

  describe('initial state', () => {
    it('should start disconnected', () => {
      const { isConnected, error, lastMessage } = useWebSocket()

      expect(isConnected.value).toBe(false)
      expect(error.value).toBeNull()
      expect(lastMessage.value).toBeNull()
    })
  })

  describe('connect', () => {
    it('should create WebSocket with correct URL', () => {
      const { connect } = useWebSocket('/api/ws')
      connect()

      expect(mockInstances.length).toBe(1)
      expect(getLatestMock().url).toContain('/api/ws')
    })

    it('should set isConnected to true on open', () => {
      const { connect, isConnected } = useWebSocket()
      connect()
      simulateOpen()

      expect(isConnected.value).toBe(true)
    })

    it('should call onConnect callback when connected', () => {
      const onConnect = vi.fn()
      const { connect } = useWebSocket('/api/ws', { onConnect })
      connect()
      simulateOpen()

      expect(onConnect).toHaveBeenCalled()
    })

    it('should clear error on successful connection', () => {
      const { connect, error } = useWebSocket()
      connect()

      // Simulate error first
      getLatestMock().onerror?.(new Event('error'))
      expect(error.value).toBe('WebSocket connection error')

      // Then simulate successful connection
      simulateOpen()
      expect(error.value).toBeNull()
    })

    it('should not create new connection if already connected', () => {
      const { connect } = useWebSocket()
      connect()
      getLatestMock().readyState = MockWebSocket.OPEN

      // Try to connect again
      connect()

      // WebSocket should only be instantiated once
      expect(mockInstances.length).toBe(1)
    })
  })

  describe('message handling', () => {
    it('should parse and store incoming messages', () => {
      const { connect, lastMessage } = useWebSocket()
      connect()
      simulateOpen()

      const testMessage = {
        type: 'alert',
        data: { alertName: 'test', status: 'firing' },
      }

      getLatestMock().onmessage?.({
        data: JSON.stringify(testMessage),
      } as MessageEvent)

      expect(lastMessage.value).toEqual(testMessage)
    })

    it('should call onMessage callback with parsed message', () => {
      const onMessage = vi.fn()
      const { connect } = useWebSocket('/api/ws', { onMessage })
      connect()
      simulateOpen()

      const testMessage = {
        type: 'connected',
        data: { message: 'Connected to WebSocket' },
      }

      getLatestMock().onmessage?.({
        data: JSON.stringify(testMessage),
      } as MessageEvent)

      expect(onMessage).toHaveBeenCalledWith(testMessage)
    })

    it('should handle multiple messages separated by newlines', () => {
      const onMessage = vi.fn()
      const { connect } = useWebSocket('/api/ws', { onMessage })
      connect()
      simulateOpen()

      const message1 = { type: 'alert', data: { alertName: 'test1' } }
      const message2 = { type: 'alert', data: { alertName: 'test2' } }

      getLatestMock().onmessage?.({
        data: `${JSON.stringify(message1)}\n${JSON.stringify(message2)}`,
      } as MessageEvent)

      expect(onMessage).toHaveBeenCalledTimes(2)
      expect(onMessage).toHaveBeenCalledWith(message1)
      expect(onMessage).toHaveBeenCalledWith(message2)
    })

    it('should handle malformed JSON gracefully', () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const { connect, lastMessage } = useWebSocket()
      connect()
      simulateOpen()

      getLatestMock().onmessage?.({
        data: 'not valid json',
      } as MessageEvent)

      expect(consoleSpy).toHaveBeenCalled()
      expect(lastMessage.value).toBeNull()

      consoleSpy.mockRestore()
    })
  })

  describe('disconnect', () => {
    it('should close WebSocket with proper code', () => {
      const { connect, disconnect } = useWebSocket()
      connect()
      getLatestMock().readyState = MockWebSocket.OPEN

      disconnect()

      expect(getLatestMock().close).toHaveBeenCalledWith(1000, 'Client disconnect')
    })

    it('should set isConnected to false', () => {
      const { connect, disconnect, isConnected } = useWebSocket()
      connect()
      simulateOpen()

      expect(isConnected.value).toBe(true)

      disconnect()

      expect(isConnected.value).toBe(false)
    })

    it('should not attempt to reconnect after manual disconnect', () => {
      const { connect, disconnect } = useWebSocket('/api/ws', {
        reconnectInterval: 1000,
      })
      connect()
      simulateOpen()

      disconnect()

      // Simulate close event
      getLatestMock().onclose?.({
        code: 1000,
        reason: 'Client disconnect',
      } as CloseEvent)

      // Fast forward time
      vi.advanceTimersByTime(5000)

      // Should not try to reconnect (only initial connect call)
      expect(mockInstances.length).toBe(1)
    })
  })

  describe('reconnection', () => {
    it('should attempt to reconnect on unexpected close', async () => {
      const { connect } = useWebSocket('/api/ws', {
        reconnectInterval: 1000,
        maxReconnectAttempts: 3,
      })
      connect()
      simulateOpen()

      const initialInstanceCount = mockInstances.length
      expect(initialInstanceCount).toBe(1)

      // Change readyState to CLOSED before triggering onclose
      const currentMock = getLatestMock()
      currentMock.readyState = MockWebSocket.CLOSED

      // Simulate unexpected close
      currentMock.onclose?.({
        code: 1006,
        reason: 'Connection lost',
      } as CloseEvent)

      // Run all pending timers
      await vi.runAllTimersAsync()

      // Should try to reconnect - check that a new WebSocket was created
      expect(mockInstances.length).toBeGreaterThan(initialInstanceCount)
    })

    it('should stop reconnecting after max attempts', () => {
      const { connect, error } = useWebSocket('/api/ws', {
        reconnectInterval: 100,
        maxReconnectAttempts: 2,
      })
      connect()
      // Initial connection - don't simulate open to keep reconnect attempts accumulating

      // Simulate close without successful connection each time
      for (let i = 0; i < 3; i++) {
        getLatestMock().onclose?.({
          code: 1006,
          reason: 'Connection lost',
        } as CloseEvent)
        vi.advanceTimersByTime(100)
      }

      expect(error.value).toBe('Connection lost. Max reconnection attempts reached.')
    })
  })

  describe('send', () => {
    it('should send JSON message when connected', () => {
      const { connect, send } = useWebSocket()
      connect()
      simulateOpen()

      const message = { type: 'ping' }
      send(message)

      expect(getLatestMock().send).toHaveBeenCalledWith(JSON.stringify(message))
    })

    it('should warn when trying to send while disconnected', () => {
      const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const { send } = useWebSocket()

      send({ type: 'ping' })

      expect(consoleSpy).toHaveBeenCalledWith('WebSocket is not connected')
      consoleSpy.mockRestore()
    })
  })

  describe('error handling', () => {
    it('should set error message on error', () => {
      const { connect, error } = useWebSocket()
      connect()

      getLatestMock().onerror?.(new Event('error'))

      expect(error.value).toBe('WebSocket connection error')
    })

    it('should call onError callback', () => {
      const onError = vi.fn()
      const { connect } = useWebSocket('/api/ws', { onError })
      connect()

      const errorEvent = new Event('error')
      getLatestMock().onerror?.(errorEvent)

      expect(onError).toHaveBeenCalledWith(errorEvent)
    })
  })
})
