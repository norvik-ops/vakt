import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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

// K5-07: mirrors vaktcomply.InterestedParty / CreateInterestedPartyInput
// (backend/internal/modules/vaktcomply/handler_ops.go). The old shape shared not
// one field name with the backend beyond id/name/category: the "needs" and
// "monitoring" columns were blank for every row, and every save blanked
// requirements and concerns. `python3 scripts/check_fe_be_fields.py` (G15) reports
// the same drift by name when it is run; it is not yet wired into any workflow or
// make target, so today the guard is InterestedPartiesPage.roundtrip.test.tsx.
interface InterestedParty {
  id: string
  org_id: string
  name: string
  category: string
  requirements?: string
  concerns?: string
  review_date?: string | null
  review_overdue: boolean
  is_system_default: boolean
  created_at: string
  updated_at: string
}

interface CreateInterestedPartyInput {
  name: string
  category: string
  requirements: string
  concerns: string
  review_date: string | null
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
  // Request type in the generics — see scripts/check_fe_be_fields.py.
  return useMutation<InterestedParty, Error, CreateInterestedPartyInput>({
    mutationFn: (input) =>
      apiFetch<InterestedParty>('/vaktcomply/interested-parties', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ['vaktcomply', 'interested-parties'] }); },
  })
}

function useUpdateIP() {
  const qc = useQueryClient()
  return useMutation<InterestedParty, Error, { id: string; input: CreateInterestedPartyInput }>({
    mutationFn: ({ id, input }) =>
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

function emptyForm(): CreateInterestedPartyInput {
  return {
    name: '',
    // 'customer', not 'external': the category `oneof` and the
    // ck_interested_parties CHECK both reject 'external' (as they did
    // 'internal' and 'regulatory') — the default alone made every create 422.
    category: 'customer',
    requirements: '',
    concerns: '',
    review_date: null,
  }
}

export default function InterestedPartiesPage() {
  const { t } = useTranslation()
  const { data = [], isLoading } = useInterestedParties()
  const createMut = useCreateIP()
  const updateMut = useUpdateIP()
  const deleteMut = useDeleteIP()
  const seedMut = useSeedDefaults()

  // Exactly the eight values the `oneof` tag and the DDL CHECK allow.
  const categoryLabels: Record<string, string> = {
    customer: t('vaktcomply.interestedParties.categoryCustomer'),
    regulator: t('vaktcomply.interestedParties.categoryRegulatory'),
    employee: t('vaktcomply.interestedParties.categoryEmployee'),
    shareholder: t('vaktcomply.interestedParties.categoryShareholder'),
    supplier: t('vaktcomply.interestedParties.categorySupplier'),
    insurer: t('vaktcomply.interestedParties.categoryInsurer'),
    it_provider: t('vaktcomply.interestedParties.categoryItProvider'),
    other: t('vaktcomply.interestedParties.categoryOther'),
  }

  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editTarget, setEditTarget] = useState<InterestedParty | null>(null)
  const [form, setForm] = useState<CreateInterestedPartyInput>(emptyForm())
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
      requirements: ip.requirements ?? '',
      concerns: ip.concerns ?? '',
      review_date: ip.review_date ?? null,
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
          <h1 className="text-2xl font-bold">{t('vaktcomply.interestedParties.title')}</h1>
          <p className="text-gray-500 text-sm mt-1">{t('vaktcomply.interestedParties.subtitle')}</p>
        </div>
        <div className="flex items-center gap-2">
          {data.length === 0 && (
            <Button variant="outline" size="sm" onClick={() => { seedMut.mutate(); }} disabled={seedMut.isPending}>
              <Wand2 className="h-4 w-4 mr-1.5" />
              {t('vaktcomply.interestedParties.seedDefaults')}
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={handleExport}>
            <Download className="h-4 w-4 mr-1.5" />
            {t('vaktcomply.interestedParties.exportPdf')}
          </Button>
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4 mr-1.5" />
            {t('vaktcomply.interestedParties.addParty')}
          </Button>
        </div>
      </div>

      {data.length === 0 ? (
        <EmptyState
          icon={Users}
          title={t('vaktcomply.interestedParties.emptyTitle')}
          description={t('vaktcomply.interestedParties.emptyDesc')}
          action={
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => { seedMut.mutate(); }} disabled={seedMut.isPending}>
                <Wand2 className="h-4 w-4 mr-1.5" />
                {t('vaktcomply.interestedParties.seedDefaults')}
              </Button>
              <Button onClick={openCreate}><Plus className="h-4 w-4 mr-1.5" />{t('vaktcomply.interestedParties.addParty')}</Button>
            </div>
          }
        />
      ) : (
        <div className="bg-white rounded-lg border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('common.name')}</TableHead>
                <TableHead>{t('vaktcomply.interestedParties.colCategory')}</TableHead>
                <TableHead>{t('vaktcomply.interestedParties.colNeeds')}</TableHead>
                <TableHead>{t('vaktcomply.interestedParties.colReviewDate')}</TableHead>
                <TableHead className="w-20"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.map((ip) => (
                <TableRow key={ip.id} className="hover:bg-gray-50">
                  <TableCell>
                    <div className="font-medium text-sm">{ip.name}</div>
                    {ip.is_system_default && (
                      <div className="text-xs text-gray-400">{t('vaktcomply.interestedParties.badgeSystemDefault')}</div>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="text-xs">
                      {categoryLabels[ip.category] ?? ip.category}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-gray-600 max-w-xs">
                    <p className="line-clamp-2">{ip.requirements}</p>
                  </TableCell>
                  <TableCell className="text-xs text-gray-500">
                    {ip.review_date ?? '—'}
                    {ip.review_overdue && (
                      <span className="ml-1.5 text-destructive font-medium">
                        {t('vaktcomply.interestedParties.badgeOverdue')}
                      </span>
                    )}
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
            <DialogTitle>{dialogMode === 'create' ? t('vaktcomply.interestedParties.dialogTitleCreate') : t('vaktcomply.interestedParties.dialogTitleEdit')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.interestedParties.labelNameRequired')}</Label>
              <Input placeholder={t('vaktcomply.interestedParties.placeholderName')} value={form.name} onChange={(e) => { setForm(f => ({ ...f, name: e.target.value })); }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.interestedParties.labelCategory')}</Label>
                <Select value={form.category} onValueChange={(v) => { setForm(f => ({ ...f, category: v })); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {Object.entries(categoryLabels).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.interestedParties.labelReviewDate')}</Label>
                <Input
                  type="date"
                  value={form.review_date ?? ''}
                  onChange={(e) => { setForm(f => ({ ...f, review_date: e.target.value || null })); }}
                />
              </div>
            </div>
            {/* ck_interested_parties has no description, monitoring_frequency or
                owner column, and CreateInterestedPartyInput binds none of them —
                the three inputs that used to be here were discarded on submit. */}
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.interestedParties.labelRequirements')}</Label>
              <Textarea rows={3} placeholder={t('vaktcomply.interestedParties.placeholderRequirements')} value={form.requirements} onChange={(e) => { setForm(f => ({ ...f, requirements: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.interestedParties.labelConcerns')}</Label>
              <Textarea rows={2} placeholder={t('vaktcomply.interestedParties.placeholderConcerns')} value={form.concerns} onChange={(e) => { setForm(f => ({ ...f, concerns: e.target.value })); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>{t('common.cancel')}</Button>
            <Button onClick={handleSubmit} disabled={!form.name.trim() || isPending}>
              {isPending ? t('common.savePending') : t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('vaktcomply.interestedParties.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('vaktcomply.interestedParties.deleteDesc')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => { setDeleteId(null); }}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => { if (deleteId) { deleteMut.mutate(deleteId); } setDeleteId(null); }} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
