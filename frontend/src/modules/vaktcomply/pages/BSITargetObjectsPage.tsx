// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Plus, Trash2, ChevronRight, Server, AppWindow, Network, Building, Workflow } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
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
  useBSITargetObjects,
  useCreateBSITargetObject,
  useDeleteBSITargetObject,
} from '../hooks/useBSICheck'
import type {
  BSITargetObjectType,
  BSIAbsicherungsniveau,
  BSISchutzbedarf,
  CreateBSITargetObjectInput,
} from '../types'

// ── helpers ────────────────────────────────────────────────────────────────────

const TYPE_LABELS: Record<BSITargetObjectType, string> = {
  it_system: 'IT-System',
  application: 'Anwendung',
  network: 'Netz',
  room: 'Raum',
  process: 'Prozess',
}

const TYPE_ICONS: Record<BSITargetObjectType, React.ElementType> = {
  it_system: Server,
  application: AppWindow,
  network: Network,
  room: Building,
  process: Workflow,
}

const NIVEAU_LABELS: Record<BSIAbsicherungsniveau, string> = {
  basis: 'Basis',
  standard: 'Standard',
  kern: 'Kern-Absicherung',
}

const NIVEAU_COLORS: Record<BSIAbsicherungsniveau, string> = {
  basis: 'bg-blue-900/30 text-blue-300 border-blue-800',
  standard: 'bg-green-900/30 text-green-300 border-green-800',
  kern: 'bg-purple-900/30 text-purple-300 border-purple-800',
}

const SCHUTZBEDARF_LABELS: Record<BSISchutzbedarf, string> = {
  normal: 'normal',
  hoch: 'hoch',
  sehr_hoch: 'sehr hoch',
}

// ── Create Dialog ──────────────────────────────────────────────────────────────

interface CreateDialogProps {
  open: boolean
  onClose: () => void
}

function CreateDialog({ open, onClose }: CreateDialogProps) {
  const create = useCreateBSITargetObject()
  const [form, setForm] = useState<CreateBSITargetObjectInput>({
    name: '',
    type: 'it_system',
    absicherungsniveau: 'standard',
  })

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
          <DialogTitle>Zielobjekt anlegen</DialogTitle>
        </DialogHeader>
        <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="to-name">Name *</Label>
            <Input
              id="to-name"
              value={form.name}
              onChange={(e) => { set('name', e.target.value) }}
              placeholder="z.B. Webserver Produktion"
              required
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Typ *</Label>
              <Select value={form.type} onValueChange={(v) => { set('type', v as BSITargetObjectType) }}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(TYPE_LABELS) as BSITargetObjectType[]).map((t) => (
                    <SelectItem key={t} value={t}>{TYPE_LABELS[t]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Absicherungsniveau</Label>
              <Select
                value={form.absicherungsniveau ?? 'standard'}
                onValueChange={(v) => { set('absicherungsniveau', v as BSIAbsicherungsniveau) }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="basis">Basis</SelectItem>
                  <SelectItem value="standard">Standard</SelectItem>
                  <SelectItem value="kern">Kern-Absicherung</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>Beschreibung</Label>
            <Input
              value={form.description ?? ''}
              onChange={(e) => { set('description', e.target.value) }}
              placeholder="Optional"
            />
          </div>

          <p className="text-xs text-secondary">
            Schutzbedarf (V/I/A) kann nach dem Anlegen bearbeitet werden.
          </p>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Abbrechen
            </Button>
            <Button type="submit" disabled={create.isPending}>
              Anlegen
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── Page ───────────────────────────────────────────────────────────────────────

export default function BSITargetObjectsPage() {
  const { data: objects = [], isLoading } = useBSITargetObjects()
  const deleteMutation = useDeleteBSITargetObject()
  const [showCreate, setShowCreate] = useState(false)

  function handleDelete(id: string, name: string) {
    if (!window.confirm(`Zielobjekt „${name}" wirklich löschen? Alle Check-Ergebnisse gehen verloren.`)) return
    deleteMutation.mutate(id)
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Strukturanalyse — Zielobjekte"
        description="IT-Systeme, Anwendungen und Räume für den IT-Grundschutz-Check erfassen"
        actions={
          <Button size="sm" onClick={() => { setShowCreate(true) }}>
            <Plus className="w-4 h-4 mr-1" />
            Zielobjekt anlegen
          </Button>
        }
      />

      <div className="p-6 space-y-3">
        {isLoading && (
          <p className="text-sm text-secondary">Lade Zielobjekte…</p>
        )}

        {!isLoading && objects.length === 0 && (
          <div className="rounded-lg border border-dashed border-border p-8 text-center space-y-2">
            <p className="text-sm font-medium text-primary">Noch keine Zielobjekte</p>
            <p className="text-xs text-secondary">
              Legen Sie Ihre IT-Systeme, Anwendungen und Räume an, um den IT-Grundschutz-Check zu starten.
            </p>
            <Button size="sm" className="mt-2" onClick={() => { setShowCreate(true) }}>
              <Plus className="w-4 h-4 mr-1" />
              Erstes Zielobjekt anlegen
            </Button>
          </div>
        )}

        {objects.map((obj) => {
          const Icon = TYPE_ICONS[obj.type]
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
              </div>

              <div className="flex items-center gap-2 shrink-0">
                <Badge className="text-[11px] border-transparent bg-surface2 text-secondary">
                  {TYPE_LABELS[obj.type]}
                </Badge>
                <Badge className={`text-[11px] border ${NIVEAU_COLORS[obj.absicherungsniveau]}`}>
                  {NIVEAU_LABELS[obj.absicherungsniveau]}
                </Badge>
                {obj.protection_c && (
                  <span className="text-[11px] text-secondary font-mono">
                    V:{SCHUTZBEDARF_LABELS[obj.protection_c][0].toUpperCase()}
                    {' '}I:{obj.protection_i ? SCHUTZBEDARF_LABELS[obj.protection_i][0].toUpperCase() : '–'}
                    {' '}A:{obj.protection_a ? SCHUTZBEDARF_LABELS[obj.protection_a][0].toUpperCase() : '–'}
                  </span>
                )}
              </div>

              <div className="flex items-center gap-1 shrink-0">
                <Link
                  to={`/vaktcomply/bsi/check/${obj.id}`}
                  className="inline-flex items-center gap-1 text-[12px] px-2.5 py-1 rounded border border-blue-700 text-blue-300 hover:bg-blue-900/20 transition-colors"
                >
                  Check <ChevronRight className="w-3 h-3" />
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
    </div>
  )
}
