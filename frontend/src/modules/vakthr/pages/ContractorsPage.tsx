import { useState } from 'react'
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
  const badge = STATUS_BADGE[contractor.status] ?? { label: contractor.status, variant: 'outline' as const }
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
            <AlertTriangle className="w-3 h-3" /> NDA fehlt
          </Badge>
        )}
        <Badge variant={badge.variant} className="text-xs">{badge.label}</Badge>
      </div>
    </div>
  )
}

export default function ContractorsPage() {
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
        title="Auftragnehmer & Freelancer"
        description="Vertragslaufzeiten, Zugangsbereiche und NDA/AVV-Status externer Mitarbeiter"
        actions={
          <Button onClick={() => { setShowCreate(true); }}>
            <Plus className="w-4 h-4 mr-2" /> Auftragnehmer anlegen
          </Button>
        }
      />

      {isLoading && <SkeletonTable rows={4} />}

      {!isLoading && !contractors?.length && (
        <EmptyState
          icon={UserCog}
          title="Noch keine Auftragnehmer"
          description="Legen Sie externe Auftragnehmer und Freelancer an, um deren Vertragslaufzeiten zu überwachen."
        />
      )}

      {expiring.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-warning mb-2">Läuft bald ab ({expiring.length})</h2>
          <div className="space-y-2">{expiring.map(c => <ContractorRow key={c.id} contractor={c} />)}</div>
        </section>
      )}

      {offboarding.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-destructive mb-2">Offboarding ({offboarding.length})</h2>
          <div className="space-y-2">{offboarding.map(c => <ContractorRow key={c.id} contractor={c} />)}</div>
        </section>
      )}

      {active.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold mb-2">Aktiv ({active.length})</h2>
          <div className="space-y-2">{active.map(c => <ContractorRow key={c.id} contractor={c} />)}</div>
        </section>
      )}

      {terminated.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-secondary mb-2">Beendet ({terminated.length})</h2>
          <div className="space-y-2">{terminated.map(c => <ContractorRow key={c.id} contractor={c} />)}</div>
        </section>
      )}

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Auftragnehmer anlegen</DialogTitle>
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
                <Label>Vorname *</Label>
                <Input value={form.first_name} onChange={e => { setForm(f => ({ ...f, first_name: e.target.value })); }} required />
              </div>
              <div>
                <Label>Nachname *</Label>
                <Input value={form.last_name} onChange={e => { setForm(f => ({ ...f, last_name: e.target.value })); }} required />
              </div>
            </div>
            <div>
              <Label>E-Mail</Label>
              <Input type="email" value={form.email ?? ''} onChange={e => { setForm(f => ({ ...f, email: e.target.value })); }} />
            </div>
            <div>
              <Label>Unternehmen</Label>
              <Input value={form.company ?? ''} onChange={e => { setForm(f => ({ ...f, company: e.target.value })); }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>Vertragsbeginn *</Label>
                <Input type="date" value={form.contract_start} onChange={e => { setForm(f => ({ ...f, contract_start: e.target.value })); }} required />
              </div>
              <div>
                <Label>Vertragsende *</Label>
                <Input type="date" value={form.contract_end} onChange={e => { setForm(f => ({ ...f, contract_end: e.target.value })); }} required />
              </div>
            </div>
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={form.nda_signed ?? false} onChange={e => { setForm(f => ({ ...f, nda_signed: e.target.checked })); }} />
                NDA unterzeichnet
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={form.avv_signed ?? false} onChange={e => { setForm(f => ({ ...f, avv_signed: e.target.checked })); }} />
                AVV unterzeichnet
              </label>
            </div>
            {createMutation.isError && (
              <p className="text-xs text-red-400 mt-1">
                {createMutation.error instanceof Error
                  ? createMutation.error.message
                  : 'Anlegen fehlgeschlagen'}
              </p>
            )}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setShowCreate(false); }}>Abbrechen</Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? 'Speichern…' : 'Anlegen'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
