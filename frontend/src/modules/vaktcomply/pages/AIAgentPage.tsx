import { useTranslation } from 'react-i18next'
import { PageHeader } from '../../../shared/components/PageHeader'
import { AgentRunPanel } from '../../../shared/components/AgentRunPanel'
import { AIDisclaimer } from '../../../shared/components/AIDisclaimer'

// Sprint 18 + S22-8: AI-Agent-Page mit Live-Visualisierung des Plan/Execute/
// Reflect-Loops. Wird im SecVitals-Modul gemountet, weil die meisten der
// initialen Tools (list_open_findings, get_control_status, …) dort wohnen.
// S120-8: i18n + Beta-Kennzeichnung (EU-AI-Act-Transparenz) — das Backend
// sendet X-Vakt-Status: experimental, die UI spiegelt das sichtbar.

export default function AIAgentPage() {
  const { t } = useTranslation()

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <PageHeader title={t('aiAgent.title')} description={t('aiAgent.description')} />
        </div>
        <span className="mt-1 inline-flex items-center rounded-full bg-amber-500/15 border border-amber-500/30 px-2.5 py-0.5 text-xs font-semibold text-amber-700 dark:text-amber-400">
          {t('aiAgent.betaBadge')}
        </span>
      </div>
      <AIDisclaimer />
      <AgentRunPanel />
      <div className="rounded-lg border border-border bg-muted/20 p-4 text-xs text-secondary leading-relaxed">
        <p className="font-semibold text-primary mb-1">{t('aiAgent.whatTitle')}</p>
        <ul className="space-y-1 list-disc list-inside">
          <li>{t('aiAgent.whatScope')}</li>
          <li>{t('aiAgent.whatRead')}</li>
          <li>{t('aiAgent.whatWrite')}</li>
          <li>{t('aiAgent.whatAudit')}</li>
        </ul>
      </div>
    </div>
  )
}
