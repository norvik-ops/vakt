import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Plus, Globe } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { Textarea } from '../../../components/ui/textarea'
import { apiFetch } from '../../../api/client'
import { EmptyState } from '../../../shared/components/EmptyState'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import type { DataTransfer, TransferComplianceStatus } from '../types'

const STATUS_BADGE_VARIANT: Record<string, 'success' | 'warning' | 'destructive' | 'outline'> = {
  adequate: 'success',
  requires_tia: 'warning',
  tia_adequate: 'success',
  tia_adequate_measures: 'warning',
  tia_inadequate: 'destructive',
  under_review: 'outline',
}

function ComplianceSummary({ status }: { status: TransferComplianceStatus }) {
  const { t } = useTranslation()
  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div className="flex flex-col gap-1 p-3 bg-surface border border-border rounded-lg">
        <span className="text-xs text-secondary">{t('vaktprivacy.transfersPage.summaryTotal')}</span>
        <span className="text-xl font-bold">{status.total_transfers}</span>
      </div>
      <div className="flex flex-col gap-1 p-3 bg-green-500/10 border border-green-500/20 rounded-lg">
        <span className="text-xs text-secondary">{t('vaktprivacy.transfersPage.summaryAdequate')}</span>
        <span className="text-xl font-bold text-green-500">{status.adequate + status.tia_adequate}</span>
      </div>
      <div className="flex flex-col gap-1 p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-lg">
        <span className="text-xs text-secondary">{t('vaktprivacy.transfersPage.summaryTiaPending')}</span>
        <span className="text-xl font-bold text-yellow-500">{status.requires_tia}</span>
      </div>
      <div className="flex flex-col gap-1 p-3 bg-red-500/10 border border-red-500/20 rounded-lg">
        <span className="text-xs text-secondary">{t('vaktprivacy.transfersPage.summaryInadequate')}</span>
        <span className="text-xl font-bold text-red-400">{status.tia_inadequate}</span>
      </div>
    </div>
  )
}

function TransferRow({ transfer, onTIA }: { transfer: DataTransfer; onTIA: (t: DataTransfer) => void }) {
  const { t } = useTranslation()
  const getMechanismLabel = (m: string) => t(`vaktprivacy.transfersPage.mechanism${m.charAt(0).toUpperCase() + m.slice(1).replace(/_([a-z])/g, (_, c: string) => c.toUpperCase())}`, { defaultValue: m })
  const getStatusLabel = (s: string) => t(`vaktprivacy.transfersPage.status${s.charAt(0).toUpperCase() + s.slice(1).replace(/_([a-z])/g, (_, c: string) => c.toUpperCase())}`, { defaultValue: s })
  const badgeVariant = STATUS_BADGE_VARIANT[transfer.status] ?? 'outline'
  return (
    <div className="flex items-start justify-between px-4 py-3 bg-surface border border-border rounded-lg gap-3">
      <div className="flex items-start gap-3 min-w-0">
        <Globe className="w-4 h-4 text-secondary mt-0.5 shrink-0" />
        <div className="min-w-0">
          <p className="text-sm font-medium truncate">{transfer.recipient_name}</p>
          <p className="text-xs text-secondary">
            {transfer.recipient_country_name} · {getMechanismLabel(transfer.transfer_mechanism)}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <Badge variant={badgeVariant} className="text-xs">{getStatusLabel(transfer.status)}</Badge>
        {transfer.status === 'requires_tia' && (
          <Button size="sm" variant="outline" onClick={() => { onTIA(transfer); }}>
            {t('vaktprivacy.transfersPage.createTIA')}
          </Button>
        )}
      </div>
    </div>
  )
}

export default function TransfersPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [tiaTarget, setTIATarget] = useState<DataTransfer | null>(null)

  const { data: transfers, isLoading, isError, error } = useQuery<DataTransfer[]>({
    queryKey: ['privacy', 'transfers'],
    queryFn: () => apiFetch<DataTransfer[]>('/vaktprivacy/transfers'),
    staleTime: 2 * 60 * 1000,
  })

  const { data: compliance } = useQuery<TransferComplianceStatus>({
    queryKey: ['privacy', 'transfers-compliance'],
    queryFn: () => apiFetch<TransferComplianceStatus>('/vaktprivacy/transfers/compliance'),
    staleTime: 2 * 60 * 1000,
  })

  const createMutation = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      apiFetch('/vaktprivacy/transfers', { method: 'POST', body: JSON.stringify(body) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['privacy', 'transfers'] })
      void qc.invalidateQueries({ queryKey: ['privacy', 'transfers-compliance'] })
      setShowCreate(false)
    },
  })

  const tiaMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: Record<string, unknown> }) =>
      apiFetch(`/vaktprivacy/transfers/${id}/tia`, { method: 'POST', body: JSON.stringify(body) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['privacy', 'transfers'] })
      void qc.invalidateQueries({ queryKey: ['privacy', 'transfers-compliance'] })
      setTIATarget(null)
    },
  })

  function handleCreate(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    createMutation.mutate({
      recipient_name: fd.get('recipient_name'),
      recipient_country: fd.get('recipient_country'),
      transfer_mechanism: fd.get('transfer_mechanism'),
      data_categories: (fd.get('data_categories') as string).split(',').map((s) => s.trim()).filter(Boolean),
    })
  }

  function handleTIA(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!tiaTarget) return
    const fd = new FormData(e.currentTarget)
    tiaMutation.mutate({
      id: tiaTarget.id,
      body: {
        legal_system_notes: fd.get('legal_system_notes'),
        surveillance_risk: fd.get('surveillance_risk'),
        data_subject_rights_available: fd.get('data_subject_rights_available') === 'on',
        encryption_in_transit: fd.get('encryption_in_transit') === 'on',
        encryption_at_rest: fd.get('encryption_at_rest') === 'on',
        pseudonymization_applied: fd.get('pseudonymization_applied') === 'on',
        access_controls_documented: fd.get('access_controls_documented') === 'on',
        outcome: fd.get('outcome'),
        supplementary_measures: fd.get('supplementary_measures'),
      },
    })
  }

  return (
    <ProGate error={isError ? error : null}>
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktprivacy.transfersPage.title')}
        description={t('vaktprivacy.transfersPage.description')}
        actions={
          <Button size="sm" onClick={() => { setShowCreate(true); }}>
            <Plus className="w-4 h-4 mr-1" />
            {t('vaktprivacy.transfersPage.addTransfer')}
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {compliance && <ComplianceSummary status={compliance} />}

        {isLoading ? (
          <SkeletonTable rows={4} cols={3} />
        ) : !transfers || transfers.length === 0 ? (
          <EmptyState
            icon={Globe}
            title={t('vaktprivacy.transfersPage.emptyTitle')}
            description={t('vaktprivacy.transfersPage.emptyDesc')}
          />
        ) : (
          <div className="space-y-2">
            {transfers.map((t) => (
              <TransferRow key={t.id} transfer={t} onTIA={setTIATarget} />
            ))}
          </div>
        )}
      </div>

      {/* Create Transfer Dialog */}
      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktprivacy.transfersPage.createDialogTitle')}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreate} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="recipient_name">{t('vaktprivacy.transfersPage.labelRecipient')}</Label>
              <Input id="recipient_name" name="recipient_name" required placeholder="z.B. AWS US-East-1" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="recipient_country">{t('vaktprivacy.transfersPage.labelCountry')}</Label>
              <Input id="recipient_country" name="recipient_country" required maxLength={2} placeholder="US" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="transfer_mechanism">{t('vaktprivacy.transfersPage.labelMechanism')}</Label>
              <Select name="transfer_mechanism" required defaultValue="scc">
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="adequacy_decision">{t('vaktprivacy.transfersPage.mechanismAdequacy')}</SelectItem>
                  <SelectItem value="scc">{t('vaktprivacy.transfersPage.mechanismSCC')}</SelectItem>
                  <SelectItem value="bcr">{t('vaktprivacy.transfersPage.mechanismBCR')}</SelectItem>
                  <SelectItem value="derogation">{t('vaktprivacy.transfersPage.mechanismDerogation')}</SelectItem>
                  <SelectItem value="other">{t('vaktprivacy.transfersPage.mechanismOther')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="data_categories">{t('vaktprivacy.transfersPage.labelDataCategories')}</Label>
              <Input id="data_categories" name="data_categories" placeholder={t('vaktprivacy.transfersPage.placeholderDataCategories')} />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setShowCreate(false); }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? t('vaktprivacy.transfersPage.savingPending') : t('common.add')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* TIA Dialog */}
      <Dialog open={!!tiaTarget} onOpenChange={(o) => { if (!o) setTIATarget(null); }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('vaktprivacy.transfersPage.tiaDialogTitle')} — {tiaTarget?.recipient_name}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleTIA} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="legal_system_notes">{t('vaktprivacy.transfersPage.labelLegalSystem')}</Label>
              <Textarea id="legal_system_notes" name="legal_system_notes" required rows={3} placeholder={t('vaktprivacy.transfersPage.placeholderLegalSystem')} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="surveillance_risk">{t('vaktprivacy.transfersPage.labelSurveillanceRisk')}</Label>
              <Select name="surveillance_risk" required defaultValue="medium">
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">{t('vaktprivacy.transfersPage.riskLow')}</SelectItem>
                  <SelectItem value="medium">{t('vaktprivacy.transfersPage.riskMedium')}</SelectItem>
                  <SelectItem value="high">{t('vaktprivacy.transfersPage.riskHigh')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid grid-cols-2 gap-2">
              {([
                ['encryption_in_transit', t('vaktprivacy.transfersPage.checkEncryptionTransit')],
                ['encryption_at_rest', t('vaktprivacy.transfersPage.checkEncryptionRest')],
                ['pseudonymization_applied', t('vaktprivacy.transfersPage.checkPseudonymization')],
                ['access_controls_documented', t('vaktprivacy.transfersPage.checkAccessControls')],
                ['data_subject_rights_available', t('vaktprivacy.transfersPage.checkDataSubjectRights')],
              ] as const).map(([name, label]) => (
                <label key={name} className="flex items-center gap-2 text-sm cursor-pointer">
                  <input type="checkbox" name={name} className="rounded border-border" />
                  {label}
                </label>
              ))}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="supplementary_measures">{t('vaktprivacy.transfersPage.labelSupplementaryMeasures')}</Label>
              <Textarea id="supplementary_measures" name="supplementary_measures" rows={2} placeholder="Optionale technische/organisatorische Schutzmaßnahmen…" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="outcome">{t('vaktprivacy.transfersPage.labelOutcome')}</Label>
              <Select name="outcome" required defaultValue="adequate">
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="adequate">{t('vaktprivacy.transfersPage.outcomeAdequate')}</SelectItem>
                  <SelectItem value="adequate_with_measures">{t('vaktprivacy.transfersPage.outcomeAdequateWithMeasures')}</SelectItem>
                  <SelectItem value="inadequate">{t('vaktprivacy.transfersPage.outcomeInadequate')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setTIATarget(null); }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={tiaMutation.isPending}>
                {tiaMutation.isPending ? t('vaktprivacy.transfersPage.savingPending') : t('vaktprivacy.transfersPage.tiaConfirm')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
    </ProGate>
  )
}
