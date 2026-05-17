import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, ChevronRight, Download, FileDown } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { useFrameworks, useDownloadAuditPackage, useDownloadSoAPDF } from '../hooks/useFrameworks'

// ─── Data ──────────────────────────────────────────────────────────────────────

type ModuleKey = 'secpulse' | 'secvault' | 'secvitals' | 'secprivacy' | 'secreflex'
  | 'secvitals/policies' | 'secvitals/incidents' | 'secvitals/audits'

interface Control {
  id: string
  title: string
  module: ModuleKey
}

interface Clause {
  id: string
  title: string
  count: number
  module: string
  modulePath: string
  controls: Control[]
}

const CLAUSES: Clause[] = [
  {
    id: 'A.5',
    title: 'Organisatorische Maßnahmen',
    count: 37,
    module: 'SecVitals',
    modulePath: '/secvitals',
    controls: [
      { id: 'A.5.1',  title: 'Informationssicherheitsrichtlinien',              module: 'secvitals/policies' },
      { id: 'A.5.2',  title: 'Rollen und Verantwortlichkeiten',                 module: 'secvitals' },
      { id: 'A.5.3',  title: 'Aufgabentrennung',                                module: 'secvitals' },
      { id: 'A.5.4',  title: 'Managementverantwortung',                         module: 'secvitals' },
      { id: 'A.5.5',  title: 'Kontakt zu Behörden',                             module: 'secvitals' },
      { id: 'A.5.6',  title: 'Kontakt zu Fachgruppen',                          module: 'secvitals' },
      { id: 'A.5.7',  title: 'Threat Intelligence',                             module: 'secpulse' },
      { id: 'A.5.8',  title: 'IS in Projektmanagement',                         module: 'secvitals' },
      { id: 'A.5.9',  title: 'Inventar von IS-Assets',                          module: 'secpulse' },
      { id: 'A.5.10', title: 'Zulässige Verwendung von Assets',                 module: 'secvitals' },
      { id: 'A.5.11', title: 'Rückgabe von Assets',                             module: 'secvitals' },
      { id: 'A.5.12', title: 'Klassifizierung von Informationen',               module: 'secvitals' },
      { id: 'A.5.13', title: 'Kennzeichnung von Informationen',                 module: 'secvitals' },
      { id: 'A.5.14', title: 'Informationsübermittlung',                        module: 'secvitals' },
      { id: 'A.5.15', title: 'Zugangssteuerung',                                module: 'secvault' },
      { id: 'A.5.16', title: 'Identitätsmanagement',                            module: 'secvault' },
      { id: 'A.5.17', title: 'Authentifizierungsinformationen',                 module: 'secvault' },
      { id: 'A.5.18', title: 'Zugriffsrechte',                                  module: 'secvault' },
      { id: 'A.5.19', title: 'IS bei Lieferantenbeziehungen',                   module: 'secprivacy' },
      { id: 'A.5.20', title: 'IS in Lieferantenverträgen',                      module: 'secprivacy' },
      { id: 'A.5.21', title: 'IS in IKT-Lieferkette',                           module: 'secprivacy' },
      { id: 'A.5.22', title: 'Überwachung von Lieferantenleistungen',           module: 'secprivacy' },
      { id: 'A.5.23', title: 'IS für Cloud-Dienste',                            module: 'secprivacy' },
      { id: 'A.5.24', title: 'Planung und Vorbereitung auf IS-Vorfälle',        module: 'secvitals/incidents' },
      { id: 'A.5.25', title: 'Bewertung und Entscheidung zu IS-Ereignissen',    module: 'secvitals/incidents' },
      { id: 'A.5.26', title: 'Reaktion auf IS-Vorfälle',                        module: 'secvitals/incidents' },
      { id: 'A.5.27', title: 'Erkenntnisse aus IS-Vorfällen',                   module: 'secvitals/incidents' },
      { id: 'A.5.28', title: 'Sammeln von Beweisen',                            module: 'secvitals' },
      { id: 'A.5.29', title: 'IS bei Betriebsunterbrechungen',                  module: 'secvitals' },
      { id: 'A.5.30', title: 'BCM-Bereitschaft für IKT',                        module: 'secvitals' },
      { id: 'A.5.31', title: 'Rechtliche Anforderungen',                        module: 'secprivacy' },
      { id: 'A.5.32', title: 'Rechte an geistigem Eigentum',                    module: 'secvitals' },
      { id: 'A.5.33', title: 'Schutz von Aufzeichnungen',                       module: 'secvitals' },
      { id: 'A.5.34', title: 'Privatsphäre und PII-Schutz',                     module: 'secprivacy' },
      { id: 'A.5.35', title: 'Unabhängige IS-Überprüfung',                      module: 'secvitals/audits' },
      { id: 'A.5.36', title: 'Einhaltung von IS-Richtlinien',                   module: 'secvitals' },
      { id: 'A.5.37', title: 'Dokumentierte Betriebsverfahren',                 module: 'secvitals/policies' },
    ],
  },
  {
    id: 'A.6',
    title: 'Personenbezogene Maßnahmen',
    count: 8,
    module: 'SecReflex',
    modulePath: '/secreflex',
    controls: [
      { id: 'A.6.1', title: 'Überprüfung von Bewerbern',          module: 'secvitals' },
      { id: 'A.6.2', title: 'Beschäftigungsbedingungen',          module: 'secvitals' },
      { id: 'A.6.3', title: 'IS-Bewusstsein, Schulung und Training', module: 'secreflex' },
      { id: 'A.6.4', title: 'Disziplinarverfahren',               module: 'secvitals' },
      { id: 'A.6.5', title: 'Verantwortlichkeiten beim Ausscheiden', module: 'secvitals' },
      { id: 'A.6.6', title: 'Vertraulichkeitsvereinbarungen',     module: 'secvitals' },
      { id: 'A.6.7', title: 'Fernarbeit',                         module: 'secvitals' },
      { id: 'A.6.8', title: 'Meldung von IS-Ereignissen',         module: 'secvitals/incidents' },
    ],
  },
  {
    id: 'A.7',
    title: 'Physische Maßnahmen',
    count: 14,
    module: 'SecVitals',
    modulePath: '/secvitals',
    controls: [
      { id: 'A.7.1',  title: 'Physische Sicherheitsbereiche',                  module: 'secvitals' },
      { id: 'A.7.2',  title: 'Physischer Zutritt',                             module: 'secvitals' },
      { id: 'A.7.3',  title: 'Sicherung von Büros und Einrichtungen',          module: 'secvitals' },
      { id: 'A.7.4',  title: 'Physische Sicherheitsüberwachung',               module: 'secvitals' },
      { id: 'A.7.5',  title: 'Schutz gegen physische Bedrohungen',             module: 'secvitals' },
      { id: 'A.7.6',  title: 'Arbeit in Sicherheitsbereichen',                 module: 'secvitals' },
      { id: 'A.7.7',  title: 'Clean Desk und Clear Screen',                    module: 'secvitals' },
      { id: 'A.7.8',  title: 'Platzierung und Schutz von Geräten',             module: 'secvitals' },
      { id: 'A.7.9',  title: 'Sicherheit von Assets außerhalb des Standorts',  module: 'secpulse' },
      { id: 'A.7.10', title: 'Speichermedien',                                 module: 'secvitals' },
      { id: 'A.7.11', title: 'Unterstützende Versorgungseinrichtungen',        module: 'secvitals' },
      { id: 'A.7.12', title: 'Verkabelungssicherheit',                         module: 'secvitals' },
      { id: 'A.7.13', title: 'Wartung von Geräten',                            module: 'secvitals' },
      { id: 'A.7.14', title: 'Sichere Entsorgung oder Wiederverwendung',       module: 'secvitals' },
    ],
  },
  {
    id: 'A.8',
    title: 'Technologische Maßnahmen',
    count: 34,
    module: 'SecPulse + SecVault',
    modulePath: '/secpulse',
    controls: [
      { id: 'A.8.1',  title: 'Benutzerendgeräte',                                          module: 'secpulse' },
      { id: 'A.8.2',  title: 'Privilegierte Zugriffsrechte',                               module: 'secvault' },
      { id: 'A.8.3',  title: 'Informationszugangsbeschränkungen',                          module: 'secvault' },
      { id: 'A.8.4',  title: 'Zugang zum Quellcode',                                       module: 'secvault' },
      { id: 'A.8.5',  title: 'Sichere Authentifizierung',                                  module: 'secvault' },
      { id: 'A.8.6',  title: 'Kapazitätsmanagement',                                       module: 'secpulse' },
      { id: 'A.8.7',  title: 'Schutz vor Malware',                                         module: 'secpulse' },
      { id: 'A.8.8',  title: 'Management technischer Schwachstellen',                      module: 'secpulse' },
      { id: 'A.8.9',  title: 'Konfigurationsmanagement',                                   module: 'secpulse' },
      { id: 'A.8.10', title: 'Löschung von Informationen',                                 module: 'secvitals' },
      { id: 'A.8.11', title: 'Datenmaskierung',                                            module: 'secprivacy' },
      { id: 'A.8.12', title: 'Verhinderung von Datenlecks',                                module: 'secvault' },
      { id: 'A.8.13', title: 'Sicherung von Informationen',                                module: 'secvitals' },
      { id: 'A.8.14', title: 'Redundanz von IT-Einrichtungen',                             module: 'secvitals' },
      { id: 'A.8.15', title: 'Protokollierung',                                            module: 'secvitals' },
      { id: 'A.8.16', title: 'Überwachungsaktivitäten',                                    module: 'secpulse' },
      { id: 'A.8.17', title: 'Uhrensynchronisation',                                       module: 'secvitals' },
      { id: 'A.8.18', title: 'Verwendung privilegierter Hilfsprogramme',                   module: 'secvault' },
      { id: 'A.8.19', title: 'Installation von Software auf Produktivsystemen',            module: 'secpulse' },
      { id: 'A.8.20', title: 'Netzwerksicherheit',                                         module: 'secpulse' },
      { id: 'A.8.21', title: 'Sicherheit von Netzwerkdiensten',                            module: 'secpulse' },
      { id: 'A.8.22', title: 'Trennung von Netzwerken',                                    module: 'secpulse' },
      { id: 'A.8.23', title: 'Webfilterung',                                               module: 'secpulse' },
      { id: 'A.8.24', title: 'Verwendung von Kryptografie',                                module: 'secvault' },
      { id: 'A.8.25', title: 'Sicherer Entwicklungslebenszyklus',                          module: 'secpulse' },
      { id: 'A.8.26', title: 'Sicherheitsanforderungen für Anwendungen',                   module: 'secpulse' },
      { id: 'A.8.27', title: 'Sichere Systemarchitektur',                                  module: 'secvitals' },
      { id: 'A.8.28', title: 'Sicheres Programmieren',                                     module: 'secvault' },
      { id: 'A.8.29', title: 'Sicherheitstests in der Entwicklung',                        module: 'secpulse' },
      { id: 'A.8.30', title: 'Ausgelagerte Entwicklung',                                   module: 'secvitals' },
      { id: 'A.8.31', title: 'Trennung von Entwicklungs-, Test- und Produktionsumgebungen', module: 'secpulse' },
      { id: 'A.8.32', title: 'Änderungsmanagement',                                        module: 'secvitals' },
      { id: 'A.8.33', title: 'Testinformationen',                                          module: 'secvitals' },
      { id: 'A.8.34', title: 'Schutz der IS-Systeme in Audits',                            module: 'secvitals' },
    ],
  },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

function moduleToPath(module: ModuleKey): string {
  if (module === 'secvitals/policies') return '/secvitals/policies'
  if (module === 'secvitals/incidents') return '/secvitals/incidents'
  if (module === 'secvitals/audits') return '/secvitals/audits'
  const base = module.split('/')[0]
  return `/${base}`
}

function moduleLabel(module: ModuleKey): string {
  const labels: Record<ModuleKey, string> = {
    secpulse: 'SecPulse',
    secvault: 'SecVault',
    secvitals: 'SecVitals',
    secprivacy: 'SecPrivacy',
    secreflex: 'SecReflex',
    'secvitals/policies': 'SecVitals · Richtlinien',
    'secvitals/incidents': 'SecVitals · Vorfälle',
    'secvitals/audits': 'SecVitals · Audits',
  }
  return labels[module]
}

function moduleBadgeClass(module: ModuleKey): string {
  if (module.startsWith('secpulse'))   return 'bg-blue-900/40 text-blue-300 border-blue-800'
  if (module.startsWith('secvault'))   return 'bg-purple-900/40 text-purple-300 border-purple-800'
  if (module.startsWith('secvitals'))  return 'bg-green-900/40 text-green-300 border-green-800'
  if (module.startsWith('secprivacy')) return 'bg-orange-900/40 text-orange-300 border-orange-800'
  if (module.startsWith('secreflex'))  return 'bg-yellow-900/40 text-yellow-300 border-yellow-800'
  return 'bg-[#374151] text-[#94a3b8] border-transparent'
}

// ─── Clause Card ──────────────────────────────────────────────────────────────

function ClauseCard({ clause, expanded, onToggle }: {
  clause: Clause
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
          <Badge className="bg-[#1e3a5f] text-[#93c5fd] border-transparent text-xs font-mono shrink-0">
            {clause.id}
          </Badge>
          <span className="text-sm font-semibold text-primary">{clause.title}</span>
          <span className="text-xs text-secondary">({clause.count} Controls)</span>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <span className="text-xs text-secondary hidden sm:block">{clause.module}</span>
          {expanded
            ? <ChevronDown className="w-4 h-4 text-secondary" />
            : <ChevronRight className="w-4 h-4 text-secondary" />
          }
        </div>
      </button>

      {/* Controls list */}
      {expanded && (
        <div className="border-t border-border divide-y divide-border">
          {clause.controls.map((control) => (
            <div
              key={control.id}
              className="flex items-center justify-between gap-3 px-4 py-2.5"
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <Badge className="bg-[#1e3a5f]/60 text-[#93c5fd] border-transparent text-[11px] font-mono shrink-0">
                  {control.id}
                </Badge>
                <span className="text-[13px] text-primary truncate">{control.title}</span>
              </div>
              <Link
                to={moduleToPath(control.module)}
                className={`shrink-0 text-[11px] px-2 py-0.5 rounded border font-medium transition-opacity hover:opacity-80 ${moduleBadgeClass(control.module)}`}
              >
                {moduleLabel(control.module)}
              </Link>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ISO27001ChecklistPage() {
  const [expanded, setExpanded] = useState<Record<string, boolean>>(
    Object.fromEntries(CLAUSES.map((c) => [c.id, true])),
  )
  const { data: frameworks } = useFrameworks()
  const downloadSoA = useDownloadSoAPDF()
  const downloadAuditPackage = useDownloadAuditPackage()

  const iso27001 = frameworks?.find((f) =>
    f.name.toLowerCase().includes('iso 27001') || f.name.toLowerCase().includes('iso27001'),
  )

  function toggle(id: string) {
    setExpanded((prev) => ({ ...prev, [id]: !prev[id] }))
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="ISO 27001:2022 Annex A"
        description="93 Controls in 4 Klauseln — Zuordnung zu Vakt-Modulen"
        actions={
          iso27001 && (
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => downloadSoA(iso27001.id, iso27001.name)}
              >
                <FileDown className="w-4 h-4 mr-1.5" />
                SoA exportieren
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => downloadAuditPackage(iso27001.id, iso27001.name)}
              >
                <Download className="w-4 h-4 mr-1.5" />
                Audit-Paket exportieren (ZIP)
              </Button>
            </div>
          )
        }
      />

      <div className="p-6 space-y-4">
        {/* Summary badges */}
        <div className="flex flex-wrap gap-2 items-center">
          <Badge className="bg-[#1e3a5f] text-[#93c5fd] border-transparent text-xs">
            ISO/IEC 27001:2022
          </Badge>
          <Badge className="bg-[#374151] text-[#94a3b8] border-transparent text-xs">
            93 Controls gesamt
          </Badge>
          <Badge className="bg-[#374151] text-[#94a3b8] border-transparent text-xs">
            4 Klauseln
          </Badge>
        </div>

        {/* Disclaimer */}
        <div className="rounded-md border border-amber-800/40 bg-amber-900/20 px-4 py-2.5 text-xs text-amber-300">
          Diese Checkliste dient als Orientierung. Den rechtsverbindlichen Text finden Sie in der ISO/IEC 27001:2022 Norm.
        </div>

        {/* Clause cards */}
        {CLAUSES.map((clause) => (
          <ClauseCard
            key={clause.id}
            clause={clause}
            expanded={expanded[clause.id] ?? true}
            onToggle={() => toggle(clause.id)}
          />
        ))}
      </div>
    </div>
  )
}
