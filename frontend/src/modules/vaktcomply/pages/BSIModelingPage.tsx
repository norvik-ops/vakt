// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useState } from 'react'
import { Network, Plus, Trash2 } from 'lucide-react'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../../../components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../../../components/ui/tabs'
import { Progress } from '../../../components/ui/progress'
import {
  useBSIModelingMatrix,
  useBSIModelingStats,
  useCreateBSIModeling,
  useDeleteBSIModeling,
} from '../hooks/useBSIModeling'
import type { BSIModelingEntry, CreateBSIModelingInput } from '../types'

// ─── Constants ────────────────────────────────────────────────────────────────

const PRIORITY_CLASS: Record<string, string> = {
  R1: 'bg-red-500/20 text-red-400 border-red-500/30',
  R2: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  R3: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
}

const PRIORITY_LABELS: Record<string, string> = {
  R1: 'R1 — Muss',
  R2: 'R2 — Soll',
  R3: 'R3 — Kann',
}

type CheckStatus = 'yes' | 'partial' | 'no' | 'not_applicable'

const STATUS_CLASS: Record<CheckStatus | 'pending', string> = {
  yes: 'bg-green-500/20 text-green-400 border-green-500/30',
  partial: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  no: 'bg-red-500/20 text-red-400 border-red-500/30',
  not_applicable: 'bg-secondary text-secondary-foreground',
  pending: 'bg-slate-500/20 text-slate-400 border-slate-500/30',
}

const STATUS_LABELS: Record<CheckStatus | 'pending', string> = {
  yes: 'Erfüllt',
  partial: 'Teilweise',
  no: 'Offen',
  not_applicable: 'N/A',
  pending: 'Ausstehend',
}

function checkStatusKey(entry: BSIModelingEntry): CheckStatus | 'pending' {
  return (entry.check_status) ?? 'pending'
}

// ─── Empty form ───────────────────────────────────────────────────────────────

function emptyForm(): CreateBSIModelingInput {
  return {
    asset_id: '',
    control_id: '',
    priority: 'R1',
    justification_for_exclusion: '',
    check_status: undefined,
    interview_notes: '',
    site_visit_notes: '',
  }
}

// ─── Matrix Tab ───────────────────────────────────────────────────────────────

function MatrixTab({
  entries,
  onDelete,
  onAdd,
}: {
  entries: BSIModelingEntry[]
  onDelete: (id: string) => void
  onAdd: () => void
}) {
  if (entries.length === 0) {
    return (
      <div className="space-y-4">
        <div className="flex justify-end">
          <Button size="sm" onClick={onAdd}>
            <Plus className="w-4 h-4 mr-1" />
            Baustein zuweisen
          </Button>
        </div>
        <EmptyState
          icon={Network}
          title="Keine Modellierungseinträge"
          description="Weisen Sie Bausteine Ihren IT-Assets zu, um die BSI-Grundschutz-Modellierung zu starten."
        />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button size="sm" onClick={onAdd}>
          <Plus className="w-4 h-4 mr-1" />
          Baustein zuweisen
        </Button>
      </div>
      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Asset</TableHead>
              <TableHead>Baustein / Control</TableHead>
              <TableHead>Priorität</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Interview-Notizen</TableHead>
              <TableHead className="w-12" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((e) => (
              <TableRow key={e.id}>
                <TableCell className="font-medium text-sm">
                  {e.asset_name || <span className="text-muted-foreground italic">Unbekannt</span>}
                </TableCell>
                <TableCell>
                  <div>
                    <p className="text-sm text-primary">
                      {e.control_title || <span className="text-muted-foreground italic">—</span>}
                    </p>
                    {e.framework_id && (
                      <p className="text-[11px] text-secondary">{e.framework_id}</p>
                    )}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge className={`text-xs border ${PRIORITY_CLASS[e.priority] ?? ''}`}>
                    {e.priority}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge className={`text-xs border ${STATUS_CLASS[checkStatusKey(e)]}`}>
                    {STATUS_LABELS[checkStatusKey(e)]}
                  </Badge>
                </TableCell>
                <TableCell className="max-w-[200px]">
                  <p className="text-xs text-secondary truncate">
                    {e.interview_notes || '—'}
                  </p>
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-destructive hover:text-destructive"
                    onClick={() => { onDelete(e.id) }}
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

// ─── By-Asset Tab ─────────────────────────────────────────────────────────────

function ByAssetTab({ entries }: { entries: BSIModelingEntry[] }) {
  const [selectedAsset, setSelectedAsset] = useState<string>('')

  // Collect unique assets
  const assets = Array.from(
    new Map(entries.map((e) => [e.asset_id, e.asset_name])).entries(),
  ).sort((a, b) => a[1].localeCompare(b[1]))

  const filtered = selectedAsset
    ? entries.filter((e) => e.asset_id === selectedAsset)
    : []

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Label className="shrink-0 text-sm">Asset auswählen:</Label>
        <Select value={selectedAsset} onValueChange={setSelectedAsset}>
          <SelectTrigger className="w-64">
            <SelectValue placeholder="Asset wählen …" />
          </SelectTrigger>
          <SelectContent>
            {assets.map(([id, name]) => (
              <SelectItem key={id} value={id}>{name || id}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {selectedAsset && filtered.length === 0 && (
        <p className="text-sm text-secondary">Keine Bausteine für dieses Asset zugewiesen.</p>
      )}

      {filtered.length > 0 && (
        <div className="rounded-lg border border-border overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Baustein / Control</TableHead>
                <TableHead>Priorität</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Begründung</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((e) => (
                <TableRow key={e.id}>
                  <TableCell>
                    <p className="text-sm">{e.control_title || '—'}</p>
                    <p className="text-[11px] text-secondary">{e.framework_id}</p>
                  </TableCell>
                  <TableCell>
                    <Badge className={`text-xs border ${PRIORITY_CLASS[e.priority] ?? ''}`}>
                      {PRIORITY_LABELS[e.priority]}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge className={`text-xs border ${STATUS_CLASS[checkStatusKey(e)]}`}>
                      {STATUS_LABELS[checkStatusKey(e)]}
                    </Badge>
                  </TableCell>
                  <TableCell className="max-w-[300px]">
                    <p className="text-xs text-secondary truncate">
                      {e.justification_for_exclusion || '—'}
                    </p>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

// ─── Stats Tab ────────────────────────────────────────────────────────────────

function StatsTab() {
  const { data: stats, isLoading } = useBSIModelingStats()

  if (isLoading) return <Spinner />
  if (!stats) return <p className="text-sm text-secondary">Keine Statistiken verfügbar.</p>

  const pct = (n: number) => stats.total > 0 ? Math.round((n / stats.total) * 100) : 0

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs text-secondary">Gesamt</CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <p className="text-2xl font-bold">{stats.total}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs text-green-400">Erfüllt</CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <p className="text-2xl font-bold text-green-400">{stats.count_yes}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs text-amber-400">Teilweise</CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <p className="text-2xl font-bold text-amber-400">{stats.count_partial}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs text-red-400">Offen</CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <p className="text-2xl font-bold text-red-400">{stats.count_no}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs text-secondary">N/A</CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <p className="text-2xl font-bold">{stats.count_na}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs text-slate-400">Ausstehend</CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <p className="text-2xl font-bold text-slate-400">{stats.count_pending}</p>
          </CardContent>
        </Card>
      </div>

      {stats.total > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">Fortschritt</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div>
              <div className="flex justify-between text-xs text-secondary mb-1">
                <span>Erfüllt</span>
                <span>{pct(stats.count_yes)}%</span>
              </div>
              <Progress value={pct(stats.count_yes)} className="h-2" />
            </div>
            <div>
              <div className="flex justify-between text-xs text-secondary mb-1">
                <span>Teilweise</span>
                <span>{pct(stats.count_partial)}%</span>
              </div>
              <Progress value={pct(stats.count_partial)} className="h-2" />
            </div>
            <div>
              <div className="flex justify-between text-xs text-secondary mb-1">
                <span>Offen</span>
                <span>{pct(stats.count_no)}%</span>
              </div>
              <Progress value={pct(stats.count_no)} className="h-2" />
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ─── Create Dialog ────────────────────────────────────────────────────────────

function CreateDialog({
  open,
  onClose,
}: {
  open: boolean
  onClose: () => void
}) {
  const [form, setForm] = useState<CreateBSIModelingInput>(emptyForm())
  const create = useCreateBSIModeling()

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    create.mutate(form, { onSuccess: () => { setForm(emptyForm()); onClose() } })
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Baustein zuweisen</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4 mt-2">
          <div className="space-y-1.5">
            <Label htmlFor="bm-asset-id">Asset-ID <span className="text-destructive">*</span></Label>
            <Input
              id="bm-asset-id"
              value={form.asset_id}
              onChange={(e) => { setForm((f) => ({ ...f, asset_id: e.target.value })) }}
              placeholder="UUID des Assets"
              required
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="bm-control-id">Control-ID <span className="text-destructive">*</span></Label>
            <Input
              id="bm-control-id"
              value={form.control_id}
              onChange={(e) => { setForm((f) => ({ ...f, control_id: e.target.value })) }}
              placeholder="UUID des Controls"
              required
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Priorität</Label>
              <Select
                value={form.priority}
                onValueChange={(v) => { setForm((f) => ({ ...f, priority: v as 'R1' | 'R2' | 'R3' })) }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="R1">R1 — Muss</SelectItem>
                  <SelectItem value="R2">R2 — Soll</SelectItem>
                  <SelectItem value="R3">R3 — Kann</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Check-Status</Label>
              <Select
                value={form.check_status ?? ''}
                onValueChange={(v) => {
                  setForm((f) => ({
                    ...f,
                    check_status: v === '' ? undefined : v as CreateBSIModelingInput['check_status'],
                  }))
                }}
              >
                <SelectTrigger><SelectValue placeholder="Ausstehend" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="">Ausstehend</SelectItem>
                  <SelectItem value="yes">Erfüllt</SelectItem>
                  <SelectItem value="partial">Teilweise</SelectItem>
                  <SelectItem value="no">Offen</SelectItem>
                  <SelectItem value="not_applicable">N/A</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          {create.isError && (
            <p className="text-xs text-destructive">{create.error.message}</p>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Abbrechen</Button>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? 'Speichern …' : 'Zuweisen'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function BSIModelingPage() {
  const [showCreate, setShowCreate] = useState(false)
  const { data: entries = [], isLoading } = useBSIModelingMatrix()
  const deleteEntry = useDeleteBSIModeling()

  function handleDelete(id: string) {
    if (window.confirm('Eintrag wirklich löschen?')) {
      deleteEntry.mutate(id)
    }
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="BSI-Grundschutz-Modellierung"
        description="Baustein-Zuweisung und IT-Grundschutz-Check pro Asset (Phase 3 & 4)"
      />

      <div className="p-6 space-y-6">
        {isLoading ? (
          <Spinner />
        ) : (
          <Tabs defaultValue="matrix">
            <TabsList>
              <TabsTrigger value="matrix">Matrix</TabsTrigger>
              <TabsTrigger value="by-asset">Nach Asset</TabsTrigger>
              <TabsTrigger value="stats">Statistiken</TabsTrigger>
            </TabsList>

            <TabsContent value="matrix" className="mt-4">
              <MatrixTab
                entries={entries}
                onDelete={handleDelete}
                onAdd={() => { setShowCreate(true) }}
              />
            </TabsContent>

            <TabsContent value="by-asset" className="mt-4">
              <ByAssetTab entries={entries} />
            </TabsContent>

            <TabsContent value="stats" className="mt-4">
              <StatsTab />
            </TabsContent>
          </Tabs>
        )}
      </div>

      <CreateDialog open={showCreate} onClose={() => { setShowCreate(false) }} />
    </div>
  )
}
