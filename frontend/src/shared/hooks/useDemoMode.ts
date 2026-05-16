import { useEffect, useState } from 'react'

let cachedDemo: boolean | null = null

export function useDemoMode(): boolean | null {
  const [demo, setDemo] = useState<boolean | null>(cachedDemo)

  useEffect(() => {
    if (cachedDemo !== null) return
    fetch('/health')
      .then((r) => r.json())
      .then((d: { demo?: boolean }) => {
        cachedDemo = d.demo === true
        setDemo(cachedDemo)
      })
      .catch(() => {
        cachedDemo = false
        setDemo(false)
      })
  }, [])

  return demo
}
