/**
 * Leser (Viewer, AuditorReadOnly) haben reinen Lesezugriff: Auf diesen Seiten
 * sind Anlegen, Bearbeiten, Löschen, Freigeben usw. für sie ausgeblendet, oben
 * steht der ReadOnlyHint. Je Seite die Gegenprobe mit Admin — sonst wäre der
 * Test auch grün, wenn der Knopf gar nicht mehr gerendert würde.
 * Sonderfälle: ISMS-Scope freigeben ist Admin-only (SecurityAnalyst ohne Knopf),
 * Managementbewertung freigeben ist InternalAuditor-only (Admin ohne, IA mit Knopf).
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import { signInAs } from '../../../test-utils/signInAs'
import ISMSScopePage from './ISMSScopePage'
import InterestedPartiesPage from './InterestedPartiesPage'
import ManagementReviewsPage from './ManagementReviewsPage'
import NIS2ReassessmentPage from './NIS2ReassessmentPage'
import PentestsPage from './PentestsPage'
import ProtectionNeedsPage from './ProtectionNeedsPage'
import QuestionnairePage from './QuestionnairePage'
import RecoveryPlansPage from './RecoveryPlansPage'
import ResilienceTestsPage from './ResilienceTestsPage'

const TS = '2026-11-17T10:00:00Z'
const SCOPE = {
  id: 's-1', org_id: 'o-1', version: 1, status: 'draft', scope_definition: 'Gesamte Org',
  exclusions: [{ item: 'Kantine', justification: 'keine IT' }], outsourcing_dependencies: '',
  change_note: '', created_at: TS, updated_at: TS,
}
const PARTY = {
  id: 'ip-1', org_id: 'o-1', name: 'Betriebsrat', category: 'employee', requirements: '',
  concerns: '', review_date: null, review_overdue: false, is_system_default: false,
  created_at: TS, updated_at: TS,
}
const REVIEW = {
  id: 'mr-1', org_id: 'o-1', review_date: '2026-11-01', review_type: 'annual', status: 'draft',
  participant_ids: [], audit_findings_summary: '', incident_summary: '', risk_status_summary: '',
  previous_actions_status: '', context_changes: '', customer_feedback: '',
  improvement_decisions: [], resource_decisions: '', isms_changes: '', created_at: TS, updated_at: TS,
}
const PENTEST = {
  id: 'pt-1', org_id: 'o-1', title: 'Externer Pentest 2026', tester_type: 'external',
  pentest_date: '2026-10-01', status: 'completed', findings_critical: 0, findings_high: 0,
  findings_medium: 0, findings_low: 0, created_at: TS, updated_at: TS,
}
const PNA = {
  id: 'pn-1', org_id: 'o-1', name: 'Mailserver', object_type: 'system', object_name: 'mx1',
  confidentiality: 'high', integrity: 'normal', availability: 'normal', overall: 'high',
  status: 'draft', created_at: TS, updated_at: TS,
}
const QUESTIONNAIRE = {
  id: 'q-1', org_id: 'o-1', name: 'Lieferanten-Check', description: '', is_template: false,
  questions: [{
    id: 'qq-1', questionnaire_id: 'q-1', order_idx: 0, question_text: 'Gibt es Backups?',
    question_type: 'yes_no', required: true, created_at: TS, updated_at: TS,
  }],
  created_at: TS, updated_at: TS,
}
const PLAN = {
  id: 'rp-1', org_id: 'o-1', title: 'Wiederanlauf Mailserver', status: 'draft', rto_hours: 4,
  steps: [], created_at: TS, updated_at: TS,
}
const RTEST = {
  id: 'rt-1', org_id: 'o-1', type: 'pentest', provider: 'SecTech', test_date: '2026-06-01T00:00:00Z',
  remediation_status: 'in_progress', created_at: TS, updated_at: TS,
}

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => {
    if (path === '/vaktcomply/isms-scope') return Promise.resolve(SCOPE)
    if (path === '/vaktcomply/isms-scope/versions') return Promise.resolve([SCOPE])
    if (path === '/vaktcomply/interested-parties') return Promise.resolve([PARTY])
    if (path === '/vaktcomply/management-reviews') return Promise.resolve([REVIEW])
    if (path === '/vaktcomply/history') return Promise.resolve({ runs: [], total: 0 })
    if (path.startsWith('/vaktcomply/pentests')) return Promise.resolve([PENTEST])
    if (path === '/vaktcomply/protection-needs/assessments') return Promise.resolve([PNA])
    if (path === '/vaktcomply/questionnaires/q-1') return Promise.resolve(QUESTIONNAIRE)
    if (path.startsWith('/vaktcomply/bcm/recovery-plans')) return Promise.resolve([PLAN])
    if (path === '/vaktcomply/resilience-tests') {
      return Promise.resolve({ tlpt_overdue_warning: false, tests: [RTEST] })
    }
    return Promise.resolve([])
  },
  registerUnauthorizedHandler: vi.fn(),
  FeatureLockedError: class extends Error {},
}))

function renderAt(element: React.ReactElement, path = '/x', routePath = '/x') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path={routePath} element={element} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const btn = (name: string | RegExp) => screen.queryAllByRole('button', { name })
const hint = () => screen.queryByRole('note')

afterEach(cleanup)

describe('Schreibknöpfe nach Rolle (Leser-Katalog C)', () => {
  it('ISMS-Scope: Viewer ohne Speichern/Genehmigen/Ausschluss, Felder gesperrt', async () => {
    signInAs('Viewer')
    renderAt(<ISMSScopePage />)
    const field = await screen.findByDisplayValue(SCOPE.scope_definition)
    expect(btn('Speichern (neue Version)')).toHaveLength(0)
    expect(btn('Genehmigen')).toHaveLength(0)
    expect(btn(i18n.t('vaktcomply.ismsScopePage.addExclusion'))).toHaveLength(0)
    expect(field).toBeDisabled()
    expect(hint()).toHaveTextContent(i18n.t('common.readOnlyHint'))
  })

  it('ISMS-Scope: SecurityAnalyst speichert, darf aber nicht genehmigen (Admin-only)', async () => {
    signInAs('SecurityAnalyst')
    renderAt(<ISMSScopePage />)
    await screen.findByDisplayValue(SCOPE.scope_definition)
    expect(btn('Speichern (neue Version)')).toHaveLength(1)
    expect(btn('Genehmigen')).toHaveLength(0)
    expect(hint()).toBeNull()
  })

  it('ISMS-Scope: Admin mit Speichern, Genehmigen und Ausschluss', async () => {
    signInAs('Admin')
    renderAt(<ISMSScopePage />)
    const field = await screen.findByDisplayValue(SCOPE.scope_definition)
    expect(btn('Speichern (neue Version)')).toHaveLength(1)
    expect(btn('Genehmigen')).toHaveLength(1)
    expect(btn(i18n.t('vaktcomply.ismsScopePage.addExclusion'))).toHaveLength(1)
    expect(field).toBeEnabled()
  })

  it('Interessierte Parteien: Viewer ohne Anlegen/Bearbeiten/Löschen, Export bleibt; Admin mit', async () => {
    signInAs('Viewer')
    renderAt(<InterestedPartiesPage />)
    await screen.findByText(PARTY.name)
    expect(btn(i18n.t('vaktcomply.interestedParties.addParty'))).toHaveLength(0)
    expect(btn(i18n.t('common.edit'))).toHaveLength(0)
    expect(btn(i18n.t('common.delete'))).toHaveLength(0)
    expect(btn(i18n.t('vaktcomply.interestedParties.exportPdf'))).toHaveLength(1)
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt(<InterestedPartiesPage />)
    await screen.findByText(PARTY.name)
    expect(btn(i18n.t('vaktcomply.interestedParties.addParty'))).toHaveLength(1)
    expect(btn(i18n.t('common.edit'))).toHaveLength(1)
    expect(btn(i18n.t('common.delete'))).toHaveLength(1)
    expect(hint()).toBeNull()
  })

  it('Managementbewertung: Viewer ohne Neu/Freigeben, PDF-Export bleibt', async () => {
    signInAs('Viewer')
    renderAt(<ManagementReviewsPage />)
    await screen.findByText(REVIEW.review_date)
    expect(btn(i18n.t('vaktcomply.managementReviews.newReview'))).toHaveLength(0)
    expect(btn(i18n.t('vaktcomply.managementReviews.approve'))).toHaveLength(0)
    expect(btn(i18n.t('vaktcomply.managementReviews.exportPdf'))).toHaveLength(1)
    expect(hint()).toBeInTheDocument()
  })

  it('Managementbewertung: Admin legt an, gibt aber nicht frei (Vier-Augen)', async () => {
    signInAs('Admin')
    renderAt(<ManagementReviewsPage />)
    await screen.findByText(REVIEW.review_date)
    expect(btn(i18n.t('vaktcomply.managementReviews.newReview'))).toHaveLength(1)
    expect(btn(i18n.t('vaktcomply.managementReviews.approve'))).toHaveLength(0)
  })

  it('Managementbewertung: InternalAuditor gibt frei, legt aber nicht an', async () => {
    signInAs('InternalAuditor')
    renderAt(<ManagementReviewsPage />)
    await screen.findByText(REVIEW.review_date)
    expect(btn(i18n.t('vaktcomply.managementReviews.approve'))).toHaveLength(1)
    expect(btn(i18n.t('vaktcomply.managementReviews.newReview'))).toHaveLength(0)
  })

  it('NIS2-Neubewertung: Viewer ohne „Neu bewerten"/„Erste Bewertung", Admin mit', async () => {
    signInAs('Viewer')
    renderAt(<NIS2ReassessmentPage />)
    await screen.findByText(i18n.t('nis2Reassessment.emptyTitle'))
    expect(btn(i18n.t('nis2Reassessment.btnReassess'))).toHaveLength(0)
    expect(btn(i18n.t('nis2Reassessment.btnStartFirst'))).toHaveLength(0)
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt(<NIS2ReassessmentPage />)
    await screen.findByText(i18n.t('nis2Reassessment.emptyTitle'))
    expect(btn(i18n.t('nis2Reassessment.btnReassess'))).toHaveLength(1)
    expect(btn(i18n.t('nis2Reassessment.btnStartFirst'))).toHaveLength(1)
  })

  it('Pentests: Viewer ohne Anlegen/Bearbeiten/Löschen, Admin mit', async () => {
    signInAs('Viewer')
    renderAt(<PentestsPage />)
    await screen.findByText(PENTEST.title)
    expect(btn(i18n.t('pentests.newPentest'))).toHaveLength(0)
    expect(btn(i18n.t('common.edit'))).toHaveLength(0)
    expect(btn(i18n.t('common.delete'))).toHaveLength(0)
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt(<PentestsPage />)
    await screen.findByText(PENTEST.title)
    expect(btn(i18n.t('pentests.newPentest'))).toHaveLength(1)
    expect(btn(i18n.t('common.edit'))).toHaveLength(1)
    expect(btn(i18n.t('common.delete'))).toHaveLength(1)
  })

  it('Schutzbedarf: Viewer ohne Anlegen/Bearbeiten/Abschließen/Löschen, Admin mit', async () => {
    signInAs('Viewer')
    renderAt(<ProtectionNeedsPage />)
    await screen.findByText(PNA.name)
    expect(btn(i18n.t('protectionNeeds.new'))).toHaveLength(0)
    expect(btn(i18n.t('common.edit'))).toHaveLength(0)
    expect(btn(i18n.t('protectionNeeds.finalize'))).toHaveLength(0)
    expect(btn(i18n.t('common.delete'))).toHaveLength(0)
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt(<ProtectionNeedsPage />)
    await screen.findByText(PNA.name)
    expect(btn(i18n.t('protectionNeeds.new'))).toHaveLength(1)
    expect(btn(i18n.t('common.edit'))).toHaveLength(1)
    expect(btn(i18n.t('protectionNeeds.finalize'))).toHaveLength(1)
    expect(btn(i18n.t('common.delete'))).toHaveLength(1)
  })

  it('Fragebogen: Viewer ohne „Frage hinzufügen" und ohne Umsortieren, Admin mit', async () => {
    signInAs('Viewer')
    renderAt(<QuestionnairePage />, '/q/q-1', '/q/:id')
    await screen.findByText(QUESTIONNAIRE.questions[0].question_text)
    expect(btn(i18n.t('vaktcomply.questionnaire.addQuestion'))).toHaveLength(0)
    expect(screen.queryAllByTestId('drag-handle')).toHaveLength(0)
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt(<QuestionnairePage />, '/q/q-1', '/q/:id')
    await screen.findByText(QUESTIONNAIRE.questions[0].question_text)
    expect(btn(i18n.t('vaktcomply.questionnaire.addQuestion'))).toHaveLength(1)
    expect(screen.getAllByTestId('drag-handle')).toHaveLength(1)
  })

  it('Wiederanlaufpläne: Viewer ohne Anlegen/Bearbeiten/Löschen, Admin mit', async () => {
    signInAs('Viewer')
    renderAt(<RecoveryPlansPage />)
    await screen.findByText(PLAN.title)
    expect(btn(i18n.t('bcm.recoveryPlans.new'))).toHaveLength(0)
    expect(btn(i18n.t('common.edit'))).toHaveLength(0)
    expect(btn(i18n.t('common.delete'))).toHaveLength(0)
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt(<RecoveryPlansPage />)
    await screen.findByText(PLAN.title)
    expect(btn(i18n.t('bcm.recoveryPlans.new'))).toHaveLength(1)
    expect(btn(i18n.t('common.edit'))).toHaveLength(1)
    expect(btn(i18n.t('common.delete'))).toHaveLength(1)
  })

  it('Resilienztests: AuditorReadOnly ohne Anlegen/Bearbeiten/Nachweis/Löschen, Admin mit', async () => {
    signInAs('AuditorReadOnly')
    renderAt(<ResilienceTestsPage />)
    await screen.findByText(/SecTech/)
    expect(btn(i18n.t('vaktcomply.resilienceTests.newTest'))).toHaveLength(0)
    expect(btn(i18n.t('vaktcomply.resilienceTests.editTitle'))).toHaveLength(0)
    expect(btn(i18n.t('vaktcomply.resilienceTests.linkEvidenceTitle'))).toHaveLength(0)
    expect(btn(i18n.t('vaktcomply.resilienceTests.deleteTitle'))).toHaveLength(0)
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt(<ResilienceTestsPage />)
    await screen.findByText(/SecTech/)
    expect(btn(i18n.t('vaktcomply.resilienceTests.newTest'))).toHaveLength(1)
    expect(btn(i18n.t('vaktcomply.resilienceTests.editTitle'))).toHaveLength(1)
    expect(btn(i18n.t('vaktcomply.resilienceTests.linkEvidenceTitle'))).toHaveLength(1)
    expect(btn(i18n.t('vaktcomply.resilienceTests.deleteTitle'))).toHaveLength(1)
  })
})
