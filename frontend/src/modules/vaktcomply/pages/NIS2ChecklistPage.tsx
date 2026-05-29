import { Link } from 'react-router-dom'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Badge } from '../../../components/ui/badge'
import { ExternalLink, Sparkles } from 'lucide-react'

interface NIS2Requirement {
  id: string
  article: string
  title: string
  description: string
  module: string
  path: string
  checkHint: string
}

const NIS2_REQUIREMENTS: NIS2Requirement[] = [
  {
    id: 'a',
    article: 'Art. 21 Abs. 2 lit. a',
    title: 'Risikoanalyse und Sicherheitskonzepte',
    description: 'Konzepte fur die Risikoanalyse und Sicherheit von Informationssystemen',
    module: 'Vakt Comply',
    path: '/vaktcomply/risks',
    checkHint: 'Risiko-Register befullt und regelmaßig uberpruft',
  },
  {
    id: 'b',
    article: 'Art. 21 Abs. 2 lit. b',
    title: 'Bewaltigung von Sicherheitsvorfallen',
    description: 'Erkennung, Analyse, Eindammung und Reaktion auf Sicherheitsvorfalle',
    module: 'Vakt Comply',
    path: '/vaktcomply/incidents',
    checkHint: 'Vorfall-Register gefuhrt, Meldeprozesse dokumentiert',
  },
  {
    id: 'c',
    article: 'Art. 21 Abs. 2 lit. c',
    title: 'Backup-Management und Betriebskontinuitat',
    description: 'Backup-Management, Notfallwiederherstellung, Krisenmanagement',
    module: 'System',
    path: '/settings',
    checkHint: 'Regelmaßige Backups nachgewiesen (Backup-Log vorhanden)',
  },
  {
    id: 'd',
    article: 'Art. 21 Abs. 2 lit. d',
    title: 'Sicherheit der Lieferkette',
    description:
      'Sicherheit in der Lieferkette einschl. sicherheitsbezogener Aspekte der Beziehungen zwischen Einrichtungen und unmittelbaren Anbietern',
    module: 'Vakt Privacy (AVV)',
    path: '/vaktprivacy/avv',
    checkHint: 'Auftragsverarbeitungsvertrage aktuell und vollstandig',
  },
  {
    id: 'e',
    article: 'Art. 21 Abs. 2 lit. e',
    title: 'Sicherheit beim Erwerb, Entwicklung und Wartung',
    description:
      'Sicherheit beim Erwerb, bei der Entwicklung und Wartung von Netz- und Informationssystemen',
    module: 'Vakt Scan',
    path: '/vaktscan/findings',
    checkHint: 'Vulnerability-Scanning aktiv, kritische Findings geschlossen',
  },
  {
    id: 'f',
    article: 'Art. 21 Abs. 2 lit. f',
    title: 'Wirksamkeit von Risikomanagementmaßnahmen',
    description:
      'Konzepte und Verfahren zur Bewertung der Wirksamkeit der Risikomanagementmaßnahmen',
    module: 'Vakt Comply',
    path: '/vaktcomply',
    checkHint: 'Compliance-Score dokumentiert, Framework-Bewertung durchgefuhrt',
  },
  {
    id: 'g',
    article: 'Art. 21 Abs. 2 lit. g',
    title: 'Grundlegende Cyberhygiene und Schulungen',
    description:
      'Grundlegende Verfahren im Bereich der Cyberhygiene und Schulungen zur Cybersicherheit',
    module: 'Vakt Aware',
    path: '/vaktaware',
    checkHint: 'Awareness-Trainings nachweisbar durchgefuhrt',
  },
  {
    id: 'h',
    article: 'Art. 21 Abs. 2 lit. h',
    title: 'Kryptografie und Verschlusselung',
    description:
      'Konzepte und Verfahren fur den Einsatz von Kryptografie und gegebenenfalls Verschlusselung',
    module: 'Vakt Vault',
    path: '/vaktvault',
    checkHint: 'Secrets verschlusselt gespeichert, Verschlusselungsrichtlinie dokumentiert',
  },
  {
    id: 'i',
    article: 'Art. 21 Abs. 2 lit. i',
    title: 'Personalsicherheit und Zugriffskontrolle',
    description:
      'Sicherheit des Personals, Konzepte fur die Zugriffskontrolle und Asset-Management',
    module: 'Vakt Scan (Assets)',
    path: '/vaktscan/assets',
    checkHint: 'Asset-Inventar vollstandig, Benutzerrechte dokumentiert',
  },
  {
    id: 'j',
    article: 'Art. 21 Abs. 2 lit. j',
    title: 'Mehrfaktor-Authentifizierung (MFA)',
    description:
      'Verwendung von Multi-Faktor-Authentifizierung oder kontinuierlicher Authentifizierung',
    module: 'System (2FA)',
    path: '/account',
    checkHint: '2FA fur alle Admin-Accounts aktiviert',
  },
]

/** Returns a Tailwind colour pair for the module badge. */
function moduleBadgeClass(module: string): string {
  if (module.startsWith('Vakt Comply')) return 'bg-blue-900/40 text-blue-300 border-blue-800'
  if (module.startsWith('Vakt Scan')) return 'bg-orange-900/40 text-orange-300 border-orange-800'
  if (module.startsWith('Vakt Vault')) return 'bg-yellow-900/40 text-yellow-300 border-yellow-800'
  if (module.startsWith('Vakt Aware')) return 'bg-purple-900/40 text-purple-300 border-purple-800'
  if (module.startsWith('Vakt Privacy')) return 'bg-teal-900/40 text-teal-300 border-teal-800'
  return 'bg-surface2 text-muted border-transparent'
}

export default function NIS2ChecklistPage() {
  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="NIS2-Anforderungen (Art. 21 Abs. 2)"
        description="Ubersicht der 10 Cybersicherheitsmaßnahmen nach NIS2-Richtlinie (EU) 2022/2555 Artikel 21 Absatz 2. Klicke auf ein Modul um den Status zu prufen."
        actions={
          <Link
            to="/vaktcomply/nis2-assistant"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-brand text-white text-sm font-medium hover:bg-brand/90 transition-colors"
          >
            <Sparkles className="w-3.5 h-3.5" aria-hidden="true" />
            NIS2-Assistent öffnen
          </Link>
        }
      />

      <div className="p-6 space-y-3">
        {NIS2_REQUIREMENTS.map((req) => (
          <div
            key={req.id}
            className="flex items-start justify-between gap-4 rounded-lg border border-border bg-surface p-4"
          >
            {/* Left: article + title + description */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1">
                <Badge className="bg-severity-info-bg text-severity-info border-transparent text-xs font-mono shrink-0">
                  {req.article}
                </Badge>
                <span className="text-sm font-semibold text-primary truncate">{req.title}</span>
              </div>
              <p className="text-xs text-secondary leading-relaxed">{req.description}</p>
            </div>

            {/* Right: module badge + open link + hint */}
            <div className="flex flex-col items-end gap-2 shrink-0 min-w-[180px]">
              <Badge className={`text-xs border ${moduleBadgeClass(req.module)}`}>
                {req.module}
              </Badge>
              <Link
                to={req.path}
                className="flex items-center gap-1 text-xs text-brand hover:underline"
              >
                Offnen
                <ExternalLink className="w-3 h-3" />
              </Link>
              <p className="text-[11px] text-secondary text-right leading-snug max-w-[200px]">
                {req.checkHint}
              </p>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
