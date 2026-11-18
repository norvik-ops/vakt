/**
 * Leser (Viewer, AuditorReadOnly) haben reinen Lesezugriff — auch kein Kommentieren.
 * Je Komponente: Leser ohne Schreibknopf, schreibende Rolle mit Knopf (Gegenprobe,
 * damit der Test nicht nur grün ist, weil der Knopf gar nicht gerendert wird).
 *
 * Backend-Rollen (nur gelesen):
 * - CommentThread, ThreatCatalogDialog: `rw` (Admin/SecurityAnalyst) in vaktcomply/routes.go.
 * - Integrations-Tabs: /integrations/cloud und /integrations/github sind komplett
 *   `auth.RequireRole("Admin")` (cmd/api/routes.go) — deshalb zusätzlich
 *   SecurityAnalyst ohne Knopf.
 *
 * Der ReadOnlyHint steht auf den umgebenden Seiten, nicht in diesen Komponenten.
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import { signInAs } from '../../../test-utils/signInAs'
import { CommentThread } from './CommentThread'
import { ThreatCatalogDialog } from './ThreatCatalogDialog'
import { AWSTab, IONOSTab } from '../integrations/CloudProviderTabs'
import { GitLabTab } from '../integrations/DevOpsTabs'
import { EntraIDTab } from '../integrations/IdentityTabs'
import { GitHubTab } from '../integrations/GitHubTab'

const COMMENT = {
  id: 'cm-1', entity_type: 'risk', entity_id: 'r-1', body: 'Bitte bis Freitag prüfen',
  author_email: 'isb@test.local', created_at: '2026-11-17T10:00:00Z',
}
const CATALOG_ITEM = {
  id: 't-1', title: 'Ransomware auf Fileserver', scenario: 'Verschlüsselung durch Schadsoftware',
  category: 'Malware', frameworks: ['ISO27001'], asset_types: ['server'], cia: ['availability'],
  default_likelihood: 3, default_impact: 4,
}
const CLOUD_STATUS = {
  last_sync_at: '2026-11-17T10:00:00Z', last_sync_status: 'success', last_sync_error: null, evidence_count: 2,
}
const GITHUB = {
  id: 'g-1', org_id: 'o-1', repo_owner: 'norvik', repo_name: 'vakt', last_synced_at: null,
  sync_status: 'pending', created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
}

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => {
    if (path.endsWith('/comments')) return Promise.resolve([COMMENT])
    if (path.startsWith('/vaktcomply/threat-catalog')) return Promise.resolve([CATALOG_ITEM])
    if (path.startsWith('/integrations/cloud/') && path.endsWith('/config')) {
      return Promise.resolve({ is_configured: true })
    }
    if (path.startsWith('/integrations/cloud/') && path.endsWith('/status')) return Promise.resolve(CLOUD_STATUS)
    if (path === '/integrations/github') return Promise.resolve([GITHUB])
    return Promise.resolve([])
  },
  registerUnauthorizedHandler: vi.fn(),
  FeatureLockedError: class extends Error {},
}))

function renderUI(element: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>{element}</MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(cleanup)

const SYNC = 'Jetzt synchronisieren'

describe('CommentThread: Kommentieren nur für Schreibrollen', () => {
  it.each(['Viewer', 'AuditorReadOnly'] as const)('%s sieht Verlauf, aber kein Formular und kein Löschen', async (role) => {
    signInAs(role)
    renderUI(<CommentThread entityType="risk" entityId="r-1" />)
    await screen.findByText(COMMENT.body)
    expect(screen.queryByRole('button', { name: 'Kommentar senden' })).toBeNull()
    expect(screen.queryByPlaceholderText('Kommentar schreiben …')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Kommentar löschen' })).toBeNull()
  })

  it('Admin sieht Formular und Löschen', async () => {
    signInAs('Admin')
    renderUI(<CommentThread entityType="risk" entityId="r-1" />)
    await screen.findByText(COMMENT.body)
    expect(screen.getByRole('button', { name: 'Kommentar senden' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Kommentar löschen' })).toBeInTheDocument()
  })
})

describe('ThreatCatalogDialog: „Risiko aus Katalog anlegen“ nur für Schreibrollen', () => {
  it('Viewer sieht den Katalog ohne Anlegen-Knopf', async () => {
    signInAs('Viewer')
    renderUI(<ThreatCatalogDialog open onClose={() => undefined} />)
    await screen.findByText(CATALOG_ITEM.title)
    expect(screen.queryByRole('button', { name: i18n.t('threatCatalog.createRisk') })).toBeNull()
  })

  it('SecurityAnalyst sieht den Anlegen-Knopf', async () => {
    signInAs('SecurityAnalyst')
    renderUI(<ThreatCatalogDialog open onClose={() => undefined} />)
    await screen.findByText(CATALOG_ITEM.title)
    expect(screen.getByRole('button', { name: i18n.t('threatCatalog.createRisk') })).toBeInTheDocument()
  })
})

const CONFIG_TABS = [
  ['AWS', <AWSTab key="aws" />],
  ['IONOS', <IONOSTab key="ionos" />],
  ['GitLab', <GitLabTab key="gitlab" />],
  ['Entra ID', <EntraIDTab key="entra" />],
] as const

describe('Integrations-Tabs: Sync und Zugangsdaten nur für Admin', () => {
  it.each(CONFIG_TABS)('%s: Viewer und SecurityAnalyst ohne Sync und Speichern', async (_name, tab) => {
    for (const role of ['Viewer', 'SecurityAnalyst'] as const) {
      signInAs(role)
      renderUI(tab)
      await screen.findByText(/Evidence-Einträge/)
      expect(screen.queryByTitle(SYNC)).toBeNull()
      expect(screen.queryByRole('button', { name: i18n.t('integrations.page.save') })).toBeNull()
      cleanup()
    }
  })

  it.each(CONFIG_TABS)('%s: Admin mit Sync und Speichern', async (_name, tab) => {
    signInAs('Admin')
    renderUI(tab)
    await screen.findByText(/Evidence-Einträge/)
    expect(screen.getByTitle(SYNC)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: i18n.t('integrations.page.save') })).toBeInTheDocument()
  })

  it('GitHub: Viewer und SecurityAnalyst ohne Verbinden, Sync und Entfernen', async () => {
    for (const role of ['Viewer', 'SecurityAnalyst'] as const) {
      signInAs(role)
      renderUI(<GitHubTab />)
      await screen.findByText('norvik/vakt')
      expect(screen.queryByRole('button', { name: i18n.t('integrations.page.connectRepo') })).toBeNull()
      expect(screen.queryByTitle(SYNC)).toBeNull()
      expect(screen.queryByTitle('Integration entfernen')).toBeNull()
      // Details ansehen bleibt — lesende Aktion.
      expect(screen.getByTitle('Details anzeigen')).toBeInTheDocument()
      cleanup()
    }
  })

  it('GitHub: Admin mit Verbinden, Sync und Entfernen', async () => {
    signInAs('Admin')
    renderUI(<GitHubTab />)
    await screen.findByText('norvik/vakt')
    expect(screen.getByRole('button', { name: i18n.t('integrations.page.connectRepo') })).toBeInTheDocument()
    expect(screen.getByTitle(SYNC)).toBeInTheDocument()
    expect(screen.getByTitle('Integration entfernen')).toBeInTheDocument()
  })
})
