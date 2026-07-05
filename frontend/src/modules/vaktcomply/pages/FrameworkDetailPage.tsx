import React, { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Spinner } from '../../../components/Spinner'
import {
  ArrowLeft,
  Copy,
  Check,
  Trash2,
  Plus,
  AlertTriangle,
  ShieldAlert,
  ChevronRight,
  ChevronDown,
  Circle,
  Clock,
  CheckCircle2,
  MinusCircle,
  FileDown,
  RefreshCw,
} from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../../components/ui/tabs'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../../../components/ui/table'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Breadcrumbs } from '../../../shared/components/Breadcrumbs'
import { trackPage } from '../../../shared/hooks/useRecentPages'
import { EmptyState } from '../../../shared/components/EmptyState'
import { BulkActionBar } from '../../../shared/components/BulkActionBar'
import { cn } from '../../../lib/utils'
import { exportToCSV } from '../../../lib/csv'
import type { AuditorLink, Control } from '../types'
import {
  useFramework,
  useReadinessReport,
  useGapAnalysis,
  useFrameworkControls,
  useDownloadFrameworkPDF,
} from '../hooks/useFrameworks'
import { useAuditorLinks, useRevokeAuditorLink, useCreateAuditorLink } from '../hooks/useAuditorLinks'
import { useUpdateControl, useBulkUpdateControls } from '../hooks/useControls'
import { toast } from '../../../shared/hooks/useToast'
import { Skeleton } from '../../../components/ui/skeleton'
import { ErrorState } from '../../../shared/components/ErrorState'
import { exportAsRTF } from '../../../shared/utils/exportRtf'
import { useMilestoneToast } from '../../../shared/components/MilestoneToast'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { useTranslation } from 'react-i18next'

// ── DORA → ISO 27001 mapping info block ──────────────────────────────────────

const DORA_ISO_DOMAIN_TABLE = [
  { domain: 'ICT-Risikomanagement (Art. 5–16)', iso: 'A.8.1, A.8.2, A.8.7, A.8.13–16, A.5.1, A.5.2, A.6.1, A.6.4, A.5.30' },
  { domain: 'Vorfallmanagement (Art. 17–23)',   iso: 'A.5.24–5.27' },
  { domain: 'Resilienztests (Art. 24–27)',       iso: 'A.5.36, A.8.8' },
  { domain: 'Drittparteienrisiken (Art. 28–44)', iso: 'A.5.19–5.21' },
]

function DORAISOMapping() {
  return (
    <div
      data-testid="dora-iso-mapping-block"
      className="p-4 bg-orange-500/5 border border-orange-500/20 rounded-lg space-y-3"
    >
      <p className="text-sm font-semibold text-orange-600">ISO 27001 Querverweise</p>
      <p className="text-xs text-secondary">
        Jedes DORA-Control ist einem oder mehreren ISO 27001:2022-Annex-A-Klauseln zugeordnet.
        Organisationen, die bereits ISO 27001 umgesetzt haben, können DORA-Anforderungen
        erheblich effizienter erfüllen.
      </p>
      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-border">
              <th className="text-left py-1.5 pr-4 font-medium text-secondary">DORA-Bereich</th>
              <th className="text-left py-1.5 font-medium text-secondary">ISO 27001:2022 Klauseln</th>
            </tr>
          </thead>
          <tbody>
            {DORA_ISO_DOMAIN_TABLE.map(({ domain, iso }) => (
              <tr key={domain} className="border-b border-border last:border-0">
                <td className="py-1.5 pr-4 text-primary">{domain}</td>
                <td className="py-1.5 font-mono text-secondary">{iso}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function scoreColor(score: number) {
  if (score >= 80) return 'text-green-600'
  if (score >= 50) return 'text-yellow-600'
  return 'text-red-600'
}

function gapSeverityVariant(reason: string) {
  if (reason === 'no_evidence') return 'destructive' as const
  return 'warning' as const
}

function gapSeverityLabel(reason: string) {
  if (reason === 'no_evidence') return 'Kein Nachweis'
  return 'Nachweis läuft ab'
}

function gapRecommendation(reason: string) {
  if (reason === 'no_evidence') return 'Nachweis hinzufügen, um dieses Control zu erfüllen.'
  return 'Bestehenden Nachweis erneuern — läuft in den nächsten 30 Tagen ab.'
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => { setCopied(false); }, 2000)
    return () => { clearTimeout(id); }
  }, [copied])

  function handleCopy() {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
    })
  }

  return (
    <Button variant="ghost" size="sm" onClick={handleCopy} className="gap-1">
      {copied ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
      {copied ? 'Kopiert' : 'Kopieren'}
    </Button>
  )
}

function AuditorLinksTab({ frameworkId }: { frameworkId: string }) {
  const { t } = useTranslation()
  const [createOpen, setCreateOpen] = useState(false)
  const [label, setLabel] = useState('')
  const [expiresInDays, setExpiresInDays] = useState('30')
  const [createdUrl, setCreatedUrl] = useState<string | null>(null)
  const { formatDate } = useFormatDate()

  const { data: links, isLoading } = useAuditorLinks()
  const revokeLink = useRevokeAuditorLink()
  const createLink = useCreateAuditorLink(frameworkId)

  const frameworkLinks = links ?? []

  function handleCreate() {
    const days = parseInt(expiresInDays, 10)
    if (!label.trim() || isNaN(days)) return
    createLink.mutate(
      { label: label.trim(), expires_in_days: days },
      {
        onSuccess: (data) => {
          setCreatedUrl(data.auditor_url)
          setLabel('')
          setExpiresInDays('30')
        },
      },
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button size="sm" onClick={() => { setCreateOpen(true); setCreatedUrl(null) }}>
          <Plus className="w-4 h-4 mr-1" />
          Auditor-Link erstellen
        </Button>
      </div>

      {isLoading && (
        <div className="flex items-center justify-center h-24">
          <Spinner size="md" />
        </div>
      )}

      {!isLoading && frameworkLinks.length === 0 && (
        <EmptyState
          icon={ShieldAlert}
          title="Keine Auditor-Links"
          description="Erstelle einen Link, um einem Auditor schreibgeschützten Zugriff auf dieses Framework zu geben."
          action={
            <Button size="sm" onClick={() => { setCreateOpen(true); setCreatedUrl(null) }}>
              <Plus className="w-4 h-4 mr-1" />
              Auditor-Link erstellen
            </Button>
          }
        />
      )}

      {!isLoading && frameworkLinks.length > 0 && (
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('vaktcomply.frameworkDetail.colLabel')}</TableHead>
                <TableHead>{t('vaktcomply.frameworkDetail.colExpires')}</TableHead>
                <TableHead>{t('vaktcomply.frameworkDetail.colAccesses')}</TableHead>
                <TableHead>{t('common.status')}</TableHead>
                <TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {frameworkLinks.map((link: AuditorLink) => (
                <TableRow key={link.id}>
                  <TableCell className="font-medium">{link.label ?? '—'}</TableCell>
                  <TableCell>
                    {formatDate(link.expires_at)}
                  </TableCell>
                  <TableCell>{link.access_count}</TableCell>
                  <TableCell>
                    {link.revoked_at ? (
                      <Badge variant="destructive">{t('vaktcomply.frameworkDetail.statusRevoked')}</Badge>
                    ) : new Date(link.expires_at) < new Date() ? (
                      <Badge variant="secondary">{t('vaktcomply.frameworkDetail.statusExpired')}</Badge>
                    ) : (
                      <Badge variant="success">{t('common.statusLabels.active')}</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    {!link.revoked_at && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-red-600 hover:text-red-700"
                        onClick={() => { revokeLink.mutate(link.id); }}
                        disabled={revokeLink.isPending}
                        aria-label={`Auditor-Link ${link.label ?? link.id} widerrufen`}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Auditor-Link erstellen</DialogTitle>
          </DialogHeader>
          {createdUrl ? (
            <div className="py-4 space-y-3">
              <p className="text-sm text-secondary">
                Teile diesen Link mit deinem Auditor. Er gewährt nur Lesezugriff.
              </p>
              <div className="flex items-center gap-2 p-3 bg-surface2 rounded-md border text-sm font-mono break-all">
                {createdUrl}
              </div>
              <div className="flex justify-end">
                <CopyButton text={createdUrl} />
              </div>
            </div>
          ) : (
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="link-label">{t('vaktcomply.frameworkDetail.colLabel')}</Label>
                <Input
                  id="link-label"
                  placeholder="z.B. Externes Audit Q3 2026"
                  value={label}
                  onChange={(e) => { setLabel(e.target.value); }}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="link-expires">{t('vaktcomply.frameworkDetail.validFor')}</Label>
                <Select value={expiresInDays} onValueChange={setExpiresInDays}>
                  <SelectTrigger id="link-expires">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="7">7 Tage</SelectItem>
                    <SelectItem value="14">14 Tage</SelectItem>
                    <SelectItem value="30">30 Tage</SelectItem>
                    <SelectItem value="90">90 Tage</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCreateOpen(false); }}>
              {createdUrl ? 'Schließen' : 'Abbrechen'}
            </Button>
            {!createdUrl && (
              <Button
                onClick={handleCreate}
                disabled={!label.trim() || createLink.isPending}
              >
                {createLink.isPending ? 'Wird erstellt…' : 'Link erstellen'}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function NotApplicableDialog({
  control,
  frameworkId,
  open,
  onClose,
}: {
  control: Control
  frameworkId: string
  open: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const [reason, setReason] = useState(control.not_applicable_reason ?? '')
  const updateControl = useUpdateControl(frameworkId)

  function handleConfirm() {
    updateControl.mutate(
      { controlId: control.id, not_applicable: true, reason, manual_status: '' },
      { onSuccess: onClose },
    )
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Control als „Nicht anwendbar" markieren</DialogTitle>
        </DialogHeader>
        <div className="py-3 space-y-3">
          <p className="text-sm text-secondary">
            <span className="font-mono text-xs bg-surface2 px-1.5 py-0.5 rounded">{control.control_id}</span>
            {' '}{control.title}
          </p>
          <div className="space-y-1.5">
            <Label htmlFor="na-reason">{t('vaktcomply.controlDetailPage.naReason')} <span className="text-secondary">{t('vaktcomply.controlDetailPage.naReasonHint')}</span></Label>
            <textarea
              id="na-reason"
              rows={3}
              className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
              value={reason}
              onChange={(e) => { setReason(e.target.value); }}
              placeholder="z.B. Trifft auf unsere Organisation nicht zu, da kein Supply-Chain-Risiko besteht."
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={handleConfirm} disabled={updateControl.isPending}>
            {updateControl.isPending ? t('common.saving') : t('common.confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

type StatusChoice = 'missing' | 'in_progress' | 'implemented' | 'not_applicable'

const STATUS_CONFIG: Record<StatusChoice, { icon: React.ReactNode; label: string; className: string }> = {
  missing:        { icon: <Circle className="w-3.5 h-3.5" />,       label: 'Offen',            className: 'text-red-500' },
  in_progress:    { icon: <Clock className="w-3.5 h-3.5" />,        label: 'In Bearbeitung',   className: 'text-yellow-600' },
  implemented:    { icon: <CheckCircle2 className="w-3.5 h-3.5" />, label: 'Umgesetzt',        className: 'text-green-600' },
  not_applicable: { icon: <MinusCircle className="w-3.5 h-3.5" />,  label: 'Nicht anwendbar',  className: 'text-secondary' },
}

function StatusIndicator({ status }: { status: StatusChoice }) {
  const cfg = STATUS_CONFIG[status]
  return (
    <span className={cn('flex items-center gap-1.5', cfg.className)}>
      {cfg.icon}
      <span>{cfg.label}</span>
    </span>
  )
}

function effectiveStatus(ctrl: Control): StatusChoice {
  if (ctrl.status === 'covered' || ctrl.status === 'implemented') return 'implemented'
  if (ctrl.status === 'not_applicable') return 'not_applicable'
  if (ctrl.status === 'partial' || ctrl.status === 'in_progress') return 'in_progress'
  return 'missing'
}

function hasParent(ctrl: Control, all: Control[]): boolean {
  return all.some((other) => other.id !== ctrl.id && ctrl.control_id.startsWith(other.control_id + '.'))
}

function getChildren(ctrl: Control, all: Control[]): Control[] {
  return all.filter(
    (other) => other.id !== ctrl.id && other.control_id.startsWith(ctrl.control_id + '.'),
  )
}

const ControlRow = React.memo(function ControlRow({
  ctrl,
  frameworkId,
  depth,
  updateControl,
  onMarkNA,
  selected,
  onToggleSelect,
}: {
  ctrl: Control
  frameworkId: string
  depth: number
  updateControl: ReturnType<typeof useUpdateControl>
  onMarkNA: (ctrl: Control) => void
  selected: boolean
  onToggleSelect: (id: string) => void
}) {
  const navigate = useNavigate()
  const status = effectiveStatus(ctrl)

  function handleChange(v: string) {
    if (v === 'not_applicable') { onMarkNA(ctrl); return }
    updateControl.mutate(
      {
        controlId: ctrl.id,
        not_applicable: false,
        reason: '',
        manual_status: v === 'missing' ? '' : v as '' | 'in_progress' | 'implemented',
      },
      {
        onSuccess: () => toast(t('common.saved'), 'success'),
        onError: (err) => toast(`${t('common.error')}: ${err.message}`, 'error'),
      },
    )
  }

  return (
    <TableRow className={cn(ctrl.not_applicable && 'opacity-50', selected && 'bg-brand/5')}>
      <TableCell className="w-10" onClick={(e) => { e.stopPropagation(); }}>
        <input
          type="checkbox"
          checked={selected}
          onChange={() => { onToggleSelect(ctrl.id); }}
          className="rounded"
        />
      </TableCell>
      <TableCell
        className="font-mono text-xs cursor-pointer"
        onClick={() => { navigate(`/vaktcomply/controls/${ctrl.id}?frameworkId=${frameworkId}`); }}
        style={{ paddingLeft: depth > 0 ? `${depth * 1.5 + 1}rem` : undefined }}
      >
        {depth > 0 && <span className="mr-1 text-border">└</span>}
        {ctrl.control_id}
      </TableCell>
      <TableCell
        className={cn('cursor-pointer', ctrl.not_applicable && 'line-through text-secondary')}
        onClick={() => { navigate(`/vaktcomply/controls/${ctrl.id}?frameworkId=${frameworkId}`); }}
      >
        {ctrl.title}
      </TableCell>
      <TableCell onClick={(e) => { e.stopPropagation(); }}>
        <Select value={status} onValueChange={handleChange} disabled={updateControl.isPending}>
          <SelectTrigger className="h-7 text-xs w-44 gap-1">
            <StatusIndicator status={status} />
          </SelectTrigger>
          <SelectContent>
            {(Object.entries(STATUS_CONFIG) as [StatusChoice, typeof STATUS_CONFIG[StatusChoice]][]).map(([val, cfg]) => (
              <SelectItem key={val} value={val} className="text-xs">
                <span className={cn('flex items-center gap-2', cfg.className)}>
                  {cfg.icon}
                  {cfg.label}
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </TableCell>
      <TableCell
        className="text-secondary cursor-pointer"
        onClick={() => { navigate(`/vaktcomply/controls/${ctrl.id}?frameworkId=${frameworkId}`); }}
      >
        <ChevronRight className="w-4 h-4" />
      </TableCell>
    </TableRow>
  )
})

function DomainSection({
  domain,
  domainControls,
  frameworkId,
  updateControl,
  onMarkNA,
  selected,
  onToggleSelect,
  onToggleDomain,
}: {
  domain: string
  domainControls: Control[]
  frameworkId: string
  updateControl: ReturnType<typeof useUpdateControl>
  onMarkNA: (ctrl: Control) => void
  selected: Set<string>
  onToggleSelect: (id: string) => void
  onToggleDomain: (ids: string[], checked: boolean) => void
}) {
  const [open, setOpen] = useState(true)
  const { t } = useTranslation()

  const rootControls = domainControls.filter((c) => !hasParent(c, domainControls))
  const allIds = domainControls.map((c) => c.id)
  const allSelected = allIds.length > 0 && allIds.every((id) => selected.has(id))
  const someSelected = allIds.some((id) => selected.has(id))
  const doneCount = domainControls.filter(
    (c) => c.status === 'covered' || c.status === 'implemented',
  ).length

  return (
    <div className="border border-border rounded-lg overflow-x-auto">
      <button
        type="button"
        className="w-full flex items-center justify-between px-4 py-2.5 bg-surface2 hover:bg-surface text-left"
        onClick={() => { setOpen((v) => !v); }}
      >
        <div className="flex items-center gap-3">
          <ChevronDown className={cn('w-4 h-4 text-secondary transition-transform', !open && '-rotate-90')} />
          <span className="text-sm font-medium">{domain}</span>
          <span className="text-xs text-secondary">({domainControls.length} Maßnahmen)</span>
        </div>
        <span className="text-xs text-secondary">
          {doneCount}/{domainControls.length} umgesetzt
        </span>
      </button>

      {open && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <input
                  type="checkbox"
                  checked={allSelected}
                  ref={(el) => {
                    if (el) el.indeterminate = someSelected && !allSelected
                  }}
                  onChange={(e) => { onToggleDomain(allIds, e.target.checked); }}
                  className="rounded"
                  onClick={(e) => { e.stopPropagation(); }}
                />
              </TableHead>
              <TableHead className="w-32">{t('frameworkDetail.colId')}</TableHead>
              <TableHead>{t('frameworkDetail.colMeasure')}</TableHead>
              <TableHead className="w-48">{t('common.status')}</TableHead>
              <TableHead className="w-8"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rootControls.map((ctrl) => {
              const children = getChildren(ctrl, domainControls)
              return (
                <React.Fragment key={ctrl.id}>
                  <ControlRow
                    ctrl={ctrl}
                    frameworkId={frameworkId}
                    depth={0}
                    updateControl={updateControl}
                    onMarkNA={onMarkNA}
                    selected={selected.has(ctrl.id)}
                    onToggleSelect={onToggleSelect}
                  />
                  {children.map((child) => (
                    <ControlRow
                      key={child.id}
                      ctrl={child}
                      frameworkId={frameworkId}
                      depth={1}
                      updateControl={updateControl}
                      onMarkNA={onMarkNA}
                      selected={selected.has(child.id)}
                      onToggleSelect={onToggleSelect}
                    />
                  ))}
                </React.Fragment>
              )
            })}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

type ControlStatusChoice = 'missing' | 'in_progress' | 'implemented' | 'not_applicable'

function ControlsTab({
  frameworkId,
  controls,
  controlsLoading,
}: {
  frameworkId: string
  controls: Control[] | undefined
  controlsLoading: boolean
}) {
  const { t } = useTranslation()
  const updateControl = useUpdateControl(frameworkId)
  const bulkUpdateControls = useBulkUpdateControls()
  const [naDialog, setNaDialog] = useState<Control | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [statusDialogOpen, setStatusDialogOpen] = useState(false)
  const [pendingStatus, setPendingStatus] = useState<ControlStatusChoice>('implemented')

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleDomain(ids: string[], checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      for (const id of ids) {
        if (checked) next.add(id)
        else next.delete(id)
      }
      return next
    })
  }

  async function handleBulkStatusApply() {
    if (selected.size === 0) return
    const ids = Array.from(selected)
    // Map UI choice to API status value
    const apiStatus = pendingStatus === 'missing' ? 'not_implemented' : pendingStatus
    try {
      await bulkUpdateControls.mutateAsync({
        ids,
        status: apiStatus,
      })
      setSelected(new Set())
      setStatusDialogOpen(false)
      toast(t('common.statusUpdated'), 'success')
    } catch {
      toast(t('common.bulkUpdateFailed'), 'error')
    }
  }

  function handleExportSelected() {
    if (!controls) return
    const rows = controls
      .filter((c) => selected.has(c.id))
      .map((c) => ({
        id: c.id,
        control_id: c.control_id,
        title: c.title,
        domain: c.domain,
        status: c.status,
        evidence_count: c.evidence_count ?? 0,
        iso27001_mapping: c.iso27001_mapping ?? '',
        not_applicable: c.not_applicable ? 'ja' : 'nein',
        not_applicable_reason: c.not_applicable_reason ?? '',
      }))
    exportToCSV('controls-export', rows)
  }

  if (controlsLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-lg" />
        ))}
      </div>
    )
  }

  if (!controls || controls.length === 0) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title="Keine Controls"
        description="Für dieses Framework wurden keine Controls gefunden."
      />
    )
  }

  const byDomain = new Map<string, Control[]>()
  for (const ctrl of controls) {
    const list = byDomain.get(ctrl.domain) ?? []
    list.push(ctrl)
    byDomain.set(ctrl.domain, list)
  }

  const total = controls.length
  const covered = controls.filter((c) => c.status === 'covered' || c.status === 'implemented').length
  const partial = controls.filter((c) => c.status === 'partial' || c.status === 'in_progress').length
  const notApplicable = controls.filter((c) => c.status === 'not_applicable').length
  const open = controls.filter((c) => c.status === 'missing').length

  return (
    <>
      {/* Bulk status dialog */}
      <Dialog open={statusDialogOpen} onOpenChange={setStatusDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.risksPage.bulkStatusTitle')}</DialogTitle>
          </DialogHeader>
          <div className="py-3 space-y-3">
            <p className="text-sm text-secondary">
              Neuen Status für {selected.size} ausgewählte{selected.size === 1 ? 's Control' : ' Controls'} setzen:
            </p>
            <Select value={pendingStatus} onValueChange={(v) => { setPendingStatus(v as ControlStatusChoice); }}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="missing">{t('vaktcomply.control.statusOpen')}</SelectItem>
                <SelectItem value="in_progress">{t('vaktcomply.control.statusInProgress')}</SelectItem>
                <SelectItem value="implemented">{t('vaktcomply.control.statusImplemented')}</SelectItem>
                <SelectItem value="not_applicable">{t('vaktcomply.control.statusNotApplicable')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setStatusDialogOpen(false); }}>{t('common.cancel')}</Button>
            <Button onClick={() => { void handleBulkStatusApply() }} disabled={bulkUpdateControls.isPending}>
              {bulkUpdateControls.isPending ? 'Wird gespeichert…' : 'Anwenden'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <div className="space-y-5">
        {/* Progress summary */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {[
            { label: t('vaktcomply.control.statusImplemented'), value: covered, color: 'text-green-600' },
            { label: t('vaktcomply.control.statusInProgress'), value: partial, color: 'text-yellow-600' },
            { label: t('vaktcomply.control.statusOpen'), value: open, color: 'text-red-500' },
            { label: t('vaktcomply.control.statusNotApplicable'), value: notApplicable, color: 'text-secondary' },
          ].map(({ label, value, color }) => (
            <div key={label} className="flex flex-col items-center py-3 px-4 bg-surface border border-border rounded-lg">
              <span className={cn('text-2xl font-bold', color)}>{value}</span>
              <span className="text-xs text-secondary mt-0.5">{label}</span>
            </div>
          ))}
        </div>

        {/* Progress bar */}
        <div className="space-y-1">
          <div className="flex justify-between text-xs text-secondary">
            <span>{t('vaktcomply.frameworkDetail.coveredOf', { covered, total })}</span>
            <span>{Math.round((covered / total) * 100)}%</span>
          </div>
          <div className="h-2 bg-surface2 rounded-full overflow-hidden">
            <div
              className="h-full bg-green-500 rounded-full transition-all"
              style={{ width: `${(covered / total) * 100}%` }}
            />
          </div>
        </div>

        {/* Controls grouped by domain — collapsible */}
        {Array.from(byDomain.entries()).map(([domain, domainControls]) => (
          <DomainSection
            key={domain}
            domain={domain}
            domainControls={domainControls}
            frameworkId={frameworkId}
            updateControl={updateControl}
            onMarkNA={setNaDialog}
            selected={selected}
            onToggleSelect={toggleSelect}
            onToggleDomain={toggleDomain}
          />
        ))}

        {naDialog && (
          <NotApplicableDialog
            control={naDialog}
            frameworkId={frameworkId}
            open
            onClose={() => { setNaDialog(null); }}
          />
        )}
      </div>

      <BulkActionBar
        selectedCount={selected.size}
        onClearSelection={() => { setSelected(new Set()); }}
        actions={[
          {
            label: 'Status setzen',
            icon: RefreshCw,
            onClick: () => { setStatusDialogOpen(true); },
          },
          {
            label: t('common.export'),
            icon: FileDown,
            onClick: handleExportSelected,
          },
        ]}
      />
    </>
  )
}

export default function FrameworkDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDate } = useFormatDate()
  const frameworkId = id ?? ''

  const { data: framework, isLoading: frameworkLoading, isError: frameworkError, refetch: refetchFramework } = useFramework(frameworkId)
  const { data: report, isLoading: reportLoading } = useReadinessReport(frameworkId)
  const { data: gaps, isLoading: gapsLoading } = useGapAnalysis(frameworkId)
  const { data: controls, isLoading: controlsLoading, isError: controlsError, refetch: refetchControls } = useFrameworkControls(frameworkId)
  const downloadPDF = useDownloadFrameworkPDF()
  const { t } = useTranslation()

  useMilestoneToast(report?.readiness_score)

  const frameworkTitle = frameworkLoading
    ? '…'
    : framework
    ? `${framework.name} Details`
    : 'Framework'

  const frameworkDesc = framework
    ? `Controls, Lückenanalyse und Auditor-Zugang für ${framework.name} verwalten.`
    : 'Readiness-Score, Lückenanalyse und Auditor-Zugang verwalten.'

  useEffect(() => {
    if (framework) trackPage(`/vaktcomply/frameworks/${frameworkId}`, framework.name, '📋')
  }, [framework?.id])

  return (
    <div className="flex flex-col h-full">
      <Breadcrumbs items={[
        { label: 'Vakt Comply', href: '/vaktcomply' },
        { label: framework?.name ?? 'Framework' },
      ]} />
      <PageHeader
        title={frameworkTitle}
        description={frameworkDesc}
        actions={
          <div className="flex items-center gap-2">
            {framework?.name === 'TISAX' && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => { navigate(`/vaktcomply/frameworks/${frameworkId}/tisax`); }}
              >
                TISAX-Ansicht öffnen
              </Button>
            )}
            {framework?.name === 'CIS' && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => { navigate('/vaktcomply/cis-controls'); }}
              >
                CIS Controls v8 öffnen
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                const sections = controls
                  ? Array.from(
                      controls.reduce((map, ctrl) => {
                        const list = map.get(ctrl.domain) ?? []
                        list.push(ctrl)
                        map.set(ctrl.domain, list)
                        return map
                      }, new Map<string, typeof controls>()),
                    ).map(([domain, domainControls]) => ({
                      heading: domain,
                      rows: domainControls.map((c) => [c.control_id, c.title, effectiveStatus(c)]),
                    }))
                  : []
                exportAsRTF(framework?.name ?? 'Framework', sections)
              }}
            >
              <FileDown className="w-4 h-4 mr-1" />
              RTF Export
            </Button>
            <Button variant="outline" size="sm" onClick={() => { downloadPDF(frameworkId, framework?.name); }}>
              <FileDown className="w-4 h-4 mr-1" />
              PDF Export
            </Button>
            <Button variant="outline" size="sm" onClick={() => { navigate('/vaktcomply/frameworks'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              Zurück
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {frameworkError && (
          <ErrorState
            message="Framework konnte nicht geladen werden."
            onRetry={() => void refetchFramework()}
          />
        )}

        {!frameworkError && controlsError && (
          <ErrorState
            message="Controls konnten nicht geladen werden."
            onRetry={() => void refetchControls()}
          />
        )}

        {/* Readiness score header */}
        <div className="flex items-center gap-6 p-6 bg-surface border border-border rounded-lg">
          {reportLoading ? (
            <Spinner size="lg" />
          ) : report ? (
            <>
              <div className="text-center">
                <div className={cn('text-5xl font-bold', scoreColor(report.readiness_score))}>
                  {Math.round(report.readiness_score)}%
                </div>
                <div className="text-sm text-secondary mt-1">Readiness Score</div>
              </div>
              <div className="flex gap-6 text-sm">
                <div className="text-center">
                  <div className="text-xl font-semibold text-green-600">{report.covered}</div>
                  <div className="text-secondary">{t('vaktcomply.control.statusImplemented')}</div>
                </div>
                <div className="text-center">
                  <div className="text-xl font-semibold text-yellow-600">{report.partial}</div>
                  <div className="text-secondary">{t('vaktcomply.frameworkDetail.statusPartial')}</div>
                </div>
                <div className="text-center">
                  <div className="text-xl font-semibold text-red-600">{report.missing}</div>
                  <div className="text-secondary">{t('vaktcomply.control.statusOpen')}</div>
                </div>
                <div className="text-center">
                  <div className="text-xl font-semibold text-secondary">{report.total_controls}</div>
                  <div className="text-secondary">{t('vaktcomply.frameworkDetail.total')}</div>
                </div>
              </div>
            </>
          ) : (
            <p className="text-sm text-secondary">Keine Readiness-Daten verfügbar.</p>
          )}
        </div>

        {/* DORA → ISO 27001 mapping block */}
        {framework?.name === 'DORA' && <DORAISOMapping />}

        {/* Tabs */}
        <Tabs defaultValue="controls">
          <TabsList>
            <TabsTrigger value="controls">
              Controls {controls ? `(${controls.length})` : ''}
            </TabsTrigger>
            <TabsTrigger value="gaps">
              Lücken {gaps ? `(${gaps.gaps.length})` : ''}
            </TabsTrigger>
            <TabsTrigger value="auditor-links">Auditor-Links</TabsTrigger>
          </TabsList>

          {/* Controls Tab */}
          <TabsContent value="controls">
            <ControlsTab
              frameworkId={frameworkId}
              controls={controls}
              controlsLoading={controlsLoading}
            />
          </TabsContent>

          {/* Gaps Tab */}
          <TabsContent value="gaps">
            {gapsLoading ? (
              <div className="flex items-center justify-center h-32">
                <Spinner size="md" />
              </div>
            ) : gaps && gaps.gaps.length > 0 ? (
              <div className="space-y-3">
                {gaps.gaps.map((gap) => (
                  <div
                    key={gap.control.id}
                    className="flex items-start gap-4 p-4 bg-surface border border-border rounded-lg"
                  >
                    <AlertTriangle className="w-5 h-5 text-yellow-500 mt-0.5 shrink-0" />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="font-mono text-xs text-secondary">{gap.control.control_id}</span>
                        <Badge variant={gapSeverityVariant(gap.reason)}>{gapSeverityLabel(gap.reason)}</Badge>
                        {gap.expires_at && (
                          <span className="text-xs text-secondary">
                            Läuft ab: {formatDate(gap.expires_at)}
                          </span>
                        )}
                      </div>
                      <p className="text-sm font-medium text-primary">{gap.control.title}</p>
                      <p className="text-sm text-secondary mt-1">{gapRecommendation(gap.reason)}</p>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyState
                icon={ShieldAlert}
                title="Keine Lücken gefunden"
                description="Keine Compliance-Lücken für dieses Framework erkannt."
              />
            )}
          </TabsContent>

          {/* Auditor Links Tab */}
          <TabsContent value="auditor-links">
            <AuditorLinksTab frameworkId={frameworkId} />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
