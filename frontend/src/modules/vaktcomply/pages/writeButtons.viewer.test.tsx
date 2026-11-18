/**
 * R1-KW-44 (Rest aus UX-06): Außerhalb der Kernweg-Seiten zeigten weitere Seiten
 * einem Viewer Schreibknöpfe, die der Server mit 403 ablehnt (alle Routen hier
 * sind `rw` = Admin/SecurityAnalyst in vaktcomply/routes.go). Je Seite zwei Fälle:
 * Viewer ohne Knopf, Admin mit Knopf — die Gegenprobe verhindert, dass der Test
 * nur grün ist, weil der Knopf gar nicht gerendert wird.
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import { signInAs } from '../../../test-utils/signInAs'
import CAPAsPage from './CAPAsPage'
import AuditsPage from './AuditsPage'
import AuditDetailPage from './AuditDetailPage'
import PolicyAcceptancePage from './PolicyAcceptancePage'
import { NIS2StagePanel } from '../components/NIS2StagePanel'

const CAPA = {
  id: 'c-1', org_id: 'o-1', title: 'Firewall-Regeln prüfen', description: '', source_type: 'audit',
  priority: 'high', status: 'open', root_cause: '', action_plan: '', verification_note: '',
  created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
}
const AUDIT = {
  id: 'a-1', org_id: 'o-1', title: 'Internes Audit 2026', scope: 'ISMS', auditor: 'ISB',
  audit_date: '2026-11-01', status: 'planned', findings: '', recommendations: '',
  created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
}
const NIS2_STATUS = {
  is_reportable: false, reporting_stage: 'none', deadlines: {}, completed_stages: [], reports: [],
}

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => {
    if (path.startsWith('/vaktcomply/capas')) {
      return Promise.resolve({ data: [CAPA], pagination: { page: 1, limit: 25, total: 1, total_pages: 1 } })
    }
    if (path === '/vaktcomply/audits') return Promise.resolve([AUDIT])
    if (path === '/vaktcomply/audits/a-1') return Promise.resolve(AUDIT)
    if (path.endsWith('/nis2-status')) return Promise.resolve(NIS2_STATUS)
    if (path === '/vaktcomply/policies/p-1') return Promise.resolve({ id: 'p-1', title: 'Passwortrichtlinie' })
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

afterEach(cleanup)

describe('Schreibknöpfe nach Rolle (R1-KW-44)', () => {
  it('CAPAs: Viewer ohne „Neue Korrekturmaßnahme", Papierkorb und Auswahl', async () => {
    signInAs('Viewer')
    renderAt('/capas', '/capas', <CAPAsPage />)
    await screen.findByText(CAPA.title)
    expect(screen.queryByRole('button', { name: i18n.t('capas.createBtn') })).toBeNull()
    expect(screen.queryByRole('button', { name: i18n.t('capas.deleteAria') })).toBeNull()
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(screen.getByRole('note')).toHaveTextContent(i18n.t('common.readOnlyHint'))
  })

  it('CAPAs: Admin mit Anlegen, Papierkorb und Auswahl', async () => {
    signInAs('Admin')
    renderAt('/capas', '/capas', <CAPAsPage />)
    await screen.findByText(CAPA.title)
    expect(screen.getByRole('button', { name: i18n.t('capas.createBtn') })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: i18n.t('capas.deleteAria') })).toBeInTheDocument()
    expect(screen.getByRole('checkbox')).toBeInTheDocument()
    expect(screen.queryByRole('note')).toBeNull()
  })

  it('Audits: Viewer ohne „Audit anlegen", Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/audits', '/audits', <AuditsPage />)
    await screen.findByText(AUDIT.title)
    expect(screen.queryByRole('button', { name: i18n.t('vaktcomply.auditsPage.createAudit') })).toBeNull()
    expect(screen.getByRole('note')).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderAt('/audits', '/audits', <AuditsPage />)
    await screen.findByText(AUDIT.title)
    expect(screen.getByRole('button', { name: i18n.t('vaktcomply.auditsPage.createAudit') })).toBeInTheDocument()
  })

  it('Audit-Detail: Viewer ohne Speichern/CAPA, Felder gesperrt; Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/audits/a-1', '/audits/:id', <AuditDetailPage />)
    await screen.findByDisplayValue(AUDIT.title)
    expect(screen.queryByRole('button', { name: i18n.t('vaktcomply.auditDetail.save') })).toBeNull()
    expect(screen.queryByRole('button', { name: i18n.t('vaktcomply.auditDetail.createCAPA') })).toBeNull()
    expect(screen.getByDisplayValue(AUDIT.title)).toBeDisabled()
    cleanup()

    signInAs('Admin')
    renderAt('/audits/a-1', '/audits/:id', <AuditDetailPage />)
    await screen.findByDisplayValue(AUDIT.title)
    expect(screen.getByRole('button', { name: i18n.t('vaktcomply.auditDetail.save') })).toBeInTheDocument()
    expect(screen.getByDisplayValue(AUDIT.title)).toBeEnabled()
  })

  it('Akzeptanz-Kampagnen: Viewer ohne „Neue Kampagne", Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/policies/p-1/acceptance', '/policies/:id/acceptance', <PolicyAcceptancePage />)
    await screen.findByText(i18n.t('policyAcceptance.emptyTitle'))
    expect(screen.queryByRole('button', { name: new RegExp(i18n.t('policyAcceptance.newCampaign')) })).toBeNull()
    cleanup()

    signInAs('Admin')
    renderAt('/policies/p-1/acceptance', '/policies/:id/acceptance', <PolicyAcceptancePage />)
    await screen.findByText(i18n.t('policyAcceptance.emptyTitle'))
    expect(screen.getAllByRole('button', { name: new RegExp(i18n.t('policyAcceptance.newCampaign')) }).length).toBeGreaterThan(0)
  })

  it('NIS2-Meldewesen: Viewer ohne „Meldepflicht bewerten", Admin mit', async () => {
    signInAs('Viewer')
    renderAt('/inc', '/inc', <NIS2StagePanel incidentId="i-1" />)
    await screen.findByText(/NIS2/)
    expect(screen.queryByTestId('nis2-assess-btn')).toBeNull()
    cleanup()

    signInAs('Admin')
    renderAt('/inc', '/inc', <NIS2StagePanel incidentId="i-1" />)
    expect(await screen.findByTestId('nis2-assess-btn')).toBeInTheDocument()
  })
})
