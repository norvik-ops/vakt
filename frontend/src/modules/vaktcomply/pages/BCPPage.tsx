import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ShieldAlert, Plus, Pencil, Trash2, ChevronDown, ChevronUp, AlertTriangle } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  useBCPPlans,
  useCreateBCPPlan,
  useUpdateBCPPlan,
  useDeleteBCPPlan,
  useBCPTests,
  useAddBCPTest,
} from '../hooks/useBCP'
import type { BCPPlan, BCPTest, CreateBCPPlanInput, CreateBCPTestInput } from '../types'

// ─── Constants ────────────────────────────────────────────────────────────────

const STATUS_CLASS: Record<BCPPlan['status'], string> = {
  draft: 'bg-secondary text-secondary-foreground',
  active: 'bg-green-500/20 text-green-400 border-green-500/30',
  archived: 'bg-muted text-muted-foreground',
}

const OUTCOME_CLASS: Record<BCPTest['outcome'], string> = {
  passed: 'bg-green-500/20 text-green-400 border-green-500/30',
  failed: 'bg-red-500/20 text-red-400 border-red-500/30',
  partial: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
}

const STALE_THRESHOLD_MS = 365 * 24 * 60 * 60 * 1000

function isTestStale(test: BCPTest | undefined): boolean {
  if (!test) return true
  return Date.now() - new Date(test.test_date).getTime() > STALE_THRESHOLD_MS
}

// ─── Empty forms ──────────────────────────────────────────────────────────────

// ESK-12: rto/rpo/schutzbedarfsklasse starten als null — "noch nicht festgelegt".
// Ein Vorbelegen mit 72/24/2 waere genau der Defekt, den dieser Fix beseitigt:
// eine BSI-200-4-Angabe, die aussieht, als haette jemand sie entschieden.
function emptyPlanForm(): CreateBCPPlanInput {
  return {
    title: '',
    scope: '',
    version: '1.0',
    status: 'draft',
    owner: '',
    rto_hours: null,
    rpo_hours: null,
    schutzbedarfsklasse: null,
  }
}

// Traegt ALLE Felder in das Formular, damit das Bearbeiten-Formular den
// gespeicherten Zustand zeigt und nicht einen leeren.
//
// Zur Semantik des PATCH (REV-ESK12 B1): ein FEHLENDES Feld laesst den
// gespeicherten Wert unveraendert, ein `null` loescht ihn. Diese Funktion darf
// deshalb nicht `undefined` mit `null` verwechseln — sie uebernimmt die Werte
// des Plans unveraendert, und ein Feld, das der Nutzer im Dialog leert, geht als
// ausdrueckliches `null` ans Backend (numOrNull). Waere hier ein Feld
// vergessen, saehe der Nutzer es im Formular nicht; geloescht wuerde es nicht.
function planToForm(p: BCPPlan): CreateBCPPlanInput {
  return {
    title: p.title,
    scope: p.scope,
    version: p.version,
    status: p.status,
    owner: p.owner,
    rto_hours: p.rto_hours,
    rpo_hours: p.rpo_hours,
    schutzbedarfsklasse: p.schutzbedarfsklasse,
  }
}

// Leeres Zahlenfeld -> null ("nicht festgelegt"), nicht 0. Die 0 wuerde die API
// mit 422 ablehnen (rto_hours >= 1) und saehe in der Anzeige aus wie "sofort".
function numOrNull(raw: string): number | null {
  const trimmed = raw.trim()
  if (trimmed === '') return null
  const n = Number(trimmed)
  return Number.isFinite(n) ? n : null
}

function emptyTestForm(planId: string): CreateBCPTestInput {
  return { plan_id: planId, test_date: '', test_type: 'tabletop', outcome: 'passed', findings: '' }
}

// ─── Test list sub-component ──────────────────────────────────────────────────

function BCPTestList({ plan }: { plan: BCPPlan }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [testDialogOpen, setTestDialogOpen] = useState(false)
  const [testForm, setTestForm] = useState<CreateBCPTestInput>(emptyTestForm(plan.id))
  const { data: tests = [], isLoading } = useBCPTests(open ? plan.id : '')
  const addTest = useAddBCPTest()

  const latestTest = tests.length > 0 ? tests[0] : undefined
  const stale = isTestStale(latestTest)

  function handleAddTest() {
    addTest.mutate(testForm, {
      onSuccess: () => {
        setTestDialogOpen(false)
        setTestForm(emptyTestForm(plan.id))
      },
    })
  }

  return (
    <div className="mt-3 border-t pt-3">
      <button
        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
        onClick={() => { setOpen((v) => !v); }}
      >
        {open ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
        {t('bcp.tests')}
        {stale && open && (
          <span className="ml-1 flex items-center gap-0.5 text-amber-400">
            <AlertTriangle className="w-3 h-3" />
            {t('bcp.testStaleWarning')}
          </span>
        )}
      </button>
      {open && (
        <div className="mt-2 space-y-2">
          {stale && (
            <div className="p-2 rounded bg-amber-500/10 border border-amber-500/30 text-amber-400 text-xs flex items-start gap-1.5">
              <AlertTriangle className="w-3.5 h-3.5 shrink-0 mt-0.5" />
              {t('bcp.testStaleDescription')}
            </div>
          )}
          {isLoading && <Spinner size="sm" color="primary" />}
          {tests.map((test) => (
            <div key={test.id} className="flex items-center gap-2 text-xs">
              <Badge className={OUTCOME_CLASS[test.outcome]} variant="outline">
                {test.outcome}
              </Badge>
              <span className="text-muted-foreground">{test.test_type}</span>
              <span>{test.test_date?.slice(0, 10)}</span>
              {test.findings && (
                <span className="text-muted-foreground truncate max-w-xs">{test.findings}</span>
              )}
            </div>
          ))}
          <Button
            variant="outline"
            size="sm"
            className="h-6 text-xs"
            onClick={() => {
              setTestForm(emptyTestForm(plan.id))
              setTestDialogOpen(true)
            }}
          >
            <Plus className="w-3 h-3 mr-1" />
            {t('bcp.addTest')}
          </Button>
        </div>
      )}

      <Dialog open={testDialogOpen} onOpenChange={setTestDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('bcp.addTest')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>{t('bcp.testDate')} *</Label>
              <Input
                type="date"
                value={testForm.test_date}
                onChange={(e) => { setTestForm((f) => ({ ...f, test_date: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('bcp.testType')} *</Label>
              <Select
                value={testForm.test_type}
                onValueChange={(v) => { setTestForm((f) => ({ ...f, test_type: v as BCPTest['test_type'] })); }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="tabletop">{t('bcp.tabletop')}</SelectItem>
                  <SelectItem value="walkthrough">{t('bcp.walkthrough')}</SelectItem>
                  <SelectItem value="fulltest">{t('bcp.fulltest')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>{t('bcp.outcome')} *</Label>
              <Select
                value={testForm.outcome}
                onValueChange={(v) => { setTestForm((f) => ({ ...f, outcome: v as BCPTest['outcome'] })); }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="passed">{t('bcp.passed')}</SelectItem>
                  <SelectItem value="failed">{t('bcp.failed')}</SelectItem>
                  <SelectItem value="partial">{t('bcp.partial')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>{t('bcp.findings')}</Label>
              <Textarea
                rows={3}
                value={testForm.findings ?? ''}
                onChange={(e) => { setTestForm((f) => ({ ...f, findings: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setTestDialogOpen(false); }}>{t('common.cancel')}</Button>
            <Button
              disabled={!testForm.test_date || addTest.isPending}
              onClick={handleAddTest}
            >
              {addTest.isPending ? t('common.saving') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ─── BSI-200-4-Kennzahlen des Plans ───────────────────────────────────────────

// ESK-12: Ein nicht festgelegter Wert wird als "nicht festgelegt" ausgewiesen,
// nicht als Zahl. Wer RTO/RPO fuer einen Auditor liest, muss "noch offen" von
// "72 Stunden entschieden" unterscheiden koennen — bis zu diesem Fix lieferte
// die API fuer jeden Plan 72/24/2 und die Seite zeigte nichts davon.
function BCPPlanTargets({ plan }: { plan: BCPPlan }) {
  const { t } = useTranslation()
  const notSet = <span className="text-muted-foreground italic">{t('bcp.notSet')}</span>
  const klasseLabel = plan.schutzbedarfsklasse
    ? `${String(plan.schutzbedarfsklasse)} — ${t(`bcm.bia.klasse${String(plan.schutzbedarfsklasse)}`)}`
    : null

  return (
    <dl className="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
      <div className="flex gap-1.5">
        <dt className="text-muted-foreground">RTO (h)</dt>
        <dd>{plan.rto_hours ?? notSet}</dd>
      </div>
      <div className="flex gap-1.5">
        <dt className="text-muted-foreground">RPO (h)</dt>
        <dd>{plan.rpo_hours ?? notSet}</dd>
      </div>
      <div className="flex gap-1.5">
        <dt className="text-muted-foreground">{t('bcm.bia.schutzbedarfsklasse')}</dt>
        <dd>{klasseLabel ?? notSet}</dd>
      </div>
      <div className="flex gap-1.5">
        <dt className="text-muted-foreground">{t('bcp.lastTested')}</dt>
        <dd>{plan.last_tested_at ?? notSet}</dd>
      </div>
    </dl>
  )
}

// ─── Plan row ─────────────────────────────────────────────────────────────────

function BCPPlanCard({
  plan,
  onEdit,
  onDelete,
}: {
  plan: BCPPlan
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1 flex-1">
            <CardTitle className="text-base">{plan.title}</CardTitle>
            <div className="flex items-center gap-2 flex-wrap">
              <Badge className={STATUS_CLASS[plan.status]} variant="outline">
                {t(`bcp.status.${plan.status}`)}
              </Badge>
              <span className="text-xs text-muted-foreground">v{plan.version}</span>
              {plan.owner && (
                <span className="text-xs text-muted-foreground">{plan.owner}</span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit}>
              <Pencil className="w-3.5 h-3.5" />
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
        {plan.scope && (
          <p className="text-xs text-muted-foreground">{plan.scope}</p>
        )}
      </CardHeader>
      <CardContent className="pt-0">
        <BCPPlanTargets plan={plan} />
        <BCPTestList plan={plan} />
      </CardContent>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function BCPPage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateBCPPlanInput>(emptyPlanForm())

  const { data: plans = [], isLoading, isError } = useBCPPlans()
  const createPlan = useCreateBCPPlan()
  const updatePlan = useUpdateBCPPlan(editId ?? '')
  const deletePlan = useDeleteBCPPlan()

  function openCreate() {
    setEditId(null)
    setForm(emptyPlanForm())
    setDialogOpen(true)
  }

  function openEdit(p: BCPPlan) {
    setEditId(p.id)
    setForm(planToForm(p))
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm(t('bcp.deleteConfirm'))) {
      deletePlan.mutate(id)
    }
  }

  function handleSubmit() {
    if (editId) {
      // ESK-12 (Variante, im selben Diff mitgenommen): hier stand fest
      // `status: 'draft'`. Jede Bearbeitung eines aktiven oder archivierten
      // Plans setzte ihn damit stillschweigend auf Entwurf zurueck — dieselbe
      // Klasse wie der Befund selbst (ein Wert, den niemand gewaehlt hat, wird
      // als gewaehlt geschrieben), nur an einem anderen Feld. Der Status kommt
      // jetzt aus dem Formular und ist im Dialog auswaehlbar.
      updatePlan.mutate(
        { ...form, status: form.status ?? 'draft' },
        { onSuccess: () => { setDialogOpen(false); } },
      )
    } else {
      createPlan.mutate(form, { onSuccess: () => { setDialogOpen(false); } })
    }
  }

  const isPending = createPlan.isPending || updatePlan.isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('bcp.title')}
        description={t('bcp.description')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            {t('bcp.newPlan')}
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            {t('bcp.loadError')}
          </div>
        )}
        {!isLoading && !isError && plans.length === 0 && (
          <EmptyState
            icon={ShieldAlert}
            title={t('bcp.emptyTitle')}
            description={t('bcp.emptyDescription')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('bcp.newPlan')}
              </Button>
            }
          />
        )}
        {!isLoading && !isError && plans.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {plans.map((p) => (
              <BCPPlanCard
                key={p.id}
                plan={p}
                onEdit={() => { openEdit(p); }}
                onDelete={() => { handleDelete(p.id); }}
              />
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editId ? t('bcp.editPlan') : t('bcp.newPlan')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('bcp.planTitle')} *</Label>
              <Input
                placeholder={t('bcp.planTitlePlaceholder')}
                value={form.title}
                onChange={(e) => { setForm((f) => ({ ...f, title: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('bcp.scope')}</Label>
              <Textarea
                rows={2}
                placeholder={t('bcp.scopePlaceholder')}
                value={form.scope ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, scope: e.target.value })); }}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('bcp.version')}</Label>
                <Input
                  placeholder="1.0"
                  value={form.version ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, version: e.target.value })); }}
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('bcp.owner')}</Label>
                <Input
                  placeholder={t('bcp.ownerPlaceholder')}
                  value={form.owner ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, owner: e.target.value })); }}
                />
              </div>
            </div>
            {/* ESK-12: die drei BSI-200-4-Angaben. Leer bleiben ist erlaubt und
                heisst "noch nicht festgelegt" — es wird nichts vorbelegt. */}
            <div className="grid grid-cols-3 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="bcp-rto">RTO (h)</Label>
                <Input
                  id="bcp-rto"
                  type="number"
                  min={1}
                  max={8760}
                  placeholder={t('bcp.notSet')}
                  value={form.rto_hours ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, rto_hours: numOrNull(e.target.value) })); }}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="bcp-rpo">RPO (h)</Label>
                <Input
                  id="bcp-rpo"
                  type="number"
                  min={1}
                  max={8760}
                  placeholder={t('bcp.notSet')}
                  value={form.rpo_hours ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, rpo_hours: numOrNull(e.target.value) })); }}
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('bcm.bia.schutzbedarfsklasse')}</Label>
                <Select
                  value={form.schutzbedarfsklasse ? String(form.schutzbedarfsklasse) : 'none'}
                  onValueChange={(v) => {
                    setForm((f) => ({
                      ...f,
                      schutzbedarfsklasse: v === 'none' ? null : (Number(v) as 1 | 2 | 3),
                    }))
                  }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">{t('bcp.notSet')}</SelectItem>
                    <SelectItem value="1">1 — {t('bcm.bia.klasse1')}</SelectItem>
                    <SelectItem value="2">2 — {t('bcm.bia.klasse2')}</SelectItem>
                    <SelectItem value="3">3 — {t('bcm.bia.klasse3')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            {editId && (
              <div className="space-y-1.5">
                <Label>{t('bcp.statusLabel')}</Label>
                <Select
                  value={form.status ?? 'draft'}
                  onValueChange={(v) => { setForm((f) => ({ ...f, status: v as BCPPlan['status'] })); }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="draft">{t('bcp.status.draft')}</SelectItem>
                    <SelectItem value="active">{t('bcp.status.active')}</SelectItem>
                    <SelectItem value="archived">{t('bcp.status.archived')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={!form.title || isPending}>
              {isPending ? t('common.saving') : editId ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
