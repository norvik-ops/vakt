import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft, ChevronDown, Download, Shield } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Progress } from '../../../components/ui/progress'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../../components/ui/tabs'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../../../components/ui/table'
import { PageHeader } from '../../../shared/components/PageHeader'
import { cn } from '../../../lib/utils'
import { useFrameworks, useFrameworkControls } from '../hooks/useFrameworks'
import type { Control } from '../types'

// ── Implementation Group filter ───────────────────────────────────────────────

type IG = 'IG1' | 'IG2' | 'IG3'

const IG_LABELS: Record<IG, string> = {
  IG1: 'IG1 — Grundlegend',
  IG2: 'IG2 — Erweitert',
  IG3: 'IG3 — Fortgeschritten',
}

const IG_DESCRIPTIONS: Record<IG, string> = {
  IG1: 'Basis-Safeguards für alle Organisationen unabhängig von Größe oder Ressourcen.',
  IG2: 'Zusätzliche Safeguards für Organisationen mit erhöhtem Risikoprofil.',
  IG3: 'Vollständige Safeguard-Umsetzung für Organisationen mit hohem Risiko und Expertise.',
}

// CIS Controls v8 — 18 control groups
const CIS_GROUPS: Array<{ num: number; name: string; domain: string }> = [
  { num: 1,  name: 'Inventarisierung und Kontrolle von Unternehmens-Assets',    domain: 'Asset-Inventarisierung' },
  { num: 2,  name: 'Inventarisierung und Kontrolle von Software-Assets',         domain: 'Software-Inventarisierung' },
  { num: 3,  name: 'Datenschutz',                                                domain: 'Datenschutz' },
  { num: 4,  name: 'Sichere Konfiguration von Unternehmens-Assets und Software', domain: 'Sichere Konfiguration' },
  { num: 5,  name: 'Kontoverwaltung',                                            domain: 'Kontoverwaltung' },
  { num: 6,  name: 'Zugriffskontrollmanagement',                                 domain: 'Zugriffskontrolle' },
  { num: 7,  name: 'Kontinuierliches Schwachstellenmanagement',                  domain: 'Schwachstellenmanagement' },
  { num: 8,  name: 'Verwaltung von Audit-Logs',                                  domain: 'Audit-Log-Verwaltung' },
  { num: 9,  name: 'E-Mail- und Webbrowser-Schutz',                             domain: 'E-Mail und Web-Schutz' },
  { num: 10, name: 'Malware-Abwehr',                                             domain: 'Malware-Abwehr' },
  { num: 11, name: 'Datensicherung und -wiederherstellung',                      domain: 'Datensicherung' },
  { num: 12, name: 'Verwaltung der Netzwerkinfrastruktur',                       domain: 'Netzwerkinfrastruktur' },
  { num: 13, name: 'Netzwerküberwachung und -verteidigung',                      domain: 'Netzwerküberwachung' },
  { num: 14, name: 'Security-Awareness und Schulungen',                          domain: 'Security Awareness' },
  { num: 15, name: 'Dienstleistermanagement',                                    domain: 'Dienstleistermanagement' },
  { num: 16, name: 'Anwendungssoftware-Sicherheit',                             domain: 'Anwendungssicherheit' },
  { num: 17, name: 'Incident-Response-Management',                               domain: 'Incident Response' },
  { num: 18, name: 'Penetrationstests',                                          domain: 'Penetrationstests' },
]

// ISO 27001 mapping reference (mirrors the service seed)
const CIS_ISO_MAPPING: Array<{ cisId: string; cisTitle: string; isoIds: string[] }> = [
  { cisId: 'CIS-1',  cisTitle: 'Inventarisierung von Unternehmens-Assets', isoIds: ['A.8.1'] },
  { cisId: 'CIS-2',  cisTitle: 'Inventarisierung von Software-Assets',      isoIds: ['A.8.1'] },
  { cisId: 'CIS-3',  cisTitle: 'Datenschutz',                               isoIds: ['A.8.2', 'A.8.3'] },
  { cisId: 'CIS-4',  cisTitle: 'Sichere Konfiguration',                     isoIds: ['A.12.6'] },
  { cisId: 'CIS-5',  cisTitle: 'Kontoverwaltung',                           isoIds: ['A.9.2'] },
  { cisId: 'CIS-6',  cisTitle: 'Zugriffskontrolle',                         isoIds: ['A.9.1', 'A.9.4'] },
  { cisId: 'CIS-7',  cisTitle: 'Schwachstellenmanagement',                  isoIds: ['A.12.6'] },
  { cisId: 'CIS-8',  cisTitle: 'Audit-Log-Verwaltung',                      isoIds: ['A.12.4'] },
  { cisId: 'CIS-9',  cisTitle: 'E-Mail- und Webbrowser-Schutz',             isoIds: ['A.6.1'] },
  { cisId: 'CIS-10', cisTitle: 'Malware-Abwehr',                            isoIds: ['A.12.2'] },
  { cisId: 'CIS-11', cisTitle: 'Datensicherung',                            isoIds: ['A.12.3'] },
  { cisId: 'CIS-12', cisTitle: 'Netzwerkinfrastruktur',                     isoIds: ['A.13.1'] },
  { cisId: 'CIS-13', cisTitle: 'Netzwerküberwachung',                       isoIds: ['A.12.4', 'A.13.1'] },
  { cisId: 'CIS-14', cisTitle: 'Security Awareness',                        isoIds: ['A.7.2'] },
  { cisId: 'CIS-15', cisTitle: 'Dienstleistermanagement',                   isoIds: ['A.15.1'] },
  { cisId: 'CIS-16', cisTitle: 'Anwendungssicherheit',                      isoIds: ['A.14.1'] },
  { cisId: 'CIS-17', cisTitle: 'Incident Response',                         isoIds: ['A.16.1'] },
  { cisId: 'CIS-18', cisTitle: 'Penetrationstests',                         isoIds: ['A.12.6'] },
]

// ── Helpers ───────────────────────────────────────────────────────────────────

function statusVariant(status: Control['status']): React.ComponentProps<typeof Badge>['variant'] {
  if (status === 'covered' || status === 'implemented') return 'success'
  if (status === 'partial' || status === 'in_progress') return 'warning'
  if (status === 'not_applicable') return 'secondary'
  return 'destructive'
}

function statusLabel(status: Control['status']): string {
  if (status === 'covered' || status === 'implemented') return 'Umgesetzt'
  if (status === 'partial' || status === 'in_progress') return 'In Arbeit'
  if (status === 'not_applicable') return 'N/A'
  return 'Offen'
}

function groupNum(control: Control): number {
  const m = control.control_id.match(/^CIS-(\d+)\./)
  return m ? parseInt(m[1], 10) : 0
}

// ── Control Group Card ────────────────────────────────────────────────────────

function ControlGroupCard({
  group,
  controls,
  frameworkId,
}: {
  group: (typeof CIS_GROUPS)[number]
  controls: Control[]
  frameworkId: string
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()

  const implemented = controls.filter(
    (c) => c.status === 'covered' || c.status === 'implemented',
  ).length
  const pct = controls.length > 0 ? Math.round((implemented / controls.length) * 100) : 0

  const groupVariant: React.ComponentProps<typeof Badge>['variant'] =
    pct === 100 ? 'success' : pct >= 50 ? 'warning' : controls.length === 0 ? 'secondary' : 'destructive'

  return (
    <div className="border border-border rounded-lg overflow-hidden">
      <button
        type="button"
        className="w-full flex items-center justify-between px-4 py-3 bg-surface2 hover:bg-surface text-left gap-3"
        onClick={() => { setOpen((v) => !v); }}
      >
        <div className="flex items-center gap-3 min-w-0">
          <ChevronDown
            className={cn('w-4 h-4 text-secondary shrink-0 transition-transform', !open && '-rotate-90')}
          />
          <Badge variant="secondary" className="font-mono text-xs shrink-0">
            CIS {group.num}
          </Badge>
          <span className="text-sm font-medium truncate">{group.name}</span>
        </div>
        <div className="flex items-center gap-3 shrink-0">
          <span className="text-xs text-secondary hidden sm:block">
            {implemented}/{controls.length} Safeguards
          </span>
          <Badge variant={groupVariant} className="text-xs">
            {pct}%
          </Badge>
        </div>
      </button>

      {open && controls.length > 0 && (
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-28">ID</TableHead>
                <TableHead>Safeguard</TableHead>
                <TableHead className="w-36">{t('common.status')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {controls.map((ctrl) => (
                <TableRow
                  key={ctrl.id}
                  className="cursor-pointer hover:bg-surface2"
                  onClick={() =>
                    { navigate(`/vaktcomply/controls/${ctrl.id}?frameworkId=${frameworkId}`); }
                  }
                >
                  <TableCell className="font-mono text-xs">{ctrl.control_id}</TableCell>
                  <TableCell className="text-sm">{ctrl.title}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(ctrl.status)} className="text-xs">
                      {statusLabel(ctrl.status)}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {open && controls.length === 0 && (
        <div className="px-4 py-3 text-sm text-secondary italic">
          Keine Safeguards für diese Kontrollgruppe geladen.
        </div>
      )}
    </div>
  )
}

// ── ISO Mapping Tab ───────────────────────────────────────────────────────────

function ISOMapping() {
  return (
    <div className="space-y-3 mt-4">
      <p className="text-sm text-secondary">
        CIS Controls v8 und ISO 27001:2022 decken viele der gleichen Sicherheitsziele ab.
        Die folgende Tabelle zeigt, welche CIS-Kontrollgruppen welchen ISO 27001 Annex-A-Kontrollen entsprechen.
      </p>
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-24">CIS</TableHead>
              <TableHead>CIS-Kontrollgruppe</TableHead>
              <TableHead>ISO 27001 Kontrollen</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {CIS_ISO_MAPPING.map((row) => (
              <TableRow key={row.cisId}>
                <TableCell>
                  <Badge variant="secondary" className="font-mono text-xs">
                    {row.cisId}
                  </Badge>
                </TableCell>
                <TableCell className="text-sm">{row.cisTitle}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {row.isoIds.map((iso) => (
                      <Badge key={iso} variant="outline" className="font-mono text-xs">
                        {iso}
                      </Badge>
                    ))}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

// ── Not Enabled State ─────────────────────────────────────────────────────────

function NotEnabled({ onNavigate }: { onNavigate: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center h-64 gap-4 text-center p-8">
      <Shield className="w-12 h-12 text-secondary" />
      <div>
        <p className="text-base font-medium">CIS Controls v8 nicht aktiviert</p>
        <p className="text-sm text-secondary mt-1">
          Aktivieren Sie das CIS Controls v8 Framework über die Framework-Übersicht,
          um Ihren Umsetzungsstand zu verfolgen.
        </p>
      </div>
      <Button variant="secondary" size="sm" onClick={onNavigate}>
        Zur Framework-Übersicht
      </Button>
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function CISControlsPage() {
  const navigate = useNavigate()
  const [ig, setIG] = useState<IG>('IG1')

  // Find the CIS framework from the list of enabled frameworks
  const { data: frameworks, isLoading: frameworksLoading } = useFrameworks()
  const cisFramework = frameworks?.find((f) => f.name === 'CIS')
  const frameworkId = cisFramework?.id ?? ''

  // Load controls only when we have the framework ID
  const { data: controls, isLoading: controlsLoading } = useFrameworkControls(frameworkId)
  const isLoading = frameworksLoading || (!!frameworkId && controlsLoading)

  // Group controls by CIS control group number
  const byGroup = new Map<number, Control[]>()
  for (const ctrl of controls ?? []) {
    const num = groupNum(ctrl)
    if (num > 0) {
      const list = byGroup.get(num) ?? []
      list.push(ctrl)
      byGroup.set(num, list)
    }
  }

  // Summary stats
  const allControls = controls ?? []
  const implemented = allControls.filter(
    (c) => c.status === 'covered' || c.status === 'implemented',
  ).length
  const pct = allControls.length > 0 ? Math.round((implemented / allControls.length) * 100) : 0

  function handleExportPDF() {
    if (!frameworkId) return
    const url = `/api/v1/vaktcomply/frameworks/${frameworkId}/export-pdf`
    void fetch(url, { credentials: 'include' })
      .then((r) => r.blob())
      .then((blob) => {
        const objectUrl = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = objectUrl
        a.download = `cis-controls-readiness-${new Date().toISOString().slice(0, 10)}.pdf`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(objectUrl)
      })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="CIS Controls v8"
        description="Center for Internet Security — 18 Kontrollgruppen, gegliedert nach Implementation Groups (IG1/IG2/IG3)."
        actions={
          <div className="flex items-center gap-2">
            {cisFramework && (
              <Button variant="secondary" size="sm" onClick={handleExportPDF}>
                <Download className="w-4 h-4 mr-1" />
                Readiness-Bericht (PDF)
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => { navigate('/vaktcomply/frameworks'); }}
            >
              <ArrowLeft className="w-4 h-4 mr-1" />
              Frameworks
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6 overflow-auto">
        {isLoading && (
          <div className="flex items-center justify-center h-32">
            <Spinner size="md" />
          </div>
        )}

        {!isLoading && !cisFramework && (
          <NotEnabled onNavigate={() => { navigate('/vaktcomply/frameworks'); }} />
        )}

        {!isLoading && cisFramework && (
          <>
            {/* Progress overview */}
            <div className="flex items-center gap-4 p-4 bg-surface border border-border rounded-lg">
              <Shield className="w-8 h-8 text-brand shrink-0" />
              <div className="flex-1 min-w-0">
                <div className="flex items-baseline justify-between mb-1">
                  <span className="text-sm font-medium">
                    {implemented} von {allControls.length} IG1-Safeguards umgesetzt
                  </span>
                  <span className="text-sm font-semibold text-brand">{pct}%</span>
                </div>
                <Progress value={pct} className="h-2" />
              </div>
            </div>

            {/* IG selector + content */}
            <Tabs value={ig} onValueChange={(v) => { setIG(v as IG); }}>
              <TabsList>
                {(Object.entries(IG_LABELS) as [IG, string][]).map(([level, label]) => (
                  <TabsTrigger key={level} value={level}>
                    {label}
                  </TabsTrigger>
                ))}
              </TabsList>

              {/* IG1 — primary content with control group cards and mapping tab */}
              <TabsContent value="IG1">
                <Tabs defaultValue="controls" className="mt-4">
                  <TabsList>
                    <TabsTrigger value="controls">18 Kontrollgruppen</TabsTrigger>
                    <TabsTrigger value="mapping">CIS ↔ ISO 27001 Mapping</TabsTrigger>
                  </TabsList>

                  <TabsContent value="controls">
                    <div className="space-y-3 mt-4">
                      {CIS_GROUPS.map((group) => (
                        <ControlGroupCard
                          key={group.num}
                          group={group}
                          controls={byGroup.get(group.num) ?? []}
                          frameworkId={frameworkId}
                        />
                      ))}
                    </div>
                  </TabsContent>

                  <TabsContent value="mapping">
                    <ISOMapping />
                  </TabsContent>
                </Tabs>
              </TabsContent>

              {/* IG2 and IG3 — placeholder with info */}
              {(['IG2', 'IG3'] as IG[]).map((level) => (
                <TabsContent key={level} value={level}>
                  <div className="mt-4 p-4 bg-surface border border-border rounded-lg text-sm text-secondary space-y-2">
                    <p className="font-medium text-primary">{IG_LABELS[level]}</p>
                    <p>{IG_DESCRIPTIONS[level]}</p>
                    <p>
                      {level === 'IG2'
                        ? 'IG2-Safeguards sind eine Obermenge der IG1-Safeguards. Alle IG1-Kontrollen gelten auch für IG2.'
                        : 'IG3-Safeguards umfassen alle IG1- und IG2-Controls plus weitere erweiterte Maßnahmen.'}
                    </p>
                    <p className="text-xs italic">
                      IG2/IG3-spezifische Safeguards werden in einer zukünftigen Version abgebildet.
                    </p>
                  </div>
                </TabsContent>
              ))}
            </Tabs>
          </>
        )}
      </div>
    </div>
  )
}
