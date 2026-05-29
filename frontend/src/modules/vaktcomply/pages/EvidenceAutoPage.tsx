import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { GitBranch, Shield, Bug, CheckCircle2, Inbox, Users } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '../../../components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import { useAutoEvidence, useAssignEvidence, type AutoEvidence } from '../hooks/useEvidenceAuto'
import { useFrameworks, useFrameworkControls } from '../hooks/useFrameworks'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

// --- Source helpers ---

function SourceIcon({ type }: { type: AutoEvidence['auto_source_type'] }) {
  if (type === 'github') return <GitBranch className="w-4 h-4 text-secondary" />
  if (type === 'vaktaware') return <Shield className="w-4 h-4 text-secondary" />
  if (type === 'hr') return <Users className="w-4 h-4 text-secondary" />
  return <Bug className="w-4 h-4 text-secondary" />
}

function SourceBadge({ type }: { type: AutoEvidence['auto_source_type'] }) {
  const labels: Record<AutoEvidence['auto_source_type'], string> = {
    github: 'GitHub',
    vaktaware: 'Vakt Aware',
    vaktscan: 'Vakt Scan',
    hr: 'Vakt HR',
    ci_pipeline: 'CI/CD',
    ci_webhook: 'CI/CD',
  }
  return (
    <Badge variant="secondary" className="text-[10px] capitalize">
      {labels[type]}
    </Badge>
  )
}

// --- Assign Dialog ---

interface AssignDialogProps {
  evidence: AutoEvidence
  onClose: () => void
}

function AssignDialog({ evidence, onClose }: AssignDialogProps) {
  const { t } = useTranslation()
  const [frameworkId, setFrameworkId] = useState('')
  const [controlId, setControlId] = useState('')

  const { data: frameworks = [] } = useFrameworks()
  const { data: controls = [] } = useFrameworkControls(frameworkId, 1, 100)
  const assign = useAssignEvidence()

  function handleAssign() {
    if (!controlId) return
    assign.mutate(
      { evidenceId: evidence.id, controlId },
      { onSuccess: onClose },
    )
  }

  // Controls list — useFrameworkControls returns a sub-type, cast to Control[]
  const controlList = (controls ?? [])

  return (
    <Dialog open onOpenChange={(open) => { if (!open) { onClose(); } }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-sm font-semibold">
            Kontrolle zuordnen
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4 pt-1">
          <div>
            <p className="text-xs text-muted-foreground mb-1">{t('vaktcomply.evidenceAuto.evidence')}</p>
            <p className="text-sm font-medium leading-snug">{evidence.title}</p>
          </div>

          <div className="space-y-2">
            <label className="text-xs text-muted-foreground">{t('vaktcomply.evidenceAuto.framework')}</label>
            <Select
              value={frameworkId}
              onValueChange={(v) => { setFrameworkId(v); setControlId('') }}
            >
              <SelectTrigger>
                <SelectValue placeholder="Framework wählen…" />
              </SelectTrigger>
              <SelectContent>
                {frameworks.map((fw) => (
                  <SelectItem key={fw.id} value={fw.id}>
                    {fw.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {frameworkId && (
            <div className="space-y-2">
              <label className="text-xs text-muted-foreground">{t('vaktcomply.evidenceAuto.control')}</label>
              <Select value={controlId} onValueChange={setControlId}>
                <SelectTrigger>
                  <SelectValue placeholder="Kontrolle wählen…" />
                </SelectTrigger>
                <SelectContent>
                  {controlList.map((ctrl) => (
                    <SelectItem key={ctrl.id} value={ctrl.id}>
                      {ctrl.control_id} — {ctrl.title}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-1">
            <Button variant="outline" size="sm" onClick={onClose}>
              Abbrechen
            </Button>
            <Button
              size="sm"
              disabled={!controlId || assign.isPending}
              onClick={handleAssign}
            >
              {assign.isPending ? 'Wird zugeordnet…' : 'Zuordnen'}
            </Button>
          </div>

          {assign.isError && (
            <p className="text-xs text-destructive">
              Fehler: {assign.error.message}
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

// --- Main Page ---

export default function EvidenceAutoPage() {
  const { data: items = [], isLoading } = useAutoEvidence()
  const [assigning, setAssigning] = useState<AutoEvidence | null>(null)
  const { formatDate: fmtDate } = useFormatDate()
  function formatDate(iso?: string): string {
    if (!iso) return '—'
    return fmtDate(iso, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
  }

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <PageHeader
        title="Automatisch gesammelte Evidence"
        description="Automatisch gesammelte Nachweise aus GitHub, Trainings und Scanner-Ergebnissen — bereit zum Zuordnen"
      />

      {isLoading && (
        <p className="text-sm text-muted-foreground">Lade Evidence…</p>
      )}

      {!isLoading && items.length === 0 && (
        <Card>
          <CardContent className="py-12 text-center space-y-2">
            <CheckCircle2 className="w-10 h-10 mx-auto text-green-500" />
            <p className="font-medium">Kein ausstehender Evidence</p>
            <p className="text-sm text-muted-foreground">
              Alle automatisch gesammelten Evidence-Einträge wurden bereits
              Kontrollen zugeordnet.
            </p>
          </CardContent>
        </Card>
      )}

      {items.length > 0 && (
        <Card>
          <CardContent className="p-0">
            <div className="divide-y divide-border">
              {items.map((ev) => (
                <div
                  key={ev.id}
                  className="flex items-center gap-4 px-4 py-3 hover:bg-muted/30 transition-colors"
                >
                  <div className="shrink-0">
                    <SourceIcon type={ev.auto_source_type} />
                  </div>

                  <div className="min-w-0 flex-1 space-y-0.5">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-sm font-medium truncate">{ev.title}</span>
                      <SourceBadge type={ev.auto_source_type} />
                    </div>
                    {ev.description && (
                      <p className="text-xs text-muted-foreground line-clamp-1">
                        {ev.description}
                      </p>
                    )}
                    {ev.suggested_control_hint && (
                      <p className="text-[11px] text-blue-400">
                        Vorschlag: {ev.suggested_control_hint}
                      </p>
                    )}
                    <p className="text-xs text-muted-foreground">
                      Gesammelt: {formatDate(ev.auto_collected_at ?? ev.created_at)}
                    </p>
                  </div>

                  <Button
                    size="sm"
                    variant="outline"
                    className="shrink-0 h-7 text-xs gap-1"
                    onClick={() => { setAssigning(ev); }}
                  >
                    <Inbox className="w-3 h-3" />
                    Kontrolle zuordnen
                  </Button>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {assigning && (
        <AssignDialog
          evidence={assigning}
          onClose={() => { setAssigning(null); }}
        />
      )}
    </div>
  )
}
