import type { ReactNode } from 'react'
import { useCanWrite } from '../hooks/useCanWrite'

/**
 * Wraps an edit area: for users without write rights every input, select and
 * button inside is disabled by the browser (`<fieldset disabled>`), so a Viewer
 * cannot start a change that the backend would refuse anyway. Links stay usable.
 * `display: contents` keeps the surrounding layout untouched.
 */
export function WriteFieldset({ children }: { children: ReactNode }) {
  const canWrite = useCanWrite()
  return (
    <fieldset disabled={!canWrite} className="contents" data-testid="write-fieldset">
      {children}
    </fieldset>
  )
}
