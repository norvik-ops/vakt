import { useState } from 'react'
import { Download, FileText, CheckCircle, XCircle, Shield } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Spinner } from '../../../components/Spinner'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useTrainingMatrixReport, downloadTrainingMatrix } from '../hooks/useTrainingReport'
import { useORP3Status } from '../hooks/useORP3Status'

function formatDate(iso: string | undefined) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('de-DE')
}

export default function TrainingReportPage() {
  const { t } = useTranslation()
  const now = new Date()
  const defaultFrom = new Date(now.getFullYear() - 1, now.getMonth(), now.getDate())
    .toISOString()
    .slice(0, 10)
  const defaultTo = now.toISOString().slice(0, 10)

  const [from, setFrom] = useState(defaultFrom)
  const [to, setTo] = useState(defaultTo)

  const { data: report, isLoading } = useTrainingMatrixReport(from, to)
  const { data: orp3 } = useORP3Status()

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktaware.trainingReport.title')}
        description={t('vaktaware.trainingReport.description')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { downloadTrainingMatrix('csv', from, to) }}>
              <Download className="w-4 h-4 mr-1" />
              CSV
            </Button>
            <Button onClick={() => { downloadTrainingMatrix('pdf', from, to) }}>
              <Download className="w-4 h-4 mr-1" />
              PDF Export
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {/* Period filter */}
        <div className="flex items-center gap-4 text-sm">
          <span className="text-secondary font-medium">{t('vaktaware.trainingReport.period')}</span>
          <div className="flex items-center gap-2">
            <input
              type="date"
              value={from}
              onChange={(e) => { setFrom(e.target.value) }}
              className="border border-border rounded px-2 py-1 text-sm"
            />
            <span className="text-secondary">{t('vaktaware.trainingReport.to')}</span>
            <input
              type="date"
              value={to}
              onChange={(e) => { setTo(e.target.value) }}
              className="border border-border rounded px-2 py-1 text-sm"
            />
          </div>
        </div>

        {isLoading ? (
          <div className="flex justify-center py-16"><Spinner size="md" /></div>
        ) : (
          <>
            {/* Summary cards */}
            {report && (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <Card>
                  <CardContent className="pt-4">
                    <p className="text-2xl font-bold">{report.total_stats.total_campaigns}</p>
                    <p className="text-xs text-secondary mt-1">{t('vaktaware.trainingReport.statCampaigns')}</p>
                  </CardContent>
                </Card>
                <Card>
                  <CardContent className="pt-4">
                    <p className="text-2xl font-bold">{report.total_stats.total_participants}</p>
                    <p className="text-xs text-secondary mt-1">{t('vaktaware.trainingReport.statParticipants')}</p>
                  </CardContent>
                </Card>
                <Card>
                  <CardContent className="pt-4">
                    <p className="text-2xl font-bold">{report.total_stats.avg_click_rate.toFixed(1)}%</p>
                    <p className="text-xs text-secondary mt-1">{t('vaktaware.trainingReport.statAvgClickRate')}</p>
                  </CardContent>
                </Card>
                <Card>
                  <CardContent className="pt-4">
                    <p className="text-2xl font-bold">{report.total_stats.total_trainings_completed}</p>
                    <p className="text-xs text-secondary mt-1">{t('vaktaware.trainingReport.statTrainingsCompleted')}</p>
                  </CardContent>
                </Card>
              </div>
            )}

            {/* BSI ORP.3 badge */}
            {orp3 && (
              <Card>
                <CardHeader className="pb-2">
                  <div className="flex items-center gap-2">
                    <Shield className="w-5 h-5 text-blue-600" />
                    <CardTitle className="text-base">BSI ORP.3 Compliance</CardTitle>
                    <Badge
                      variant={orp3.fulfilled_count === orp3.total_count ? 'default' : 'secondary'}
                      className="ml-auto"
                    >
                      {t('vaktaware.trainingReport.requirementsFulfilled', { fulfilled: orp3.fulfilled_count, total: orp3.total_count })}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                    {orp3.requirements.map((req) => (
                      <div key={req.id} className="flex items-start gap-2 text-sm">
                        {req.fulfilled ? (
                          <CheckCircle className="w-4 h-4 text-green-600 shrink-0 mt-0.5" />
                        ) : (
                          <XCircle className="w-4 h-4 text-gray-400 shrink-0 mt-0.5" />
                        )}
                        <span className={req.fulfilled ? '' : 'text-secondary'}>
                          <span className="font-mono text-xs mr-1">{req.id}</span>
                          {req.title}
                        </span>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )}

            {/* Campaign table — S131-D3 (D18-03): guard campaigns against a null
                field (backend now returns [] at the root; this is defense-in-depth). */}
            {report && (report.campaigns ?? []).length > 0 && (
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-base flex items-center gap-2">
                    <FileText className="w-4 h-4" />
                    {t('vaktaware.trainingReport.campaignsOverview')}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('vaktaware.trainingReport.colCampaign')}</TableHead>
                        <TableHead>{t('vaktaware.trainingReport.colCompleted')}</TableHead>
                        <TableHead className="text-right">{t('vaktaware.trainingReport.colParticipants')}</TableHead>
                        <TableHead className="text-right">{t('vaktaware.trainingReport.colClickRate')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(report.campaigns ?? []).map((c) => (
                        <TableRow key={c.id}>
                          <TableCell className="font-medium">{c.name}</TableCell>
                          <TableCell className="text-sm text-secondary">{formatDate(c.completed_at)}</TableCell>
                          <TableCell className="text-right">{c.recipient_count}</TableCell>
                          <TableCell className="text-right">{c.click_rate.toFixed(1)}%</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            )}

            {report && (report.campaigns ?? []).length === 0 && (
              <div className="text-center py-16 text-secondary text-sm">
                {t('vaktaware.trainingReport.noCampaigns')}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
