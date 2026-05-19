import { useState, useEffect, useCallback, useMemo } from 'react'
import { useBlocker } from 'react-router-dom'
import React from 'react'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '../../components/ui/alert-dialog'

interface UseUnsavedChangesReturn {
  isDirty: boolean
  markDirty: () => void
  markClean: () => void
  ConfirmDialog: React.FC
}

export function useUnsavedChanges(): UseUnsavedChangesReturn {
  const [isDirty, setIsDirty] = useState(false)

  const markDirty = useCallback(() => setIsDirty(true), [])
  const markClean = useCallback(() => setIsDirty(false), [])

  useEffect(() => {
    if (!isDirty) return
    function handleBeforeUnload(e: BeforeUnloadEvent) {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [isDirty])

  const blocker = useBlocker(isDirty)

  const handleDiscard = useCallback(() => {
    setIsDirty(false)
    blocker.proceed?.()
  }, [blocker])

  const handleStay = useCallback(() => {
    blocker.reset?.()
  }, [blocker])

  const ConfirmDialog: React.FC = useMemo(
    () =>
      function UnsavedChangesDialog() {
        if (blocker.state !== 'blocked') return null
        return React.createElement(
          AlertDialog,
          { open: true },
          React.createElement(
            AlertDialogContent,
            null,
            React.createElement(
              AlertDialogHeader,
              null,
              React.createElement(AlertDialogTitle, null, 'Nicht gespeicherte Änderungen'),
              React.createElement(
                AlertDialogDescription,
                null,
                'Du hast ungespeicherte Änderungen. Möchtest du die Seite wirklich verlassen?',
              ),
            ),
            React.createElement(
              AlertDialogFooter,
              null,
              React.createElement(AlertDialogCancel, { onClick: handleStay }, 'Weiter bearbeiten'),
              React.createElement(
                AlertDialogAction,
                {
                  onClick: handleDiscard,
                  className: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
                },
                'Verwerfen',
              ),
            ),
          ),
        )
      },
    [blocker.state, handleDiscard, handleStay],
  )

  return { isDirty, markDirty, markClean, ConfirmDialog }
}
