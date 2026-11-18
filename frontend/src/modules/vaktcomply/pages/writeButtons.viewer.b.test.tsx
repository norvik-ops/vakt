/**
 * Leser (Viewer, AuditorReadOnly) haben reinen Lesezugriff: Auf diesen Seiten
 * sind alle Schreibrouten `rw` (Admin/SecurityAnalyst) in vaktcomply/routes.go,
 * nur das Löschen von Ausnahmen ist Admin-only. Je Seite zwei Fälle: Leser ohne
 * Knopf (mit Lesehinweis), Admin mit Knopf — die Gegenprobe verhindert, dass der
 * Test nur grün ist, weil der Knopf gar nicht gerendert wird.
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import { signInAs } from '../../../test-utils/signInAs'
import BSITargetObjectsPage from './BSITargetObjectsPage'
import BackupEvidencePage from './BackupEvidencePage'
import CCMPage from './CCMPage'
import CertificationTimelinePage from './CertificationTimelinePage'
import CryptoKeysPage from './CryptoKeysPage'
import DORAThirdPartiesPage from './DORAThirdPartiesPage'
import EmergencyContactsPage from './EmergencyContactsPage'
import ExceptionsPage from './ExceptionsPage'

const TS = '2026-11-17T10:00:00Z'

const BSI_OBJECT = {
  id: 'b-1', org_id: 'o-1', name: 'Fileserver FS01', type: 'it_system', absicherungsniveau: 'standard',
  created_at: TS, updated_at: TS,
}
const BACKUP_JOB = {
  id: 'j-1', org_id: 'o-1', name: 'Postgres Nightly', source: 'db', destination: 'storage',
  frequency: 'daily', encrypted: true, last_success_at: TS, last_status: 'success',
  restore_max_age_days: 365, notes: '', last_restore_test_at: '2026-11-01',
  backup_status: 'on_track', restore_status: 'on_track', created_at: TS, updated_at: TS,
}
const CCM_CHECK = {
  id: 'cc-1', org_id: 'o-1', control_id: 'ctl-1', name: 'API Health Check', check_type: 'http',
  config: {}, interval_hours: 24, enabled: true, created_at: TS, updated_at: TS,
}
const MILESTONE = {
  id: 'm-1', org_id: 'o-1', title: 'Zertifizierungsaudit', milestone_date: '2026-12-01',
  milestone_type: 'external_audit', status: 'upcoming', days_remaining: 14,
  created_at: TS, updated_at: TS,
}
const CRYPTO_KEY = {
  id: 'k-1', org_id: 'o-1', name: 'TLS Wildcard', key_type: 'tls_certificate', algorithm: 'RSA',
  key_length: 4096, purpose: 'Webserver', is_weak_algorithm: false, rotation_status: 'ok',
  created_at: TS, updated_at: TS,
}
const DORA_TP = {
  id: 'd-1', org_id: 'o-1', name: 'Cloud Hoster AG', service_type: 'cloud', criticality: 'kritisch',
  has_subcontractors: false, data_location: 'EU', exit_strategy: true, created_at: TS, updated_at: TS,
}
const CONTACT = {
  id: 'ec-1', org_id: 'o-1', name: 'Max Mustermann', role: 'CISO', phone: '+49 228 12345',
  email: 'max@example.com', escalation_level: 1, available_24_7: true, notes: '',
  created_at: TS, updated_at: TS,
}
const EXCEPTION = {
  id: 'x-1', org_id: 'o-1', control_id: 'ctl-1', title: 'Altsystem ohne MFA', reason: 'Hersteller',
  risk_accepted: 'gering', status: 'active', created_at: TS, updated_at: TS,
}

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => {
    if (path === '/vaktcomply/bsi/target-objects') return Promise.resolve([BSI_OBJECT])
    if (path === '/vaktcomply/backup/jobs') return Promise.resolve([BACKUP_JOB])
    if (path === '/vaktcomply/backup/summary') {
      return Promise.resolve({ total_jobs: 1, tested_jobs: 1, overdue_backups: 0, overdue_restores: 0 })
    }
    if (path === '/vaktcomply/ccm/checks') return Promise.resolve([CCM_CHECK])
    if (path.startsWith('/vaktcomply/milestones')) return Promise.resolve([MILESTONE])
    if (path === '/vaktcomply/crypto-keys') return Promise.resolve([CRYPTO_KEY])
    if (path.startsWith('/vaktcomply/dora/third-parties')) return Promise.resolve([DORA_TP])
    if (path === '/vaktcomply/bcm/emergency-contacts') return Promise.resolve([CONTACT])
    if (path === '/vaktcomply/exceptions') return Promise.resolve([EXCEPTION])
    return Promise.resolve([])
  },
  registerUnauthorizedHandler: vi.fn(),
  FeatureLockedError: class extends Error {},
}))

function renderPage(element: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>{element}</MemoryRouter>
    </QueryClientProvider>,
  )
}

const btn = (name: string | RegExp) => screen.queryByRole('button', { name })
const hint = () => screen.queryByRole('note')

afterEach(cleanup)

describe('Schreibknöpfe nach Rolle (Leser ohne Schreibzugriff, Teil b)', () => {
  it('BSI-Zielobjekte: Viewer ohne Anlegen/Override/Löschen, Abhängigkeiten bleiben; Admin mit', async () => {
    signInAs('Viewer')
    renderPage(<BSITargetObjectsPage />)
    await screen.findByText(BSI_OBJECT.name)
    expect(btn(i18n.t('bsi.targetObjects.addButton'))).toBeNull()
    expect(btn(i18n.t('bsi.targetObjects.overrideButton'))).toBeNull()
    expect(btn(i18n.t('common.delete'))).toBeNull()
    expect(btn(i18n.t('bsi.targetObjects.depsButton'))).toBeInTheDocument()
    expect(hint()).toHaveTextContent(i18n.t('common.readOnlyHint'))
    cleanup()

    signInAs('Admin')
    renderPage(<BSITargetObjectsPage />)
    await screen.findByText(BSI_OBJECT.name)
    expect(btn(i18n.t('bsi.targetObjects.addButton'))).toBeInTheDocument()
    expect(btn(i18n.t('bsi.targetObjects.overrideButton'))).toBeInTheDocument()
    expect(btn(i18n.t('common.delete'))).toBeInTheDocument()
    expect(hint()).toBeNull()
  })

  it('Backup-Nachweise: Viewer ohne Anlegen/Bearbeiten/Löschen/Wiederherstellungstest; Admin mit', async () => {
    signInAs('Viewer')
    renderPage(<BackupEvidencePage />)
    await screen.findByText(BACKUP_JOB.name)
    expect(btn(i18n.t('backup.new'))).toBeNull()
    expect(btn(i18n.t('common.edit'))).toBeNull()
    expect(btn(i18n.t('common.delete'))).toBeNull()
    expect(btn(i18n.t('backup.restoreTest.document'))).toBeNull()
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderPage(<BackupEvidencePage />)
    await screen.findByText(BACKUP_JOB.name)
    expect(btn(i18n.t('backup.new'))).toBeInTheDocument()
    expect(btn(i18n.t('common.edit'))).toBeInTheDocument()
    expect(btn(i18n.t('common.delete'))).toBeInTheDocument()
    expect(btn(i18n.t('backup.restoreTest.document'))).toBeInTheDocument()
  })

  it('CCM: Viewer ohne Anlegen/Ausführen/Ein-Aus/Löschen, Ergebnisse bleiben; Admin mit', async () => {
    signInAs('AuditorReadOnly')
    renderPage(<CCMPage />)
    await screen.findByText(CCM_CHECK.name)
    expect(btn('Neuen Check anlegen')).toBeNull()
    expect(btn('Jetzt ausführen')).toBeNull()
    expect(btn('Aus')).toBeNull()
    expect(btn('Check löschen')).toBeNull()
    expect(btn('Ergebnisse anzeigen')).toBeInTheDocument()
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderPage(<CCMPage />)
    await screen.findByText(CCM_CHECK.name)
    expect(btn('Neuen Check anlegen')).toBeInTheDocument()
    expect(btn('Jetzt ausführen')).toBeInTheDocument()
    expect(btn('Aus')).toBeInTheDocument()
    expect(btn('Check löschen')).toBeInTheDocument()
  })

  it('Zertifizierungs-Zeitplan: Viewer ohne Anlegen/Erledigt/Löschen; Admin mit', async () => {
    signInAs('Viewer')
    renderPage(<CertificationTimelinePage />)
    await screen.findAllByText(MILESTONE.title)
    expect(btn(i18n.t('certificationTimeline.addMilestone'))).toBeNull()
    expect(btn(i18n.t('certificationTimeline.markDone'))).toBeNull()
    expect(btn(i18n.t('certificationTimeline.done'))).toBeNull()
    expect(btn(i18n.t('common.delete'))).toBeNull()
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderPage(<CertificationTimelinePage />)
    await screen.findAllByText(MILESTONE.title)
    expect(btn(i18n.t('certificationTimeline.addMilestone'))).toBeInTheDocument()
    expect(btn(i18n.t('certificationTimeline.markDone'))).toBeInTheDocument()
    expect(btn(i18n.t('certificationTimeline.done'))).toBeInTheDocument()
    expect(btn(i18n.t('common.delete'))).toBeInTheDocument()
  })

  it('Krypto-Schlüssel: Viewer ohne Anlegen/Rotieren/Löschen; Admin mit', async () => {
    signInAs('Viewer')
    renderPage(<CryptoKeysPage />)
    await screen.findByText(CRYPTO_KEY.name)
    expect(screen.queryByTestId('add-key-btn')).toBeNull()
    expect(screen.queryByTestId('rotate-btn-k-1')).toBeNull()
    expect(screen.queryByTestId('delete-btn-k-1')).toBeNull()
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderPage(<CryptoKeysPage />)
    await screen.findByText(CRYPTO_KEY.name)
    expect(screen.getByTestId('add-key-btn')).toBeInTheDocument()
    expect(screen.getByTestId('rotate-btn-k-1')).toBeInTheDocument()
    expect(screen.getByTestId('delete-btn-k-1')).toBeInTheDocument()
  })

  it('DORA-Drittparteien: Viewer ohne Anlegen/Bearbeiten/Löschen; Admin mit', async () => {
    signInAs('Viewer')
    renderPage(<DORAThirdPartiesPage />)
    await screen.findByText(DORA_TP.name)
    expect(btn(i18n.t('vaktcomply.doraThirdParties.addProvider'))).toBeNull()
    expect(btn(i18n.t('vaktcomply.doraThirdParties.titleEdit'))).toBeNull()
    expect(btn(i18n.t('vaktcomply.doraThirdParties.titleDelete'))).toBeNull()
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderPage(<DORAThirdPartiesPage />)
    await screen.findByText(DORA_TP.name)
    expect(btn(i18n.t('vaktcomply.doraThirdParties.addProvider'))).toBeInTheDocument()
    expect(btn(i18n.t('vaktcomply.doraThirdParties.titleEdit'))).toBeInTheDocument()
    expect(btn(i18n.t('vaktcomply.doraThirdParties.titleDelete'))).toBeInTheDocument()
  })

  it('Notfallkontakte: Viewer ohne Anlegen/Bearbeiten/Löschen, Telefonlink bleibt; Admin mit', async () => {
    signInAs('Viewer')
    renderPage(<EmergencyContactsPage />)
    await screen.findByText(CONTACT.name)
    expect(btn(i18n.t('bcm.emergencyContacts.new'))).toBeNull()
    expect(btn(i18n.t('common.edit'))).toBeNull()
    expect(btn(i18n.t('common.delete'))).toBeNull()
    expect(screen.getByRole('link', { name: /\+49 228 12345/ })).toBeInTheDocument()
    expect(hint()).toBeInTheDocument()
    cleanup()

    signInAs('Admin')
    renderPage(<EmergencyContactsPage />)
    await screen.findByText(CONTACT.name)
    expect(btn(i18n.t('bcm.emergencyContacts.new'))).toBeInTheDocument()
    expect(btn(i18n.t('common.edit'))).toBeInTheDocument()
    expect(btn(i18n.t('common.delete'))).toBeInTheDocument()
  })

  it('Ausnahmen: Löschen nur für Admin — Viewer und SecurityAnalyst ohne, Admin mit', async () => {
    const deleteName = i18n.t('vaktcomply.exceptions.delete')

    signInAs('Viewer')
    renderPage(<ExceptionsPage />)
    await screen.findByText(EXCEPTION.title)
    expect(btn(deleteName)).toBeNull()
    expect(btn(i18n.t('vaktcomply.exceptions.showControl'))).toBeInTheDocument()
    expect(hint()).toBeInTheDocument()
    cleanup()

    // DELETE /exceptions/:id ist `admin` — die Schreibrolle SecurityAnalyst darf es nicht.
    signInAs('SecurityAnalyst')
    renderPage(<ExceptionsPage />)
    await screen.findByText(EXCEPTION.title)
    expect(btn(deleteName)).toBeNull()
    expect(hint()).toBeNull()
    cleanup()

    signInAs('Admin')
    renderPage(<ExceptionsPage />)
    await screen.findByText(EXCEPTION.title)
    expect(btn(deleteName)).toBeInTheDocument()
  })
})
