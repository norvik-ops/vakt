import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Bot, Plus, Pencil, Trash2, FlaskConical, FileText } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button, buttonVariants } from '../../../components/ui/button'
import { cn } from '../../../lib/utils'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../../../components/ui/alert-dialog'
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
  useAISystems,
  useDeleteAISystem,
  useCreateAISystem,
  useUpdateAISystem,
} from '../hooks/useAISystems'
import { AISystemStatusBadge } from '../components/AISystemStatusBadge'
import { AIClassificationWizard } from '../components/AIClassificationWizard'
import {
  RISK_CLASS_CSS as RISK_CLASS,
  RISK_CLASS_LABELS as RISK_LABELS,
} from '../components/aiRiskClassConfig'
import type { AISystem, CreateAISystemInput, UpdateAISystemInput } from '../types'
import { FeatureLockedError } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'

// AUTONOMY_I18N_KEY: map domain enums → i18n keys. Resolved with t() inside
// the component body. Module-level so the strings remain a static lookup.
const AUTONOMY_I18N_KEY: Record<string, string> = {
  assistive: 'vaktcomply.aiSystems.autonomyLevel.assistive',
  partial: 'vaktcomply.aiSystems.autonomyLevel.semiAutonomous',
  full: 'vaktcomply.aiSystems.autonomyLevel.fullyAutonomous',
}

function emptyForm(): CreateAISystemInput {
  return {
    name: '',
    description: '',
    provider: '',
    use_case: '',
    affected_groups: '',
    autonomy_level: 'assistive',
    risk_class: undefined,
    classification_rationale: '',
  }
}

function systemToForm(a: AISystem): UpdateAISystemInput {
  return {
    name: a.name,
    description: a.description ?? '',
    provider: a.provider ?? '',
    use_case: a.use_case ?? '',
    affected_groups: a.affected_groups ?? '',
    autonomy_level: a.autonomy_level,
    in_production_since: a.in_production_since ? a.in_production_since.slice(0, 10) : undefined,
    status: a.status,
    risk_class: a.risk_class,
    classification_rationale: a.classification_rationale ?? '',
    classified_by: a.classified_by ?? '',
  }
}

/**
 * Inventory card. UX-33: the actions were four bare icons (flask, sheet, pencil,
 * bin) — the central step of Kernweg 10, „Risikoklasse bestimmen", was an unlabelled
 * flask. The two main actions are now labelled buttons; edit and delete stay icons
 * but carry a tooltip and an aria-label.
 */
function AISystemCard({
  system,
  onEdit,
  onDelete,
  onClassify,
}: {
  system: AISystem
  onEdit: () => void
  onDelete: () => void
  onClassify: () => void
}) {
  const { t } = useTranslation()
  const editLabel = t('vaktcomply.aiSystems.actions.edit')
  const deleteLabel = t('vaktcomply.aiSystems.actions.delete')
  return (
    <Card>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm">{system.name}</p>
          <div className="flex items-center gap-1 shrink-0">
            <AISystemStatusBadge status={system.status} />
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              title={editLabel}
              aria-label={`${editLabel}: ${system.name}`}
              onClick={onEdit}
            >
              <Pencil className="w-3.5 h-3.5" aria-hidden="true" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-red-400 hover:text-red-300"
              title={deleteLabel}
              aria-label={`${deleteLabel}: ${system.name}`}
              data-testid="delete-ai-system-btn"
              onClick={onDelete}
            >
              <Trash2 className="w-3.5 h-3.5" aria-hidden="true" />
            </Button>
          </div>
        </div>
        {system.provider && (
          <p className="text-xs text-muted-foreground">Anbieter: {system.provider}</p>
        )}
        {system.use_case && (
          <p className="text-xs text-muted-foreground line-clamp-2">{system.use_case}</p>
        )}
        <div className="flex flex-wrap gap-1.5">
          {system.risk_class ? (
            <Badge className={RISK_CLASS[system.risk_class] ?? ''}>
              {RISK_LABELS[system.risk_class] ?? system.risk_class}
            </Badge>
          ) : (
            <Badge variant="outline" className="text-xs" data-testid="risk-class-open">
              {t('vaktcomply.aiSystems.riskClassOpen')}
            </Badge>
          )}
          <Badge variant="outline" className="text-xs">
            {AUTONOMY_I18N_KEY[system.autonomy_level]
              ? t(AUTONOMY_I18N_KEY[system.autonomy_level])
              : system.autonomy_level}
          </Badge>
        </div>
        {system.affected_groups && (
          <p className="text-xs text-muted-foreground">
            Betroffene Gruppen: {system.affected_groups}
          </p>
        )}
        <div className="flex flex-wrap gap-2 pt-1">
          <Button
            variant={system.risk_class ? 'outline' : 'default'}
            size="sm"
            data-testid="classify-ai-system-btn"
            onClick={onClassify}
          >
            <FlaskConical className="w-3.5 h-3.5" aria-hidden="true" />
            {t('vaktcomply.aiSystems.actions.classify')}
          </Button>
          {/* R1-SA28-01: der Link stand als `ai-systems/${system.id}/documentation` hier.
              Diese Seite ist SELBST unter `ai-systems` gemountet (SecVitalsRoutes.tsx:106),
              ein relativer Link haengt also an den bereits vorhandenen Pfad an — Ergebnis
              war `/vaktcomply/ai-systems/ai-systems/<id>/documentation`. Kein Treffer,
              path='*' greift, und der Nutzer landet STILL auf der Comply-Uebersicht statt
              auf einem 404. Das war der einzige Einstieg in die Art.-11-Technikdoku des
              EU AI Act. */}
          <Link
            to={`${system.id}/documentation`}
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
          >
            <FileText className="w-3.5 h-3.5" aria-hidden="true" />
            {t('vaktcomply.aiSystems.actions.documentation')}
          </Link>
        </div>
      </CardContent>
    </Card>
  )
}

export default function AISystemsPage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateAISystemInput | UpdateAISystemInput>(emptyForm())
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [filterRiskClass, setFilterRiskClass] = useState('')
  const [filterStatus, setFilterStatus] = useState('')
  const [wizardSystem, setWizardSystem] = useState<AISystem | null>(null)

  const filters = {
    riskClass: filterRiskClass || undefined,
    status: filterStatus || undefined,
  }

  const { data: systems, isLoading, isError, error } = useAISystems(filters)
  // Gesperrt (402): keine Schreibknöpfe neben der Sperre anbieten.
  const locked = error instanceof FeatureLockedError
  const createSystem = useCreateAISystem()
  const updateSystem = useUpdateAISystem(editId ?? '')
  const deleteSystem = useDeleteAISystem()

  function openCreate() {
    setEditId(null)
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function openEdit(a: AISystem) {
    setEditId(a.id)
    setForm(systemToForm(a))
    setDialogOpen(true)
  }

  function handleSubmit() {
    if (editId) {
      updateSystem.mutate(form, {
        onSuccess: () => {
          setDialogOpen(false)
        },
      })
    } else {
      createSystem.mutate(form, {
        onSuccess: () => {
          setDialogOpen(false)
        },
      })
    }
  }

  function confirmDelete() {
    if (!deleteId) return
    deleteSystem.mutate(deleteId, {
      onSuccess: () => {
        setDeleteId(null)
      },
    })
  }

  const isPending = createSystem.isPending || updateSystem.isPending
  const isUpdate = !!editId
  const editSystem = editId ? systems?.find((s) => s.id === editId) : undefined

  const hasFilters = filterRiskClass || filterStatus

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktcomply.aiSystems.title')}
        description={t('vaktcomply.aiSystems.description')}
        actions={
          locked ? undefined : (
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vaktcomply.aiSystems.add')}
            </Button>
          )
        }
      />

      {/* Filter Toolbar */}
      <div className="px-6 pb-2 flex flex-wrap gap-3 items-center" data-testid="ai-filter-toolbar">
        <div className="flex items-center gap-2">
          <Label className="text-xs">{t('vaktcomply.aiSystems.fields.riskClass')}</Label>
          <Select
            value={filterRiskClass || '_all'}
            onValueChange={(v) => {
              setFilterRiskClass(v === '_all' ? '' : v)
            }}
          >
            <SelectTrigger className="h-8 w-44" data-testid="filter-risk-class">
              <SelectValue placeholder={t('vaktcomply.aiSystems.filterAll')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">{t('vaktcomply.aiSystems.filterAll')}</SelectItem>
              <SelectItem value="unacceptable">
                {t('vaktcomply.aiSystems.riskClassLevel.prohibited')}
              </SelectItem>
              <SelectItem value="high">{t('vaktcomply.aiSystems.riskClassLevel.high')}</SelectItem>
              <SelectItem value="limited">
                {t('vaktcomply.aiSystems.riskClassLevel.limited')}
              </SelectItem>
              <SelectItem value="minimal">
                {t('vaktcomply.aiSystems.riskClassLevel.minimal')}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2">
          <Label className="text-xs">{t('vaktcomply.aiSystems.fields.status')}</Label>
          <Select
            value={filterStatus || '_all'}
            onValueChange={(v) => {
              setFilterStatus(v === '_all' ? '' : v)
            }}
          >
            <SelectTrigger className="h-8 w-44" data-testid="filter-status">
              <SelectValue placeholder={t('vaktcomply.aiSystems.filterAll')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">{t('vaktcomply.aiSystems.filterAll')}</SelectItem>
              <SelectItem value="under_review">
                {t('vaktcomply.aiSystems.statusLevel.classified')}
              </SelectItem>
              <SelectItem value="approved">
                {t('vaktcomply.aiSystems.statusLevel.approved')}
              </SelectItem>
              <SelectItem value="compliant">
                {t('vaktcomply.aiSystems.statusLevel.compliant')}
              </SelectItem>
              <SelectItem value="decommissioned">
                {t('vaktcomply.aiSystems.statusLevel.decommissioned')}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        {hasFilters && (
          <Button
            variant="ghost"
            size="sm"
            className="text-xs"
            onClick={() => {
              setFilterRiskClass('')
              setFilterStatus('')
            }}
          >
            Filter zurücksetzen
          </Button>
        )}
      </div>

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}
        {isError && (
          <ProGate error={locked ? error : null}>
            <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
              Fehler beim Laden des KI-Inventars.
            </div>
          </ProGate>
        )}
        {!isLoading && !isError && systems?.length === 0 && (
          <EmptyState
            icon={Bot}
            title={t('vaktcomply.aiSystems.emptyTitle')}
            description={t('vaktcomply.aiSystems.emptyDescription')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('vaktcomply.aiSystems.add')}
              </Button>
            }
          />
        )}
        {!isLoading && !isError && systems && systems.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {systems.map((a) => (
              <AISystemCard
                key={a.id}
                system={a}
                onEdit={() => {
                  openEdit(a)
                }}
                onDelete={() => {
                  setDeleteId(a.id)
                }}
                onClassify={() => {
                  setWizardSystem(a)
                }}
              />
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {isUpdate ? t('vaktcomply.aiSystems.edit') : t('vaktcomply.aiSystems.add')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.aiSystems.fields.name')} *</Label>
              <Input
                placeholder={t('vaktcomply.aiSystems.placeholders.name')}
                value={form.name}
                onChange={(e) => {
                  setForm((f) => ({ ...f, name: e.target.value }))
                }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.aiSystems.fields.provider')}</Label>
              <Input
                placeholder={t('vaktcomply.aiSystems.placeholders.provider')}
                value={form.provider ?? ''}
                onChange={(e) => {
                  setForm((f) => ({ ...f, provider: e.target.value }))
                }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.aiSystems.fields.useCase')}</Label>
              <Textarea
                rows={2}
                placeholder={t('vaktcomply.aiSystems.placeholders.useCase')}
                value={form.use_case ?? ''}
                onChange={(e) => {
                  setForm((f) => ({ ...f, use_case: e.target.value }))
                }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.aiSystems.fields.description')}</Label>
              <Textarea
                rows={2}
                placeholder={t('vaktcomply.aiSystems.placeholders.description')}
                value={form.description ?? ''}
                onChange={(e) => {
                  setForm((f) => ({ ...f, description: e.target.value }))
                }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.aiSystems.fields.affectedGroups')}</Label>
              <Input
                placeholder={t('vaktcomply.aiSystems.placeholders.affectedGroups')}
                value={form.affected_groups ?? ''}
                onChange={(e) => {
                  setForm((f) => ({ ...f, affected_groups: e.target.value }))
                }}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.aiSystems.fields.autonomy')}</Label>
                <Select
                  value={form.autonomy_level ?? 'assistive'}
                  onValueChange={(v) => {
                    setForm((f) => ({ ...f, autonomy_level: v as AISystem['autonomy_level'] }))
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="assistive">
                      {t('vaktcomply.aiSystems.autonomyLevel.assistive')}
                    </SelectItem>
                    <SelectItem value="partial">
                      {t('vaktcomply.aiSystems.autonomyLevel.semiAutonomous')}
                    </SelectItem>
                    <SelectItem value="full">
                      {t('vaktcomply.aiSystems.autonomyLevel.fullyAutonomous')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            {/* UX-34: one way to the risk class — the assistant. The dialog used to offer
                a free choice next to it, which skipped the assistant's reasoning (Art. 5,
                Annex III) and its history entry. Here the class is only shown; the form
                passes the stored class, rationale and classifier through unchanged. */}
            <div className="space-y-1.5 rounded-md border border-border p-3" data-testid="ai-risk-class-info">
              <p className="text-sm font-medium">{t('vaktcomply.aiSystems.fields.riskClass')}</p>
              {form.risk_class ? (
                <Badge className={RISK_CLASS[form.risk_class] ?? ''}>
                  {RISK_LABELS[form.risk_class] ?? form.risk_class}
                </Badge>
              ) : (
                <p className="text-sm text-muted-foreground">{t('vaktcomply.aiSystems.riskClassOpen')}</p>
              )}
              {form.classification_rationale && (
                <p className="text-xs text-muted-foreground whitespace-pre-wrap">
                  {t('vaktcomply.aiSystems.fields.classification')}: {form.classification_rationale}
                </p>
              )}
              <p className="text-xs text-muted-foreground">{t('vaktcomply.aiSystems.riskClassViaWizard')}</p>
              {editSystem && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setDialogOpen(false)
                    setWizardSystem(editSystem)
                  }}
                >
                  <FlaskConical className="w-3.5 h-3.5" aria-hidden="true" />
                  {t('vaktcomply.aiSystems.actions.classify')}
                </Button>
              )}
            </div>
            {isUpdate && (
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.aiSystems.fields.status')}</Label>
                <Select
                  value={(form as UpdateAISystemInput).status ?? 'under_review'}
                  onValueChange={(v) => {
                    setForm((f) => ({ ...f, status: v as AISystem['status'] }))
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="under_review">
                      {t('vaktcomply.aiSystems.statusOptions.under_review')}
                    </SelectItem>
                    <SelectItem value="approved">
                      {t('vaktcomply.aiSystems.statusOptions.approved')}
                    </SelectItem>
                    <SelectItem value="prohibited">
                      {t('vaktcomply.aiSystems.statusOptions.prohibited')}
                    </SelectItem>
                    <SelectItem value="decommissioned">
                      {t('vaktcomply.aiSystems.statusOptions.decommissioned')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.aiSystems.fields.inProductionSince')}</Label>
              <Input
                type="date"
                value={(form as UpdateAISystemInput).in_production_since ?? ''}
                onChange={(e) => {
                  setForm((f) => ({ ...f, in_production_since: e.target.value || undefined }))
                }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setDialogOpen(false)
              }}
            >
              Abbrechen
            </Button>
            <Button onClick={handleSubmit} disabled={!form.name || isPending}>
              {isPending ? 'Speichern …' : isUpdate ? 'Speichern' : 'Erfassen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {wizardSystem && (
        <AIClassificationWizard
          systemId={wizardSystem.id}
          systemName={wizardSystem.name}
          open={!!wizardSystem}
          onClose={() => {
            setWizardSystem(null)
          }}
        />
      )}

      <AlertDialog
        open={!!deleteId}
        onOpenChange={(open) => {
          if (!open) setDeleteId(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('vaktcomply.aiSystems.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('vaktcomply.aiSystems.deleteDescription')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-red-600 hover:bg-red-700"
              onClick={confirmDelete}
              disabled={deleteSystem.isPending}
              data-testid="confirm-delete-btn"
            >
              Löschen
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
