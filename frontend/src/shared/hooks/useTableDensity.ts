import { useCallback, useEffect, useState } from 'react'

type Density = 'comfortable' | 'compact'

const KEY = 'vakt_table_density'

function read(): Density {
  return (localStorage.getItem(KEY) as Density) ?? 'comfortable'
}

function apply(d: Density) {
  document.documentElement.setAttribute('data-density', d)
}

export function useTableDensity(): [Density, () => void] {
  const [density, setDensity] = useState<Density>(read)

  useEffect(() => {
    apply(density)
  }, [density])

  const toggle = useCallback(() => {
    setDensity((prev) => {
      const next: Density = prev === 'comfortable' ? 'compact' : 'comfortable'
      localStorage.setItem(KEY, next)
      return next
    })
  }, [])

  return [density, toggle]
}
