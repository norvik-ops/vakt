import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, ChevronRight, CheckCircle2, Circle, ArrowRight } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Badge } from '../../../components/ui/badge'
import { TermTooltip } from '../../../shared/components/TermTooltip'

// ─── Data ──────────────────────────────────────────────────────────────────────

type ModuleKey = 'vaktscan' | 'vaktvault' | 'vaktcomply' | 'vaktprivacy' | 'vaktaware'

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
      { id: 'ISMS.1', title: 'Sicherheitsmanagement', module: 'vaktcomply', description: 'Aufbau und Betrieb des ISMS' },
    ],
  },
  {
    category: 'ORP',
    title: 'Organisation und Personal',
    bausteins: [
      { id: 'ORP.1', title: 'Organisation',                            module: 'vaktcomply', description: 'Sicherheitsorganisation aufbauen' },
      { id: 'ORP.2', title: 'Personal',                                module: 'vaktcomply', description: 'Personalsicherheit' },
      { id: 'ORP.3', title: 'Sensibilisierung und Schulung',           module: 'vaktaware', description: 'Security Awareness Trainings' },
      { id: 'ORP.4', title: 'Identitäts- und Berechtigungsmanagement', module: 'vaktvault',  description: 'Zugriffsrechte verwalten' },
    ],
  },
  {
    category: 'CON',
    title: 'Konzepte und Vorgehensweisen',
    bausteins: [
      { id: 'CON.1',  title: 'Kryptokonzept',                              module: 'vaktvault',   description: 'Kryptografische Maßnahmen' },
      { id: 'CON.2',  title: 'Datenschutz',                                module: 'vaktprivacy', description: 'DSGVO-Dokumentation und VVT' },
      { id: 'CON.3',  title: 'Datensicherungskonzept',                     module: 'vaktcomply',  description: 'Backup-Strategie dokumentieren' },
      { id: 'CON.6',  title: 'Löschen und Vernichten',                     module: 'vaktcomply',  description: 'Datenlöschung und -entsorgung' },
      { id: 'CON.7',  title: 'Informationssicherheit auf Auslandsreisen',  module: 'vaktcomply',  description: 'Reisesicherheit' },
      { id: 'CON.10', title: 'Entwicklung von Webanwendungen',             module: 'vaktscan',   description: 'Sichere Entwicklung' },
    ],
  },
  {
    category: 'OPS',
    title: 'Betrieb',
    bausteins: [
      { id: 'OPS.1.1.2', title: 'Ordnungsgemäße IT-Administration',        module: 'vaktvault',   description: 'Admin-Zugriffe und Rechte' },
      { id: 'OPS.1.1.3', title: 'Patch- und Änderungsmanagement',          module: 'vaktscan',   description: 'Schwachstellen und Updates' },
      { id: 'OPS.1.1.4', title: 'Schutz vor Schadprogrammen',              module: 'vaktscan',   description: 'Malware-Erkennung' },
      { id: 'OPS.1.1.5', title: 'Protokollierung',                         module: 'vaktcomply',  description: 'Audit-Logs und Monitoring' },
      { id: 'OPS.1.1.6', title: 'Software-Tests und -Freigaben',           module: 'vaktscan',   description: 'Testmanagement' },
      { id: 'OPS.1.2.2', title: 'Archivierung',                            module: 'vaktcomply',  description: 'Langzeitarchivierung' },
      { id: 'OPS.2.2',   title: 'Cloud-Nutzung',                           module: 'vaktprivacy', description: 'Cloud-Dienste und AVV' },
      { id: 'OPS.2.3',   title: 'Outsourcing für Kunden',                  module: 'vaktprivacy', description: 'Auftragsverarbeitung' },
    ],
  },
  {
    category: 'DER',
    title: 'Detektion und Reaktion',
    bausteins: [
      { id: 'DER.1',   title: 'Detektion von sicherheitsrelevanten Ereignissen',   module: 'vaktscan',  description: 'Scanner-Integration und Findings' },
      { id: 'DER.2.1', title: 'Behandlung von Sicherheitsvorfällen',               module: 'vaktcomply', description: 'Incident Register und Response' },
      { id: 'DER.2.2', title: 'Vorsorge für die IT-Forensik',                      module: 'vaktcomply', description: 'Forensik-Vorbereitung' },
      { id: 'DER.2.3', title: 'Bereinigung weitreichender Sicherheitsvorfälle',    module: 'vaktcomply', description: 'Incident-Bereinigung' },
      { id: 'DER.3.1', title: 'Audits und Revisionen',                             module: 'vaktcomply', description: 'Interne Audits' },
      { id: 'DER.4',   title: 'Notfallmanagement',                                 module: 'vaktcomply', description: 'BCM und Notfallplanung' },
    ],
  },
  {
    category: 'APP',
    title: 'Anwendungen',
    bausteins: [
      { id: 'APP.1.1', title: 'Office-Produkte',                  module: 'vaktscan', description: 'Schwachstellen in Office-Software' },
      { id: 'APP.2.1', title: 'Allgemeiner Verzeichnisdienst',    module: 'vaktvault', description: 'AD/LDAP Sicherheit' },
      { id: 'APP.3.1', title: 'Webanwendungen und Web-Services',  module: 'vaktscan', description: 'Web-Schwachstellen' },
      { id: 'APP.4.4', title: 'Kubernetes',                       module: 'vaktvault', description: 'K8s Secrets und Scanner' },
      { id: 'APP.5.4', title: 'Unified Communications',           module: 'vaktscan', description: 'Kommunikationssicherheit' },
    ],
  },
  {
    category: 'SYS',
    title: 'IT-Systeme',
    bausteins: [
      { id: 'SYS.1.1',   title: 'Allgemeiner Server',            module: 'vaktscan', description: 'Server-Härtung und Scanning' },
      { id: 'SYS.1.3',   title: 'Server unter Linux und Unix',   module: 'vaktscan', description: 'Linux-Server Sicherheit' },
      { id: 'SYS.2.1',   title: 'Allgemeiner Client',            module: 'vaktscan', description: 'Client-Endgeräte' },
      { id: 'SYS.3.2.2', title: 'Mobile Device Management',      module: 'vaktvault', description: 'MDM und Zugriffsschutz' },
    ],
  },
  {
    category: 'NET',
    title: 'Netze und Kommunikation',
    bausteins: [
      { id: 'NET.1.1', title: 'Netzarchitektur und -design', module: 'vaktscan', description: 'Netzwerk-Scanning' },
      { id: 'NET.1.2', title: 'Netzmanagement',              module: 'vaktscan', description: 'Netzwerk-Management' },
      { id: 'NET.2.1', title: 'WLAN-Betrieb',                module: 'vaktscan', description: 'WLAN-Sicherheit' },
      { id: 'NET.3.1', title: 'Router und Switches',         module: 'vaktscan', description: 'Netzwerk-Infrastruktur' },
      { id: 'NET.3.2', title: 'Firewall',                    module: 'vaktscan', description: 'Firewall-Konfiguration' },
    ],
  },
  {
    category: 'INF',
    title: 'Infrastruktur',
    bausteins: [
      { id: 'INF.1', title: 'Allgemeines Gebäude',          module: 'vaktcomply', description: 'Physische Sicherheit' },
      { id: 'INF.2', title: 'Rechenzentrum und Serverraum', module: 'vaktcomply', description: 'Serverraum-Sicherheit' },
      { id: 'INF.6', title: 'Datenträgerarchiv',            module: 'vaktcomply', description: 'Medien und Archivierung' },
    ],
  },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

function moduleToPath(module: ModuleKey): string {
  return `/${module}`
}

function moduleLabel(module: ModuleKey): string {
  const labels: Record<ModuleKey, string> = {
    vaktscan: 'Vakt Scan',
    vaktvault: 'Vakt Vault',
    vaktcomply: 'Vakt Comply',
    vaktprivacy: 'Vakt Privacy',
    vaktaware: 'Vakt Aware',
  }
  return labels[module]
}

function moduleBadgeClass(module: ModuleKey): string {
  if (module === 'vaktscan')   return 'bg-blue-900/40 text-blue-300 border-blue-800'
  if (module === 'vaktvault')   return 'bg-purple-900/40 text-purple-300 border-purple-800'
  if (module === 'vaktcomply')  return 'bg-green-900/40 text-green-300 border-green-800'
  if (module === 'vaktprivacy') return 'bg-orange-900/40 text-orange-300 border-orange-800'
  if (module === 'vaktaware')  return 'bg-yellow-900/40 text-yellow-300 border-yellow-800'
  return 'bg-surface2 text-muted border-transparent'
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
        className="w-full flex items-center justify-between px-4 py-3 hover:bg-muted/50 transition-colors text-left"
      >
        <div className="flex items-center gap-3">
          <Badge className="bg-severity-info-bg text-severity-info border-transparent text-xs font-mono shrink-0">
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
                <Badge className="bg-severity-info-bg/60 text-severity-info border-transparent text-[11px] font-mono shrink-0 mt-0.5">
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

// ─── BSI Phase Progress ───────────────────────────────────────────────────────

interface Phase {
  number: number
  label: string
  description: string
  done: boolean
  linkTo?: string
  linkLabel?: string
}

const BSI_PHASES: Phase[] = [
  {
    number: 1,
    label: 'Strukturanalyse',
    description: 'IT-Assets erfassen',
    done: true,
  },
  {
    number: 2,
    label: 'Schutzbedarfsfeststellung',
    description: 'C/I/A-Bewertung je Asset',
    done: true,
    linkTo: '/vaktcomply/protection-needs',
    linkLabel: 'Öffnen',
  },
  {
    number: 3,
    label: 'Modellierung',
    description: 'Bausteine Assets zuweisen',
    done: false,
    linkTo: '/vaktcomply/bsi-modeling',
    linkLabel: 'Starten',
  },
  {
    number: 4,
    label: 'IT-Grundschutz-Check',
    description: 'Anforderungen prüfen',
    done: false,
    linkTo: '/vaktcomply/bsi/target-objects',
    linkLabel: 'Starten',
  },
]

function PhaseProgressCard() {
  return (
    <div className="rounded-lg border border-border bg-surface p-4">
      <p className="text-xs font-semibold text-secondary uppercase tracking-wide mb-3">
        BSI-Vorgehensweise — 4 Phasen
      </p>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {BSI_PHASES.map((phase) => (
          <div
            key={phase.number}
            className={`rounded-md border p-3 flex flex-col gap-1.5 ${
              phase.done ? 'border-green-600/40 bg-green-900/10' : 'border-border bg-surface2'
            }`}
          >
            <div className="flex items-center gap-1.5">
              {phase.done
                ? <CheckCircle2 className="w-4 h-4 text-green-400 shrink-0" />
                : <Circle className="w-4 h-4 text-secondary shrink-0" />
              }
              <span className="text-[11px] font-semibold text-secondary">Phase {phase.number}</span>
            </div>
            <p className="text-[13px] font-medium text-primary leading-tight">{phase.label}</p>
            <p className="text-[11px] text-secondary">{phase.description}</p>
            {phase.linkTo && (
              <Link
                to={phase.linkTo}
                className="mt-auto inline-flex items-center gap-1 text-[11px] text-blue-400 hover:text-blue-300 transition-colors"
              >
                {phase.linkLabel}
                <ArrowRight className="w-3 h-3" />
              </Link>
            )}
          </div>
        ))}
      </div>
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
        {/* Phase progress */}
        <PhaseProgressCard />

        {/* Summary badges */}
        <div className="flex flex-wrap gap-2 items-center">
          <Badge className="bg-severity-info-bg text-severity-info border-transparent text-xs">
            BSI IT-Grundschutz-Kompendium
          </Badge>
          <Badge className="bg-surface2 text-muted border-transparent text-xs">
            {totalBausteins} Bausteine abgedeckt
          </Badge>
          <Badge className="bg-surface2 text-muted border-transparent text-xs">
            {BAUSTEINE.length} Kategorien
          </Badge>
        </div>

        {/* Intro */}
        <p className="text-sm text-secondary leading-relaxed">
          Das <TermTooltip term="BSI IT-Grundschutz" glossaryKey="BSI200">BSI IT-Grundschutz</TermTooltip>-Kompendium definiert Bausteine für die systematische Absicherung
          von IT-Systemen. Diese Übersicht zeigt, welche Vakt-Module die jeweiligen
          Anforderungen unterstützen.
        </p>

        {/* Category cards */}
        {BAUSTEINE.map((cat) => (
          <CategoryCard
            key={cat.category}
            cat={cat}
            expanded={expanded[cat.category] ?? true}
            onToggle={() => { toggle(cat.category); }}
          />
        ))}
      </div>
    </div>
  )
}
