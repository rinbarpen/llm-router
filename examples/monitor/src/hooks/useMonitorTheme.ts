import { useCallback, useEffect, useMemo, useState } from 'react'

export type ThemeMode = 'light' | 'dark'

const STORAGE_KEY = 'llm-router-monitor-theme'

function getInitialTheme(): ThemeMode {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'light' || stored === 'dark') return stored
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function useMonitorTheme() {
  const [themeMode, setThemeMode] = useState<ThemeMode>(getInitialTheme)

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, themeMode)
    const isDark = themeMode === 'dark'
    document.documentElement.classList.toggle('dark', isDark)
    document.documentElement.dataset.theme = themeMode
  }, [themeMode])

  const toggleTheme = useCallback(() => {
    setThemeMode((prev) => (prev === 'light' ? 'dark' : 'light'))
  }, [])

  return useMemo(
    () => ({
      themeMode,
      toggleTheme,
    }),
    [themeMode, toggleTheme],
  )
}
