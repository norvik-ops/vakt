import { useState, useRef, useCallback } from 'react'
import { Upload, Download, ChevronDown, ChevronUp } from 'lucide-react'
import { Button } from '../../components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../components/ui/dialog'
import { Progress } from '../../components/ui/progress'

// ─── Types ────────────────────────────────────────────────────────────────────

export interface CSVImportDialogProps {
  open: boolean
  onClose: () => void
  /** Backend endpoint, e.g. "/api/v1/vaktscan/findings/import/csv" */
  endpoint: string
  entityLabel: string
  /** Column names for template generation and header validation */
  columns: string[]
  /** Called after a successful import */
  onSuccess?: () => void
}

type Step = 'upload' | 'preview' | 'result'

interface ImportResult {
  imported: number
  skipped: number
  errors: string[]
}

// ─── CSV Parsing ──────────────────────────────────────────────────────────────

function parseCSV(text: string): string[][] {
  const lines = text.trim().split('\n')
  return lines.map((line) => {
    const row: string[] = []
    let current = ''
    let inQuote = false
    for (let i = 0; i < line.length; i++) {
      const ch = line[i]
      if (ch === '"') {
        inQuote = !inQuote
      } else if (ch === ',' && !inQuote) {
        row.push(current.trim())
        current = ''
      } else {
        current += ch
      }
    }
    row.push(current.trim())
    return row
  })
}

// ─── Template Download ────────────────────────────────────────────────────────

function downloadTemplate(columns: string[], entityLabel: string) {
  const csv = columns.join(',') + '\n'
  const blob = new Blob([csv], { type: 'text/csv' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${entityLabel.toLowerCase()}-template.csv`
  a.click()
  URL.revokeObjectURL(url)
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function CSVImportDialog({
  open,
  onClose,
  endpoint,
  entityLabel,
  columns,
  onSuccess,
}: CSVImportDialogProps) {
  const [step, setStep] = useState<Step>('upload')
  const [isDragOver, setIsDragOver] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [previewRows, setPreviewRows] = useState<string[][]>([])
  const [headers, setHeaders] = useState<string[]>([])
  const [invalidCols, setInvalidCols] = useState<Set<number>>(new Set())
  const [isUploading, setIsUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [result, setResult] = useState<ImportResult | null>(null)
  const [errorsOpen, setErrorsOpen] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Reset state when dialog closes/re-opens
  function reset() {
    setStep('upload')
    setFile(null)
    setPreviewRows([])
    setHeaders([])
    setInvalidCols(new Set())
    setIsUploading(false)
    setUploadProgress(0)
    setResult(null)
    setErrorsOpen(false)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  function handleClose() {
    reset()
    onClose()
  }

  function handleOpenChange(v: boolean) {
    if (!v) handleClose()
  }

  // ── File selection ─────────────────────────────────────────────────────────

  function processFile(selectedFile: File) {
    setFile(selectedFile)

    const reader = new FileReader()
    reader.onload = (e) => {
      const text = e.target?.result as string
      const rows = parseCSV(text)
      if (rows.length === 0) return

      const hdrs = rows[0]
      const dataRows = rows.slice(1, 6) // preview up to 5 rows

      // Identify invalid columns (not in expected columns list)
      const invalid = new Set<number>()
      hdrs.forEach((h, i) => {
        if (!columns.includes(h.toLowerCase().trim())) {
          invalid.add(i)
        }
      })

      setHeaders(hdrs)
      setPreviewRows(dataRows)
      setInvalidCols(invalid)
      setStep('preview')
    }
    reader.readAsText(selectedFile)
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0]
    if (f) processFile(f)
  }

  const handleDrop = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragOver(false)
    const f = e.dataTransfer.files.item(0)
    if (f) processFile(f)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  function handleDragOver(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault()
    setIsDragOver(true)
  }

  function handleDragLeave() {
    setIsDragOver(false)
  }

  // ── Upload ─────────────────────────────────────────────────────────────────

  async function handleUpload() {
    if (!file) return
    setIsUploading(true)
    setUploadProgress(10)

    const formData = new FormData()
    formData.append('file', file)

    try {
      // Simulate progress since fetch doesn't support upload progress natively
      const timer = setInterval(() => {
        setUploadProgress((p) => Math.min(p + 15, 85))
      }, 200)

      const res = await fetch(endpoint, {
        method: 'POST',
        credentials: 'include',
        body: formData,
      })

      clearInterval(timer)
      setUploadProgress(100)

      const body = (await res.json().catch(() => ({}))) as {
        imported?: number
        inserted?: number
        skipped?: number
        errored?: number
        errors?: string[]
        error?: string
      }

      if (!res.ok) {
        throw new Error(body.error ?? `HTTP ${String(res.status)}`)
      }

      setResult({
        imported: body.imported ?? body.inserted ?? 0,
        skipped: body.skipped ?? body.errored ?? 0,
        errors: body.errors ?? [],
      })
      setStep('result')
      if ((body.imported ?? body.inserted ?? 0) > 0) {
        onSuccess?.()
      }
    } catch (err) {
      setResult({
        imported: 0,
        skipped: 0,
        errors: [err instanceof Error ? err.message : 'Upload fehlgeschlagen'],
      })
      setStep('result')
    } finally {
      setIsUploading(false)
    }
  }

  // ── Render ─────────────────────────────────────────────────────────────────

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{entityLabel} importieren</DialogTitle>
        </DialogHeader>

        {/* Step 1: Upload */}
        {step === 'upload' && (
          <div className="space-y-4 py-2">
            {/* Drop zone */}
            <div
              onDrop={handleDrop}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onClick={() => fileInputRef.current?.click()}
              className={`
                border-2 border-dashed rounded-xl p-10 flex flex-col items-center justify-center gap-3
                cursor-pointer transition-colors
                ${isDragOver
                  ? 'border-brand bg-brand/5'
                  : 'border-border hover:border-brand/40 hover:bg-surface2'
                }
              `}
            >
              <Upload className="w-8 h-8 text-secondary" aria-hidden="true" />
              <div className="text-center">
                <p className="text-sm font-medium text-primary">CSV-Datei hierher ziehen</p>
                <p className="text-xs text-secondary mt-0.5">oder klicken zum Auswählen (.csv)</p>
              </div>
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv,.xlsx,.xls"
                className="hidden"
                onChange={handleFileChange}
              />
            </div>

            {/* Template download */}
            <div className="flex items-center justify-between text-xs text-secondary border border-border rounded-lg px-4 py-2.5 bg-surface2">
              <span>
                Erwartete Spalten:{' '}
                <code className="font-mono text-primary">{columns.join(', ')}</code>
              </span>
              <button
                type="button"
                onClick={() => { downloadTemplate(columns, entityLabel); }}
                className="flex items-center gap-1 text-brand hover:underline font-medium"
              >
                <Download className="w-3.5 h-3.5" />
                Template
              </button>
            </div>
          </div>
        )}

        {/* Step 2: Preview */}
        {step === 'preview' && (
          <div className="space-y-4 py-2">
            <div className="flex items-center justify-between">
              <p className="text-sm text-secondary">
                Vorschau:{' '}
                <span className="font-medium text-primary">{file?.name}</span>
              </p>
              {invalidCols.size > 0 && (
                <span className="text-xs text-amber-600 bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800 px-2 py-0.5 rounded">
                  {invalidCols.size} unbekannte Spalte{invalidCols.size !== 1 ? 'n' : ''}
                </span>
              )}
            </div>

            {/* Preview table */}
            <div className="overflow-x-auto rounded-lg border border-border">
              <table className="w-full text-xs">
                <thead className="bg-surface2">
                  <tr>
                    {headers.map((h, i) => (
                      <th
                        key={i}
                        className={`
                          px-3 py-2 text-left font-semibold text-primary border-b border-border
                          ${invalidCols.has(i) ? 'text-red-500 bg-red-500/10' : ''}
                        `}
                      >
                        {h}
                        {invalidCols.has(i) && (
                          <span className="ml-1 text-[10px] text-red-400">(unbekannt)</span>
                        )}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {previewRows.map((row, ri) => (
                    <tr key={ri} className="border-b border-border last:border-0">
                      {row.map((cell, ci) => (
                        <td
                          key={ci}
                          className={`
                            px-3 py-1.5 text-secondary truncate max-w-[200px]
                            ${invalidCols.has(ci) ? 'bg-red-500/5' : ''}
                          `}
                          title={cell}
                        >
                          {cell || <span className="opacity-40">—</span>}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <p className="text-xs text-secondary">
              Zeige die ersten {previewRows.length} Datenzeilen. Alle Zeilen werden beim Import verarbeitet.
            </p>

            {isUploading && (
              <div className="space-y-1.5">
                <p className="text-xs text-secondary">Importiere…</p>
                <Progress value={uploadProgress} className="h-1.5" />
              </div>
            )}
          </div>
        )}

        {/* Step 3: Result */}
        {step === 'result' && result && (
          <div className="space-y-4 py-2">
            <div className={`
              rounded-lg p-4 space-y-2
              ${result.errors.length > 0 ? 'bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800' : 'bg-green-50 dark:bg-green-950/30 border border-green-200 dark:border-green-800'}
            `}>
              <p className={`font-semibold text-sm ${result.errors.length > 0 ? 'text-amber-800 dark:text-amber-300' : 'text-green-800 dark:text-green-300'}`}>
                {result.errors.length === 0
                  ? `${String(result.imported)} ${entityLabel} erfolgreich importiert`
                  : `${String(result.imported)} importiert, ${String(result.skipped)} übersprungen`
                }
              </p>
              {result.skipped > 0 && (
                <p className="text-xs text-amber-700 dark:text-amber-400">
                  {result.skipped} Einträge konnten nicht importiert werden.
                </p>
              )}
            </div>

            {result.errors.length > 0 && (
              <div className="border border-border rounded-lg overflow-hidden">
                <button
                  type="button"
                  className="w-full flex items-center justify-between px-4 py-2.5 text-sm font-medium text-primary bg-surface2 hover:bg-surface transition-colors"
                  onClick={() => { setErrorsOpen((v) => !v); }}
                >
                  <span>{result.errors.length} Fehler</span>
                  {errorsOpen ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                </button>
                {errorsOpen && (
                  <div className="max-h-48 overflow-y-auto divide-y divide-border">
                    {result.errors.map((err, i) => (
                      <p key={i} className="px-4 py-2 text-xs text-red-500">
                        {err}
                      </p>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          {step === 'upload' && (
            <Button variant="outline" onClick={handleClose}>
              Abbrechen
            </Button>
          )}

          {step === 'preview' && (
            <>
              <Button variant="outline" onClick={() => { setStep('upload'); }} disabled={isUploading}>
                Zurück
              </Button>
              <Button onClick={() => { void handleUpload() }} disabled={isUploading}>
                {isUploading ? 'Importiere…' : 'Importieren'}
              </Button>
            </>
          )}

          {step === 'result' && (
            <>
              <Button variant="outline" onClick={reset}>
                Weitere importieren
              </Button>
              <Button onClick={handleClose}>
                Fertig
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
