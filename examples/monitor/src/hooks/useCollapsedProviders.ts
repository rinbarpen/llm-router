import { useState, useCallback } from 'react'

function loadFromStorage(key: string): Set<string> {
  try {
    const saved = localStorage.getItem(key)
    if (saved) {
      const array = JSON.parse(saved) as string[]
      return new Set(array)
    }
  } catch (error) {
    console.error('Failed to load collapsed providers:', error)
  }
  return new Set()
}

function saveToStorage(key: string, collapsed: Set<string>) {
  try {
    localStorage.setItem(key, JSON.stringify(Array.from(collapsed)))
  } catch (error) {
    console.error('Failed to save collapsed providers:', error)
  }
}

/**
 * 持久化到 localStorage 的折叠集合
 * @param storageKey localStorage 键名
 * @returns [collapsedSet, toggleCollapse]
 */
export function useCollapsedProviders(storageKey: string): [Set<string>, (providerName: string, collapsed: boolean) => void] {
  const [collapsed, setCollapsed] = useState<Set<string>>(() => loadFromStorage(storageKey))

  const toggleCollapse = useCallback(
    (providerName: string, isCollapsed: boolean) => {
      setCollapsed((prev) => {
        const next = new Set(prev)
        if (isCollapsed) {
          next.add(providerName)
        } else {
          next.delete(providerName)
        }
        saveToStorage(storageKey, next)
        return next
      })
    },
    [storageKey]
  )

  return [collapsed, toggleCollapse]
}
