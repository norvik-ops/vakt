import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Activity, Plus, Play, Trash2, ChevronDown, ChevronUp } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  useCCMChecks,
  useCreateCCMCheck,
  useDeleteCCMCheck,
  useToggleCCMCheck,
  useTriggerCCMCheck,
  useCCMResults,
} from '../hooks/useCCM'
import { ProGate } from '../../../shared/components/ProGate'
import { useFrameworks } from '../hooks/useFrameworks'
import type { CCMCheck, CCMCheckType, CCMStatus, CreateCCMCheckInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

// ─── Constants ────────────────────────────────────────────────────────────────

const CHECK_TYPE_LABELS: Record<CCMCheckType, string> = {
  http_endpoint: 'HTTP Endpoint',
  trivy_no_critical: 'Trivy — Keine kritischen CVEs',
  evidence_freshness: 'Nachweis-Aktualität',
  custom_script: 'Custom Script',
}

const STATUS_CLASS: Record<CCMStatus, string> = {
  pass: 'bg-green-500/20 text-green-400 border-green-500/30',
  fail: 'bg-red-500/20 text-red-400 border-red-500/30',
  unknown: 'bg-secondary text-secondary-foreground',
}

const STATUS_LABEL: Record<CCMStatus, string> = {
  pass: 'OK',
  fail: 'Fehler',
  unknown: 'Unbekannt',
}

// ─── Config key hints per check type ─────────────────────────────────────────

const CONFIG_HINTS: Record<CCMCheckType, { key: string; placeholder: string }[]> = {
  http_endpoint: [{ key: 'url', placeholder: 'https://example.com/health' }],
  trivy_no_critical: [],
  evidence_freshness: [{ key: 'max_days', placeholder: '90' }],
  custom_script: [],
}

// ─── Results dialog ───────────────────────────────────────────────────────────

function ResultsDialog({
  check,
  onClose,
}: {
  check: CCMCheck
  onClose: () => void
}) {
  const { data: results, isLoading } = useCCMResults(check.id)
  const { formatDateTime } = useFormatDate()

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="max-w-lg max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Ergebnisse — {check.name}</DialogTitle>
        </DialogHeader>
        {isLoading && (
          <div className="flex items-center justify-center h-24">
            <Spinner size="md" color="primary" />
          </div>
        )}
        {!isLoading && (!results || results.length === 0) && (
          <p className="text-sm text-muted-foreground py-4">Noch keine Ergebnisse vorhanden.</p>
        )}
        {!isLoading && results && results.length > 0 && (
          <div className="space-y-2">
            {results.map((r) => (
              <div
                key={r.id}
                className="p-3 rounded-md border bg-card text-sm space-y-1"
              >
                <div className="flex items-center justify-between gap-2">
                  <Badge className={STATUS_CLASS[r.status]}>
                    {STATUS_LABEL[r.status] ?? r.status}
                  </Badge>
                  <span className="text-xs text-muted-foreground">
                    {formatDateTime(r.ran_at)}
                  </span>
                </div>
                {r.output && (
                  <p className="text-xs text-muted-foreground font-mono break-all">{r.output}</p>
                )}
              </div>
            ))}
          </div>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Schließen
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Check row ────────────────────────────────────────────────────────────────

function CCMCheckRow({
  check,
  onDelete,
  onToggle,
  onTrigger,
  onShowResults,
  isRunning,
}: {
  check: CCMCheck
  onDelete: () => void
  onToggle: () => void
  onTrigger: () => void
  onShowResults: () => void
  isRunning: boolean
}) {
  const { formatDateTime } = useFormatDate()
  const status = (check.last_status ?? 'unknown')

  return (
    <Card className={!check.enabled ? 'opacity-60' : ''}>
      <CardContent className="pt-4 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1 flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-sm font-medium truncate">{check.name}</span>
              {check.last_status && (
                <Badge className={STATUS_CLASS[status]}>
                  {STATUS_LABEL[status] ?? status}
                </Badge>
              )}
              {!check.enabled && (
                <Badge className="bg-secondary text-secondary-foreground text-xs">
                  Deaktiviert
                </Badge>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {CHECK_TYPE_LABELS[check.check_type] ?? check.check_type}
              {' · '}alle {check.interval_hours}h
            </p>
            {check.last_run_at && (
              <p className="text-xs text-muted-foreground">
                Zuletzt: {formatDateTime(check.last_run_at)}
                {check.last_output ? ` — ${check.last_output}` : ''}
              </p>
            )}
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              title="Jetzt ausführen"
              onClick={onTrigger}
              disabled={isRunning}
            >
              <Play className="w-3.5 h-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              title="Ergebnisse anzeigen"
              onClick={onShowResults}
            >
              {check.enabled ? (
                <ChevronDown className="w-3.5 h-3.5" />
              ) : (
                <ChevronUp className="w-3.5 h-3.5" />
              )}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs"
              onClick={onToggle}
            >
              {check.enabled ? 'Aus' : 'Ein'}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-red-400 hover:text-red-300"
              onClick={onDelete}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ─── Config key-value editor ──────────────────────────────────────────────────

function ConfigEditor({
  checkType,
  config,
  onChange,
}: {
  checkType: CCMCheckType | ''
  config: Record<string, string>
  onChange: (config: Record<string, string>) => void
}) {
  const { t } = useTranslation()
  if (!checkType) return null
  const hints = CONFIG_HINTS[checkType] ?? []
  if (hints.length === 0) return null

  return (
    <div className="space-y-1.5">
      <Label>{t('vaktcomply.ccmPage.configLabel')}</Label>
      {hints.map((hint) => (
        <div key={hint.key} className="flex gap-2 items-center">
          <span className="text-xs text-muted-foreground w-20 shrink-0">{hint.key}</span>
          <Input
            className="text-sm"
            placeholder={hint.placeholder}
            value={config[hint.key] ?? ''}
            onChange={(e) => { onChange({ ...config, [hint.key]: e.target.value }); }}
          />
        </div>
      ))}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

function emptyForm(): CreateCCMCheckInput {
  return {
    control_id: '',
    name: '',
    check_type: '' as CCMCheckType,
    config: {},
    interval_hours: 24,
  }
}

export default function CCMPage() {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<CreateCCMCheckInput>(emptyForm())
  const [resultsCheck, setResultsCheck] = useState<CCMCheck | null>(null)
  const [triggeringId, setTriggeringId] = useState<string | null>(null)

  const { data: checks, isLoading, isError, error } = useCCMChecks()
  const { data: frameworksData } = useFrameworks()

  const createCheck = useCreateCCMCheck()
  const deleteCheck = useDeleteCCMCheck()
  const toggleCheck = useToggleCCMCheck()
  const triggerCheck = useTriggerCCMCheck()

  // Collect all controls across frameworks for the control selector.
  const frameworks = frameworksData ?? []

  function openCreate() {
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm('CCM-Check wirklich löschen?')) {
      deleteCheck.mutate(id)
    }
  }

  function handleToggle(check: CCMCheck) {
    toggleCheck.mutate({ id: check.id, enabled: !check.enabled })
  }

  function handleTrigger(check: CCMCheck) {
    setTriggeringId(check.id)
    triggerCheck.mutate(check.id, {
      onSettled: () => { setTriggeringId(null); },
    })
  }

  function handleSubmit() {
    if (!form.control_id || !form.name || !form.check_type) return
    createCheck.mutate(form, {
      onSuccess: () => {
        setDialogOpen(false)
        setForm(emptyForm())
      },
    })
  }

  const checkList = checks ?? []

  if (isLoading) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-center h-48">
          <Spinner size="lg" color="primary" />
        </div>
      </div>
    )
  }

  return (
    <ProGate error={error}>
      <div className="flex flex-col h-full">
        <PageHeader
          title="Continuous Control Monitoring"
          description="Automatisierte Checks für Compliance-Controls — HTTP-Endpunkte, CVE-Scans, Nachweis-Aktualität."
          actions={
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              Neuen Check anlegen
            </Button>
          }
        />

        <div className="flex-1 p-6">
          {isError && (
            <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
              Fehler beim Laden der CCM-Checks.
            </div>
          )}
          {!isError && checkList.length === 0 && (
            <EmptyState
              icon={Activity}
              title="Keine CCM-Checks definiert"
              description="Legen Sie automatisierte Checks an, um Control-Status kontinuierlich zu überwachen."
              action={
                <Button onClick={openCreate}>
                  <Plus className="w-4 h-4 mr-1" />
                  Neuen Check anlegen
                </Button>
              }
            />
          )}
          {!isError && checkList.length > 0 && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {checkList.map((check) => (
                <CCMCheckRow
                  key={check.id}
                  check={check}
                  onDelete={() => { handleDelete(check.id); }}
                  onToggle={() => { handleToggle(check); }}
                  onTrigger={() => { handleTrigger(check); }}
                  onShowResults={() => { setResultsCheck(check); }}
                  isRunning={triggeringId === check.id}
                />
              ))}
            </div>
          )}
        </div>

        {/* Create dialog */}
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Neuen Check anlegen</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-2">
              <div className="space-y-1.5">
                <Label>Name *</Label>
                <Input
                  placeholder="z.B. API Health Check"
                  value={form.name}
                  onChange={(e) => { setForm((f) => ({ ...f, name: e.target.value })); }}
                />
              </div>

              <div className="space-y-1.5">
                <Label>Check-Typ *</Label>
                <Select
                  value={form.check_type}
                  onValueChange={(v) =>
                    { setForm((f) => ({ ...f, check_type: v as CCMCheckType, config: {} })); }
                  }
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Typ auswählen …" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="http_endpoint">HTTP Endpoint</SelectItem>
                    <SelectItem value="trivy_no_critical">Trivy — Keine kritischen CVEs</SelectItem>
                    <SelectItem value="evidence_freshness">Nachweis-Aktualität</SelectItem>
                    <SelectItem value="custom_script">Custom Script</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label>Control-ID *</Label>
                <Input
                  placeholder="UUID des Controls"
                  value={form.control_id}
                  onChange={(e) => { setForm((f) => ({ ...f, control_id: e.target.value })); }}
                />
                {frameworks.length > 0 && (
                  <p className="text-xs text-muted-foreground">
                    Öffnen Sie einen Framework-Control und kopieren Sie die ID aus der URL.
                  </p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label>Intervall (Stunden) *</Label>
                <Input
                  type="number"
                  min={1}
                  max={8760}
                  value={form.interval_hours}
                  onChange={(e) =>
                    { setForm((f) => ({ ...f, interval_hours: parseInt(e.target.value) || 24 })); }
                  }
                />
              </div>

              <ConfigEditor
                checkType={form.check_type}
                config={form.config}
                onChange={(config) => { setForm((f) => ({ ...f, config })); }}
              />
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => { setDialogOpen(false); }}>
                Abbrechen
              </Button>
              <Button
                onClick={handleSubmit}
                disabled={
                  !form.name ||
                  !form.check_type ||
                  !form.control_id ||
                  createCheck.isPending
                }
              >
                {createCheck.isPending ? 'Speichern …' : 'Anlegen'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Results dialog */}
        {resultsCheck && (
          <ResultsDialog check={resultsCheck} onClose={() => { setResultsCheck(null); }} />
        )}
      </div>
    </ProGate>
  )
}
