import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import { useAssets } from '../hooks/useAssets'
import { apiFetch } from '../../../api/client'

interface ImportFindingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess?: (imported: number) => void
}

type ImportFormat = 'sarif' | 'cyclonedx' | 'csv'

/**
 * Uploads a scanner export for one asset and returns how many findings the
 * server accepted.
 *
 * Extracted from the component so the network contract is reachable from a
 * test: the asset picker above it is a Radix Select, which cannot be opened in
 * jsdom, so driving the dialog end-to-end would leave this request — and with
 * it the CSRF header (R1-18-D4) — untested.
 */
export async function importFindings(
  assetId: string,
  format: ImportFormat,
  file: File,
): Promise<number> {
  const formData = new FormData()
  formData.append('file', file)

  const url = `/vaktscan/findings/import?asset_id=${encodeURIComponent(assetId)}&format=${format}`

  const body = await apiFetch<{ imported?: number }>(url, {
    method: 'POST',
    body: formData,
  })

  return body.imported ?? 0
}

export function ImportFindingsDialog({
  open,
  onOpenChange,
  onSuccess,
}: ImportFindingsDialogProps) {
  const { t } = useTranslation()
  const [assetId, setAssetId] = useState('')
  const [format, setFormat] = useState<ImportFormat>('sarif')
  const [file, setFile] = useState<File | null>(null)
  const [isPending, setIsPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { data: assets } = useAssets()
  const assetList = assets ?? []

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files?.[0] ?? null
    setFile(selected)
    setError(null)
    setSuccess(null)
  }

  async function handleUpload() {
    if (!assetId) {
      setError('Please select an asset.')
      return
    }
    if (!file) {
      setError('Please select a file to upload.')
      return
    }

    setIsPending(true)
    setError(null)
    setSuccess(null)

    try {
      const imported = await importFindings(assetId, format, file)
      setSuccess(`Successfully imported ${String(imported)} finding${imported !== 1 ? 's' : ''}.`)
      setFile(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
      onSuccess?.(imported)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed.')
    } finally {
      setIsPending(false)
    }
  }

  function handleClose(open: boolean) {
    if (!open) {
      setError(null)
      setSuccess(null)
      setFile(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
    onOpenChange(open)
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t('vaktscan.importDialog.title')}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Asset selector */}
          <div className="space-y-1">
            <label className="text-sm font-medium text-primary">{t('vaktscan.importDialog.assetLabel')}</label>
            <Select value={assetId} onValueChange={setAssetId}>
              <SelectTrigger>
                <SelectValue placeholder="Asset auswahlen..." />
              </SelectTrigger>
              <SelectContent>
                {Array.isArray(assetList) && assetList.map((a) => (
                  <SelectItem key={a.id} value={a.id}>
                    {a.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Format selector */}
          <div className="space-y-1">
            <label className="text-sm font-medium text-primary">{t('vaktscan.importDialog.formatLabel')}</label>
            <Select value={format} onValueChange={(v) => { setFormat(v as ImportFormat); }}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="sarif">SARIF 2.1.0</SelectItem>
                <SelectItem value="cyclonedx">CycloneDX 1.4 (JSON)</SelectItem>
                <SelectItem value="csv">Generic CSV</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-secondary mt-1">
              {format === 'csv' && 'Erwartet Spalten: title, severity, cve_id, description, cvss_score'}
              {format === 'sarif' && 'SARIF 2.1.0 JSON — z.B. aus CodeQL, Semgrep, Trivy'}
              {format === 'cyclonedx' && 'CycloneDX 1.4 BOM JSON mit vulnerabilities-Array'}
            </p>
          </div>

          {/* File upload */}
          <div className="space-y-1">
            <label className="text-sm font-medium text-primary">{t('vaktscan.importDialog.fileLabel')}</label>
            <div className="flex items-center gap-2">
              <input
                ref={fileInputRef}
                type="file"
                accept=".json,.xml,.csv,.sarif"
                onChange={handleFileChange}
                className="hidden"
                id="import-file-input"
              />
              <label
                htmlFor="import-file-input"
                className="flex items-center gap-2 px-3 py-2 rounded-md border border-border bg-surface2 text-sm cursor-pointer hover:bg-surface transition-colors"
              >
                <Upload className="w-4 h-4" />
                {file ? file.name : 'Datei auswahlen...'}
              </label>
            </div>
            {file && (
              <p className="text-xs text-secondary">
                {(file.size / 1024).toFixed(1)} KB
              </p>
            )}
          </div>

          {/* Feedback */}
          {error && (
            <p className="text-sm text-red-500 bg-red-500/10 rounded-md px-3 py-2">{error}</p>
          )}
          {success && (
            <p className="text-sm text-green-500 bg-green-500/10 rounded-md px-3 py-2">{success}</p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => { handleClose(false); }} disabled={isPending}>
            Abbrechen
          </Button>
          <Button onClick={() => { void handleUpload() }} disabled={isPending || !file || !assetId}>
            {isPending ? 'Importieren...' : 'Importieren'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
