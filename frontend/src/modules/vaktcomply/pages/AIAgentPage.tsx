import { PageHeader } from '../../../shared/components/PageHeader'
import { AgentRunPanel } from '../../../shared/components/AgentRunPanel'

// Sprint 18 + S22-8: AI-Agent-Page mit Live-Visualisierung des Plan/Execute/
// Reflect-Loops. Wird im SecVitals-Modul gemountet, weil die meisten der
// initialen Tools (list_open_findings, get_control_status, …) dort wohnen.

export default function AIAgentPage() {
  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="AI-Agent"
        description="Lass den Agenten Tools deiner Vakt-Instanz nutzen, um konkrete Aufträge zu erledigen — Plan/Execute/Reflect, alle Schritte transparent."
      />
      <AgentRunPanel />
      <div className="rounded-lg border border-border bg-muted/20 p-4 text-xs text-secondary leading-relaxed">
        <p className="font-semibold text-primary mb-1">Was darf der Agent?</p>
        <ul className="space-y-1 list-disc list-inside">
          <li>Nur Tools nutzen, deren <code className="font-mono">RequireScope</code> deine Rolle abdeckt.</li>
          <li>Lesende Tools (Findings, Controls, Risks listen) sind freigegeben.</li>
          <li>Mutierende Tools (Evidence anlegen, Status ändern) sind Pro-Tier — kommen mit Approve-Before-Apply.</li>
          <li>Jeder Lauf wird vollständig im Audit-Log persistiert (org-scoped).</li>
        </ul>
      </div>
    </div>
  )
}
