import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Users, Plus, ChevronDown, ChevronUp, CheckCircle2, Clock } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useQuery } from '@tanstack/react-query'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { apiFetch } from '../../../api/client'
import type { Policy } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import {
  useCampaigns,
  useCreateCampaign,
  useCampaignStats,
  useCampaignRequests,
  type PolicyAcceptanceCampaign,
  type PolicyAcceptanceRequest,
} from '../hooks/usePolicyAcceptance'

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function ProgressBar({ accepted, total }: { accepted: number; total: number }) {
  const { t } = useTranslation()
  const pct = total > 0 ? Math.round((accepted / total) * 100) : 0
  return (
    <div className="w-full">
      <div className="flex justify-between text-xs text-muted-foreground mb-1">
        <span>{t('policyAcceptance.confirmedOf', { accepted, total })}</span>
        <span>{pct}%</span>
      </div>
      <div className="w-full h-2 bg-secondary rounded-full overflow-hidden">
        <div
          className="h-full bg-green-500 transition-all"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}

function CampaignStatsRow({ campaignId }: { campaignId: string }) {
  const { data: stats } = useCampaignStats(campaignId)
  if (!stats) return null
  return <ProgressBar accepted={stats.accepted} total={stats.total} />
}

function RequestRow({ req }: { req: PolicyAcceptanceRequest }) {
  const { t } = useTranslation()
  const { formatDateTime } = useFormatDate()
  const isAccepted = !!req.accepted_at
  const acceptedDate = req.accepted_at
    ? formatDateTime(req.accepted_at)
    : null

  return (
    <tr className="border-b border-border/50 last:border-0">
      <td className="py-2 pr-4 text-sm">{req.recipient_email}</td>
      <td className="py-2 pr-4 text-sm text-muted-foreground">{req.recipient_name || '—'}</td>
      <td className="py-2 text-sm">
        {isAccepted ? (
          <span className="flex items-center gap-1 text-green-500">
            <CheckCircle2 size={14} />
            {t('policyAcceptance.accepted', { date: acceptedDate })}
          </span>
        ) : (
          <span className="flex items-center gap-1 text-muted-foreground">
            <Clock size={14} />
            {t('policyAcceptance.pendingStatus')}
          </span>
        )}
      </td>
    </tr>
  )
}

function CampaignDetails({ campaignId }: { campaignId: string }) {
  const { t } = useTranslation()
  const { data: requests, isLoading } = useCampaignRequests(campaignId)

  if (isLoading) return <p className="text-sm text-muted-foreground py-2">{t('policyAcceptance.loadingRequests')}</p>
  if (!requests || requests.length === 0) {
    return <p className="text-sm text-muted-foreground py-2">{t('policyAcceptance.noRecipients')}</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full mt-3">
        <thead>
          <tr className="text-xs text-muted-foreground border-b border-border">
            <th className="text-left pb-1 pr-4 font-medium">{t('policyAcceptance.colEmail')}</th>
            <th className="text-left pb-1 pr-4 font-medium">{t('common.name')}</th>
            <th className="text-left pb-1 font-medium">{t('common.status')}</th>
          </tr>
        </thead>
        <tbody>
          {requests.map((req) => (
            <RequestRow key={req.id} req={req} />
          ))}
        </tbody>
      </table>
    </div>
  )
}

function CampaignCard({ campaign }: { campaign: PolicyAcceptanceCampaign }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const { formatDate } = useFormatDate()
  const createdAt = formatDate(campaign.created_at, { day: '2-digit', month: 'short', year: 'numeric' })

  return (
    <Card>
      <CardContent className="pt-4 space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div>
            <p className="font-medium text-sm">{campaign.name}</p>
            <p className="text-xs text-muted-foreground mt-0.5">{t('policyAcceptance.createdAt', { date: createdAt })}</p>
            {campaign.deadline && (
              <p className="text-xs text-muted-foreground">{t('policyAcceptance.deadline', { date: campaign.deadline })}</p>
            )}
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => { setOpen((v) => !v); }}
            className="flex items-center gap-1 text-xs"
          >
            {t('policyAcceptance.details')} {open ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
          </Button>
        </div>

        <CampaignStatsRow campaignId={campaign.id} />

        {open && <CampaignDetails campaignId={campaign.id} />}
      </CardContent>
    </Card>
  )
}

// ---------------------------------------------------------------------------
// Create Campaign Dialog
// ---------------------------------------------------------------------------

interface CreateDialogProps {
  policyId: string
  policyTitle: string
  open: boolean
  onClose: () => void
}

function CreateCampaignDialog({ policyId, policyTitle, open, onClose }: CreateDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [message, setMessage] = useState('')
  const [deadline, setDeadline] = useState('')
  const [emailsRaw, setEmailsRaw] = useState('')
  const [error, setError] = useState('')

  const createMutation = useCreateCampaign(policyId)

  function handleSubmit() {
    setError('')
    const lines = emailsRaw
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean)

    if (!name.trim()) {
      setError(t('policyAcceptance.errorNameRequired'))
      return
    }
    if (lines.length === 0) {
      setError(t('policyAcceptance.errorEmailRequired'))
      return
    }

    const emails = lines.map((line) => {
      const [email, ...rest] = line.split(',').map((s) => s.trim())
      return { email, name: rest[0] ?? '' }
    })

    createMutation.mutate(
      {
        policy_id: policyId,
        name: name.trim(),
        message: message.trim() || undefined,
        deadline: deadline || undefined,
        emails,
      },
      {
        onSuccess: () => {
          setName('')
          setMessage('')
          setDeadline('')
          setEmailsRaw('')
          onClose()
        },
        onError: (err) => {
          setError(err.message)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('policyAcceptance.createDialogTitle', { policyTitle })}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div>
            <Label htmlFor="camp-name">{t('policyAcceptance.labelCampaignName')}</Label>
            <Input
              id="camp-name"
              value={name}
              onChange={(e) => { setName(e.target.value); }}
              placeholder="z. B. Jährliche Bestätigung 2025"
              className="mt-1"
            />
          </div>

          <div>
            <Label htmlFor="camp-message">{t('policyAcceptance.labelMessage')}</Label>
            <Textarea
              id="camp-message"
              value={message}
              onChange={(e) => { setMessage(e.target.value); }}
              placeholder="Zusätzliche Informationen für die Empfänger..."
              rows={3}
              className="mt-1"
            />
          </div>

          <div>
            <Label htmlFor="camp-deadline">{t('policyAcceptance.labelDeadline')}</Label>
            <Input
              id="camp-deadline"
              type="date"
              value={deadline}
              onChange={(e) => { setDeadline(e.target.value); }}
              className="mt-1"
            />
          </div>

          <div>
            <Label htmlFor="camp-emails">{t('policyAcceptance.labelEmails')}</Label>
            <p className="text-xs text-muted-foreground mb-1">
              {t('policyAcceptance.emailsHint')} <code>email,Name</code>
            </p>
            <Textarea
              id="camp-emails"
              value={emailsRaw}
              onChange={(e) => { setEmailsRaw(e.target.value); }}
              placeholder={"max.muster@beispiel.de,Max Muster\nanna.schmidt@beispiel.de"}
              rows={5}
              className="mt-1 font-mono text-xs"
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={handleSubmit} disabled={createMutation.isPending}>
            {createMutation.isPending ? t('policyAcceptance.submitting') : t('policyAcceptance.submitButton')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function PolicyAcceptancePage() {
  const { t } = useTranslation()
  const { id: policyId = '' } = useParams<{ id: string }>()
  const [dialogOpen, setDialogOpen] = useState(false)

  const { data: policy } = useQuery<Policy>({
    queryKey: ['vaktcomply', 'policies', policyId],
    queryFn: () => apiFetch<Policy>(`/vaktcomply/policies/${policyId}`),
    enabled: !!policyId,
  })

  const { data: campaigns, isLoading } = useCampaigns(policyId)

  const policyTitle = policy?.title ?? t('policyAcceptance.defaultPolicy')

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('policyAcceptance.title')}
        description={t('policyAcceptance.description', { policyTitle })}
        actions={
          <Button size="sm" onClick={() => { setDialogOpen(true); }}>
            <Plus size={14} className="mr-1" /> {t('policyAcceptance.newCampaign')}
          </Button>
        }
      />

      {isLoading && (
        <div className="flex justify-center py-12">
          <Spinner size="md" />
        </div>
      )}

      {!isLoading && (!campaigns || campaigns.length === 0) && (
        <EmptyState
          icon={Users}
          title={t('policyAcceptance.emptyTitle')}
          description={t('policyAcceptance.emptyDescription')}
          action={
            <Button size="sm" onClick={() => { setDialogOpen(true); }}>
              <Plus size={14} className="mr-1" />
              {t('policyAcceptance.newCampaign')}
            </Button>
          }
        />
      )}

      {campaigns && campaigns.length > 0 && (
        <div className="space-y-3">
          {campaigns.map((c) => (
            <CampaignCard key={c.id} campaign={c} />
          ))}
        </div>
      )}

      {policyId && (
        <CreateCampaignDialog
          policyId={policyId}
          policyTitle={policyTitle}
          open={dialogOpen}
          onClose={() => { setDialogOpen(false); }}
        />
      )}
    </div>
  )
}
