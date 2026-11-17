import { render, screen, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, afterEach } from 'vitest'
import { SidebarNav, type SidebarNavProps } from './SidebarNav'
import { SIDEBAR_SECTIONS_KEY, sectionForPath } from './sidebarSections'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

function renderNav(props: Partial<SidebarNavProps> = {}) {
  const all: SidebarNavProps = {
    pathname: '/',
    collapsed: false,
    isUserPro: false,
    isAdminOrOwner: false,
    overdueCount: 0,
    autoEvidenceCount: 0,
    pendingApprovalCount: 0,
    onNavigate: () => undefined,
    ...props,
  }
  return render(
    <MemoryRouter initialEntries={[all.pathname]}>
      <nav aria-label="Hauptnavigation">
        <SidebarNav {...all} />
      </nav>
    </MemoryRouter>,
  )
}

function sectionButton(label: string) {
  return screen.getByRole('button', { name: label })
}

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('SidebarNav — Bereiche statt Modul-Ebene', () => {
  it('zeigt Dashboard, fünf Bereiche und SYSTEM, aber keine Modul-Ebene „Vakt Comply"', () => {
    renderNav()
    expect(screen.getByRole('link', { name: 'nav.dashboard' })).toHaveAttribute('href', '/')
    for (const key of ['frameworks', 'operations', 'documentation', 'thirdParty', 'bcm']) {
      expect(sectionButton(`nav.comply.group.${key}`)).toBeInTheDocument()
    }
    expect(screen.getByText('nav.system')).toBeInTheDocument()
    expect(screen.queryByText('nav.comply.root')).not.toBeInTheDocument()
    expect(screen.queryByText('nav.modules')).not.toBeInTheDocument()
  })

  it('Comply-Übersicht steht direkt unter Dashboard und ist nur auf genau /vaktcomply aktiv', () => {
    const first = renderNav({ pathname: '/vaktcomply' })
    const links = screen.getAllByRole('link')
    expect(links[0]).toHaveAttribute('href', '/')
    expect(links[1]).toHaveAttribute('href', '/vaktcomply')
    expect(links[1]).toHaveTextContent('nav.comply.overview')
    expect(links[1]).toHaveAttribute('aria-current', 'page')
    expect(links[0]).not.toHaveAttribute('aria-current')
    first.unmount()

    renderNav({ pathname: '/vaktcomply/risks' })
    const overview = screen.getAllByRole('link')[1]
    expect(overview).toHaveAttribute('href', '/vaktcomply')
    expect(overview).not.toHaveAttribute('aria-current')
  })

  it('„Übersicht“ ist eindeutig: der Frameworks-Eintrag heißt „Alle Frameworks“', () => {
    renderNav({ pathname: '/vaktcomply/frameworks' })
    expect(screen.getAllByRole('link', { name: 'nav.comply.overview' })).toHaveLength(1)
    expect(screen.getByRole('link', { name: 'nav.comply.allFrameworks' }))
      .toHaveAttribute('href', '/vaktcomply/frameworks')
  })

  it('Icon-Modus: Comply-Übersicht bleibt als Icon-Link mit Tooltip erreichbar', () => {
    renderNav({ collapsed: true })
    const overview = screen.getByRole('link', { name: 'nav.comply.overview' })
    expect(overview).toHaveAttribute('href', '/vaktcomply')
    expect(overview).toHaveAttribute('title', 'nav.comply.overview')
  })

  it('öffnet automatisch den Bereich der aktiven Route, die übrigen bleiben zu', () => {
    renderNav({ pathname: '/vaktcomply/risks/123' })
    const ops = sectionButton('nav.comply.group.operations')
    expect(ops).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('link', { name: 'nav.comply.risks' })).toHaveAttribute('aria-current', 'page')
    expect(sectionButton('nav.comply.group.frameworks')).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('link', { name: 'nav.comply.nis2' })).not.toBeInTheDocument()
  })

  it('aria-controls zeigt auf die Liste des Bereichs', () => {
    renderNav({ pathname: '/vaktcomply/policies' })
    const doc = sectionButton('nav.comply.group.documentation')
    const list = document.getElementById(doc.getAttribute('aria-controls') ?? '')
    expect(list).not.toBeNull()
    expect(within(list as HTMLElement).getByRole('link', { name: 'nav.comply.policies' })).toBeInTheDocument()
  })

  it('Klick klappt einen Bereich auf und wieder zu; aria-expanded folgt', () => {
    renderNav()
    const fw = sectionButton('nav.comply.group.frameworks')
    expect(fw).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(fw)
    expect(fw).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('link', { name: /nav\.comply\.multiFramework/ })).toHaveAttribute('href', '/nis2-check/multi')
    fireEvent.click(fw)
    expect(fw).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('link', { name: /nav\.comply\.multiFramework/ })).not.toBeInTheDocument()
  })

  it('der Bereich der aktiven Route bleibt offen, auch wenn man ihn anklickt', () => {
    renderNav({ pathname: '/vaktcomply/bcm/bia' })
    const bcm = sectionButton('nav.comply.group.bcm')
    fireEvent.click(bcm)
    expect(bcm).toHaveAttribute('aria-expanded', 'true')
  })

  it('merkt manuell geöffnete Bereiche in localStorage', () => {
    const first = renderNav()
    fireEvent.click(sectionButton('nav.comply.group.thirdParty'))
    expect(JSON.parse(localStorage.getItem(SIDEBAR_SECTIONS_KEY) ?? '{}')).toEqual({ 'third-party': true })
    first.unmount()
    renderNav()
    expect(sectionButton('nav.comply.group.thirdParty')).toHaveAttribute('aria-expanded', 'true')
  })

  it('rendert und klappt auch, wenn jeder Storage-Zugriff wirft', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    renderNav({ pathname: '/vaktcomply/assets' })
    expect(sectionButton('nav.comply.group.operations')).toHaveAttribute('aria-expanded', 'true')
    const fw = sectionButton('nav.comply.group.frameworks')
    fireEvent.click(fw)
    expect(fw).toHaveAttribute('aria-expanded', 'true')
  })

  it('ignoriert kaputten gespeicherten Zustand', () => {
    localStorage.setItem(SIDEBAR_SECTIONS_KEY, '{kaputt')
    renderNav()
    expect(sectionButton('nav.comply.group.frameworks')).toHaveAttribute('aria-expanded', 'false')
  })

  it('Pro-Kennzeichen und „nicht im Angebot" bleiben ohne Pro-Lizenz sichtbar', () => {
    renderNav({ pathname: '/vaktcomply/frameworks' })
    expect(screen.getByRole('link', { name: /nav\.comply\.euAiAct/ })).toHaveTextContent('Pro')
    expect(screen.getByRole('link', { name: /nav\.comply\.dora/ }))
      .toHaveTextContent('vaktcomply.frameworksPage.frameworkStatusNotOffered')
    expect(screen.getByRole('link', { name: 'nav.comply.nis2' })).not.toHaveTextContent('Pro')
  })

  it('mit Pro-Lizenz entfällt das Pro-Kennzeichen', () => {
    renderNav({ pathname: '/vaktcomply/frameworks', isUserPro: true })
    expect(screen.getByRole('link', { name: /nav\.comply\.euAiAct/ })).not.toHaveTextContent('Pro')
  })

  it('Administration nur für Admin/Owner', () => {
    renderNav()
    expect(screen.queryByRole('link', { name: 'nav.administration' })).not.toBeInTheDocument()
  })

  it('Icon-Modus: Bereichs-Icon öffnet per Klick ein Flyout mit allen Einträgen', () => {
    renderNav({ collapsed: true, isAdminOrOwner: true })
    expect(screen.getByRole('link', { name: 'nav.administration' })).toBeInTheDocument()
    const bcm = screen.getByRole('button', { name: 'nav.comply.group.bcm' })
    expect(bcm).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(bcm)
    expect(bcm).toHaveAttribute('aria-expanded', 'true')
    const flyout = screen.getByRole('navigation', { name: 'nav.comply.group.bcm' })
    expect(bcm).toHaveAttribute('aria-controls', flyout.id)
    expect(within(flyout).getAllByRole('link')).toHaveLength(5)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.queryByRole('navigation', { name: 'nav.comply.group.bcm' })).not.toBeInTheDocument()
  })
})

describe('sectionForPath', () => {
  it.each([
    ['/', null],
    ['/vaktcomply', null],
    ['/nis2-check/multi', 'frameworks'],
    ['/vaktcomply/overdue-reviews', 'operations'],
    ['/vaktcomply/evidence/auto', 'documentation'],
    ['/vaktcomply/ai-systems/7', 'third-party'],
    ['/vaktcomply/backup', 'bcm'],
    ['/vaktcomply/bcmx', null],
  ])('%s → %s', (path, key) => {
    expect(sectionForPath(path)).toBe(key)
  })
})
