import { useState, useCallback } from 'react'

const STORAGE_KEY = 'vakt_favorites'

function readFavorites(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as string[]) : []
  } catch {
    return []
  }
}

function writeFavorites(favorites: string[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(favorites))
  } catch {
    // localStorage may be unavailable
  }
}

export function useFavorites(): {
  favorites: string[]
  isFavorite: (path: string) => boolean
  toggleFavorite: (path: string) => void
} {
  const [favorites, setFavorites] = useState<string[]>(readFavorites)

  const isFavorite = useCallback(
    (path: string) => favorites.includes(path),
    [favorites],
  )

  const toggleFavorite = useCallback((path: string) => {
    setFavorites((prev) => {
      const next = prev.includes(path)
        ? prev.filter((p) => p !== path)
        : [...prev, path]
      writeFavorites(next)
      return next
    })
  }, [])

  return { favorites, isFavorite, toggleFavorite }
}
