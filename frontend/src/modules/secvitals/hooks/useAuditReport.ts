import { useState } from 'react'
import { getAuthToken, FeatureLockedError } from '../../../api/client'

export function useAuditReport() {
  const [isGenerating, setIsGenerating] = useState(false)
  const [error, setError] = useState<unknown>(null)

  async function generate() {
    setIsGenerating(true)
    setError(null)
    try {
      const token = getAuthToken() ?? ''
      const res = await fetch('/api/v1/secvitals/audit-report', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.status === 402) throw new FeatureLockedError('audit-report')
      if (!res.ok) throw new Error('Bericht konnte nicht generiert werden.')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `vakt-audit-${new Date().toISOString().slice(0, 10)}.pdf`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(err)
    } finally {
      setIsGenerating(false)
    }
  }

  return { generate, isGenerating, error }
}
