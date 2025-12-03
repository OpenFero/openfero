import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useTheme } from '../useTheme'

describe('useTheme', () => {
  let mockMatchMedia: ReturnType<typeof vi.fn>
  let mockAddEventListener: ReturnType<typeof vi.fn>
  let mockLocalStorage: { [key: string]: string }

  beforeEach(() => {
    // Reset document class
    document.documentElement.className = ''

    // Mock localStorage
    mockLocalStorage = {}
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(
      (key: string) => mockLocalStorage[key] || null,
    )
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation((key: string, value: string) => {
      mockLocalStorage[key] = value
    })

    // Mock matchMedia
    mockAddEventListener = vi.fn()
    mockMatchMedia = vi.fn().mockReturnValue({
      matches: false, // Default to light mode
      addEventListener: mockAddEventListener,
    })
    Object.defineProperty(window, 'matchMedia', {
      value: mockMatchMedia,
      writable: true,
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    document.documentElement.className = ''
  })

  describe('theme state', () => {
    it('should initialize with light theme by default', () => {
      const { theme } = useTheme()
      expect(theme.value).toBe('light')
    })
  })

  describe('toggleTheme', () => {
    it('should toggle from light to dark', () => {
      const { theme, toggleTheme, initTheme } = useTheme()
      initTheme()

      expect(theme.value).toBe('light')

      toggleTheme()

      expect(theme.value).toBe('dark')
      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })

    it('should toggle from dark to light', () => {
      mockLocalStorage['openfero-theme'] = 'dark'
      const { theme, toggleTheme, initTheme } = useTheme()
      initTheme()

      expect(theme.value).toBe('dark')

      toggleTheme()

      expect(theme.value).toBe('light')
      expect(document.documentElement.classList.contains('dark')).toBe(false)
    })

    it('should persist theme preference to localStorage', () => {
      const { toggleTheme, initTheme } = useTheme()
      initTheme()

      toggleTheme()

      expect(localStorage.setItem).toHaveBeenCalledWith('openfero-theme', 'dark')
    })
  })

  describe('initTheme', () => {
    it('should use stored theme if available', () => {
      mockLocalStorage['openfero-theme'] = 'dark'

      const { theme, initTheme } = useTheme()
      initTheme()

      expect(theme.value).toBe('dark')
      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })

    it('should use system theme if no stored preference', () => {
      mockMatchMedia.mockReturnValue({
        matches: true, // System prefers dark
        addEventListener: mockAddEventListener,
      })

      const { theme, initTheme } = useTheme()
      initTheme()

      expect(theme.value).toBe('dark')
    })

    it('should add dark class to html element for dark theme', () => {
      mockLocalStorage['openfero-theme'] = 'dark'

      const { initTheme } = useTheme()
      initTheme()

      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })

    it('should remove dark class for light theme', () => {
      document.documentElement.classList.add('dark')
      mockLocalStorage['openfero-theme'] = 'light'

      const { initTheme } = useTheme()
      initTheme()

      expect(document.documentElement.classList.contains('dark')).toBe(false)
    })
  })

  describe('getStoredTheme', () => {
    it('should return null for invalid stored values', () => {
      mockLocalStorage['openfero-theme'] = 'invalid-theme'

      const { initTheme, theme } = useTheme()
      initTheme()

      // Should fall back to system theme (light in this mock)
      expect(theme.value).toBe('light')
    })
  })

  describe('system theme detection', () => {
    it('should detect dark system preference', () => {
      mockMatchMedia.mockReturnValue({
        matches: true,
        addEventListener: mockAddEventListener,
      })

      const { theme, initTheme } = useTheme()
      initTheme()

      expect(theme.value).toBe('dark')
    })

    it('should detect light system preference', () => {
      mockMatchMedia.mockReturnValue({
        matches: false,
        addEventListener: mockAddEventListener,
      })

      const { theme, initTheme } = useTheme()
      initTheme()

      expect(theme.value).toBe('light')
    })
  })
})
