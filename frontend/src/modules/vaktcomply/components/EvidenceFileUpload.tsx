import React, { useRef, useState } from 'react'
import { Paperclip, Eye, Trash2, Upload } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Button } from '../../../components/ui/button'
import {
  useEvidenceFilesByControl,
  useUploadEvidenceFile,
  useDeleteEvidenceFile,
} from '../hooks/useEvidenceFiles'
import { FilePreviewDialog, type PreviewFile } from '../../../shared/components/FilePreviewDialog'

interface EvidenceFileUploadProps {
  controlId: string
  evidenceId?: string
}

const ACCEPTED_TYPES = '.pdf,.png,.jpg,.jpeg,.txt,.csv,.xlsx,.docx,.zip'
const MAX_SIZE_BYTES = 50 * 1024 * 1024 // 50 MB

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function EvidenceFileUpload({ controlId }: EvidenceFileUploadProps) {
  const { data: files, isLoading } = useEvidenceFilesByControl(controlId)
  const upload = useUploadEvidenceFile(controlId)
  const deleteFile = useDeleteEvidenceFile(controlId)
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragOver, setDragOver] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)
  const [previewFile, setPreviewFile] = useState<PreviewFile | null>(null)

  const PREVIEWABLE = ['application/pdf', 'image/png', 'image/jpg', 'image/jpeg', 'image/gif', 'image/webp']

  function openPreview(f: { original_name: string; mime_type: string; download_url: string }) {
    setPreviewFile({ name: f.original_name, url: f.download_url, mimeType: f.mime_type })
  }

  function handleFile(file: File) {
    setLocalError(null)
    if (file.size > MAX_SIZE_BYTES) {
      setLocalError('Datei zu groß. Maximal 50 MB erlaubt.')
      return
    }
    const ext = file.name.slice(file.name.lastIndexOf('.')).toLowerCase()
    const allowed = ['.pdf', '.png', '.jpg', '.jpeg', '.txt', '.csv', '.xlsx', '.docx', '.zip']
    if (!allowed.includes(ext)) {
      setLocalError(`Dateityp nicht erlaubt: ${ext}`)
      return
    }
    upload.mutate(file, {
      onError: (err) => { setLocalError(err.message); },
    })
  }

  function onInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (file) handleFile(file)
    // Reset so the same file can be re-selected
    e.target.value = ''
  }

  function onDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault()
    setDragOver(false)
    const file = e.dataTransfer.files?.[0]
    if (file) handleFile(file)
  }

  function onDragOver(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault()
    setDragOver(true)
  }

  function onDragLeave() {
    setDragOver(false)
  }

  return (
    <div className="space-y-3">
      <FilePreviewDialog file={previewFile} onClose={() => { setPreviewFile(null); }} />
      {/* Drop zone */}
      <div
        className={`flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-6 transition-colors cursor-pointer ${
          dragOver ? 'border-brand bg-brand/5' : 'border-border hover:border-brand/50 hover:bg-surface'
        }`}
        onDrop={onDrop}
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onClick={() => inputRef.current?.click()}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') inputRef.current?.click() }}
      >
        <Upload className="w-6 h-6 text-secondary" />
        <p className="text-sm text-secondary text-center">
          Datei hier ablegen oder <span className="text-brand font-medium">auswählen</span>
        </p>
        <p className="text-xs text-secondary">
          PDF, PNG, JPG, DOCX, XLSX, ZIP, TXT, CSV — max. 50 MB
        </p>
        <input
          ref={inputRef}
          type="file"
          accept={ACCEPTED_TYPES}
          className="hidden"
          onChange={onInputChange}
        />
      </div>

      {/* Upload state */}
      {upload.isPending && (
        <div className="flex items-center gap-2 text-sm text-secondary">
          <Spinner size="sm" />
          <span>Wird hochgeladen…</span>
        </div>
      )}

      {/* Error */}
      {(localError ?? upload.error) && (
        <p className="text-sm text-red-600">
          {localError ?? upload.error?.message}
        </p>
      )}

      {/* File list */}
      {isLoading ? (
        <div className="flex justify-center py-4">
          <Spinner size="sm" />
        </div>
      ) : files && files.length > 0 ? (
        <ul className="divide-y divide-border rounded-lg border border-border overflow-hidden">
          {files.map((f) => (
            <li key={f.id} className="flex items-center gap-3 px-4 py-2.5 bg-surface text-sm">
              <Paperclip className="w-4 h-4 text-secondary shrink-0" />
              <span className="flex-1 min-w-0 truncate font-medium" title={f.original_name}>
                {f.original_name}
              </span>
              <span className="text-xs text-secondary shrink-0">
                {formatBytes(f.size_bytes)}
              </span>
              <Button
                variant="ghost"
                size="sm"
                className="shrink-0 h-6 w-6 p-0 text-brand hover:text-brand/80"
                title={PREVIEWABLE.includes(f.mime_type) ? 'Vorschau' : 'Herunterladen'}
                onClick={() => {
                  if (PREVIEWABLE.includes(f.mime_type)) {
                    openPreview(f)
                  } else {
                    window.open(f.download_url, '_blank', 'noopener,noreferrer')
                  }
                }}
              >
                <Eye className="w-4 h-4" />
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="shrink-0 h-6 w-6 p-0 text-secondary hover:text-red-600"
                title="Löschen"
                onClick={() => { deleteFile.mutate(f.id); }}
                disabled={deleteFile.isPending}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-xs text-secondary text-center py-2">Noch keine Anhänge hochgeladen.</p>
      )}
    </div>
  )
}
