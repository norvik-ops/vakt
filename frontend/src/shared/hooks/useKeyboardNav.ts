import { useEffect, useCallback } from 'react'

interface UseKeyboardNavOptions {
  itemCount: number
  onUp?: () => void
  onDown?: () => void
  onSelect?: (index: number) => void
  onSearch?: () => void
  onEdit?: (index: number) => void
  enabled?: boolean
}

export function useKeyboardNav(
  currentIndex: number,
  setIndex: (i: number) => void,
  options: UseKeyboardNavOptions
) {
  const { itemCount, onSelect, onSearch, onEdit, enabled = true } = options

  const handleKey = useCallback((e: KeyboardEvent) => {
    if (!enabled) return
    // Don't intercept when typing in an input/textarea/select
    const target = e.target as HTMLElement
    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable) {
      return
    }

    switch (e.key) {
      case 'j':
      case 'ArrowDown':
        e.preventDefault()
        setIndex(Math.min(currentIndex + 1, itemCount - 1))
        break
      case 'k':
      case 'ArrowUp':
        e.preventDefault()
        setIndex(Math.max(currentIndex - 1, 0))
        break
      case 'Enter':
        if (onSelect && currentIndex >= 0) {
          e.preventDefault()
          onSelect(currentIndex)
        }
        break
      case 'e':
        if (onEdit && currentIndex >= 0) {
          e.preventDefault()
          onEdit(currentIndex)
        }
        break
      case '/':
        if (onSearch) {
          e.preventDefault()
          onSearch()
        }
        break
      case 'Escape':
        setIndex(-1)
        break
    }
  }, [currentIndex, itemCount, onSelect, onSearch, onEdit, enabled, setIndex])

  useEffect(() => {
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [handleKey])
}
