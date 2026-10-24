import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ChevronDown, ChevronRight, CheckCircle2, Circle, ArrowRight } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Badge } from '../../../components/ui/badge'

// ─── Data ──────────────────────────────────────────────────────────────────────

// Comply page a Baustein is documented on, relative to /vaktcomply. Without a
// target the Baustein points to the Comply overview.
type ComplyTarget = 'assets' | 'access-reviews' | 'suppliers' | 'dsgvo/tom' | 'crypto-keys'

interface Baustein {
  id: string
  title: string
  target?: ComplyTarget
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
      { id: 'ISMS.1', title: 'Sicherheitsmanagement', description: 'Aufbau und Betrieb des ISMS' },
    ],
  },
  {
    category: 'ORP',
    title: 'Organisation und Personal',
    bausteins: [
      { id: 'ORP.1', title: 'Organisation', description: 'Sicherheitsorganisation aufbauen' },
      { id: 'ORP.2', title: 'Personal', description: 'Personalsicherheit' },
      { id: 'ORP.3', title: 'Sensibilisierung und Schulung', description: 'Sensibilisierung und Schulungen nachweisen' },
      { id: 'ORP.4', title: 'Identitäts- und Berechtigungsmanagement', target: 'access-reviews', description: 'Zugriffsrechte verwalten' },
    ],
  },
  {
    category: 'CON',
    title: 'Konzepte und Vorgehensweisen',
    bausteins: [
      { id: 'CON.1', title: 'Kryptokonzept', target: 'crypto-keys', description: 'Kryptografische Maßnahmen' },
      { id: 'CON.2', title: 'Datenschutz', target: 'dsgvo/tom', description: 'Datenschutz-Dokumentation (TOM)' },
      { id: 'CON.3', title: 'Datensicherungskonzept', description: 'Backup-Strategie dokumentieren' },
      { id: 'CON.6', title: 'Löschen und Vernichten', description: 'Datenlöschung und -entsorgung' },
      { id: 'CON.7', title: 'Informationssicherheit auf Auslandsreisen', description: 'Reisesicherheit' },
      { id: 'CON.10', title: 'Entwicklung von Webanwendungen', description: 'Sichere Entwicklung' },
    ],
  },
  {
    category: 'OPS',
    title: 'Betrieb',
    bausteins: [
      { id: 'OPS.1.1.2', title: 'Ordnungsgemäße IT-Administration', target: 'access-reviews', description: 'Admin-Zugriffe und Rechte' },
      { id: 'OPS.1.1.3', title: 'Patch- und Änderungsmanagement', description: 'Schwachstellen und Updates' },
      { id: 'OPS.1.1.4', title: 'Schutz vor Schadprogrammen', description: 'Malware-Erkennung' },
      { id: 'OPS.1.1.5', title: 'Protokollierung', description: 'Audit-Logs und Monitoring' },
      { id: 'OPS.1.1.6', title: 'Software-Tests und -Freigaben', description: 'Testmanagement' },
      { id: 'OPS.1.2.2', title: 'Archivierung', description: 'Langzeitarchivierung' },
      { id: 'OPS.2.2', title: 'Cloud-Nutzung', target: 'suppliers', description: 'Cloud-Dienste und AVV' },
      { id: 'OPS.2.3', title: 'Outsourcing für Kunden', target: 'suppliers', description: 'Auftragsverarbeitung' },
    ],
  },
  {
    category: 'DER',
    title: 'Detektion und Reaktion',
    bausteins: [
      { id: 'DER.1', title: 'Detektion von sicherheitsrelevanten Ereignissen', description: 'Erkennung sicherheitsrelevanter Ereignisse' },
      { id: 'DER.2.1', title: 'Behandlung von Sicherheitsvorfällen', description: 'Incident Register und Response' },
      { id: 'DER.2.2', title: 'Vorsorge für die IT-Forensik', description: 'Forensik-Vorbereitung' },
      { id: 'DER.2.3', title: 'Bereinigung weitreichender Sicherheitsvorfälle', description: 'Incident-Bereinigung' },
      { id: 'DER.3.1', title: 'Audits und Revisionen', description: 'Interne Audits' },
      { id: 'DER.4', title: 'Notfallmanagement', description: 'BCM und Notfallplanung' },
    ],
  },
  {
    category: 'APP',
    title: 'Anwendungen',
    bausteins: [
      { id: 'APP.1.1', title: 'Office-Produkte', description: 'Schwachstellen in Office-Software' },
      { id: 'APP.2.1', title: 'Allgemeiner Verzeichnisdienst', target: 'access-reviews', description: 'AD/LDAP Sicherheit' },
      { id: 'APP.3.1', title: 'Webanwendungen und Web-Services', description: 'Web-Schwachstellen' },
      { id: 'APP.4.4', title: 'Kubernetes', description: 'Kubernetes-Absicherung' },
      { id: 'APP.5.4', title: 'Unified Communications', description: 'Kommunikationssicherheit' },
    ],
  },
  {
    category: 'SYS',
    title: 'IT-Systeme',
    bausteins: [
      { id: 'SYS.1.1', title: 'Allgemeiner Server', target: 'assets', description: 'Server-Härtung' },
      { id: 'SYS.1.3', title: 'Server unter Linux und Unix', target: 'assets', description: 'Linux-Server Sicherheit' },
      { id: 'SYS.2.1', title: 'Allgemeiner Client', target: 'assets', description: 'Client-Endgeräte' },
      { id: 'SYS.3.2.2', title: 'Mobile Device Management', target: 'assets', description: 'MDM und Zugriffsschutz' },
    ],
  },
  {
    category: 'NET',
    title: 'Netze und Kommunikation',
    bausteins: [
      { id: 'NET.1.1', title: 'Netzarchitektur und -design', description: 'Netzarchitektur dokumentieren' },
      { id: 'NET.1.2', title: 'Netzmanagement', description: 'Netzwerk-Management' },
      { id: 'NET.2.1', title: 'WLAN-Betrieb', description: 'WLAN-Sicherheit' },
      { id: 'NET.3.1', title: 'Router und Switches', description: 'Netzwerk-Infrastruktur' },
      { id: 'NET.3.2', title: 'Firewall', description: 'Firewall-Konfiguration' },
    ],
  },
  {
    category: 'INF',
    title: 'Infrastruktur',
    bausteins: [
      { id: 'INF.1', title: 'Allgemeines Gebäude', description: 'Physische Sicherheit' },
      { id: 'INF.2', title: 'Rechenzentrum und Serverraum', description: 'Serverraum-Sicherheit' },
      { id: 'INF.6', title: 'Datenträgerarchiv', description: 'Medien und Archivierung' },
    ],
  },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

const TARGET_LABELS: Record<ComplyTarget, string> = {
  assets: 'Asset-Inventar',
  'access-reviews': 'Zugriffsprüfungen',
  suppliers: 'Lieferanten',
  'dsgvo/tom': 'TOM',
  'crypto-keys': 'Kryptoschlüssel',
}

function targetPath(target?: ComplyTarget): string {
  return target ? `/vaktcomply/${target}` : '/vaktcomply'
}

function targetLabel(target?: ComplyTarget): string {
  return target ? `Vakt Comply · ${TARGET_LABELS[target]}` : 'Vakt Comply'
}

// ─── Category Card ────────────────────────────────────────────────────────────

function CategoryCard({ cat, expanded, onToggle }: {
  cat: Category
  expanded: boolean
  onToggle: () => void
}) {
  const { t } = useTranslation()
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
          <span className="text-xs text-secondary">{t('bsiGrundschutz.categoryBausteine', { count: cat.bausteins.length })}</span>
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
                to={targetPath(b.target)}
                className="shrink-0 text-[11px] px-2 py-0.5 rounded border font-medium transition-opacity hover:opacity-80 bg-green-900/40 text-green-300 border-green-800"
              >
                {targetLabel(b.target)}
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

function PhaseProgressCard() {
  const { t } = useTranslation()
  const phases: Phase[] = [
    { number: 1, label: t('bsiGrundschutz.phase1Label'), description: t('bsiGrundschutz.phase1Desc'), done: true },
    { number: 2, label: t('bsiGrundschutz.phase2Label'), description: t('bsiGrundschutz.phase2Desc'), done: true, linkTo: '/vaktcomply/protection-needs', linkLabel: t('bsiGrundschutz.phase2Link') },
    { number: 3, label: t('bsiGrundschutz.phase3Label'), description: t('bsiGrundschutz.phase3Desc'), done: false, linkTo: '/vaktcomply/bsi-modeling', linkLabel: t('bsiGrundschutz.phase3Link') },
    { number: 4, label: t('bsiGrundschutz.phase4Label'), description: t('bsiGrundschutz.phase4Desc'), done: false, linkTo: '/vaktcomply/bsi/target-objects', linkLabel: t('bsiGrundschutz.phase4Link') },
  ]
  return (
    <div className="rounded-lg border border-border bg-surface p-4">
      <p className="text-xs font-semibold text-secondary uppercase tracking-wide mb-3">
        {t('bsiGrundschutz.phaseTitle')}
      </p>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {phases.map((phase) => (
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
              <span className="text-[11px] font-semibold text-secondary">{t('bsiGrundschutz.phaseLabel', { number: phase.number })}</span>
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
  const { t } = useTranslation()
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
        title={t('bsiGrundschutz.title')}
        description={t('bsiGrundschutz.description')}
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
            {t('bsiGrundschutz.bausteinCovered', { count: totalBausteins })}
          </Badge>
          <Badge className="bg-surface2 text-muted border-transparent text-xs">
            {t('bsiGrundschutz.categoryCount', { count: BAUSTEINE.length })}
          </Badge>
        </div>

        {/* Intro */}
        <p className="text-sm text-secondary leading-relaxed">
          {t('bsiGrundschutz.intro')}
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
