import { onMounted, ref } from 'vue'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'openfero-theme'

export function useTheme() {
  const theme = ref<Theme>('light')

  function getSystemTheme(): Theme {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }

  function getStoredTheme(): Theme | null {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored === 'light' || stored === 'dark' ? stored : null
  }

  function applyTheme(newTheme: Theme) {
    // Apply Tailwind dark mode class
    if (newTheme === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
    theme.value = newTheme
  }

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

  // Watch for system theme changes
  onMounted(() => {
    initTheme()

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', (e) => {
      // Only auto-switch if no user preference is stored
      if (!getStoredTheme()) {
        applyTheme(e.matches ? 'dark' : 'light')
      }
    })
  })

  return {
    theme,
    toggleTheme,
    initTheme,
  }
}
