import { getCurrentInstance, onMounted, onUnmounted, ref } from 'vue'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'openfero-theme'

// Shared reactive theme state across all callers
const theme = ref<Theme>('light')
let activeListeners = 0
let mediaQuery: MediaQueryList | null = null

function getSystemTheme(): Theme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function getStoredTheme(): Theme | null {
  const stored = localStorage.getItem(STORAGE_KEY)
  return stored === 'light' || stored === 'dark' ? stored : null
}

function applyTheme(newTheme: Theme) {
  if (newTheme === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
  theme.value = newTheme
}

function handleSystemThemeChange(e: MediaQueryListEvent) {
  // Only auto-switch if no user preference is stored
  if (!getStoredTheme()) {
    applyTheme(e.matches ? 'dark' : 'light')
  }
}

export function useTheme() {
  function toggleTheme() {
    const newTheme = theme.value === 'light' ? 'dark' : 'light'
    localStorage.setItem(STORAGE_KEY, newTheme)
    applyTheme(newTheme)
  }

  function initTheme() {
    const stored = getStoredTheme()
    const initial = stored || getSystemTheme()
    applyTheme(initial)
  }

  // Register lifecycle hooks only when called inside a component setup()
  if (getCurrentInstance()) {
    onMounted(() => {
      initTheme()
      if (activeListeners === 0) {
        mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
        mediaQuery.addEventListener?.('change', handleSystemThemeChange)
      }
      activeListeners++
    })

    onUnmounted(() => {
      activeListeners = Math.max(0, activeListeners - 1)
      if (activeListeners === 0 && mediaQuery) {
        mediaQuery.removeEventListener?.('change', handleSystemThemeChange)
        mediaQuery = null
      }
    })
  }

  return {
    theme,
    toggleTheme,
    initTheme,
  }
}
