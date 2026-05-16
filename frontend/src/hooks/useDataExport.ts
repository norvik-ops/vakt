import { useState } from 'react'
import { getAuthToken } from '../api/client'

/**
 * useExportData returns a function that triggers a full-data ZIP download
 * and loading/error state for UI feedback.
 */
export function useExportData() {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function exportData() {
    setIsLoading(true)
    setError(null)
    try {
      const token = getAuthToken() ?? ''
      const res = await fetch('/api/v1/export/full', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        throw new Error(`Export fehlgeschlagen (${res.status})`)
      }
      const blob = await res.blob()
      const cd = res.headers.get('content-disposition') ?? ''
      const filename = cd.match(/filename="([^"]+)"/)?.[1] ?? 'vakt-export.zip'
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Export fehlgeschlagen')
    } finally {
      setIsLoading(false)
    }
  }

  return { exportData, isLoading, error }
}
