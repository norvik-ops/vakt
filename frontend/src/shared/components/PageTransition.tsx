import { useLocation } from 'react-router-dom'

interface PageTransitionProps {
  children: React.ReactNode
}

/**
 * Fades the page content in on each route change.
 *
 * ponytail: CSS-only (S98-1). framer-motion's AnimatePresence previously drove an
 * enter+exit transition, but keeping framer-motion on this always-eager path pinned
 * ~41 KiB gzip in the initial bundle. Route *exit* animation is dropped (standard for
 * SPAs); the `key` on location.pathname remounts the node so the enter fade replays
 * on every navigation.
 */
export function PageTransition({ children }: PageTransitionProps) {
  const location = useLocation()
  return (
    <div key={location.pathname} className="vakt-fade-in h-full">
      {children}
    </div>
  )
}
