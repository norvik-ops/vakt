import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, UserCog, AlertTriangle, CheckCircle2 } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { apiFetch } from '../../../api/client'
import { EmptyState } from '../../../shared/components/EmptyState'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { PageHeader } from '../../../shared/components/PageHeader'
import type { Contractor, CreateContractorInput } from '../types'

const STATUS_BADGE: Record<string, { label: string; variant: 'outline' | 'warning' | 'success' | 'destructive' }> = {
  active:         { label: 'Aktiv', variant: 'success' },
  expiring_soon:  { label: 'Läuft bald ab', variant: 'warning' },
  offboarding:    { label: 'Offboarding', variant: 'destructive' },
  terminated:     { label: 'Beendet', variant: 'outline' },
}

function ContractorRow({ contractor }: { contractor: Contractor }) {
  const { t } = useTranslation()
  const badge = STATUS_BADGE[contractor.status] ?? { label: contractor.status, variant: 'outline' as const }
  const statusLabels: Record<string, string> = {
    active: t('vakthr.status.active'),
    expiring_soon: t('vakthr.contractors.statusExpiring'),
    offboarding: t('vakthr.status.offboarding'),
    terminated: t('vakthr.contractors.statusTerminated'),
  }
  const displayLabel = statusLabels[contractor.status] ?? badge.label
  return (
    <div className="flex items-center justify-between px-4 py-3 bg-surface border border-border rounded-lg gap-3">
      <div className="flex items-center gap-3 min-w-0">
        <UserCog className="w-4 h-4 text-secondary shrink-0" />
        <div className="min-w-0">
          <p className="text-sm font-medium">
            {contractor.first_name} {contractor.last_name}
            {contractor.company && <span className="text-secondary ml-1 font-normal">({contractor.company})</span>}
          </p>
          <p className="text-xs text-secondary">
            {new Date(contractor.contract_start).toLocaleDateString('de-DE')} –{' '}
            {new Date(contractor.contract_end).toLocaleDateString('de-DE')}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        {contractor.nda_signed && (
          <Badge variant="outline" className="text-xs gap-1">
            <CheckCircle2 className="w-3 h-3" /> NDA
          </Badge>
        )}
        {contractor.avv_signed && (
          <Badge variant="outline" className="text-xs gap-1">
            <CheckCircle2 className="w-3 h-3" /> AVV
          </Badge>
        )}
        {!contractor.nda_signed && (
          <Badge variant="warning" className="text-xs gap-1">
            <AlertTriangle className="w-3 h-3" /> {t('vakthr.contractors.ndaMissing')}
          </Badge>
        )}
        <Badge variant={badge.variant} className="text-xs">{displayLabel}</Badge>
      </div>
    </div>
  )
}

export default function ContractorsPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState<CreateContractorInput>({
    first_name: '', last_name: '', contract_start: '', contract_end: '',
  })

  const { data: contractors, isLoading } = useQuery<Contractor[]>({
    queryKey: ['hr', 'contractors'],
    queryFn: () => apiFetch('/vakthr/contractors'),
  })

  const createMutation = useMutation({
    mutationFn: (input: CreateContractorInput) =>
      apiFetch('/vakthr/contractors', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['hr', 'contractors'] })
      setShowCreate(false)
      setForm({ first_name: '', last_name: '', contract_start: '', contract_end: '' })
    },
  })

  const active    = contractors?.filter(c => c.status === 'active') ?? []
  const expiring  = contractors?.filter(c => c.status === 'expiring_soon') ?? []
  const offboarding = contractors?.filter(c => c.status === 'offboarding') ?? []
  const terminated = contractors?.filter(c => c.status === 'terminated') ?? []

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('vakthr.contractors.title')}
        description={t('vakthr.contractors.description')}
        actions={
          <Button onClick={() => { setShowCreate(true); }}>
            <Plus className="w-4 h-4 mr-2" /> {t('vakthr.contractors.create')}
          </Button>
        }
      />

      {isLoading && <SkeletonTable rows={4} />}

      {!isLoading && !contractors?.length && (
        <EmptyState
          icon={UserCog}
          title={t('vakthr.contractors.emptyTitle')}
          description={t('vakthr.contractors.emptyDesc')}
        />
      )}

      {expiring.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-warning mb-2">{t('vakthr.contractors.sectionExpiring', { count: expiring.length })}</h2>
          <div className="space-y-2">{expiring.map(c => <ContractorRow key={c.id} contractor={c} />)}</div>
        </section>
      )}

      {offboarding.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-destructive mb-2">{t('vakthr.contractors.sectionOffboarding', { count: offboarding.length })}</h2>
          <div className="space-y-2">{offboarding.map(c => <ContractorRow key={c.id} contractor={c} />)}</div>
        </section>
      )}

      {active.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold mb-2">{t('vakthr.contractors.sectionActive', { count: active.length })}</h2>
          <div className="space-y-2">{active.map(c => <ContractorRow key={c.id} contractor={c} />)}</div>
        </section>
      )}

      {terminated.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-secondary mb-2">{t('vakthr.contractors.sectionTerminated', { count: terminated.length })}</h2>
          <div className="space-y-2">{terminated.map(c => <ContractorRow key={c.id} contractor={c} />)}</div>
        </section>
      )}

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vakthr.contractors.create')}</DialogTitle>
          </DialogHeader>
          <form
            className="space-y-4 mt-2"
            onSubmit={e => {
              e.preventDefault()
              createMutation.mutate(form)
            }}
          >
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>{t('vakthr.employees.labelFirstName')} *</Label>
                <Input value={form.first_name} onChange={e => { setForm(f => ({ ...f, first_name: e.target.value })); }} required />
              </div>
              <div>
                <Label>{t('vakthr.employees.labelLastName')} *</Label>
                <Input value={form.last_name} onChange={e => { setForm(f => ({ ...f, last_name: e.target.value })); }} required />
              </div>
            </div>
            <div>
              <Label>{t('common.email')}</Label>
              <Input type="email" value={form.email ?? ''} onChange={e => { setForm(f => ({ ...f, email: e.target.value })); }} />
            </div>
            <div>
              <Label>{t('vakthr.contractors.company')}</Label>
              <Input value={form.company ?? ''} onChange={e => { setForm(f => ({ ...f, company: e.target.value })); }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>{t('vakthr.contractors.contractStart')} *</Label>
                <Input type="date" value={form.contract_start} onChange={e => { setForm(f => ({ ...f, contract_start: e.target.value })); }} required />
              </div>
              <div>
                <Label>{t('vakthr.contractors.contractEnd')} *</Label>
                <Input type="date" value={form.contract_end} onChange={e => { setForm(f => ({ ...f, contract_end: e.target.value })); }} required />
              </div>
            </div>
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={form.nda_signed ?? false} onChange={e => { setForm(f => ({ ...f, nda_signed: e.target.checked })); }} />
                {t('vakthr.contractors.ndaSigned')}
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={form.avv_signed ?? false} onChange={e => { setForm(f => ({ ...f, avv_signed: e.target.checked })); }} />
                {t('vakthr.contractors.avvSigned')}
              </label>
            </div>
            {createMutation.isError && (
              <p className="text-xs text-red-400 mt-1">
                {createMutation.error instanceof Error
                  ? createMutation.error.message
                  : t('vakthr.contractors.createFailed')}
              </p>
            )}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setShowCreate(false); }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? t('common.saving') : t('vakthr.contractors.submit')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
