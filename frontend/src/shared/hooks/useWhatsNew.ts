import { useCallback, useMemo } from 'react'
import { useUpdateCheck } from './useUpdateCheck'

const STORAGE_KEY = 'vakt_last_seen_version'

export function useWhatsNew() {
  const { data: updateInfo } = useUpdateCheck()

  const currentVersion = updateInfo?.current_version ?? null

  const isNew = useMemo(() => {
    if (!currentVersion) return false
    const lastSeen = localStorage.getItem(STORAGE_KEY)
    return lastSeen !== currentVersion
  }, [currentVersion])

  const dismiss = useCallback(() => {
    if (currentVersion) {
      localStorage.setItem(STORAGE_KEY, currentVersion)
    }
  }, [currentVersion])

  return { isNew, currentVersion, dismiss }
}
