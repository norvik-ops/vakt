/**
 * Leser (Viewer, AuditorReadOnly) haben reinen Lesezugriff: Schreibknöpfe, die der
 * Server mit 403 ablehnen würde, erscheinen gar nicht erst. Je Seite: Leser ohne
 * Knopf + ReadOnlyHint, Gegenprobe mit der Rolle, die die Route wirklich erlaubt
 * (`rw` = Admin/SecurityAnalyst, `admin` = nur Admin, `internalAuditor`) — sonst
 * wäre der Test auch grün, wenn der Knopf nie gerendert würde.
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import { signInAs } from '../../../test-utils/signInAs'
import AIDocumentationPage from './AIDocumentationPage'
import AISystemsPage from './AISystemsPage'
import AccessReviewsPage from './AccessReviewsPage'
import ApprovalsPage from './ApprovalsPage'
import AuditProgramPage from './AuditProgramPage'
import BCPPage from './BCPPage'
import BIAPage from './BIAPage'
import BSICheckSheetPage from './BSICheckSheetPage'
import BSIModelingPage from './BSIModelingPage'

const TS = '2026-11-17T10:00:00Z'
const AI_SYSTEM = {
  id: 'ai-1', org_id: 'o-1', name: 'Bewerber-Vorauswahl', autonomy_level: 'assistive',
  status: 'under_review', created_at: TS, updated_at: TS,
}
const AI_DOC = { id: 'd-1', system_description: 'Beschreibung', status: 'draft', version: 1, created_at: TS }
const CAMPAIGN = {
  id: 'ar-1', org_id: 'o-1', title: 'Quartalsprüfung Q4', reviewer_email: 'isb@test.local',
  status: 'active', created_at: TS, updated_at: TS,
}
const REVIEW_ITEM = {
  id: 'it-1', campaign_id: 'ar-1', user_email: 'max@test.local', access_level: 'Admin',
  decision: 'pending', created_at: TS, updated_at: TS,
}
const APPROVAL = {
  id: 'ap-1', control_ref: 'A.5.1', control_title: 'Richtlinien', requester_email: 'sa@test.local',
  current_status: 'missing', requested_status: 'implemented', comment: '', created_at: TS,
}
const AUDIT = {
  id: 'pa-1', org_id: 'o-1', title: 'Internes ISMS-Audit 2026', audit_type: 'isms_internal',
  scope: 'ISMS', planned_date: '2026-12-01', status: 'planned', created_at: TS, updated_at: TS,
}
const BCP_PLAN = {
  id: 'b-1', org_id: 'o-1', title: 'Notfallplan Rechenzentrum', status: 'active', version: 1,
  created_at: TS, updated_at: TS,
}
const BIA_PROCESS = {
  id: 'p-1', org_id: 'o-1', name: 'Auftragsabwicklung', description: '', process_owner: '',
  criticality: 'high', schutzbedarfsklasse: 2, rto_hours: 4, rpo_hours: 1, mbco_percent: 50,
  dependencies: [], created_at: TS, updated_at: TS,
}
const TARGET_OBJECT = { id: 'to-1', name: 'Fileserver', absicherungsniveau: 'standard' }
const CHECK_ROW = {
  id: 'c-1', org_id: 'o-1', target_object_id: 'to-1', anforderung_id: 'SYS.1.1.A1',
  anforderung_title: 'Zugriffsschutz', baustein_id: 'SYS.1.1', requirement_level: 'basis',
  umsetzungsstatus: 'teilweise', updated_at: TS,
}
const MODELING_ENTRY = {
  id: 'm-1', org_id: 'o-1', asset_id: 'as-1', control_id: 'ct-1', priority: 'R1',
  justification_for_exclusion: '', interview_notes: '', site_visit_notes: '',
  asset_name: 'Fileserver', control_title: 'SYS.1.1 Allgemeiner Server', framework_id: 'bsi',
  created_by: 'u', created_at: TS, updated_at: TS,
}

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => {
    if (path === '/vaktcomply/ai-systems/ai-1') return Promise.resolve(AI_SYSTEM)
    if (path === '/vaktcomply/ai-systems/ai-1/documentation') return Promise.resolve(AI_DOC)
    if (path.startsWith('/vaktcomply/ai-systems')) {
      return Promise.resolve(path.includes('/documentation/versions') ? [] : [AI_SYSTEM])
    }
    if (path === '/vaktcomply/access-reviews') return Promise.resolve([CAMPAIGN])
    if (path === '/vaktcomply/access-reviews/ar-1/items') return Promise.resolve([REVIEW_ITEM])
    if (path === '/vaktcomply/approvals') return Promise.resolve([APPROVAL])
    if (path === '/vaktcomply/audit-program/summary') {
      return Promise.resolve({ audits_planned_this_year: 1, audits_completed: 0, open_findings: 0, overdue_capas_from_audits: 0 })
    }
    if (path === '/vaktcomply/audit-program') return Promise.resolve([AUDIT])
    if (path === '/vaktcomply/bcp/plans') return Promise.resolve([BCP_PLAN])
    if (path === '/vaktcomply/bia/processes') return Promise.resolve([BIA_PROCESS])
    if (path === '/vaktcomply/bsi/target-objects/to-1') return Promise.resolve(TARGET_OBJECT)
    if (path === '/vaktcomply/bsi/target-objects/to-1/check') return Promise.resolve([CHECK_ROW])
    if (path === '/vaktcomply/bsi/target-objects/to-1/check/summary') {
      return Promise.resolve({ umsetzungsgrad_pct: 0, ja: 0, teilweise: 1, entbehrlich: 0, nein: 0, total: 1 })
    }
    if (path === '/vaktcomply/bsi-modeling') return Promise.resolve([MODELING_ENTRY])
    return Promise.resolve([])
  },
  registerUnauthorizedHandler: vi.fn(),
  FeatureLockedError: class extends Error {},
}))

function renderAt(path: string, routePath: string, element: React.ReactElement) {
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

const readOnlyNote = () => screen.queryByRole('note')

afterEach(cleanup)

describe('Schreibknöpfe nach Rolle — Seiten A (AI … BSI)', () => {
  it('KI-Dossier: Viewer ohne Speichern/Finalisieren, Felder gesperrt, PDF-Export bleibt; Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/ai-systems/ai-1/documentation', '/ai-systems/:id/documentation', <AIDocumentationPage />)
    await screen.findByDisplayValue(AI_DOC.system_description)
    expect(screen.queryByTestId('save-draft-btn')).toBeNull()
    expect(screen.queryByTestId('save-final-btn')).toBeNull()
    expect(screen.queryByTestId('finalize-doc-btn')).toBeNull()
    expect(screen.getByTestId('export-pdf-btn')).toBeInTheDocument()
    expect(screen.getByDisplayValue(AI_DOC.system_description)).toBeDisabled()
    expect(readOnlyNote()).toHaveTextContent(i18n.t('common.readOnlyHint'))
    cleanup()

    signInAs('Admin')
    renderAt('/ai-systems/ai-1/documentation', '/ai-systems/:id/documentation', <AIDocumentationPage />)
    await screen.findByDisplayValue(AI_DOC.system_description)
    expect(screen.getByTestId('save-draft-btn')).toBeInTheDocument()
    expect(screen.getByTestId('finalize-doc-btn')).toBeInTheDocument()
    expect(screen.getByDisplayValue(AI_DOC.system_description)).toBeEnabled()
    expect(readOnlyNote()).toBeNull()
  })

  it('KI-Inventar: Viewer ohne Erfassen/Einstufen/Bearbeiten/Löschen, Dossier-Link bleibt; Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/ai-systems', '/ai-systems', <AISystemsPage />)
    await screen.findByText(AI_SYSTEM.name)
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('vaktcomply.aiSystems.add')) })).toBeNull()
    expect(screen.queryByTestId('classify-ai-system-btn')).toBeNull()
    expect(screen.queryByTestId('delete-ai-system-btn')).toBeNull()
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('vaktcomply.aiSystems.actions.edit')) })).toBeNull()
    expect(screen.getByRole('link', { name: new RegExp(i18n.t('vaktcomply.aiSystems.actions.documentation')) })).toBeInTheDocument()
    expect(readOnlyNote()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt('/ai-systems', '/ai-systems', <AISystemsPage />)
    await screen.findByText(AI_SYSTEM.name)
    expect(screen.getByRole('button', { name: new RegExp(i18n.t('vaktcomply.aiSystems.add')) })).toBeInTheDocument()
    expect(screen.getByTestId('classify-ai-system-btn')).toBeInTheDocument()
    expect(screen.getByTestId('delete-ai-system-btn')).toBeInTheDocument()
  })

  it('Zugriffsprüfungen: Viewer ohne Kampagnen- und Entscheidungsknöpfe', async () => {
    signInAs('Viewer')
    renderAt('/access-reviews', '/access-reviews', <AccessReviewsPage />)
    fireEvent.click(await screen.findByText(CAMPAIGN.title))
    await screen.findByText(REVIEW_ITEM.user_email)
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('vaktcomply.accessReviews.addCampaign')) })).toBeNull()
    expect(screen.queryByTitle('Bestätigen')).toBeNull()
    expect(screen.queryByTitle('Widerrufen')).toBeNull()
    expect(screen.queryByRole('button', { name: /Benutzer hinzufügen/ })).toBeNull()
    expect(readOnlyNote()).toBeInTheDocument()
  })

  it('Zugriffsprüfungen: SecurityAnalyst entscheidet Einträge (`ar`), legt aber keine Kampagne an (`admin`)', async () => {
    signInAs('SecurityAnalyst')
    renderAt('/access-reviews', '/access-reviews', <AccessReviewsPage />)
    fireEvent.click(await screen.findByText(CAMPAIGN.title))
    await screen.findByText(REVIEW_ITEM.user_email)
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('vaktcomply.accessReviews.addCampaign')) })).toBeNull()
    expect(screen.getByTitle('Bestätigen')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Benutzer hinzufügen/ })).toBeInTheDocument()
    expect(readOnlyNote()).toBeNull()
  })

  it('Zugriffsprüfungen: Admin mit „Kampagne anlegen"', async () => {
    signInAs('Admin')
    renderAt('/access-reviews', '/access-reviews', <AccessReviewsPage />)
    await screen.findByText(CAMPAIGN.title)
    expect(screen.getByRole('button', { name: new RegExp(i18n.t('vaktcomply.accessReviews.addCampaign')) })).toBeInTheDocument()
  })

  it('Freigaben (Handler prüft Admin): Viewer und SecurityAnalyst ohne Freigeben, Admin mit', async () => {
    for (const role of ['Viewer', 'SecurityAnalyst'] as const) {
      signInAs(role)
      renderAt('/approvals', '/approvals', <ApprovalsPage />)
      expect(screen.getByText(i18n.t('vaktcomply.approvalsPage.noPermissionTitle'))).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: new RegExp(i18n.t('vaktcomply.approvalsPage.approve')) })).toBeNull()
      cleanup()
    }

    signInAs('Admin')
    renderAt('/approvals', '/approvals', <ApprovalsPage />)
    await screen.findByText(APPROVAL.control_title)
    expect(screen.getByRole('button', { name: new RegExp(i18n.t('vaktcomply.approvalsPage.approve')) })).toBeInTheDocument()
  })

  it('Auditprogramm: Viewer ohne Anlegen/Feststellung/Abschließen; Admin ohne Abschließen; InternalAuditor mit', async () => {
    const createBtn = () => screen.queryByRole('button', { name: new RegExp(i18n.t('vaktcomply.auditProgram.createBtn')) })
    const findingBtn = () => screen.queryByRole('button', { name: new RegExp(i18n.t('vaktcomply.auditProgram.findingBtn')) })
    const completeBtn = () => screen.queryByRole('button', { name: new RegExp(i18n.t('vaktcomply.auditProgram.completeBtn')) })

    signInAs('Viewer')
    renderAt('/audit-program', '/audit-program', <AuditProgramPage />)
    await screen.findByText(AUDIT.title)
    expect(createBtn()).toBeNull()
    expect(findingBtn()).toBeNull()
    expect(completeBtn()).toBeNull()
    expect(readOnlyNote()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt('/audit-program', '/audit-program', <AuditProgramPage />)
    await screen.findByText(AUDIT.title)
    expect(createBtn()).toBeInTheDocument()
    expect(findingBtn()).toBeInTheDocument()
    expect(completeBtn()).toBeNull()
    cleanup()

    signInAs('SecurityAnalyst')
    renderAt('/audit-program', '/audit-program', <AuditProgramPage />)
    await screen.findByText(AUDIT.title)
    expect(completeBtn()).toBeNull()
    cleanup()

    signInAs('InternalAuditor')
    renderAt('/audit-program', '/audit-program', <AuditProgramPage />)
    await screen.findByText(AUDIT.title)
    expect(completeBtn()).toBeInTheDocument()
  })

  it('Notfallpläne: Viewer ohne Neu/Bearbeiten/Löschen/Test erfassen; Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/bcp', '/bcp', <BCPPage />)
    await screen.findByText(BCP_PLAN.title)
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('bcp.newPlan')) })).toBeNull()
    expect(screen.queryByTestId('bcp-edit-btn')).toBeNull()
    expect(screen.queryByTestId('bcp-delete-btn')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: new RegExp(i18n.t('bcp.tests')) }))
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('bcp.addTest')) })).toBeNull()
    expect(readOnlyNote()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt('/bcp', '/bcp', <BCPPage />)
    await screen.findByText(BCP_PLAN.title)
    expect(screen.getByRole('button', { name: new RegExp(i18n.t('bcp.newPlan')) })).toBeInTheDocument()
    expect(screen.getByTestId('bcp-delete-btn')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: new RegExp(i18n.t('bcp.tests')) }))
    expect(screen.getByRole('button', { name: new RegExp(i18n.t('bcp.addTest')) })).toBeInTheDocument()
  })

  it('BIA: Viewer ohne Neu/Bearbeiten/Löschen; Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/bia', '/bia', <BIAPage />)
    await screen.findByText(BIA_PROCESS.name)
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('bcm.bia.new')) })).toBeNull()
    expect(screen.queryByTestId('bia-edit-btn')).toBeNull()
    expect(screen.queryByTestId('bia-delete-btn')).toBeNull()
    expect(readOnlyNote()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt('/bia', '/bia', <BIAPage />)
    await screen.findByText(BIA_PROCESS.name)
    expect(screen.getByRole('button', { name: new RegExp(i18n.t('bcm.bia.new')) })).toBeInTheDocument()
    expect(screen.getByTestId('bia-delete-btn')).toBeInTheDocument()
  })

  it('BSI-Check: Viewer sieht den Status nur als Badge, CSV bleibt; Admin mit Auswahl', async () => {
    signInAs('Viewer')
    renderAt('/bsi/target-objects/to-1/check', '/bsi/target-objects/:id/check', <BSICheckSheetPage />)
    await screen.findByText(CHECK_ROW.anforderung_title)
    expect(screen.getByTestId('bsi-check-status-readonly')).toBeInTheDocument()
    // Einzige Auswahl ist der Stufen-Filter im Kopf (lesend).
    expect(screen.getAllByRole('combobox')).toHaveLength(1)
    expect(screen.getByRole('button', { name: /CSV/ })).toBeInTheDocument()
    expect(readOnlyNote()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt('/bsi/target-objects/to-1/check', '/bsi/target-objects/:id/check', <BSICheckSheetPage />)
    await screen.findByText(CHECK_ROW.anforderung_title)
    expect(screen.queryByTestId('bsi-check-status-readonly')).toBeNull()
    expect(screen.getAllByRole('combobox')).toHaveLength(2)
  })

  it('BSI-Modellierung: Viewer ohne Zuordnen/Mehrere/Löschen; Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/bsi-modeling', '/bsi-modeling', <BSIModelingPage />)
    await screen.findByText(MODELING_ENTRY.control_title)
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('bsiModeling.assignModule')) })).toBeNull()
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('bsiModeling.bulkAssign')) })).toBeNull()
    expect(screen.queryByRole('button', { name: i18n.t('common.delete') })).toBeNull()
    expect(readOnlyNote()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt('/bsi-modeling', '/bsi-modeling', <BSIModelingPage />)
    await screen.findByText(MODELING_ENTRY.control_title)
    expect(screen.getByRole('button', { name: new RegExp(i18n.t('bsiModeling.assignModule')) })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: new RegExp(i18n.t('bsiModeling.bulkAssign')) })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: i18n.t('common.delete') })).toBeInTheDocument()
  })
})
