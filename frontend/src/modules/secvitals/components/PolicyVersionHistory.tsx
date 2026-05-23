import { useState } from 'react'
import { History, Eye, X } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../../../components/ui/dialog'
import { usePolicyVersions, type PolicyVersion } from '../hooks/usePolicyVersions'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

interface Props {
  policyId: string
  currentVersion: number
}

const STATUS_LABELS: Record<string, string> = {
  draft: 'Entwurf',
  active: 'Aktiv',
  archived: 'Archiviert',
}

function VersionDetailDialog({
  version,
  onClose,
}: {
  version: PolicyVersion | null
  onClose: () => void
}) {
  const { formatDate } = useFormatDate()
  if (!version) return null
  return (
    <Dialog open={!!version} onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <History className="w-4 h-4 text-muted-foreground" />
            Version {version.version} — {version.title}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
            <span>
              Gespeichert: {formatDate(version.created_at, { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
            </span>
            {version.updated_by && <span>· Von: {version.updated_by}</span>}
            {version.status && <span>· Status: {STATUS_LABELS[version.status] ?? version.status}</span>}
          </div>
          {version.version_note && (
            <div className="p-3 rounded-lg bg-accent text-sm">
              <p className="text-xs font-medium text-muted-foreground mb-1">Änderungsnotiz</p>
              <p>{version.version_note}</p>
            </div>
          )}
          {version.content ? (
            <div className="space-y-1.5">
              <p className="text-xs font-medium text-muted-foreground">Inhalt dieser Version</p>
              <pre className="text-xs whitespace-pre-wrap font-sans leading-relaxed p-4 rounded-lg bg-accent/50 border border-border">
                {version.content}
              </pre>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground italic">Kein Inhalt für diese Version gespeichert.</p>
          )}
        </div>
        <div className="flex justify-end pt-2">
          <Button variant="outline" size="sm" onClick={onClose}>
            <X className="w-3 h-3 mr-1" />
            Schließen
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

export default function PolicyVersionHistory({ policyId, currentVersion }: Props) {
  const { data: versions, isLoading, isError } = usePolicyVersions(policyId)
  const { formatDate } = useFormatDate()
  const [selected, setSelected] = useState<PolicyVersion | null>(null)

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm flex items-center gap-2">
          <History className="w-4 h-4 text-muted-foreground" />
          Versionshistorie
        </CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading && (
          <div className="flex items-center justify-center h-16">
            <Spinner size="sm" color="primary" />
          </div>
        )}
        {isError && (
          <p className="text-xs text-red-400">Fehler beim Laden der Versionen.</p>
        )}
        {!isLoading && !isError && (!versions || versions.length === 0) && (
          <p className="text-xs text-muted-foreground">Noch keine Versionen gespeichert. Beim nächsten Speichern wird ein Snapshot erstellt.</p>
        )}
        {!isLoading && !isError && versions && versions.length > 0 && (
          <ol className="space-y-2">
            {versions.map((v) => {
              const isCurrent = v.version === currentVersion - 1 // last snapshot = the version before current
              return (
                <li key={v.id} className="flex items-start gap-3">
                  {/* Timeline dot */}
                  <div className="flex flex-col items-center mt-0.5">
                    <div className={`w-2.5 h-2.5 rounded-full shrink-0 ${isCurrent ? 'bg-blue-500' : 'bg-border'}`} />
                    <div className="w-px flex-1 bg-border mt-1" />
                  </div>
                  {/* Content */}
                  <div className="flex-1 min-w-0 pb-2">
                    <div className="flex items-center gap-2 flex-wrap">
                      <Badge
                        className={`text-xs px-1.5 py-0 ${
                          isCurrent
                            ? 'bg-blue-500/20 text-blue-400 border-blue-500/30'
                            : 'bg-secondary text-secondary-foreground'
                        }`}
                      >
                        v{v.version}
                      </Badge>
                      <span className="text-xs text-muted-foreground">
                        {formatDate(v.created_at, { year: 'numeric', month: 'short', day: 'numeric' })}
                      </span>
                      {v.updated_by && (
                        <span className="text-xs text-muted-foreground">· {v.updated_by}</span>
                      )}
                    </div>
                    {v.version_note && (
                      <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{v.version_note}</p>
                    )}
                  </div>
                  {/* Action */}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="shrink-0 h-7 px-2 text-xs"
                    onClick={() => { setSelected(v); }}
                  >
                    <Eye className="w-3 h-3 mr-1" />
                    Anzeigen
                  </Button>
                </li>
              )
            })}
          </ol>
        )}
      </CardContent>

      <VersionDetailDialog version={selected} onClose={() => { setSelected(null); }} />
    </Card>
  )
}
