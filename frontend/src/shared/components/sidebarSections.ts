import {
  FileCheck, Key, ShieldCheck, ShieldAlert, Siren, BookOpen, ClipboardList, Server, Shield, FlaskConical, Building2, Bot, UserCheck, ClipboardCheck, CalendarClock, Inbox, ScrollText, CalendarDays, Cpu, Landmark, ListChecks, Cloud, Banknote, LayoutGrid, FileBarChart, ActivitySquare, Phone, DatabaseBackup, Layers, Briefcase, LifeBuoy,
} from 'lucide-react'

export interface NavChild {
  path: string
  label: string
  icon: React.ElementType
  pro?: boolean
  /** 'pro' = mit Pro-Lizenz nutzbar. 'unsold' = gatet auf ein Merkmal, das in keinem
   *  Tarif verkauft wird (features.UnsoldFeatures) — kein Kauf schaltet es frei. */
  tier?: 'pro' | 'unsold'
}

export interface NavSection {
  /** Stabiler Schlüssel für DOM-IDs und den gemerkten Auf-/Zu-Zustand. */
  key: string
  label: string
  icon: React.ElementType
  items: NavChild[]
}

// Die Seitenleiste hat keine Modul-Ebene mehr: Vakt Comply ist das einzige Modul, also
// stehen seine fünf Bereiche direkt unter dem Dashboard. Inhalte, Reihenfolge und
// Pro-Kennzeichen sind 1:1 die frühere Comply-Gruppierung.
export const NAV_SECTIONS: NavSection[] = [
  {
    key: 'frameworks',
    label: 'nav.comply.group.frameworks',
    icon: ShieldCheck,
    items: [
      { path: '/vaktcomply/frameworks',          label: 'nav.comply.allFrameworks', icon: ShieldCheck },
      { path: '/vaktcomply/nis2',                label: 'nav.comply.nis2',          icon: Shield },
      { path: '/vaktcomply/iso27001',            label: 'nav.comply.iso27001',      icon: FileCheck },
      { path: '/vaktcomply/grundschutz',         label: 'nav.comply.bsi',           icon: Landmark },
      { path: '/vaktcomply/bsi/target-objects',  label: 'nav.comply.bsiCheck',      icon: ClipboardCheck, pro: true },
      { path: '/vaktcomply/bsi/cockpit',         label: 'nav.comply.bsiCockpit',    icon: LayoutGrid, pro: true },
      { path: '/vaktcomply/bsi/reports',         label: 'nav.comply.bsiReports',    icon: FileBarChart, pro: true },
      { path: '/vaktcomply/cis-controls',        label: 'nav.comply.cisv8',         icon: ListChecks },
      { path: '/vaktcomply/ccm',                 label: 'nav.comply.ccm',           icon: Cloud },
      { path: '/vaktcomply/dora/dashboard',      label: 'nav.comply.dora',          icon: Banknote, pro: true, tier: 'unsold' },
      { path: '/vaktcomply/eu-ai-act/dashboard', label: 'nav.comply.euAiAct',       icon: Bot, pro: true },
      { path: '/nis2-check/multi',               label: 'nav.comply.multiFramework', icon: Layers, pro: true },
    ],
  },
  {
    key: 'operations',
    label: 'nav.comply.group.operations',
    icon: Briefcase,
    items: [
      { path: '/vaktcomply/risks',           label: 'nav.comply.risks',       icon: ShieldAlert },
      { path: '/vaktcomply/assets',          label: 'nav.comply.assets',      icon: Server },
      { path: '/vaktcomply/incidents',       label: 'nav.comply.incidents',   icon: Siren },
      { path: '/vaktcomply/audits',          label: 'nav.comply.audits',      icon: ClipboardList },
      { path: '/vaktcomply/capas',           label: 'nav.comply.capas',       icon: ClipboardCheck },
      { path: '/vaktcomply/approvals',       label: 'nav.comply.approvals',   icon: UserCheck },
      { path: '/vaktcomply/overdue-reviews', label: 'nav.comply.overdue',     icon: CalendarClock },
    ],
  },
  {
    key: 'documentation',
    label: 'nav.comply.group.documentation',
    icon: BookOpen,
    items: [
      { path: '/vaktcomply/policies',               label: 'nav.comply.policies',     icon: BookOpen },
      { path: '/vaktcomply/soa',                    label: 'nav.comply.soa',           icon: ScrollText },
      { path: '/vaktcomply/evidence',               label: 'nav.comply.evidence',      icon: Inbox },
      { path: '/vaktcomply/crypto-keys',            label: 'nav.comply.cryptography',  icon: Key },
      { path: '/vaktcomply/certification-timeline', label: 'nav.comply.certTimeline',  icon: CalendarDays },
    ],
  },
  {
    key: 'third-party',
    label: 'nav.comply.group.thirdParty',
    icon: Building2,
    items: [
      { path: '/vaktcomply/suppliers',        label: 'nav.comply.suppliers',  icon: Building2, pro: true },
      { path: '/vaktcomply/ai-systems',       label: 'nav.comply.aiSystems',  icon: Cpu, pro: true },
      { path: '/vaktcomply/resilience-tests', label: 'nav.comply.resilience', icon: FlaskConical, pro: true, tier: 'unsold' },
    ],
  },
  {
    key: 'bcm',
    label: 'nav.comply.group.bcm',
    icon: LifeBuoy,
    items: [
      { path: '/vaktcomply/bcm',                      label: 'nav.comply.bcmDashboard',       icon: ShieldCheck, pro: true },
      { path: '/vaktcomply/bcm/bia',                  label: 'nav.comply.bcmBia',             icon: ActivitySquare, pro: true },
      { path: '/vaktcomply/bcm/recovery-plans',       label: 'nav.comply.bcmRecoveryPlans',   icon: ListChecks, pro: true },
      { path: '/vaktcomply/bcm/emergency-contacts',   label: 'nav.comply.bcmEmergencyContacts', icon: Phone, pro: true },
      { path: '/vaktcomply/backup',                   label: 'nav.comply.backup',             icon: DatabaseBackup },
    ],
  },
]

/** Manuell geöffnete/geschlossene Bereiche, als JSON-Objekt { key: boolean }. */
export const SIDEBAR_SECTIONS_KEY = 'vakt_sidebar_sections'

export function matchesPath(pathname: string, path: string): boolean {
  return pathname === path || pathname.startsWith(path + '/')
}

/** Schlüssel des Bereichs, der die aktuelle Route enthält — null z. B. auf dem Dashboard. */
export function sectionForPath(pathname: string): string | null {
  for (const s of NAV_SECTIONS) {
    if (s.items.some((c) => matchesPath(pathname, c.path))) return s.key
  }
  return null
}
