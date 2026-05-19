import { useState } from 'react'
import { Download } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { toast } from '../hooks/useToast'

interface ExportButtonProps {
  endpoint: string
  filename: string
  label?: string
  format?: 'csv' | 'xlsx'
}

export function ExportButton({
  endpoint,
  filename,
  label = 'Exportieren',
  format = 'xlsx',
}: ExportButtonProps) {
  const [isLoading, setIsLoading] = useState(false)

  async function handleClick() {
    setIsLoading(true)
    try {
      const res = await fetch(endpoint, {
        credentials: 'include',
      })

      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as { error?: string }
        throw new Error(body.error ?? `HTTP ${res.status.toString()}`)
      }

      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename.endsWith(`.${format}`) ? filename : `${filename}.${format}`
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      toast(
        err instanceof Error ? err.message : 'Export fehlgeschlagen',
        'error',
      )
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Button
      variant="outline"
      size="sm"
      onClick={() => { void handleClick() }}
      disabled={isLoading}
      aria-label={label}
    >
      {isLoading ? (
        <>
          <div className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin mr-1.5" aria-hidden="true" />
          Wird exportiert…
        </>
      ) : (
        <>
          <Download className="w-3.5 h-3.5 mr-1" aria-hidden="true" />
          {label}
        </>
      )}
    </Button>
  )
}
