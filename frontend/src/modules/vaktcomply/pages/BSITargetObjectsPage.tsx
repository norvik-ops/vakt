// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2, ChevronRight, Server, AppWindow, Network, Building, Workflow, GitBranch, ArrowRightLeft, ShieldAlert, Pencil } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
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
  useBSITargetObjects,
  useCreateBSITargetObject,
  useDeleteBSITargetObject,
  useBSIObjectDependencies,
  useCreateBSIObjectDependency,
  useDeleteBSIObjectDependency,
  useUpdateBSIObjectProtectionOverride,
  useBSIRisks,
  useCreateBSIRisk,
  useUpdateBSIRisk,
  useDeleteBSIRisk,
  useBSIThreats,
} from '../hooks/useBSICheck'
import type {
  BSITargetObject,
  BSITargetObjectType,
  BSIAbsicherungsniveau,
  BSISchutzbedarf,
  BSIDependencyType,
  BSIOverrideEffect,
  BSIRiskAssessment,
  BSIEintrittshaeufigkeit,
  BSISchadensauswirkung,
  BSIBehandlungsoption,
  BSIRisikokategorie,
  CreateBSITargetObjectInput,
  UpdateBSIObjectProtectionOverrideInput,
  UpdateBSIRiskInput,
} from '../types'

// ── helpers ────────────────────────────────────────────────────────────────────

const TYPE_ICONS: Record<BSITargetObjectType, React.ElementType> = {
  it_system: Server,
  application: AppWindow,
  network: Network,
  room: Building,
  process: Workflow,
}

const NIVEAU_COLORS: Record<BSIAbsicherungsniveau, string> = {
  basis: 'bg-blue-900/30 text-blue-300 border-blue-800',
  standard: 'bg-green-900/30 text-green-300 border-green-800',
  kern: 'bg-purple-900/30 text-purple-300 border-purple-800',
}

const SCHUTZBEDARF_COLORS: Record<BSISchutzbedarf, string> = {
  normal: 'text-green-400',
  hoch: 'text-yellow-400',
  sehr_hoch: 'text-red-400',
}

function CIABadge({ label, value, inheritedFrom, objectName }: {
  label: string
  value?: BSISchutzbedarf
  inheritedFrom?: string
  objectName?: string
}) {
  const { t } = useTranslation()
  if (!value) return <span className="text-[11px] text-secondary">{label}:–</span>
  const color = SCHUTZBEDARF_COLORS[value]
  return (
    <span
      className={`text-[11px] font-mono ${color}`}
      title={inheritedFrom ? t('bsi.cia.inherited', { name: objectName ?? inheritedFrom }) : undefined}
    >
      {label}:{value[0].toUpperCase()}
      {inheritedFrom && <span className="ml-0.5 text-[9px] opacity-70">↑</span>}
    </span>
  )
}

// ── Create Dialog ──────────────────────────────────────────────────────────────

interface CreateDialogProps {
  open: boolean
  onClose: () => void
}

function CreateDialog({ open, onClose }: CreateDialogProps) {
  const { t } = useTranslation()
  const create = useCreateBSITargetObject()
  const [form, setForm] = useState<CreateBSITargetObjectInput>({
    name: '',
    type: 'it_system',
    absicherungsniveau: 'standard',
  })

  const typeLabels: Record<BSITargetObjectType, string> = {
    it_system: t('bsi.type.it_system'),
    application: t('bsi.type.application'),
    network: t('bsi.type.network'),
    room: t('bsi.type.room'),
    process: t('bsi.type.process'),
  }

  function set<K extends keyof CreateBSITargetObjectInput>(k: K, v: CreateBSITargetObjectInput[K]) {
    setForm((f) => ({ ...f, [k]: v }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) return
    await create.mutateAsync(form)
    onClose()
    setForm({ name: '', type: 'it_system', absicherungsniveau: 'standard' })
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('bsi.createDialog.title')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="to-name">{t('bsi.createDialog.nameLabel')} *</Label>
            <Input
              id="to-name"
              value={form.name}
              onChange={(e) => { set('name', e.target.value) }}
              placeholder={t('bsi.createDialog.namePlaceholder')}
              required
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>{t('bsi.createDialog.typeLabel')} *</Label>
              <Select value={form.type} onValueChange={(v) => { set('type', v as BSITargetObjectType) }}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(typeLabels) as BSITargetObjectType[]).map((tp) => (
                    <SelectItem key={tp} value={tp}>{typeLabels[tp]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>{t('bsi.createDialog.niveauLabel')}</Label>
              <Select
                value={form.absicherungsniveau ?? 'standard'}
                onValueChange={(v) => { set('absicherungsniveau', v as BSIAbsicherungsniveau) }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="basis">{t('bsi.niveau.basis')}</SelectItem>
                  <SelectItem value="standard">{t('bsi.niveau.standard')}</SelectItem>
                  <SelectItem value="kern">{t('bsi.niveau.kern')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>{t('bsi.createDialog.descriptionLabel')}</Label>
            <Input
              value={form.description ?? ''}
              onChange={(e) => { set('description', e.target.value) }}
              placeholder={t('bsi.createDialog.descriptionPlaceholder')}
            />
          </div>

          <p className="text-xs text-secondary">
            {t('bsi.createDialog.hint')}
          </p>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              {t('bsi.createDialog.cancel')}
            </Button>
            <Button type="submit" disabled={create.isPending}>
              {t('bsi.createDialog.create')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── Dependencies Dialog ────────────────────────────────────────────────────────

interface DepsDialogProps {
  object: BSITargetObject
  allObjects: BSITargetObject[]
  onClose: () => void
}

function DepsDialog({ object, allObjects, onClose }: DepsDialogProps) {
  const { t } = useTranslation()
  const { data: deps = [], isLoading } = useBSIObjectDependencies(object.id)
  const createDep = useCreateBSIObjectDependency(object.id)
  const deleteDep = useDeleteBSIObjectDependency(object.id)
  const [targetId, setTargetId] = useState('')
  const [depType, setDepType] = useState<BSIDependencyType>('runs_on')

  const depTypeLabels: Record<BSIDependencyType, string> = {
    runs_on: t('bsi.depType.runs_on'),
    located_in: t('bsi.depType.located_in'),
    connected_to: t('bsi.depType.connected_to'),
    processes_for: t('bsi.depType.processes_for'),
  }

  const typeLabels: Record<BSITargetObjectType, string> = {
    it_system: t('bsi.type.it_system'),
    application: t('bsi.type.application'),
    network: t('bsi.type.network'),
    room: t('bsi.type.room'),
    process: t('bsi.type.process'),
  }

  const available = allObjects.filter((o) => o.id !== object.id)

  async function handleAdd() {
    if (!targetId) return
    await createDep.mutateAsync({ target_id: targetId, dependency_type: depType })
    setTargetId('')
  }

  const nameById = (id: string) => allObjects.find((o) => o.id === id)?.name ?? id

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('bsi.depsDialog.title', { name: object.name })}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <p className="text-xs text-secondary">
            {t('bsi.depsDialog.hint')}
            <br />{t('bsi.depsDialog.sourceHint')}
          </p>

          {isLoading && <p className="text-sm text-secondary">{t('bsi.depsDialog.loading')}</p>}

          {!isLoading && deps.length === 0 && (
            <p className="text-sm text-secondary italic">{t('bsi.depsDialog.empty')}</p>
          )}

          {deps.map((d) => (
            <div key={d.id} className="flex items-center justify-between gap-2 rounded-md border border-border px-3 py-2">
              <div className="text-sm min-w-0">
                <span className="font-medium text-primary">{d.source_id === object.id ? object.name : nameById(d.source_id)}</span>
                <span className="mx-1 text-secondary text-xs">{depTypeLabels[d.dependency_type]}</span>
                <span className="font-medium text-primary">{d.target_id === object.id ? object.name : nameById(d.target_id)}</span>
              </div>
              <Button
                size="icon"
                variant="ghost"
                className="w-6 h-6 text-red-400 hover:text-red-300 shrink-0"
                onClick={() => { deleteDep.mutate(d.id) }}
              >
                <Trash2 className="w-3 h-3" />
              </Button>
            </div>
          ))}

          <div className="border-t border-border pt-3 space-y-2">
            <p className="text-xs font-medium text-primary">{t('bsi.depsDialog.addNew')}</p>
            <div className="flex gap-2">
              <Select value={depType} onValueChange={(v) => { setDepType(v as BSIDependencyType) }}>
                <SelectTrigger className="w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(depTypeLabels) as BSIDependencyType[]).map((tp) => (
                    <SelectItem key={tp} value={tp}>{depTypeLabels[tp]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={targetId} onValueChange={setTargetId}>
                <SelectTrigger className="flex-1">
                  <SelectValue placeholder={t('bsi.depsDialog.targetPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  {available.map((o) => (
                    <SelectItem key={o.id} value={o.id}>{typeLabels[o.type]}: {o.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                size="sm"
                onClick={() => { void handleAdd() }}
                disabled={!targetId || createDep.isPending}
              >
                <Plus className="w-4 h-4" />
              </Button>
            </div>
            {createDep.isError && (
              <p className="text-xs text-red-400">
                {createDep.error.message || t('bsi.depsDialog.errorAdd')}
              </p>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>{t('bsi.depsDialog.close')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Override Dialog ────────────────────────────────────────────────────────────

interface OverrideDialogProps {
  object: BSITargetObject
  onClose: () => void
}

function OverrideDialog({ object, onClose }: OverrideDialogProps) {
  const { t } = useTranslation()
  const update = useUpdateBSIObjectProtectionOverride(object.id)
  const [form, setForm] = useState<UpdateBSIObjectProtectionOverrideInput>({
    override_c: object.override_c ?? null,
    override_i: object.override_i ?? null,
    override_a: object.override_a ?? null,
    override_reason: object.override_reason ?? '',
    override_effect: object.override_effect ?? null,
  })

  function setF<K extends keyof UpdateBSIObjectProtectionOverrideInput>(
    k: K,
    v: UpdateBSIObjectProtectionOverrideInput[K],
  ) {
    setForm((f) => ({ ...f, [k]: v }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    await update.mutateAsync(form)
    onClose()
  }

  const schutzOptionen = [
    { value: '', label: t('bsi.overrideDialog.noOverride') },
    { value: 'normal', label: t('bsi.schutzbedarf.normal') },
    { value: 'hoch', label: t('bsi.schutzbedarf.hoch') },
    { value: 'sehr_hoch', label: t('bsi.schutzbedarf.sehr_hoch') },
  ]

  const ciaLabels: Record<'c' | 'i' | 'a', string> = {
    c: t('bsi.cia.c'),
    i: t('bsi.cia.i'),
    a: t('bsi.cia.a'),
  }

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('bsi.overrideDialog.title', { name: object.name })}</DialogTitle>
        </DialogHeader>
        <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
          <p className="text-xs text-secondary">
            {t('bsi.overrideDialog.hint')}
          </p>

          <div className="grid grid-cols-3 gap-2">
            {(['c', 'i', 'a'] as const).map((dim) => {
              const key = `override_${dim}` as const
              return (
                <div key={dim} className="space-y-1">
                  <Label className="text-[11px]">{ciaLabels[dim]}</Label>
                  <Select
                    value={form[key] ?? ''}
                    onValueChange={(v) => { setF(key, v === '' ? null : v as BSISchutzbedarf) }}
                  >
                    <SelectTrigger className="h-8 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {schutzOptionen.map((o) => (
                        <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )
            })}
          </div>

          <div className="space-y-1.5">
            <Label>{t('bsi.overrideDialog.effectLabel')}</Label>
            <Select
              value={form.override_effect ?? ''}
              onValueChange={(v) => { setF('override_effect', v === '' ? null : v as BSIOverrideEffect) }}
            >
              <SelectTrigger>
                <SelectValue placeholder={t('bsi.overrideDialog.effectPlaceholder')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">—</SelectItem>
                <SelectItem value="kumulation">{t('bsi.overrideDialog.kumulation')}</SelectItem>
                <SelectItem value="verteilung">{t('bsi.overrideDialog.verteilung')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="override-reason">
              {t('bsi.overrideDialog.reasonLabel')}
              {(form.override_c || form.override_i || form.override_a) && (
                <span className="text-red-400 ml-1">*</span>
              )}
            </Label>
            <Input
              id="override-reason"
              value={form.override_reason}
              onChange={(e) => { setF('override_reason', e.target.value) }}
              placeholder={t('bsi.overrideDialog.reasonPlaceholder')}
            />
            <p className="text-[11px] text-secondary">
              {t('bsi.overrideDialog.reasonHint')}
            </p>
          </div>

          {update.isError && (
            <p className="text-xs text-red-400">{update.error.message || 'Fehler'}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>{t('bsi.overrideDialog.cancel')}</Button>
            <Button type="submit" disabled={update.isPending}>{t('bsi.overrideDialog.save')}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── Risk Dialog ────────────────────────────────────────────────────────────────

const RISIKO_COLORS: Record<BSIRisikokategorie, string> = {
  gering: 'bg-green-900/30 text-green-300 border-green-800',
  mittel: 'bg-yellow-900/30 text-yellow-300 border-yellow-800',
  hoch: 'bg-orange-900/30 text-orange-300 border-orange-800',
  sehr_hoch: 'bg-red-900/30 text-red-300 border-red-800',
}

interface EditRiskFormProps {
  risk: BSIRiskAssessment
  targetObjectId: string
  onDone: () => void
}

function EditRiskForm({ risk, targetObjectId, onDone }: EditRiskFormProps) {
  const { t } = useTranslation()
  const update = useUpdateBSIRisk(targetObjectId, risk.id)
  const [form, setForm] = useState<UpdateBSIRiskInput>({
    eintrittshaeufigkeit: risk.eintrittshaeufigkeit,
    schadensauswirkung: risk.schadensauswirkung,
    behandlungsoption: risk.behandlungsoption ?? undefined,
    massnahme: risk.massnahme,
    verantwortlicher: risk.verantwortlicher,
    zieldatum: risk.zieldatum ?? '',
    restrisiko: risk.restrisiko ?? undefined,
  })

  const haeufigkeitOptions: { value: BSIEintrittshaeufigkeit; label: string }[] = [
    { value: 'selten', label: t('bsi.eintrittshaeufigkeit.selten') },
    { value: 'mittel', label: t('bsi.eintrittshaeufigkeit.mittel') },
    { value: 'haeufig', label: t('bsi.eintrittshaeufigkeit.haeufig') },
    { value: 'sehr_haeufig', label: t('bsi.eintrittshaeufigkeit.sehr_haeufig') },
  ]

  const auswirkungOptions: { value: BSISchadensauswirkung; label: string }[] = [
    { value: 'vernachlaessigbar', label: t('bsi.schadensauswirkung.vernachlaessigbar') },
    { value: 'begrenzt', label: t('bsi.schadensauswirkung.begrenzt') },
    { value: 'betraechtlich', label: t('bsi.schadensauswirkung.betraechtlich') },
    { value: 'existenzbedrohend', label: t('bsi.schadensauswirkung.existenzbedrohend') },
  ]

  const behandlungOptions: { value: BSIBehandlungsoption; label: string }[] = [
    { value: 'reduzieren', label: t('bsi.behandlungsoption.reduzieren') },
    { value: 'akzeptieren', label: t('bsi.behandlungsoption.akzeptieren') },
    { value: 'vermeiden', label: t('bsi.behandlungsoption.vermeiden') },
    { value: 'transferieren', label: t('bsi.behandlungsoption.transferieren') },
  ]

  const risikoOptions: { value: BSIRisikokategorie; label: string }[] = [
    { value: 'gering', label: t('bsi.risikokategorie.gering') },
    { value: 'mittel', label: t('bsi.risikokategorie.mittel') },
    { value: 'hoch', label: t('bsi.risikokategorie.hoch') },
    { value: 'sehr_hoch', label: t('bsi.risikokategorie.sehr_hoch') },
  ]

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    await update.mutateAsync(form)
    onDone()
  }

  return (
    <form onSubmit={(e) => void handleSave(e)} className="mt-2 border border-border rounded-md p-3 space-y-3 bg-surface2">
      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1">
          <Label className="text-[11px]">{t('bsi.riskDialog.haeufigkeitLabel')}</Label>
          <Select
            value={form.eintrittshaeufigkeit}
            onValueChange={(v) => { setForm((f) => ({ ...f, eintrittshaeufigkeit: v as BSIEintrittshaeufigkeit })) }}
          >
            <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              {haeufigkeitOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1">
          <Label className="text-[11px]">{t('bsi.riskDialog.auswirkungLabel')}</Label>
          <Select
            value={form.schadensauswirkung}
            onValueChange={(v) => { setForm((f) => ({ ...f, schadensauswirkung: v as BSISchadensauswirkung })) }}
          >
            <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              {auswirkungOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1">
          <Label className="text-[11px]">{t('bsi.riskDialog.behandlungLabel')}</Label>
          <Select
            value={form.behandlungsoption ?? ''}
            onValueChange={(v) => { setForm((f) => ({ ...f, behandlungsoption: v === '' ? undefined : v as BSIBehandlungsoption })) }}
          >
            <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="">—</SelectItem>
              {behandlungOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1">
          <Label className="text-[11px]">{t('bsi.riskDialog.restrisikoLabel')}</Label>
          <Select
            value={form.restrisiko ?? ''}
            onValueChange={(v) => { setForm((f) => ({ ...f, restrisiko: v === '' ? undefined : v as BSIRisikokategorie })) }}
          >
            <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="">—</SelectItem>
              {risikoOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="space-y-1">
        <Label className="text-[11px]">{t('bsi.riskDialog.massnahmeLabel')}</Label>
        <Textarea
          value={form.massnahme ?? ''}
          onChange={(e) => { setForm((f) => ({ ...f, massnahme: e.target.value })) }}
          rows={2}
          className="text-xs"
          placeholder={t('bsi.riskDialog.massnahmePlaceholder')}
        />
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1">
          <Label className="text-[11px]">{t('bsi.riskDialog.verantwortlicherLabel')}</Label>
          <Input
            value={form.verantwortlicher ?? ''}
            onChange={(e) => { setForm((f) => ({ ...f, verantwortlicher: e.target.value })) }}
            className="h-8 text-xs"
          />
        </div>
        <div className="space-y-1">
          <Label className="text-[11px]">{t('bsi.riskDialog.zieldatumLabel')}</Label>
          <Input
            type="date"
            value={form.zieldatum ?? ''}
            onChange={(e) => { setForm((f) => ({ ...f, zieldatum: e.target.value })) }}
            className="h-8 text-xs"
          />
        </div>
      </div>
      {update.isError && (
        <p className="text-xs text-red-400">{update.error.message}</p>
      )}
      <div className="flex gap-2 justify-end">
        <Button type="button" size="sm" variant="ghost" onClick={onDone}>
          {t('bsi.riskDialog.cancel')}
        </Button>
        <Button type="submit" size="sm" disabled={update.isPending}>
          {t('bsi.riskDialog.save')}
        </Button>
      </div>
    </form>
  )
}

interface RiskDialogProps {
  object: BSITargetObject
  onClose: () => void
}

function RiskDialog({ object, onClose }: RiskDialogProps) {
  const { t } = useTranslation()
  const { data: risks = [], isLoading } = useBSIRisks(object.id)
  const { data: threats = [] } = useBSIThreats()
  const createRisk = useCreateBSIRisk(object.id)
  const deleteRisk = useDeleteBSIRisk(object.id)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [threatId, setThreatId] = useState('')
  const [haeufigkeit, setHaeufigkeit] = useState<BSIEintrittshaeufigkeit>('selten')
  const [auswirkung, setAuswirkung] = useState<BSISchadensauswirkung>('begrenzt')

  const haeufigkeitOptions: { value: BSIEintrittshaeufigkeit; label: string }[] = [
    { value: 'selten', label: t('bsi.eintrittshaeufigkeit.selten') },
    { value: 'mittel', label: t('bsi.eintrittshaeufigkeit.mittel') },
    { value: 'haeufig', label: t('bsi.eintrittshaeufigkeit.haeufig') },
    { value: 'sehr_haeufig', label: t('bsi.eintrittshaeufigkeit.sehr_haeufig') },
  ]

  const auswirkungOptions: { value: BSISchadensauswirkung; label: string }[] = [
    { value: 'vernachlaessigbar', label: t('bsi.schadensauswirkung.vernachlaessigbar') },
    { value: 'begrenzt', label: t('bsi.schadensauswirkung.begrenzt') },
    { value: 'betraechtlich', label: t('bsi.schadensauswirkung.betraechtlich') },
    { value: 'existenzbedrohend', label: t('bsi.schadensauswirkung.existenzbedrohend') },
  ]

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    if (!threatId) return
    await createRisk.mutateAsync({ threat_id: threatId, eintrittshaeufigkeit: haeufigkeit, schadensauswirkung: auswirkung })
    setThreatId('')
    setHaeufigkeit('selten')
    setAuswirkung('begrenzt')
  }

  const risikoLabel = (k: BSIRisikokategorie) => t(`bsi.risikokategorie.${k}`)
  const behandlungLabel = (k?: BSIBehandlungsoption) => k ? t(`bsi.behandlungsoption.${k}`) : '—'

  const threatTitle = (r: BSIRiskAssessment) =>
    r.threat_title ?? threats.find((th) => th.id === r.threat_id)?.title ?? r.threat_id

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('bsi.riskDialog.title', { name: object.name })}</DialogTitle>
        </DialogHeader>

        <div className="space-y-3">
          {isLoading && <p className="text-sm text-secondary">{t('bsi.riskDialog.loading')}</p>}

          {!isLoading && risks.length === 0 && (
            <p className="text-sm text-secondary italic">{t('bsi.riskDialog.empty')}</p>
          )}

          {risks.map((r) => (
            <div key={r.id} className="rounded-md border border-border px-3 py-2">
              <div className="flex items-start gap-2">
                <Badge className={`text-[10px] border shrink-0 mt-0.5 ${RISIKO_COLORS[r.risikokategorie]}`}>
                  {risikoLabel(r.risikokategorie)}
                </Badge>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-primary truncate">{threatTitle(r)}</p>
                  <p className="text-[11px] text-secondary">
                    {behandlungLabel(r.behandlungsoption)}
                    {r.massnahme ? ` · ${r.massnahme.slice(0, 60)}${r.massnahme.length > 60 ? '…' : ''}` : ''}
                  </p>
                </div>
                <div className="flex gap-1 shrink-0">
                  <Button
                    size="icon"
                    variant="ghost"
                    className="w-6 h-6 text-secondary hover:text-primary"
                    onClick={() => { setEditingId(editingId === r.id ? null : r.id) }}
                    title={t('bsi.riskDialog.editTitle')}
                  >
                    <Pencil className="w-3 h-3" />
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="w-6 h-6 text-red-400 hover:text-red-300"
                    onClick={() => { deleteRisk.mutate(r.id) }}
                  >
                    <Trash2 className="w-3 h-3" />
                  </Button>
                </div>
              </div>
              {editingId === r.id && (
                <EditRiskForm
                  risk={r}
                  targetObjectId={object.id}
                  onDone={() => { setEditingId(null) }}
                />
              )}
            </div>
          ))}

          <div className="border-t border-border pt-3 space-y-2">
            <p className="text-xs font-semibold text-primary">{t('bsi.riskDialog.addTitle')}</p>
            <form onSubmit={(e) => void handleAdd(e)} className="space-y-2">
              <div className="space-y-1">
                <Label className="text-[11px]">{t('bsi.riskDialog.threatLabel')}</Label>
                <Select value={threatId} onValueChange={setThreatId}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('bsi.riskDialog.threatPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    {threats.map((th) => (
                      <SelectItem key={th.id} value={th.id}>
                        <span className="font-mono text-[11px] text-secondary mr-2">{th.threat_id}</span>
                        {th.title}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div className="space-y-1">
                  <Label className="text-[11px]">{t('bsi.riskDialog.haeufigkeitLabel')}</Label>
                  <Select value={haeufigkeit} onValueChange={(v) => { setHaeufigkeit(v as BSIEintrittshaeufigkeit) }}>
                    <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {haeufigkeitOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1">
                  <Label className="text-[11px]">{t('bsi.riskDialog.auswirkungLabel')}</Label>
                  <Select value={auswirkung} onValueChange={(v) => { setAuswirkung(v as BSISchadensauswirkung) }}>
                    <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {auswirkungOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              {createRisk.isError && (
                <p className="text-xs text-red-400">{createRisk.error.message}</p>
              )}
              <Button type="submit" size="sm" disabled={!threatId || createRisk.isPending}>
                <Plus className="w-3.5 h-3.5 mr-1" />
                {t('bsi.riskDialog.add')}
              </Button>
            </form>
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>{t('bsi.riskDialog.close')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Page ───────────────────────────────────────────────────────────────────────

export default function BSITargetObjectsPage() {
  const { t } = useTranslation()
  const { data: objects = [], isLoading, isError, error } = useBSITargetObjects()
  const deleteMutation = useDeleteBSITargetObject()
  const [showCreate, setShowCreate] = useState(false)
  const [depsObject, setDepsObject] = useState<BSITargetObject | null>(null)
  const [overrideObject, setOverrideObject] = useState<BSITargetObject | null>(null)
  const [riskObject, setRiskObject] = useState<BSITargetObject | null>(null)
  const [deleteId, setDeleteId] = useState<{ id: string; name: string } | null>(null)

  const typeLabels: Record<BSITargetObjectType, string> = {
    it_system: t('bsi.type.it_system'),
    application: t('bsi.type.application'),
    network: t('bsi.type.network'),
    room: t('bsi.type.room'),
    process: t('bsi.type.process'),
  }

  const niveauLabels: Record<BSIAbsicherungsniveau, string> = {
    basis: t('bsi.niveau.basis'),
    standard: t('bsi.niveau.standard'),
    kern: t('bsi.niveau.kern'),
  }

  function handleDelete(id: string, name: string) {
    setDeleteId({ id, name })
  }

  function confirmDelete() {
    if (deleteId) deleteMutation.mutate(deleteId.id)
    setDeleteId(null)
  }

  const nameById = (id: string) => objects.find((o) => o.id === id)?.name

  return (
    <ProGate error={isError ? error : null}>
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('bsi.targetObjects.title')}
        description={t('bsi.targetObjects.description')}
        actions={
          <Button size="sm" onClick={() => { setShowCreate(true) }}>
            <Plus className="w-4 h-4 mr-1" />
            {t('bsi.targetObjects.addButton')}
          </Button>
        }
      />

      <div className="p-6 space-y-3">
        {isLoading && (
          <p className="text-sm text-secondary">{t('bsi.targetObjects.loading')}</p>
        )}

        {!isLoading && objects.length === 0 && (
          <div className="rounded-lg border border-dashed border-border p-8 text-center space-y-2">
            <p className="text-sm font-medium text-primary">{t('bsi.targetObjects.emptyTitle')}</p>
            <p className="text-xs text-secondary">
              {t('bsi.targetObjects.emptyDescription')}
            </p>
            <Button size="sm" className="mt-2" onClick={() => { setShowCreate(true) }}>
              <Plus className="w-4 h-4 mr-1" />
              {t('bsi.targetObjects.emptyAddButton')}
            </Button>
          </div>
        )}

        {objects.map((obj) => {
          const Icon = TYPE_ICONS[obj.type]
          const hasInheritance =
            obj.inherited_from_c || obj.inherited_from_i || obj.inherited_from_a
          const hasOverride =
            obj.override_c || obj.override_i || obj.override_a

          return (
            <div
              key={obj.id}
              className="rounded-lg border border-border bg-surface flex items-center gap-3 px-4 py-3"
            >
              <Icon className="w-5 h-5 text-secondary shrink-0" />

              <div className="min-w-0 flex-1">
                <p className="text-sm font-semibold text-primary truncate">{obj.name}</p>
                {obj.description && (
                  <p className="text-xs text-secondary truncate">{obj.description}</p>
                )}
                <div className="flex items-center gap-2 mt-0.5">
                  <CIABadge
                    label={t('bsi.cia.cShort')}
                    value={obj.effective_c}
                    inheritedFrom={obj.inherited_from_c}
                    objectName={nameById(obj.inherited_from_c ?? '')}
                  />
                  <CIABadge
                    label={t('bsi.cia.iShort')}
                    value={obj.effective_i}
                    inheritedFrom={obj.inherited_from_i}
                    objectName={nameById(obj.inherited_from_i ?? '')}
                  />
                  <CIABadge
                    label={t('bsi.cia.aShort')}
                    value={obj.effective_a}
                    inheritedFrom={obj.inherited_from_a}
                    objectName={nameById(obj.inherited_from_a ?? '')}
                  />
                  {hasInheritance && (
                    <span className="text-[10px] text-secondary italic">{t('bsi.targetObjects.inherited')}</span>
                  )}
                  {hasOverride && (
                    <span className="text-[10px] text-orange-400 italic">{t('bsi.targetObjects.override')}</span>
                  )}
                </div>
              </div>

              <div className="flex items-center gap-2 shrink-0">
                <Badge className="text-[11px] border-transparent bg-surface2 text-secondary">
                  {typeLabels[obj.type]}
                </Badge>
                <Badge className={`text-[11px] border ${NIVEAU_COLORS[obj.absicherungsniveau]}`}>
                  {niveauLabels[obj.absicherungsniveau]}
                </Badge>
              </div>

              <div className="flex items-center gap-1 shrink-0">
                <Button
                  size="icon"
                  variant="ghost"
                  className="w-7 h-7 text-secondary hover:text-primary"
                  title={t('bsi.targetObjects.depsButton')}
                  onClick={() => { setDepsObject(obj) }}
                >
                  <GitBranch className="w-3.5 h-3.5" />
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="w-7 h-7 text-secondary hover:text-primary"
                  title={t('bsi.targetObjects.overrideButton')}
                  onClick={() => { setOverrideObject(obj) }}
                >
                  <ArrowRightLeft className="w-3.5 h-3.5" />
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="w-7 h-7 text-secondary hover:text-primary"
                  title={t('bsi.riskDialog.risikoButton')}
                  onClick={() => { setRiskObject(obj) }}
                >
                  <ShieldAlert className="w-3.5 h-3.5" />
                </Button>
                <Link
                  to={`/vaktcomply/bsi/check/${obj.id}`}
                  className="inline-flex items-center gap-1 text-[12px] px-2.5 py-1 rounded border border-blue-700 text-blue-300 hover:bg-blue-900/20 transition-colors"
                >
                  {t('bsi.targetObjects.checkButton')} <ChevronRight className="w-3 h-3" />
                </Link>
                <Button
                  size="icon"
                  variant="ghost"
                  className="w-7 h-7 text-red-400 hover:text-red-300"
                  onClick={() => { handleDelete(obj.id, obj.name) }}
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </Button>
              </div>
            </div>
          )
        })}
      </div>

      <CreateDialog open={showCreate} onClose={() => { setShowCreate(false) }} />

      {depsObject && (
        <DepsDialog
          object={depsObject}
          allObjects={objects}
          onClose={() => { setDepsObject(null) }}
        />
      )}

      {overrideObject && (
        <OverrideDialog
          object={overrideObject}
          onClose={() => { setOverrideObject(null) }}
        />
      )}

      {riskObject && (
        <RiskDialog
          object={riskObject}
          onClose={() => { setRiskObject(null) }}
        />
      )}

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('common.confirmDeleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('bsi.targetObjects.deleteConfirm', { name: deleteId?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
    </ProGate>
  )
}
