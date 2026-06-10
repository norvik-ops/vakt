import { useCallback, useMemo } from 'react'
import { useUpdateCheck } from './useUpdateCheck'

const STORAGE_KEY = 'vakt_last_seen_version'

export function useWhatsNew() {
  const { data: updateInfo, isLoading } = useUpdateCheck()

  const currentVersion = updateInfo?.current_version ?? null

  const isNew = useMemo(() => {
    if (!currentVersion) return false
    // SHA-based builds have no release notes — modal only for tagged v* releases
    if (currentVersion.startsWith('demo-') || currentVersion.startsWith('staging-')) return false
    const lastSeen = localStorage.getItem(STORAGE_KEY)
    return lastSeen !== currentVersion
  }, [currentVersion])

  const dismiss = useCallback(() => {
    if (currentVersion) {
      localStorage.setItem(STORAGE_KEY, currentVersion)
    }
  }, [currentVersion])

  return { isNew, currentVersion, dismiss, isLoading }
}
