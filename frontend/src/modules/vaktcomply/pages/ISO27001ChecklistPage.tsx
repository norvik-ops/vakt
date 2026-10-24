import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, ChevronRight, Download, FileDown } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { useFrameworks, useDownloadAuditPackage, useDownloadSoAPDF } from '../hooks/useFrameworks'
import { TermTooltip } from '../../../shared/components/TermTooltip'

// ─── Data ──────────────────────────────────────────────────────────────────────

// Comply page a control is documented on, relative to /vaktcomply. Without a
// target the control points to the Comply overview.
type ComplyTarget = 'policies' | 'incidents' | 'audits' | 'assets' | 'access-reviews'
  | 'suppliers' | 'dsgvo/tom' | 'crypto-keys' | 'pentests'

interface AnnexControl {
  id: string
  title: string
  target?: ComplyTarget
}

interface Clause {
  id: string
  title: string
  count: number
  controls: AnnexControl[]
}

const CLAUSES: Clause[] = [
  {
    id: 'A.5',
    title: 'Organisatorische Maßnahmen',
    count: 37,
    controls: [
      { id: 'A.5.1',  title: 'Informationssicherheitsrichtlinien', target: 'policies' },
      { id: 'A.5.2',  title: 'Rollen und Verantwortlichkeiten' },
      { id: 'A.5.3',  title: 'Aufgabentrennung' },
      { id: 'A.5.4',  title: 'Managementverantwortung' },
      { id: 'A.5.5',  title: 'Kontakt zu Behörden' },
      { id: 'A.5.6',  title: 'Kontakt zu Fachgruppen' },
      { id: 'A.5.7',  title: 'Threat Intelligence' },
      { id: 'A.5.8',  title: 'IS in Projektmanagement' },
      { id: 'A.5.9',  title: 'Inventar von IS-Assets', target: 'assets' },
      { id: 'A.5.10', title: 'Zulässige Verwendung von Assets' },
      { id: 'A.5.11', title: 'Rückgabe von Assets' },
      { id: 'A.5.12', title: 'Klassifizierung von Informationen' },
      { id: 'A.5.13', title: 'Kennzeichnung von Informationen' },
      { id: 'A.5.14', title: 'Informationsübermittlung' },
      { id: 'A.5.15', title: 'Zugangssteuerung', target: 'access-reviews' },
      { id: 'A.5.16', title: 'Identitätsmanagement', target: 'access-reviews' },
      { id: 'A.5.17', title: 'Authentifizierungsinformationen' },
      { id: 'A.5.18', title: 'Zugriffsrechte', target: 'access-reviews' },
      { id: 'A.5.19', title: 'IS bei Lieferantenbeziehungen', target: 'suppliers' },
      { id: 'A.5.20', title: 'IS in Lieferantenverträgen', target: 'suppliers' },
      { id: 'A.5.21', title: 'IS in IKT-Lieferkette', target: 'suppliers' },
      { id: 'A.5.22', title: 'Überwachung von Lieferantenleistungen', target: 'suppliers' },
      { id: 'A.5.23', title: 'IS für Cloud-Dienste', target: 'suppliers' },
      { id: 'A.5.24', title: 'Planung und Vorbereitung auf IS-Vorfälle', target: 'incidents' },
      { id: 'A.5.25', title: 'Bewertung und Entscheidung zu IS-Ereignissen', target: 'incidents' },
      { id: 'A.5.26', title: 'Reaktion auf IS-Vorfälle', target: 'incidents' },
      { id: 'A.5.27', title: 'Erkenntnisse aus IS-Vorfällen', target: 'incidents' },
      { id: 'A.5.28', title: 'Sammeln von Beweisen' },
      { id: 'A.5.29', title: 'IS bei Betriebsunterbrechungen' },
      { id: 'A.5.30', title: 'BCM-Bereitschaft für IKT' },
      { id: 'A.5.31', title: 'Rechtliche Anforderungen' },
      { id: 'A.5.32', title: 'Rechte an geistigem Eigentum' },
      { id: 'A.5.33', title: 'Schutz von Aufzeichnungen' },
      { id: 'A.5.34', title: 'Privatsphäre und PII-Schutz', target: 'dsgvo/tom' },
      { id: 'A.5.35', title: 'Unabhängige IS-Überprüfung', target: 'audits' },
      { id: 'A.5.36', title: 'Einhaltung von IS-Richtlinien' },
      { id: 'A.5.37', title: 'Dokumentierte Betriebsverfahren', target: 'policies' },
    ],
  },
  {
    id: 'A.6',
    title: 'Personenbezogene Maßnahmen',
    count: 8,
    controls: [
      { id: 'A.6.1', title: 'Überprüfung von Bewerbern' },
      { id: 'A.6.2', title: 'Beschäftigungsbedingungen' },
      { id: 'A.6.3', title: 'IS-Bewusstsein, Schulung und Training' },
      { id: 'A.6.4', title: 'Disziplinarverfahren' },
      { id: 'A.6.5', title: 'Verantwortlichkeiten beim Ausscheiden' },
      { id: 'A.6.6', title: 'Vertraulichkeitsvereinbarungen' },
      { id: 'A.6.7', title: 'Fernarbeit' },
      { id: 'A.6.8', title: 'Meldung von IS-Ereignissen', target: 'incidents' },
    ],
  },
  {
    id: 'A.7',
    title: 'Physische Maßnahmen',
    count: 14,
    controls: [
      { id: 'A.7.1',  title: 'Physische Sicherheitsbereiche' },
      { id: 'A.7.2',  title: 'Physischer Zutritt' },
      { id: 'A.7.3',  title: 'Sicherung von Büros und Einrichtungen' },
      { id: 'A.7.4',  title: 'Physische Sicherheitsüberwachung' },
      { id: 'A.7.5',  title: 'Schutz gegen physische Bedrohungen' },
      { id: 'A.7.6',  title: 'Arbeit in Sicherheitsbereichen' },
      { id: 'A.7.7',  title: 'Clean Desk und Clear Screen' },
      { id: 'A.7.8',  title: 'Platzierung und Schutz von Geräten' },
      { id: 'A.7.9',  title: 'Sicherheit von Assets außerhalb des Standorts', target: 'assets' },
      { id: 'A.7.10', title: 'Speichermedien' },
      { id: 'A.7.11', title: 'Unterstützende Versorgungseinrichtungen' },
      { id: 'A.7.12', title: 'Verkabelungssicherheit' },
      { id: 'A.7.13', title: 'Wartung von Geräten' },
      { id: 'A.7.14', title: 'Sichere Entsorgung oder Wiederverwendung' },
    ],
  },
  {
    id: 'A.8',
    title: 'Technologische Maßnahmen',
    count: 34,
    controls: [
      { id: 'A.8.1',  title: 'Benutzerendgeräte', target: 'assets' },
      { id: 'A.8.2',  title: 'Privilegierte Zugriffsrechte', target: 'access-reviews' },
      { id: 'A.8.3',  title: 'Informationszugangsbeschränkungen', target: 'access-reviews' },
      { id: 'A.8.4',  title: 'Zugang zum Quellcode' },
      { id: 'A.8.5',  title: 'Sichere Authentifizierung' },
      { id: 'A.8.6',  title: 'Kapazitätsmanagement' },
      { id: 'A.8.7',  title: 'Schutz vor Malware' },
      { id: 'A.8.8',  title: 'Management technischer Schwachstellen', target: 'pentests' },
      { id: 'A.8.9',  title: 'Konfigurationsmanagement' },
      { id: 'A.8.10', title: 'Löschung von Informationen' },
      { id: 'A.8.11', title: 'Datenmaskierung' },
      { id: 'A.8.12', title: 'Verhinderung von Datenlecks' },
      { id: 'A.8.13', title: 'Sicherung von Informationen' },
      { id: 'A.8.14', title: 'Redundanz von IT-Einrichtungen' },
      { id: 'A.8.15', title: 'Protokollierung' },
      { id: 'A.8.16', title: 'Überwachungsaktivitäten' },
      { id: 'A.8.17', title: 'Uhrensynchronisation' },
      { id: 'A.8.18', title: 'Verwendung privilegierter Hilfsprogramme', target: 'access-reviews' },
      { id: 'A.8.19', title: 'Installation von Software auf Produktivsystemen' },
      { id: 'A.8.20', title: 'Netzwerksicherheit' },
      { id: 'A.8.21', title: 'Sicherheit von Netzwerkdiensten' },
      { id: 'A.8.22', title: 'Trennung von Netzwerken' },
      { id: 'A.8.23', title: 'Webfilterung' },
      { id: 'A.8.24', title: 'Verwendung von Kryptografie', target: 'crypto-keys' },
      { id: 'A.8.25', title: 'Sicherer Entwicklungslebenszyklus' },
      { id: 'A.8.26', title: 'Sicherheitsanforderungen für Anwendungen' },
      { id: 'A.8.27', title: 'Sichere Systemarchitektur' },
      { id: 'A.8.28', title: 'Sicheres Programmieren' },
      { id: 'A.8.29', title: 'Sicherheitstests in der Entwicklung', target: 'pentests' },
      { id: 'A.8.30', title: 'Ausgelagerte Entwicklung' },
      { id: 'A.8.31', title: 'Trennung von Entwicklungs-, Test- und Produktionsumgebungen' },
      { id: 'A.8.32', title: 'Änderungsmanagement' },
      { id: 'A.8.33', title: 'Testinformationen' },
      { id: 'A.8.34', title: 'Schutz der IS-Systeme in Audits' },
    ],
  },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

const TARGET_LABELS: Record<ComplyTarget, string> = {
  policies: 'Richtlinien',
  incidents: 'Vorfälle',
  audits: 'Audits',
  assets: 'Asset-Inventar',
  'access-reviews': 'Zugriffsprüfungen',
  suppliers: 'Lieferanten',
  'dsgvo/tom': 'TOM',
  'crypto-keys': 'Kryptoschlüssel',
  pentests: 'Pentests',
}

function targetPath(target?: ComplyTarget): string {
  return target ? `/vaktcomply/${target}` : '/vaktcomply'
}

function targetLabel(target?: ComplyTarget): string {
  return target ? `Vakt Comply · ${TARGET_LABELS[target]}` : 'Vakt Comply'
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
          <Badge className="bg-severity-info-bg text-severity-info border-transparent text-xs font-mono shrink-0">
            {clause.id}
          </Badge>
          <span className="text-sm font-semibold text-primary">{clause.title}</span>
          <span className="text-xs text-secondary">({clause.count} Controls)</span>
        </div>
        <div className="flex items-center gap-2 shrink-0">
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
                <Badge className="bg-severity-info-bg/60 text-severity-info border-transparent text-[11px] font-mono shrink-0">
                  {control.id}
                </Badge>
                <span className="text-[13px] text-primary truncate">{control.title}</span>
              </div>
              <Link
                to={targetPath(control.target)}
                className="shrink-0 text-[11px] px-2 py-0.5 rounded border font-medium transition-opacity hover:opacity-80 bg-green-900/40 text-green-300 border-green-800"
              >
                {targetLabel(control.target)}
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
        description="93 Controls in 4 Klauseln — wo Sie sie in Vakt Comply dokumentieren"
        actions={
          iso27001 && (
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => { downloadSoA(iso27001.id, iso27001.name); }}
              >
                <FileDown className="w-4 h-4 mr-1.5" />
                SoA exportieren
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => { downloadAuditPackage(iso27001.id, iso27001.name); }}
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
          <Badge className="bg-severity-info-bg text-severity-info border-transparent text-xs">
            <TermTooltip term="ISO 27001" glossaryKey="ISO27001">ISO/IEC 27001:2022</TermTooltip>
          </Badge>
          <Badge className="bg-surface2 text-muted border-transparent text-xs">
            93 Controls gesamt
          </Badge>
          <Badge className="bg-surface2 text-muted border-transparent text-xs">
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
            onToggle={() => { toggle(clause.id); }}
          />
        ))}
      </div>
    </div>
  )
}
