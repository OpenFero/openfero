import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { useDateTime } from '../useDateTime'

describe('useDateTime', () => {
  // Mock navigator.language
  const originalNavigator = global.navigator

  beforeEach(() => {
    // Mock navigator with en-US locale for consistent tests
    Object.defineProperty(global, 'navigator', {
      value: {
        language: 'en-US',
        languages: ['en-US'],
      },
      writable: true,
    })
  })

  afterEach(() => {
    Object.defineProperty(global, 'navigator', {
      value: originalNavigator,
      writable: true,
    })
  })

  describe('formatDateTime', () => {
    it('should format a Date object', () => {
      const { formatDateTime } = useDateTime()
      const date = new Date('2025-12-04T14:30:00.123Z')
      const result = formatDateTime(date)

      // Should contain date parts
      expect(result).toMatch(/Dec/)
      expect(result).toMatch(/04/)
      expect(result).toMatch(/2025/)
    })

    it('should format an ISO string', () => {
      const { formatDateTime } = useDateTime()
      const result = formatDateTime('2025-12-04T14:30:00.123Z')

      expect(result).toMatch(/Dec/)
      expect(result).toMatch(/04/)
      expect(result).toMatch(/2025/)
    })

    it('should format a timestamp number', () => {
      const { formatDateTime } = useDateTime()
      const timestamp = new Date('2025-12-04T14:30:00.000Z').getTime()
      const result = formatDateTime(timestamp)

      expect(result).toMatch(/Dec/)
      expect(result).toMatch(/2025/)
    })

    it('should include milliseconds by default', () => {
      const { formatDateTime } = useDateTime()
      const result = formatDateTime('2025-12-04T14:30:00.123Z')

      // Should contain milliseconds pattern
      expect(result).toMatch(/\d{2}:\d{2}:\d{2}\.\d{3}/)
    })

    it('should exclude milliseconds when option is false', () => {
      const { formatDateTime } = useDateTime()
      const result = formatDateTime('2025-12-04T14:30:00.123Z', {
        includeMilliseconds: false,
      })

      // Should not contain milliseconds pattern (no .XXX after seconds)
      expect(result).not.toMatch(/\d{2}:\d{2}:\d{2}\.\d{3}/)
    })

    it('should return "Invalid date" for invalid input', () => {
      const { formatDateTime } = useDateTime()
      const result = formatDateTime('not-a-date')

      expect(result).toBe('Invalid date')
    })

    it('should use dateStyle/timeStyle when provided', () => {
      const { formatDateTime } = useDateTime()
      const result = formatDateTime('2025-12-04T14:30:00Z', {
        dateStyle: 'short',
        timeStyle: 'short',
      })

      // Should be a shorter format
      expect(result.length).toBeLessThan(50)
    })
  })

  describe('formatDate', () => {
    it('should format date only (no time)', () => {
      const { formatDate } = useDateTime()
      const result = formatDate('2025-12-04T14:30:00Z')

      expect(result).toMatch(/Dec/)
      expect(result).toMatch(/2025/)
      // Should not contain time separator
      expect(result).not.toMatch(/\d{2}:\d{2}/)
    })
  })

  describe('formatTime', () => {
    it('should format time only', () => {
      const { formatTime } = useDateTime()
      const result = formatTime('2025-12-04T14:30:45Z')

      // Should contain time pattern
      expect(result).toMatch(/\d{1,2}:\d{2}/)
    })

    it('should include seconds by default', () => {
      const { formatTime } = useDateTime()
      const result = formatTime('2025-12-04T14:30:45Z')

      // Should contain seconds (HH:MM:SS pattern)
      expect(result).toMatch(/\d{1,2}:\d{2}:\d{2}/)
    })

    it('should exclude seconds when option is false', () => {
      const { formatTime } = useDateTime()
      const result = formatTime('2025-12-04T14:30:45Z', false)

      // Count colons - should be only 1 (HH:MM)
      const colonCount = (result.match(/:/g) || []).length
      expect(colonCount).toBe(1)
    })

    it('should return "Invalid time" for invalid input', () => {
      const { formatTime } = useDateTime()
      const result = formatTime('not-a-time')

      expect(result).toBe('Invalid time')
    })
  })

  describe('formatRelative', () => {
    it('should format time in the past', () => {
      const { formatRelative } = useDateTime()
      const pastDate = new Date(Date.now() - 60 * 60 * 1000) // 1 hour ago
      const result = formatRelative(pastDate)

      expect(result).toMatch(/hour|ago/i)
    })

    it('should format time in the future', () => {
      const { formatRelative } = useDateTime()
      const futureDate = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000) // 2 days from now
      const result = formatRelative(futureDate)

      expect(result).toMatch(/day|in/i)
    })

    it('should handle "just now" for very recent times', () => {
      const { formatRelative } = useDateTime()
      const now = new Date()
      const result = formatRelative(now)

      expect(result).toMatch(/now|second/i)
    })

    it('should return "Invalid date" for invalid input', () => {
      const { formatRelative } = useDateTime()
      const result = formatRelative('invalid')

      expect(result).toBe('Invalid date')
    })
  })

  describe('toISOString', () => {
    it('should convert Date to ISO string', () => {
      const { toISOString } = useDateTime()
      const date = new Date('2025-12-04T14:30:00.000Z')
      const result = toISOString(date)

      expect(result).toBe('2025-12-04T14:30:00.000Z')
    })

    it('should handle string input', () => {
      const { toISOString } = useDateTime()
      const result = toISOString('2025-12-04T14:30:00.000Z')

      expect(result).toBe('2025-12-04T14:30:00.000Z')
    })

    it('should handle timestamp number', () => {
      const { toISOString } = useDateTime()
      const timestamp = new Date('2025-12-04T14:30:00.000Z').getTime()
      const result = toISOString(timestamp)

      expect(result).toBe('2025-12-04T14:30:00.000Z')
    })
  })

  describe('userLocale', () => {
    it('should return the browser locale', () => {
      const { userLocale } = useDateTime()

      expect(userLocale.value).toBe('en-US')
    })
  })
})
