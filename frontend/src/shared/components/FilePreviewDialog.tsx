import { Download, X } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../components/ui/dialog'
import { Button } from '../../components/ui/button'

export interface PreviewFile {
  name: string
  url: string
  mimeType: string
}

interface FilePreviewDialogProps {
  file: PreviewFile | null
  onClose: () => void
}

function isImage(mimeType: string): boolean {
  return mimeType.startsWith('image/')
}

function isPdf(mimeType: string): boolean {
  return mimeType === 'application/pdf'
}

export function FilePreviewDialog({ file, onClose }: FilePreviewDialogProps) {
  if (!file) return null

  return (
    <Dialog open={!!file} onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent className="max-w-4xl w-full">
        <DialogHeader>
          <DialogTitle className="pr-8 truncate" title={file.name}>
            {file.name}
          </DialogTitle>
        </DialogHeader>

        <div className="mt-2">
          {isImage(file.mimeType) ? (
            <div className="flex items-center justify-center bg-surface rounded-lg overflow-hidden">
              <img
                src={file.url}
                alt={file.name}
                className="max-h-[70vh] w-auto object-contain rounded"
              />
            </div>
          ) : isPdf(file.mimeType) ? (
            <iframe
              src={file.url}
              title={file.name}
              className="w-full rounded-lg border border-border"
              style={{ height: '70vh' }}
            />
          ) : (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <p className="text-sm text-secondary">
                Vorschau nicht verfügbar für diesen Dateityp.
              </p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => window.open(file.url, '_blank', 'noopener,noreferrer')}
              >
                <Download className="w-4 h-4 mr-2" />
                Herunterladen
              </Button>
            </div>
          )}
        </div>

        <DialogFooter className="mt-4">
          <Button
            variant="outline"
            onClick={() => window.open(file.url, '_blank', 'noopener,noreferrer')}
          >
            <Download className="w-4 h-4 mr-2" />
            Herunterladen
          </Button>
          <Button variant="ghost" onClick={onClose}>
            <X className="w-4 h-4 mr-2" />
            Schließen
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
