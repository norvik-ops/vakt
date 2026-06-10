import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Users, Plus, Pencil, Trash2, Download, Wand2 } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '../../../components/ui/table'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../../../components/ui/alert-dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { apiFetch } from '../../../api/client'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { EmptyState } from '../../../shared/components/EmptyState'

interface InterestedParty {
  id: string
  name: string
  category: string
  description: string
  needs_and_expectations: string
  relevant_requirements: string
  monitoring_frequency: string
  owner: string
  created_at: string
  updated_at: string
}

interface IPInput {
  name: string
  category: string
  description: string
  needs_and_expectations: string
  relevant_requirements: string
  monitoring_frequency: string
  owner: string
}

const CATEGORY_LABELS: Record<string, string> = {
  internal: 'Intern',
  external: 'Extern',
  regulatory: 'Behörden/Regulierung',
  customer: 'Kunden',
  supplier: 'Lieferanten',
  other: 'Sonstige',
}

const FREQ_LABELS: Record<string, string> = {
  continuous: 'Laufend',
  monthly: 'Monatlich',
  quarterly: 'Quartalsweise',
  annually: 'Jährlich',
  as_needed: 'Bei Bedarf',
}

function useInterestedParties() {
  return useQuery<InterestedParty[]>({
    queryKey: ['vaktcomply', 'interested-parties'],
    queryFn: () => apiFetch<InterestedParty[]>('/vaktcomply/interested-parties'),
    staleTime: 2 * 60 * 1000,
  })
}

function useCreateIP() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: IPInput) =>
      apiFetch<InterestedParty>('/vaktcomply/interested-parties', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ['vaktcomply', 'interested-parties'] }); },
  })
}

function useUpdateIP() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: IPInput }) =>
      apiFetch<InterestedParty>(`/vaktcomply/interested-parties/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ['vaktcomply', 'interested-parties'] }); },
  })
}

function useDeleteIP() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/vaktcomply/interested-parties/${id}`, { method: 'DELETE' }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ['vaktcomply', 'interested-parties'] }); },
  })
}

function useSeedDefaults() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () =>
      apiFetch<void>('/vaktcomply/interested-parties/seed-defaults', { method: 'POST' }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ['vaktcomply', 'interested-parties'] }); },
  })
}

function emptyForm(): IPInput {
  return {
    name: '',
    category: 'external',
    description: '',
    needs_and_expectations: '',
    relevant_requirements: '',
    monitoring_frequency: 'annually',
    owner: '',
  }
}

export default function InterestedPartiesPage() {
  const { data = [], isLoading } = useInterestedParties()
  const createMut = useCreateIP()
  const updateMut = useUpdateIP()
  const deleteMut = useDeleteIP()
  const seedMut = useSeedDefaults()

  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editTarget, setEditTarget] = useState<InterestedParty | null>(null)
  const [form, setForm] = useState<IPInput>(emptyForm())
  const [deleteId, setDeleteId] = useState<string | null>(null)

  function openCreate() {
    setForm(emptyForm())
    setEditTarget(null)
    setDialogMode('create')
  }

  function openEdit(ip: InterestedParty) {
    setForm({
      name: ip.name,
      category: ip.category,
      description: ip.description,
      needs_and_expectations: ip.needs_and_expectations,
      relevant_requirements: ip.relevant_requirements,
      monitoring_frequency: ip.monitoring_frequency,
      owner: ip.owner,
    })
    setEditTarget(ip)
    setDialogMode('edit')
  }

  function handleSubmit() {
    if (dialogMode === 'create') {
      createMut.mutate(form, { onSuccess: () => { setDialogMode(null); } })
    } else if (dialogMode === 'edit' && editTarget) {
      updateMut.mutate({ id: editTarget.id, input: form }, { onSuccess: () => { setDialogMode(null); } })
    }
  }

  function handleExport() {
    const a = document.createElement('a')
    a.href = '/api/v1/vaktcomply/interested-parties/export'
    a.download = `interested-parties-${new Date().toISOString().slice(0, 10)}.pdf`
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  const isPending = createMut.isPending || updateMut.isPending

  if (isLoading) return <div className="p-8"><SkeletonTable rows={6} cols={5} /></div>

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">Interessierte Parteien</h1>
          <p className="text-gray-500 text-sm mt-1">ISO 27001 Klausel 4.2 — Anforderungen relevanter Stakeholder</p>
        </div>
        <div className="flex items-center gap-2">
          {data.length === 0 && (
            <Button variant="outline" size="sm" onClick={() => { seedMut.mutate(); }} disabled={seedMut.isPending}>
              <Wand2 className="h-4 w-4 mr-1.5" />
              DACH-Standards einfügen
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={handleExport}>
            <Download className="h-4 w-4 mr-1.5" />
            PDF exportieren
          </Button>
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4 mr-1.5" />
            Partei hinzufügen
          </Button>
        </div>
      </div>

      {data.length === 0 ? (
        <EmptyState
          icon={Users}
          title="Keine interessierten Parteien"
          description="Dokumentieren Sie relevante Stakeholder und ihre Anforderungen gemäß ISO 27001 Klausel 4.2."
          action={
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => { seedMut.mutate(); }} disabled={seedMut.isPending}>
                <Wand2 className="h-4 w-4 mr-1.5" />
                DACH-Standards einfügen
              </Button>
              <Button onClick={openCreate}><Plus className="h-4 w-4 mr-1.5" />Partei hinzufügen</Button>
            </div>
          }
        />
      ) : (
        <div className="bg-white rounded-lg border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Kategorie</TableHead>
                <TableHead>Bedürfnisse & Erwartungen</TableHead>
                <TableHead>Überwachung</TableHead>
                <TableHead className="w-20"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.map((ip) => (
                <TableRow key={ip.id} className="hover:bg-gray-50">
                  <TableCell>
                    <div className="font-medium text-sm">{ip.name}</div>
                    {ip.owner && <div className="text-xs text-gray-400">{ip.owner}</div>}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="text-xs">
                      {CATEGORY_LABELS[ip.category] ?? ip.category}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-gray-600 max-w-xs">
                    <p className="line-clamp-2">{ip.needs_and_expectations}</p>
                  </TableCell>
                  <TableCell className="text-xs text-gray-500">
                    {FREQ_LABELS[ip.monitoring_frequency] ?? ip.monitoring_frequency}
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => { openEdit(ip); }}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7 text-destructive"
                        onClick={() => { setDeleteId(ip.id); }}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <Dialog open={dialogMode !== null} onOpenChange={(open) => { if (!open) setDialogMode(null); }}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{dialogMode === 'create' ? 'Interessierte Partei hinzufügen' : 'Partei bearbeiten'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Name *</Label>
              <Input placeholder="z.B. Datenschutzbehörde" value={form.name} onChange={(e) => { setForm(f => ({ ...f, name: e.target.value })); }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>Kategorie</Label>
                <Select value={form.category} onValueChange={(v) => { setForm(f => ({ ...f, category: v })); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {Object.entries(CATEGORY_LABELS).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>Überwachungsfrequenz</Label>
                <Select value={form.monitoring_frequency} onValueChange={(v) => { setForm(f => ({ ...f, monitoring_frequency: v })); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {Object.entries(FREQ_LABELS).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>Beschreibung</Label>
              <Textarea rows={2} value={form.description} onChange={(e) => { setForm(f => ({ ...f, description: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>Bedürfnisse und Erwartungen</Label>
              <Textarea rows={3} placeholder="Was erwartet diese Partei vom ISMS?" value={form.needs_and_expectations} onChange={(e) => { setForm(f => ({ ...f, needs_and_expectations: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>Relevante Anforderungen</Label>
              <Textarea rows={2} placeholder="Gesetze, Normen, Verträge …" value={form.relevant_requirements} onChange={(e) => { setForm(f => ({ ...f, relevant_requirements: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>Verantwortliche Person</Label>
              <Input placeholder="Name oder Team" value={form.owner} onChange={(e) => { setForm(f => ({ ...f, owner: e.target.value })); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>Abbrechen</Button>
            <Button onClick={handleSubmit} disabled={!form.name.trim() || isPending}>
              {isPending ? 'Speichern…' : 'Speichern'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Partei löschen?</AlertDialogTitle>
            <AlertDialogDescription>Diese Aktion kann nicht rückgängig gemacht werden.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => { setDeleteId(null); }}>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={() => { if (deleteId) deleteMut.mutate(deleteId); setDeleteId(null); }} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">Löschen</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
