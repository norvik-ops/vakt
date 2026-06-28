import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Fish, Plus, Workflow, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useCampaigns, useCreateCampaign } from '../hooks/useCampaigns'
import { useTemplates } from '../hooks/useTemplates'
import { useTargetGroups } from '../hooks/useTargetGroups'
import { ProGate } from '../../../shared/components/ProGate'
import { campaignStatusVariant } from '../../../lib/statusMapping'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const statusVariant = campaignStatusVariant

export default function CampaignsPage() {
  const { t } = useTranslation()
  const statusLabel = (s: string) => t('vaktaware.campaignStatus.' + s, { defaultValue: s })
  const navigate = useNavigate()
  const { formatDate, formatDateTime } = useFormatDate()
  const { data: campaigns, isLoading, error: campaignsError } = useCampaigns()
  const { data: templates } = useTemplates()
  const { data: groups } = useTargetGroups()
  const createCampaign = useCreateCampaign()

  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [templateId, setTemplateId] = useState('')
  const [groupId, setGroupId] = useState('')
  const [fromName, setFromName] = useState('')
  const [fromEmail, setFromEmail] = useState('')
  const [subject, setSubject] = useState('')
  const [scheduledAt, setScheduledAt] = useState('')
  const [betriebsratMode, setBetriebsratMode] = useState(true)
  const [trackOpens, setTrackOpens] = useState(false)

  function resetForm() {
    setName(''); setTemplateId(''); setGroupId(''); setFromName('')
    setFromEmail(''); setSubject(''); setScheduledAt('')
    setBetriebsratMode(true); setTrackOpens(false)
  }

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    createCampaign.mutate(
      {
        name,
        template_id: templateId,
        target_group_id: groupId,
        from_name: fromName,
        from_email: fromEmail,
        subject,
        scheduled_at: scheduledAt || undefined,
        betriebsrat_mode: betriebsratMode,
        track_opens: trackOpens && !betriebsratMode,
      },
      {
        onSuccess: (campaign) => {
          setOpen(false)
          resetForm()
          navigate(`/vaktaware/campaigns/${campaign.id}`)
        },
      },
    )
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktaware.campaignsPage.title')}
        description={t('vaktaware.campaignsPage.description')}
        actions={
          <Button onClick={() => { setOpen(true); }}>
            <Plus className="w-4 h-4 mr-1" />
            {t('vaktaware.campaignsPage.newCampaign')}
          </Button>
        }
      />

      <InfoBanner icon={Workflow} title={t('vaktaware.campaignsPage.infoBannerTitle')}>
        <p>Workflow in 3 Schritten: <strong>1. Vorlage</strong> anlegen (die Phishing-E-Mail) → <strong>2. Zielgruppe</strong> definieren (Empfänger-Liste) → <strong>3. Kampagne</strong> starten.</p>
        <p className="mt-1">Die Auswertung ist betriebsratskonform: Es werden nur anonymisierte Klickraten je Abteilung angezeigt, keine personenbezogenen Einzelergebnisse. SMTP-Zugangsdaten trägst du unter <strong>Settings → E-Mail</strong> ein.</p>
      </InfoBanner>

      <div className="flex-1 p-6">
        <ProGate error={campaignsError}>
          {isLoading ? (
            <div className="flex justify-center py-16">
              <Spinner size="md" />
            </div>
          ) : !campaigns || campaigns.length === 0 ? (
            <EmptyState
              icon={Fish}
              title={t('vaktaware.campaignsPage.noCampaigns')}
              description={t('vaktaware.campaignsPage.noCampaignsDesc')}
              action={
                <Button onClick={() => { setOpen(true); }}>
                  <Plus className="w-4 h-4 mr-1" />{t('vaktaware.campaignsPage.createCampaign')}
                </Button>
              }
            />
          ) : (
            <div className="rounded-md border border-border bg-surface overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('common.name')}</TableHead>
                    <TableHead>{t('common.status')}</TableHead>
                    <TableHead>{t('vaktaware.campaignsPage.colMode')}</TableHead>
                    <TableHead>{t('vaktaware.campaignsPage.colScheduled')}</TableHead>
                    <TableHead>{t('vaktaware.campaignsPage.colCreated')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {campaigns.map((c) => (
                    <TableRow
                      key={c.id}
                      className="cursor-pointer hover:bg-surface2"
                      onClick={() => { navigate(`/vaktaware/campaigns/${c.id}`); }}
                    >
                      <TableCell className="font-medium">{c.name}</TableCell>
                      <TableCell>
                        <Badge variant={statusVariant[c.status]}>{statusLabel(c.status)}</Badge>
                      </TableCell>
                      <TableCell>
                        {c.betriebsrat_mode ? (
                          <span className="inline-flex items-center gap-1 text-xs text-green-600 font-medium">
                            <ShieldCheck className="w-3 h-3" />{t('vaktaware.campaignsPage.brCompliant')}
                          </span>
                        ) : (
                          <span className="text-xs text-amber-600">{t('vaktaware.campaignsPage.fullTracking')}</span>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-secondary">
                        {c.scheduled_at ? formatDateTime(c.scheduled_at) : '—'}
                      </TableCell>
                      <TableCell className="text-sm text-secondary">
                        {formatDate(c.created_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </ProGate>
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('vaktaware.campaignsPage.dialogTitle')}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { handleCreate(e) }}>
            <div className="py-4 space-y-4 max-h-[60vh] overflow-y-auto pr-1">
              <div className="space-y-1.5">
                <Label htmlFor="camp-name">{t('vaktaware.campaignsPage.labelCampaignName')}</Label>
                <Input id="camp-name" value={name} onChange={(e) => { setName(e.target.value); }} required />
              </div>
              <div className="space-y-1.5">
                <Label>{t('vaktaware.campaignsPage.labelTemplate')}</Label>
                <Select value={templateId} onValueChange={setTemplateId} required>
                  <SelectTrigger><SelectValue placeholder={t('vaktaware.campaignsPage.placeholderTemplate')} /></SelectTrigger>
                  <SelectContent>
                    {templates?.map((t) => (
                      <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('vaktaware.campaignsPage.labelTargetGroup')}</Label>
                <Select value={groupId} onValueChange={setGroupId} required>
                  <SelectTrigger><SelectValue placeholder={t('vaktaware.campaignsPage.placeholderTargetGroup')} /></SelectTrigger>
                  <SelectContent>
                    {groups?.map((g) => (
                      <SelectItem key={g.id} value={g.id}>{g.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="from-name">{t('vaktaware.campaignsPage.labelFromName')}</Label>
                  <Input id="from-name" value={fromName} onChange={(e) => { setFromName(e.target.value); }} required />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="from-email">From Email</Label>
                  <Input id="from-email" type="email" value={fromEmail} onChange={(e) => { setFromEmail(e.target.value); }} required />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="subject">{t('vaktaware.campaignsPage.labelSubject')}</Label>
                <Input id="subject" value={subject} onChange={(e) => { setSubject(e.target.value); }} required />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="scheduled-at">Schedule (optional)</Label>
                <Input id="scheduled-at" type="datetime-local" value={scheduledAt} onChange={(e) => { setScheduledAt(e.target.value); }} />
              </div>

              {/* Betriebsrat Mode — ON by default */}
              <div className="rounded-lg border border-border bg-bg p-4 space-y-3">
                <div className="flex items-start gap-3">
                  <input
                    id="betriebsrat-mode"
                    type="checkbox"
                    checked={betriebsratMode}
                    onChange={(e) => { setBetriebsratMode(e.target.checked); }}
                    className="mt-0.5 h-4 w-4 rounded border-border accent-brand"
                  />
                  <div>
                    <Label htmlFor="betriebsrat-mode" className="flex items-center gap-1.5 cursor-pointer">
                      <ShieldCheck className="w-3.5 h-3.5 text-green-500" />
                      {t('vaktaware.campaignsPage.betriebsratMode')}
                    </Label>
                    <p className="text-xs text-secondary mt-0.5">
                      {t('vaktaware.campaignsPage.betriebsratModeDesc')}
                    </p>
                  </div>
                </div>
                {!betriebsratMode && (
                  <div className="flex items-start gap-3 pl-7">
                    <input
                      id="track-opens"
                      type="checkbox"
                      checked={trackOpens}
                      onChange={(e) => { setTrackOpens(e.target.checked); }}
                      className="mt-0.5 h-4 w-4 rounded border-border accent-brand"
                    />
                    <div>
                      <Label htmlFor="track-opens" className="cursor-pointer text-amber-600">{t('vaktaware.campaignsPage.trackOpens')}</Label>
                      <p className="text-xs text-secondary mt-0.5">
                        {t('vaktaware.campaignsPage.trackOpensDesc')}
                      </p>
                    </div>
                  </div>
                )}
                {!betriebsratMode && (
                  <p className="text-xs text-amber-600 pl-7">
                    {t('vaktaware.campaignsPage.betriebsratWarning')}
                  </p>
                )}
              </div>
            </div>
            <DialogFooter className="mt-2">
              <Button type="button" variant="outline" onClick={() => { setOpen(false); resetForm() }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={createCampaign.isPending}>
                {createCampaign.isPending ? 'Creating…' : 'Create Campaign'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
