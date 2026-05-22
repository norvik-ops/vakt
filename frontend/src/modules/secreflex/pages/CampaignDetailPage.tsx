import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Play, Square, BarChart2, FileDown } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { useCampaign, useCampaignStats, useLaunchCampaign, useAbortCampaign, useDownloadCampaignReport } from '../hooks/useCampaigns'
import { campaignStatusVariant } from '../../../lib/statusMapping'

const statusVariant = campaignStatusVariant

function StatCard({ label, value, pct }: { label: string; value: number; pct?: number }) {
  return (
    <div className="text-center p-4 bg-surface border border-border rounded-lg">
      <div className="text-2xl font-bold text-primary">{value}</div>
      {pct != null && (
        <div className="text-sm font-medium text-brand">{(pct * 100).toFixed(1)}%</div>
      )}
      <div className="text-xs text-secondary mt-1">{label}</div>
    </div>
  )
}

export default function CampaignDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
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
      <Button variant="outline" className="mt-4" onClick={() => navigate('/secreflex/campaigns')}>
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
              <Button onClick={() => launch.mutate()} disabled={launch.isPending}>
                <Play className="w-4 h-4 mr-1" />
                {launch.isPending ? 'Launching…' : 'Launch'}
              </Button>
            )}
            {campaign.status === 'running' && (
              <Button variant="destructive" onClick={() => abort.mutate()} disabled={abort.isPending}>
                <Square className="w-4 h-4 mr-1" />
                {abort.isPending ? 'Aborting…' : 'Abort'}
              </Button>
            )}
            {(campaign.status === 'completed' || campaign.status === 'running') && (
              <Button variant="outline" size="sm" onClick={() => downloadReport(campaignId, campaign.name)}>
                <FileDown className="w-4 h-4 mr-1" />
                PDF
              </Button>
            )}
            <Button variant="outline" size="sm" onClick={() => navigate('/secreflex/campaigns')}>
              <ArrowLeft className="w-4 h-4 mr-1" />Back
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        <Card>
          <CardHeader className="flex flex-row items-center gap-3 pb-3">
            <CardTitle>Details</CardTitle>
            <Badge variant={statusVariant[campaign.status]} className="capitalize">{campaign.status}</Badge>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-x-8 gap-y-3 text-sm">
              <div>
                <dt className="text-secondary">From</dt>
                <dd className="mt-0.5 text-primary">{campaign.from_name} &lt;{campaign.from_email}&gt;</dd>
              </div>
              <div>
                <dt className="text-secondary">Scheduled</dt>
                <dd className="mt-0.5 text-primary">
                  {campaign.scheduled_at ? new Date(campaign.scheduled_at).toLocaleString() : 'Not scheduled'}
                </dd>
              </div>
              <div>
                <dt className="text-secondary">Created</dt>
                <dd className="mt-0.5 text-primary">{new Date(campaign.created_at).toLocaleDateString()}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        {stats && (
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2">
                <BarChart2 className="w-4 h-4 text-secondary" />
                <CardTitle>Campaign Statistics</CardTitle>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
                <StatCard label="Targets" value={stats.total_targets} />
                <StatCard label="Emails Sent" value={stats.emails_sent} />
                <StatCard label="Opened" value={Math.round(stats.open_rate * stats.emails_sent)} pct={stats.open_rate} />
                <StatCard label="Clicked" value={Math.round(stats.click_rate * stats.emails_sent)} pct={stats.click_rate} />
                <StatCard label="Submitted" value={Math.round(stats.submission_rate * stats.emails_sent)} pct={stats.submission_rate} />
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
