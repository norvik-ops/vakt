import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import {
  ShieldCheck, AlertTriangle, ListChecks, Phone, FileDown,
  CheckCircle2, XCircle, Clock, Users, ActivitySquare,
} from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button, buttonVariants } from '../../../components/ui/button'
import { cn } from '../../../lib/utils'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { ProBadge } from '../../../shared/components/ProBadge'
import { useFeature } from '../../../shared/hooks/useFeature'
import { useBCMReadinessScore } from '../hooks/useBCMScore'
import { useBIASummary } from '../hooks/useBIA'
import { useRecoveryPlans } from '../hooks/useRecoveryPlans'
import { useEmergencyContacts } from '../hooks/useEmergencyContacts'

function ScoreGauge({ score }: { score: number }) {
  const color =
    score >= 80 ? 'text-green-400' :
    score >= 50 ? 'text-amber-400' :
    'text-red-400'
  const ringColor =
    score >= 80 ? 'stroke-green-400' :
    score >= 50 ? 'stroke-amber-400' :
    'stroke-red-400'
  const circumference = 2 * Math.PI * 36
  const offset = circumference - (score / 100) * circumference

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="relative w-24 h-24">
        <svg className="w-24 h-24 -rotate-90" viewBox="0 0 80 80">
          <circle cx="40" cy="40" r="36" fill="none" stroke="currentColor" strokeWidth="8"
            className="text-muted/30" />
          <circle cx="40" cy="40" r="36" fill="none" strokeWidth="8" strokeLinecap="round"
            className={ringColor}
            strokeDasharray={circumference}
            strokeDashoffset={offset} />
        </svg>
        <div className={`absolute inset-0 flex items-center justify-center text-2xl font-bold ${color}`}>
          {score}
        </div>
      </div>
      <span className="text-xs text-muted-foreground">/100</span>
    </div>
  )
}

export default function BCMDashboardPage() {
  const { t } = useTranslation()
  const { data: scoreData, isLoading: scoreLoading } = useBCMReadinessScore()
  const { data: summary } = useBIASummary()
  const { data: plans = [] } = useRecoveryPlans()
  const { data: contacts = [] } = useEmergencyContacts()
  const { enabled: pdfEnabled } = useFeature('audit_pdf')

  const testedPlans = plans.filter((p) => p.status === 'tested').length
  const level1Contacts = contacts.filter((c) => c.escalation_level === 1).length

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('bcm.dashboard.title')}
        description={t('bcm.dashboard.description')}
        actions={
          pdfEnabled ? (
            <a
              href="/api/v1/vaktcomply/bcm/report.pdf"
              target="_blank"
              rel="noopener noreferrer"
              className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
            >
              <FileDown className="w-4 h-4 mr-1.5" />
              {t('bcm.dashboard.exportPdf')}
            </a>
          ) : (
            <Button variant="outline" size="sm" disabled>
              <FileDown className="w-4 h-4 mr-1.5" />
              {t('bcm.dashboard.exportPdf')}
              <ProBadge />
            </Button>
          )
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {/* Readiness Score */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-primary" />
              {t('bcm.dashboard.readinessScore')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {scoreLoading && <Spinner size="sm" color="primary" />}
            {scoreData && (
              <div className="flex flex-col sm:flex-row items-start sm:items-center gap-6">
                <ScoreGauge score={scoreData.score} />
                <div className="flex-1 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
                  {scoreData.criteria.map((c) => (
                    <div key={c.key} className="flex items-start gap-2 p-2 rounded-md bg-muted/30">
                      {c.met
                        ? <CheckCircle2 className="w-4 h-4 text-green-400 shrink-0 mt-0.5" />
                        : <XCircle className="w-4 h-4 text-red-400 shrink-0 mt-0.5" />}
                      <div className="min-w-0">
                        <p className="text-xs font-medium leading-tight">{c.description}</p>
                        <p className="text-xs text-muted-foreground">{c.points} {t('bcm.dashboard.points')}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* KPI row */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <Card>
            <CardContent className="pt-5 flex flex-col gap-1">
              <div className="flex items-center gap-2 text-muted-foreground text-xs">
                <ActivitySquare className="w-3.5 h-3.5" />
                {t('bcm.dashboard.biaProcesses')}
              </div>
              <p className="text-2xl font-bold">{summary?.total ?? '—'}</p>
              {(summary?.high_critical ?? 0) > 0 && (
                <Badge className="bg-red-500/20 text-red-400 border-red-500/30 text-xs w-fit" variant="outline">
                  {summary?.high_critical} {t('bcm.dashboard.highCritical')}
                </Badge>
              )}
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-5 flex flex-col gap-1">
              <div className="flex items-center gap-2 text-muted-foreground text-xs">
                <Clock className="w-3.5 h-3.5" />
                {t('bcm.dashboard.avgRto')}
              </div>
              <p className="text-2xl font-bold">
                {summary?.avg_rto_hours != null ? `${summary.avg_rto_hours}h` : '—'}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-5 flex flex-col gap-1">
              <div className="flex items-center gap-2 text-muted-foreground text-xs">
                <ListChecks className="w-3.5 h-3.5" />
                {t('bcm.dashboard.recoveryPlans')}
              </div>
              <p className="text-2xl font-bold">{plans.length}</p>
              <p className="text-xs text-muted-foreground">
                {testedPlans} {t('bcm.dashboard.tested')}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-5 flex flex-col gap-1">
              <div className="flex items-center gap-2 text-muted-foreground text-xs">
                <Users className="w-3.5 h-3.5" />
                {t('bcm.dashboard.emergencyContacts')}
              </div>
              <p className="text-2xl font-bold">{contacts.length}</p>
              <p className="text-xs text-muted-foreground">
                {level1Contacts} {t('bcm.dashboard.primaryContacts')}
              </p>
            </CardContent>
          </Card>
        </div>

        {/* Quick links */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <Link to="/vaktcomply/bcm/bia" className="block">
            <Card className="hover:border-primary/50 transition-colors cursor-pointer h-full">
              <CardContent className="pt-5 flex items-start gap-3">
                <ActivitySquare className="w-5 h-5 text-primary mt-0.5 shrink-0" />
                <div>
                  <p className="font-medium text-sm">{t('bcm.bia.title')}</p>
                  <p className="text-xs text-muted-foreground mt-0.5">{t('bcm.dashboard.biaHint')}</p>
                </div>
              </CardContent>
            </Card>
          </Link>
          <Link to="/vaktcomply/bcm/recovery-plans" className="block">
            <Card className="hover:border-primary/50 transition-colors cursor-pointer h-full">
              <CardContent className="pt-5 flex items-start gap-3">
                <ListChecks className="w-5 h-5 text-primary mt-0.5 shrink-0" />
                <div>
                  <p className="font-medium text-sm">{t('bcm.recoveryPlans.title')}</p>
                  <p className="text-xs text-muted-foreground mt-0.5">{t('bcm.dashboard.wapHint')}</p>
                </div>
              </CardContent>
            </Card>
          </Link>
          <Link to="/vaktcomply/bcm/emergency-contacts" className="block">
            <Card className="hover:border-primary/50 transition-colors cursor-pointer h-full">
              <CardContent className="pt-5 flex items-start gap-3">
                <Phone className="w-5 h-5 text-primary mt-0.5 shrink-0" />
                <div>
                  <p className="font-medium text-sm">{t('bcm.emergencyContacts.title')}</p>
                  <p className="text-xs text-muted-foreground mt-0.5">{t('bcm.dashboard.contactsHint')}</p>
                </div>
              </CardContent>
            </Card>
          </Link>
        </div>

        {/* Incomplete warning */}
        {scoreData && scoreData.score < 60 && (
          <div className="p-4 rounded-lg bg-amber-500/10 border border-amber-500/30 flex items-start gap-3">
            <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            <div>
              <p className="text-sm font-medium text-amber-300">{t('bcm.dashboard.warningTitle')}</p>
              <p className="text-xs text-amber-400/80 mt-0.5">{t('bcm.dashboard.warningDescription')}</p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
