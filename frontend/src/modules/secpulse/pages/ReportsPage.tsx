import { useState } from 'react'
import { FileText, TrendingUp, Download } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useTranslation } from 'react-i18next'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useRiskTrend, useReports, useCreateReport, useDownloadReport } from '../hooks/useReports'
import { ProGate } from '../../../shared/components/ProGate'
import { jobStatusVariant } from '../../../lib/statusMapping'

const statusVariant = jobStatusVariant

export default function ReportsPage() {
  const { t } = useTranslation()
  const { data: trend } = useRiskTrend()
  const { data: reports, isLoading, error: reportsError } = useReports()
  const createReport = useCreateReport()
  const downloadReport = useDownloadReport()
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')

  const chartData = trend
    ? trend.labels.map((label, i) => ({ date: label, score: trend.scores[i] }))
    : []

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    await createReport.mutateAsync({ title })
    setOpen(false)
    setTitle('')
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('secpulse.reportsPage.title')}
        description={t('secpulse.reportsPage.description')}
        actions={
          <Button onClick={() => setOpen(true)}>
            <FileText className="w-4 h-4 mr-1" />
            {t('secpulse.reportsPage.createReport')}
          </Button>
        }
      />

      <div className="p-6 space-y-6">
        <ProGate error={reportsError}>
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <TrendingUp className="w-4 h-4 text-secondary" />
              <CardTitle>{t('secpulse.reportsPage.riskTrend')}</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            {chartData.length === 0 ? (
              <p className="text-sm text-secondary py-8 text-center">{t('secpulse.reportsPage.noTrendData')}</p>
            ) : (
              <ResponsiveContainer width="100%" height={220}>
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                  <XAxis dataKey="date" tick={{ fontSize: 11 }} />
                  <YAxis domain={[0, 100]} tick={{ fontSize: 11 }} />
                  <Tooltip />
                  <Line type="monotone" dataKey="score" stroke="#2563eb" strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>{t('secpulse.reportsPage.reportHistory')}</CardTitle></CardHeader>
          <CardContent className="p-0">
            {isLoading ? (
              <div className="flex justify-center py-8">
                <Spinner size="md" />
              </div>
            ) : !reports || reports.length === 0 ? (
              <p className="text-sm text-secondary py-8 text-center">{t('secpulse.reportsPage.noReports')}</p>
            ) : (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('secpulse.reportsPage.colTitle')}</TableHead>
                      <TableHead>{t('secpulse.reportsPage.colStatus')}</TableHead>
                      <TableHead>{t('secpulse.reportsPage.colCreated')}</TableHead>
                      <TableHead>{t('secpulse.reportsPage.colExpires')}</TableHead>
                      <TableHead></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {reports.map((r) => (
                      <TableRow key={r.id}>
                        <TableCell className="font-medium">{r.title || '—'}</TableCell>
                        <TableCell>
                          <Badge variant={statusVariant[r.status]} className="capitalize">{r.status}</Badge>
                        </TableCell>
                        <TableCell className="text-sm text-secondary">
                          {new Date(r.created_at).toLocaleDateString()}
                        </TableCell>
                        <TableCell className="text-sm text-secondary">
                          {r.expires_at ? new Date(r.expires_at).toLocaleDateString() : '—'}
                        </TableCell>
                        <TableCell>
                          {r.status === 'completed' && (
                            <button
                              onClick={() => downloadReport(r.id, r.title)}
                              className="flex items-center gap-1 text-xs text-brand hover:underline"
                            >
                              <Download className="w-3.5 h-3.5" />
                              PDF
                            </button>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
        </ProGate>
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('secpulse.reportsPage.createReport')}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { void handleCreate(e) }}>
            <div className="py-4 space-y-3">
              <div className="space-y-1">
                <Label htmlFor="report-title">{t('secpulse.reportsPage.reportTitleLabel')}</Label>
                <Input
                  id="report-title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="Q2 2026 Security Report"
                  required
                />
              </div>
              {createReport.isError && (
                <p className="text-sm text-red-500">{createReport.error?.message ?? 'Report creation failed'}</p>
              )}
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={createReport.isPending}>
                {createReport.isPending ? t('secpulse.reportsPage.generating') : t('secpulse.reportsPage.generate')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
