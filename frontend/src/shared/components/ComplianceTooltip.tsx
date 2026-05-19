import { useState } from 'react'
import { HelpCircle } from 'lucide-react'

const DEFINITIONS: Record<string, { de: string; en?: string }> = {
  control: {
    de: 'Eine konkrete Sicherheitsmaßnahme aus einem Compliance-Framework (z.B. ISO 27001 A.8.1). Controls werden bewertet und bilden den Compliance-Score.',
  },
  capa: {
    de: 'Corrective and Preventive Action — eine dokumentierte Maßnahme zur Behebung oder Vorbeugung einer Schwachstelle oder eines Vorfalls.',
  },
  dpia: {
    de: 'Datenschutz-Folgenabschätzung (Art. 35 DSGVO) — Pflicht bei hohem Risiko für personenbezogene Daten. Dokumentiert Risiken und Schutzmaßnahmen.',
  },
  evidence: {
    de: 'Nachweis, dass ein Control tatsächlich umgesetzt ist — z.B. ein Screenshot, ein Konfigurationsexport oder ein Audit-Protokoll.',
  },
  tom: {
    de: 'Technische und Organisatorische Maßnahmen (Art. 32 DSGVO) — dokumentierte Schutzmaßnahmen für personenbezogene Daten.',
  },
  risk: {
    de: 'Ein identifiziertes Risiko für die Informationssicherheit. Bewertet nach Eintrittswahrscheinlichkeit × Auswirkung = Risikoscore.',
  },
  nis2: {
    de: 'Network and Information Security Directive 2 — EU-Richtlinie für Cybersicherheit kritischer Infrastrukturen. Gilt für viele Unternehmen ab 2024.',
  },
  soa: {
    de: 'Statement of Applicability — Dokument (ISO 27001 Anhang A), das alle Controls listet und begründet, welche angewendet werden.',
  },
  vvt: {
    de: 'Verzeichnis von Verarbeitungstätigkeiten (Art. 30 DSGVO) — Pflicht für alle Unternehmen ab 250 Mitarbeitern, und bei risikoreichen Verarbeitungen.',
  },
  avv: {
    de: 'Auftragsverarbeitungsvertrag (Art. 28 DSGVO) — Vertrag mit Dienstleistern, die personenbezogene Daten im Auftrag verarbeiten.',
  },
  dsr: {
    de: 'Data Subject Request — Antrag einer betroffenen Person auf Auskunft, Löschung oder Berichtigung ihrer Daten (Art. 15-22 DSGVO).',
  },
  incident: {
    de: 'Sicherheitsvorfall oder Datenpanne. NIS2-meldepflichtige Vorfälle müssen innerhalb von 72h an die Behörde gemeldet werden.',
  },
  framework: {
    de: 'Compliance-Rahmenwerk (z.B. ISO 27001, BSI IT-Grundschutz, NIS2). Definiert die Controls, die bewertet und umgesetzt werden.',
  },
  audit: {
    de: 'Systematische Überprüfung der Umsetzung von Controls und Sicherheitsmaßnahmen. Interne Audits sind Pflicht nach ISO 27001 (Abschnitt 9.2) und empfohlen unter NIS2.',
  },
}

let _tooltipId = 0
function nextTooltipId() { return `compliance-tooltip-${++_tooltipId}` }

interface ComplianceTooltipProps {
  term: keyof typeof DEFINITIONS
  children: React.ReactNode
}

export function ComplianceTooltip({ term, children }: ComplianceTooltipProps) {
  const def = DEFINITIONS[term]
  const [visible, setVisible] = useState(false)
  const [tooltipId] = useState(nextTooltipId)

  if (!def) return <>{children}</>

  return (
    <span
      className="relative inline-flex items-center gap-0.5 cursor-help border-b border-dashed border-slate-500/50"
      tabIndex={0}
      role="button"
      aria-describedby={tooltipId}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
      onFocus={() => setVisible(true)}
      onBlur={() => setVisible(false)}
    >
      {children}
      <HelpCircle className="w-3 h-3 text-muted-foreground/60 shrink-0" aria-hidden="true" />
      <span
        id={tooltipId}
        role="tooltip"
        className={[
          'pointer-events-none absolute bottom-full left-1/2 -translate-x-1/2 mb-2 z-50',
          'w-64 rounded-md bg-gray-900 px-3 py-2 text-xs leading-relaxed text-white shadow-lg',
          'transition-opacity duration-150',
          visible ? 'opacity-100' : 'opacity-0',
        ].join(' ')}
      >
        {def.de}
        <span className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900" />
      </span>
    </span>
  )
}
