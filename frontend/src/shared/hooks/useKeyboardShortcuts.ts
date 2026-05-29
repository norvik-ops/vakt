import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'

function isInputFocused(): boolean {
  const el = document.activeElement
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || (el as HTMLElement).isContentEditable
}

interface UseKeyboardShortcutsOptions {
  onOpenHelp?: () => void
}

export function useKeyboardShortcuts({ onOpenHelp }: UseKeyboardShortcutsOptions = {}) {
  const navigate = useNavigate()
  // Track 'g' prefix for goto shortcuts
  const gPressedRef = useRef(false)
  const gTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      // Cmd/Ctrl+K → open global search
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        window.dispatchEvent(new CustomEvent('vakt:open-search'))
        return
      }

      // Escape → close open modals
      if (e.key === 'Escape') {
        window.dispatchEvent(new CustomEvent('vakt:close-modals'))
        return
      }

      // Don't handle single-char shortcuts when input is focused
      if (isInputFocused()) return

      // '?' → open help modal
      if (e.key === '?') {
        e.preventDefault()
        onOpenHelp?.()
        return
      }

      // 'g' prefix: start goto sequence
      if (e.key === 'g') {
        // If already in a g-sequence, navigate to dashboard
        gPressedRef.current = true
        if (gTimerRef.current) clearTimeout(gTimerRef.current)
        gTimerRef.current = setTimeout(() => {
          gPressedRef.current = false
        }, 500)
        return
      }

      // Handle second key in goto sequence
      if (gPressedRef.current) {
        gPressedRef.current = false
        if (gTimerRef.current) clearTimeout(gTimerRef.current)

        if (e.key === 'd') {
          e.preventDefault()
          navigate('/')
        } else if (e.key === 'f') {
          e.preventDefault()
          navigate('/vaktscan/findings')
        } else if (e.key === 'r') {
          e.preventDefault()
          navigate('/vaktcomply/risks')
        } else if (e.key === 'i') {
          e.preventDefault()
          navigate('/vaktcomply/incidents')
        }
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      if (gTimerRef.current) clearTimeout(gTimerRef.current)
    }
  }, [navigate, onOpenHelp])
}
