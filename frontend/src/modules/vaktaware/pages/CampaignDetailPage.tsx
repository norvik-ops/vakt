import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Play, Square, BarChart2, FileDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { useCampaign, useCampaignStats, useLaunchCampaign, useAbortCampaign, useDownloadCampaignReport } from '../hooks/useCampaigns'
import { campaignStatusVariant } from '../../../lib/statusMapping'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const statusVariant = campaignStatusVariant

// StatCard shows one number and, optionally, the rate it represents.
//
// `pct` is a PERCENTAGE (0–100), exactly as the API sends it — it is not
// multiplied by 100 here. It used to be, and the value beside it was computed as
// `rate * emails_sent`, which treated the same field as a fraction. Both were
// wrong, and both were invisible while emails_sent was stuck at 0 (anything times
// zero is zero, and so is 0 × 100). The moment the send count became real, the
// card would have claimed "5000.0%" and "50 clicked" out of one mail sent.
function StatCard({ label, value, pct }: { label: string; value: number; pct?: number }) {
  return (
    <div className="text-center p-4 bg-surface border border-border rounded-lg">
      <div className="text-2xl font-bold text-primary">{value}</div>
      {pct != null && (
        <div className="text-sm font-medium text-brand">{pct.toFixed(1)}%</div>
      )}
      <div className="text-xs text-secondary mt-1">{label}</div>
    </div>
  )
}

export default function CampaignDetailPage() {
  const { t } = useTranslation()
  const statusLabel = (s: string) => t('vaktaware.campaignStatus.' + s, { defaultValue: s })
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDate, formatDateTime } = useFormatDate()
  const campaignId = id ?? ''

  const { data: campaign, isLoading, error } = useCampaign(campaignId)
  const { data: stats } = useCampaignStats(campaignId)
  const launch = useLaunchCampaign(campaignId)
  const abort = useAbortCampaign(campaignId)
  const downloadReport = useDownloadCampaignReport()

  if (isLoading) return (
    <div className="flex justify-center py-16">
      <Spinner size="md" />
    </div>
  )

  if (error || !campaign) return (
    <div className="p-6">
      <p className="text-sm text-red-600">{error?.message ?? 'Campaign not found'}</p>
      <Button variant="outline" className="mt-4" onClick={() => { navigate('/vaktaware/campaigns'); }}>
        <ArrowLeft className="w-4 h-4 mr-1" />Back
      </Button>
    </div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={campaign.name}
        description={`Subject: ${campaign.subject}`}
        actions={
          <div className="flex items-center gap-2">
            {campaign.status === 'draft' && (
              <Button onClick={() => { launch.mutate(); }} disabled={launch.isPending}>
                <Play className="w-4 h-4 mr-1" />
                {launch.isPending ? 'Launching…' : 'Launch'}
              </Button>
            )}
            {campaign.status === 'running' && (
              <Button variant="destructive" onClick={() => { abort.mutate(); }} disabled={abort.isPending}>
                <Square className="w-4 h-4 mr-1" />
                {abort.isPending ? 'Aborting…' : 'Abort'}
              </Button>
            )}
            {(campaign.status === 'completed' || campaign.status === 'running') && (
              <Button variant="outline" size="sm" onClick={() => { downloadReport(campaignId, campaign.name); }}>
                <FileDown className="w-4 h-4 mr-1" />
                PDF
              </Button>
            )}
            <Button variant="outline" size="sm" onClick={() => { navigate('/vaktaware/campaigns'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />Back
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        <Card>
          <CardHeader className="flex flex-row items-center gap-3 pb-3">
            <CardTitle>{t('vaktaware.campaignDetail.detailsTitle')}</CardTitle>
            <Badge variant={statusVariant[campaign.status]}>{statusLabel(campaign.status)}</Badge>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-x-8 gap-y-3 text-sm">
              <div>
                <dt className="text-secondary">{t('vaktaware.campaignDetail.sender')}</dt>
                <dd className="mt-0.5 text-primary">{campaign.from_name} &lt;{campaign.from_email}&gt;</dd>
              </div>
              <div>
                <dt className="text-secondary">{t('vaktaware.campaignDetail.scheduled')}</dt>
                <dd className="mt-0.5 text-primary">
                  {campaign.scheduled_at ? formatDateTime(campaign.scheduled_at) : t('vaktaware.campaignDetail.notScheduled')}
                </dd>
              </div>
              <div>
                <dt className="text-secondary">{t('vaktaware.campaignDetail.created')}</dt>
                <dd className="mt-0.5 text-primary">{formatDate(campaign.created_at)}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        {stats && (
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2">
                <BarChart2 className="w-4 h-4 text-secondary" />
                <CardTitle>{t('vaktaware.campaignDetail.statsTitle')}</CardTitle>
              </div>
            </CardHeader>
            <CardContent>
              {!stats.tracking_measured && campaign.status !== 'draft' && (
                <div className="mb-4 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-200">
                  {t('vaktaware.campaignDetail.trackingUnmeasured')}
                </div>
              )}
              <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
                <StatCard label={t('vaktaware.campaignDetail.statTargets')} value={stats.total_targets} />
                <StatCard label={t('vaktaware.campaignDetail.statSent')} value={stats.emails_sent} />
                {/* The counts come from the API. They used to be derived as
                    `rate × emails_sent`, which is not a count and, with a
                    percentage on the left of that multiplication, not even the
                    right order of magnitude. */}
                <StatCard label={t('vaktaware.campaignDetail.statOpened')} value={stats.opens} pct={stats.open_rate} />
                <StatCard label={t('vaktaware.campaignDetail.statClicked')} value={stats.clicks} pct={stats.click_rate} />
                <StatCard label={t('vaktaware.campaignDetail.statSubmitted')} value={stats.form_submissions} pct={stats.submission_rate} />
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
