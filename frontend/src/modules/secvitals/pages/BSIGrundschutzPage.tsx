import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Badge } from '../../../components/ui/badge'

// ─── Data ──────────────────────────────────────────────────────────────────────

type ModuleKey = 'secpulse' | 'secvault' | 'secvitals' | 'secprivacy' | 'secreflex'

interface Baustein {
  id: string
  title: string
  module: ModuleKey
  description: string
}

interface Category {
  category: string
  title: string
  bausteins: Baustein[]
}

const BAUSTEINE: Category[] = [
  {
    category: 'ISMS',
    title: 'Informationssicherheitsmanagement',
    bausteins: [
      { id: 'ISMS.1', title: 'Sicherheitsmanagement', module: 'secvitals', description: 'Aufbau und Betrieb des ISMS' },
    ],
  },
  {
    category: 'ORP',
    title: 'Organisation und Personal',
    bausteins: [
      { id: 'ORP.1', title: 'Organisation',                            module: 'secvitals', description: 'Sicherheitsorganisation aufbauen' },
      { id: 'ORP.2', title: 'Personal',                                module: 'secvitals', description: 'Personalsicherheit' },
      { id: 'ORP.3', title: 'Sensibilisierung und Schulung',           module: 'secreflex', description: 'Security Awareness Trainings' },
      { id: 'ORP.4', title: 'Identitäts- und Berechtigungsmanagement', module: 'secvault',  description: 'Zugriffsrechte verwalten' },
    ],
  },
  {
    category: 'CON',
    title: 'Konzepte und Vorgehensweisen',
    bausteins: [
      { id: 'CON.1',  title: 'Kryptokonzept',                              module: 'secvault',   description: 'Kryptografische Maßnahmen' },
      { id: 'CON.2',  title: 'Datenschutz',                                module: 'secprivacy', description: 'DSGVO-Dokumentation und VVT' },
      { id: 'CON.3',  title: 'Datensicherungskonzept',                     module: 'secvitals',  description: 'Backup-Strategie dokumentieren' },
      { id: 'CON.6',  title: 'Löschen und Vernichten',                     module: 'secvitals',  description: 'Datenlöschung und -entsorgung' },
      { id: 'CON.7',  title: 'Informationssicherheit auf Auslandsreisen',  module: 'secvitals',  description: 'Reisesicherheit' },
      { id: 'CON.10', title: 'Entwicklung von Webanwendungen',             module: 'secpulse',   description: 'Sichere Entwicklung' },
    ],
  },
  {
    category: 'OPS',
    title: 'Betrieb',
    bausteins: [
      { id: 'OPS.1.1.2', title: 'Ordnungsgemäße IT-Administration',        module: 'secvault',   description: 'Admin-Zugriffe und Rechte' },
      { id: 'OPS.1.1.3', title: 'Patch- und Änderungsmanagement',          module: 'secpulse',   description: 'Schwachstellen und Updates' },
      { id: 'OPS.1.1.4', title: 'Schutz vor Schadprogrammen',              module: 'secpulse',   description: 'Malware-Erkennung' },
      { id: 'OPS.1.1.5', title: 'Protokollierung',                         module: 'secvitals',  description: 'Audit-Logs und Monitoring' },
      { id: 'OPS.1.1.6', title: 'Software-Tests und -Freigaben',           module: 'secpulse',   description: 'Testmanagement' },
      { id: 'OPS.1.2.2', title: 'Archivierung',                            module: 'secvitals',  description: 'Langzeitarchivierung' },
      { id: 'OPS.2.2',   title: 'Cloud-Nutzung',                           module: 'secprivacy', description: 'Cloud-Dienste und AVV' },
      { id: 'OPS.2.3',   title: 'Outsourcing für Kunden',                  module: 'secprivacy', description: 'Auftragsverarbeitung' },
    ],
  },
  {
    category: 'DER',
    title: 'Detektion und Reaktion',
    bausteins: [
      { id: 'DER.1',   title: 'Detektion von sicherheitsrelevanten Ereignissen',   module: 'secpulse',  description: 'Scanner-Integration und Findings' },
      { id: 'DER.2.1', title: 'Behandlung von Sicherheitsvorfällen',               module: 'secvitals', description: 'Incident Register und Response' },
      { id: 'DER.2.2', title: 'Vorsorge für die IT-Forensik',                      module: 'secvitals', description: 'Forensik-Vorbereitung' },
      { id: 'DER.2.3', title: 'Bereinigung weitreichender Sicherheitsvorfälle',    module: 'secvitals', description: 'Incident-Bereinigung' },
      { id: 'DER.3.1', title: 'Audits und Revisionen',                             module: 'secvitals', description: 'Interne Audits' },
      { id: 'DER.4',   title: 'Notfallmanagement',                                 module: 'secvitals', description: 'BCM und Notfallplanung' },
    ],
  },
  {
    category: 'APP',
    title: 'Anwendungen',
    bausteins: [
      { id: 'APP.1.1', title: 'Office-Produkte',                  module: 'secpulse', description: 'Schwachstellen in Office-Software' },
      { id: 'APP.2.1', title: 'Allgemeiner Verzeichnisdienst',    module: 'secvault', description: 'AD/LDAP Sicherheit' },
      { id: 'APP.3.1', title: 'Webanwendungen und Web-Services',  module: 'secpulse', description: 'Web-Schwachstellen' },
      { id: 'APP.4.4', title: 'Kubernetes',                       module: 'secvault', description: 'K8s Secrets und Scanner' },
      { id: 'APP.5.4', title: 'Unified Communications',           module: 'secpulse', description: 'Kommunikationssicherheit' },
    ],
  },
  {
    category: 'SYS',
    title: 'IT-Systeme',
    bausteins: [
      { id: 'SYS.1.1',   title: 'Allgemeiner Server',            module: 'secpulse', description: 'Server-Härtung und Scanning' },
      { id: 'SYS.1.3',   title: 'Server unter Linux und Unix',   module: 'secpulse', description: 'Linux-Server Sicherheit' },
      { id: 'SYS.2.1',   title: 'Allgemeiner Client',            module: 'secpulse', description: 'Client-Endgeräte' },
      { id: 'SYS.3.2.2', title: 'Mobile Device Management',      module: 'secvault', description: 'MDM und Zugriffsschutz' },
    ],
  },
  {
    category: 'NET',
    title: 'Netze und Kommunikation',
    bausteins: [
      { id: 'NET.1.1', title: 'Netzarchitektur und -design', module: 'secpulse', description: 'Netzwerk-Scanning' },
      { id: 'NET.1.2', title: 'Netzmanagement',              module: 'secpulse', description: 'Netzwerk-Management' },
      { id: 'NET.2.1', title: 'WLAN-Betrieb',                module: 'secpulse', description: 'WLAN-Sicherheit' },
      { id: 'NET.3.1', title: 'Router und Switches',         module: 'secpulse', description: 'Netzwerk-Infrastruktur' },
      { id: 'NET.3.2', title: 'Firewall',                    module: 'secpulse', description: 'Firewall-Konfiguration' },
    ],
  },
  {
    category: 'INF',
    title: 'Infrastruktur',
    bausteins: [
      { id: 'INF.1', title: 'Allgemeines Gebäude',          module: 'secvitals', description: 'Physische Sicherheit' },
      { id: 'INF.2', title: 'Rechenzentrum und Serverraum', module: 'secvitals', description: 'Serverraum-Sicherheit' },
      { id: 'INF.6', title: 'Datenträgerarchiv',            module: 'secvitals', description: 'Medien und Archivierung' },
    ],
  },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

function moduleToPath(module: ModuleKey): string {
  return `/${module}`
}

function moduleLabel(module: ModuleKey): string {
  const labels: Record<ModuleKey, string> = {
    secpulse: 'SecPulse',
    secvault: 'SecVault',
    secvitals: 'SecVitals',
    secprivacy: 'SecPrivacy',
    secreflex: 'SecReflex',
  }
  return labels[module]
}

function moduleBadgeClass(module: ModuleKey): string {
  if (module === 'secpulse')   return 'bg-blue-900/40 text-blue-300 border-blue-800'
  if (module === 'secvault')   return 'bg-purple-900/40 text-purple-300 border-purple-800'
  if (module === 'secvitals')  return 'bg-green-900/40 text-green-300 border-green-800'
  if (module === 'secprivacy') return 'bg-orange-900/40 text-orange-300 border-orange-800'
  if (module === 'secreflex')  return 'bg-yellow-900/40 text-yellow-300 border-yellow-800'
  return 'bg-[#374151] text-[#94a3b8] border-transparent'
}

// ─── Category Card ────────────────────────────────────────────────────────────

function CategoryCard({ cat, expanded, onToggle }: {
  cat: Category
  expanded: boolean
  onToggle: () => void
}) {
  return (
    <div className="rounded-lg border border-border bg-surface overflow-hidden">
      {/* Header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between px-4 py-3 hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] transition-colors text-left"
      >
        <div className="flex items-center gap-3">
          <Badge className="bg-[#1e3a5f] text-[#93c5fd] border-transparent text-xs font-mono shrink-0">
            {cat.category}
          </Badge>
          <span className="text-sm font-semibold text-primary">{cat.title}</span>
          <span className="text-xs text-secondary">({cat.bausteins.length} Bausteine)</span>
        </div>
        {expanded
          ? <ChevronDown className="w-4 h-4 text-secondary shrink-0" />
          : <ChevronRight className="w-4 h-4 text-secondary shrink-0" />
        }
      </button>

      {/* Baustein rows */}
      {expanded && (
        <div className="border-t border-border divide-y divide-border">
          {cat.bausteins.map((b) => (
            <div
              key={b.id}
              className="flex items-center justify-between gap-3 px-4 py-2.5"
            >
              <div className="flex items-start gap-2.5 min-w-0">
                <Badge className="bg-[#1e3a5f]/60 text-[#93c5fd] border-transparent text-[11px] font-mono shrink-0 mt-0.5">
                  {b.id}
                </Badge>
                <div className="min-w-0">
                  <p className="text-[13px] font-medium text-primary truncate">{b.title}</p>
                  <p className="text-[11px] text-secondary">{b.description}</p>
                </div>
              </div>
              <Link
                to={moduleToPath(b.module)}
                className={`shrink-0 text-[11px] px-2 py-0.5 rounded border font-medium transition-opacity hover:opacity-80 ${moduleBadgeClass(b.module)}`}
              >
                {moduleLabel(b.module)}
              </Link>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function BSIGrundschutzPage() {
  const [expanded, setExpanded] = useState<Record<string, boolean>>(
    Object.fromEntries(BAUSTEINE.map((c) => [c.category, true])),
  )

  function toggle(category: string) {
    setExpanded((prev) => ({ ...prev, [category]: !prev[category] }))
  }

  const totalBausteins = BAUSTEINE.reduce((sum, c) => sum + c.bausteins.length, 0)

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="BSI IT-Grundschutz"
        description="Mapping der Grundschutz-Bausteine auf Vakt-Module"
      />

      <div className="p-6 space-y-4">
        {/* Summary badges */}
        <div className="flex flex-wrap gap-2 items-center">
          <Badge className="bg-[#1e3a5f] text-[#93c5fd] border-transparent text-xs">
            BSI IT-Grundschutz-Kompendium
          </Badge>
          <Badge className="bg-[#374151] text-[#94a3b8] border-transparent text-xs">
            {totalBausteins} Bausteine abgedeckt
          </Badge>
          <Badge className="bg-[#374151] text-[#94a3b8] border-transparent text-xs">
            {BAUSTEINE.length} Kategorien
          </Badge>
        </div>

        {/* Intro */}
        <p className="text-sm text-secondary leading-relaxed">
          Das BSI IT-Grundschutz-Kompendium definiert Bausteine für die systematische Absicherung
          von IT-Systemen. Diese Übersicht zeigt, welche Vakt-Module die jeweiligen
          Anforderungen unterstützen.
        </p>

        {/* Category cards */}
        {BAUSTEINE.map((cat) => (
          <CategoryCard
            key={cat.category}
            cat={cat}
            expanded={expanded[cat.category] ?? true}
            onToggle={() => toggle(cat.category)}
          />
        ))}
      </div>
    </div>
  )
}
