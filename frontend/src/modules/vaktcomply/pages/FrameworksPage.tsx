import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ShieldCheck, Plus, BookOpen, Trash2, Download } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useTranslation } from 'react-i18next'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ExportButton } from '../../../shared/components/ExportButton'
import { EmptyState } from '../../../shared/components/EmptyState'
import { useFrameworks, useEnableFramework, useDeleteFramework, useSwitchDORAVariant } from '../hooks/useFrameworks'
import { FrameworkSetupWizard } from '../components/FrameworkSetupWizard'
import type { Framework } from '../types'
import { formatLocale } from '../../../shared/utils/locale'

// Pre-defined compliance frameworks users can enable with one click
const FRAMEWORK_CATALOGUE: Array<{
  key: string
  name: string
  fullName: string
  description: string
  category: string
  controls: string
  color: string
  draft?: boolean
}> = [
  {
    key: 'NIS2',
    name: 'NIS2',
    fullName: 'NIS-2-Richtlinie (EU) 2022/2555',
    description: 'EU-weite Richtlinie zur Netz- und Informationssicherheit. Verbindlich für wesentliche und wichtige Einrichtungen ab Dezember 2026 (NIS2UmsuCG).',
    category: 'EU-Recht',
    controls: '~90 Maßnahmen',
    color: 'text-blue-500',
  },
  {
    key: 'ISO27001',
    name: 'ISO 27001',
    fullName: 'ISO/IEC 27001:2022',
    description: 'Internationaler Standard für Informationssicherheits-Management. Grundlage für Zertifizierungen und die Nachweisbarkeit gegenüber Kunden.',
    category: 'International',
    controls: '93 Controls (Annex A)',
    color: 'text-green-500',
  },
  {
    key: 'BSI',
    name: 'BSI IT-Grundschutz',
    fullName: 'BSI IT-Grundschutz-Kompendium 2023',
    description: 'Bewährter deutscher Standard des Bundesamts für Sicherheit in der Informationstechnik. Enthält detaillierte Bausteine und Umsetzungshinweise.',
    category: 'Deutschland',
    controls: '111 Bausteine',
    color: 'text-yellow-500',
  },
  {
    key: 'DORA',
    name: 'DORA',
    fullName: 'Digital Operational Resilience Act (EU) 2022/2554',
    description: 'Verbindlich seit Januar 2025 für Banken, Versicherungen, Zahlungsdienstleister und deren kritische IKT-Drittanbieter. Regelt digitale operationale Resilienz und Vorfallmeldepflichten.',
    category: 'EU-Recht / Finanz',
    controls: '15 Controls (Kap. II–V)',
    color: 'text-orange-500',
    // Aus dem Angebot genommen (v0.42.20), Backend gated auf draft-Status
    // (plugins.go builtinAvailable) — gleicher Grund wie bei TISAX.
    draft: true,
  },
  {
    key: 'EUAIACT',
    name: 'EU AI Act',
    fullName: 'EU AI Act — Verordnung (EU) 2024/1689',
    description: 'Neue EU-Verordnung für KI-Systeme. Hochrisiko-KI-Anforderungen ab August 2026. Betrifft jeden, der KI-Systeme in der EU entwickelt, betreibt oder einsetzt.',
    category: 'EU-Recht / KI',
    controls: '17 Controls (Annex III/IV)',
    color: 'text-purple-500',
  },
  {
    key: 'TISAX',
    name: 'TISAX® / VDA ISA',
    fullName: 'TISAX® — Trusted Information Security Assessment Exchange (VDA ISA 6.0)',
    description: 'Verbindlicher Informationssicherheitsstandard der Automobilindustrie. Pflicht für Zulieferer mit Zugang zu sensitiven OEM-Daten (BMW, Mercedes, VW, Bosch, Continental).',
    category: 'Automotive',
    controls: '39 Controls (Kap. 1–15)',
    color: 'text-red-500',
    // Aus dem Angebot genommen (v0.42.20), Backend gated auf draft-Status
    // (plugins.go builtinAvailable) — Katalog-Eintrag muss das spiegeln, sonst
    // zeigt der Button "Aktivieren" obwohl das Backend jede Aktivierung ablehnt.
    draft: true,
  },
  {
    key: 'ISO42001',
    name: 'ISO 42001',
    fullName: 'ISO/IEC 42001:2023',
    description: 'KI-Managementsystem-Standard für verantwortungsvolle Entwicklung und Nutzung von KI. Ergänzt den EU AI Act mit einem strukturierten Managementrahmen.',
    category: 'International / KI',
    controls: '16 Controls',
    color: 'text-cyan-500',
  },
  {
    key: 'CRA',
    name: 'EU CRA',
    fullName: 'EU Cyber Resilience Act (EU) 2024/2847',
    description: 'Sicherheitsanforderungen für Produkte mit digitalen Elementen. Gilt für Hersteller und Händler in der EU. SBOM, Patch-Management und Meldepflichten verpflichtend.',
    category: 'EU-Recht / Produkte',
    controls: '13 Controls',
    color: 'text-indigo-500',
  },
  {
    key: 'prEN18286',
    name: 'prEN 18286',
    fullName: 'prEN 18286 — EU AI Act harmonisierter Standard',
    description: 'Harmonisierter Standard zum EU AI Act für KI-Managementsysteme. Komplementär zu ISO/IEC 42001:2023. Publikation erwartet Ende 2026 — Entwurf bereits in Vakt hinterlegt.',
    category: 'EU-Recht / KI',
    controls: '8 Abschnitte (Entwurf)',
    color: 'text-amber-500',
    draft: true,
  },
]

function ScoreCircle({ score }: { score: number }) {
  const radius = 28
  const circumference = 2 * Math.PI * radius
  const progress = circumference - (score / 100) * circumference
  const color = score >= 80 ? '#16a34a' : score >= 50 ? '#ca8a04' : '#dc2626'

  return (
    <div className="relative inline-flex items-center justify-center">
      <svg width="72" height="72" className="-rotate-90">
        <circle cx="36" cy="36" r={radius} fill="none" stroke="#2d3148" strokeWidth="6" />
        <circle
          cx="36" cy="36" r={radius} fill="none"
          stroke={color} strokeWidth="6"
          strokeDasharray={circumference} strokeDashoffset={progress}
          strokeLinecap="round"
        />
      </svg>
      <span className="absolute text-sm font-bold" style={{ color }}>{score}%</span>
    </div>
  )
}

function EnabledFrameworkCard({ framework, onDelete, onSwitchVariant }: {
  framework: Framework
  onDelete: (fw: Framework) => void
  onSwitchVariant?: (fw: Framework) => void
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const enabledDate = new Date(framework.created_at).toLocaleDateString(formatLocale(), {
    year: 'numeric', month: 'short', day: 'numeric',
  })
  const isDORA = framework.name === 'DORA'
  const variant = framework.framework_variant ?? 'full'

  return (
    <Card className="hover:border-brand transition-colors">
      <CardHeader className="flex flex-row items-start justify-between pb-2">
        <div
          className="flex-1 cursor-pointer"
          onClick={() => { navigate(`/vaktcomply/frameworks/${framework.id}`); }}
        >
          <div className="flex items-center gap-2 flex-wrap">
            <CardTitle className="text-base">{framework.name}</CardTitle>
            {isDORA && (
              <Badge variant={variant === 'simplified' ? 'outline' : 'secondary'} className="text-[10px]">
                {variant === 'simplified' ? 'Vereinfacht (Art. 16)' : 'Vollständig (Art. 5–15)'}
              </Badge>
            )}
          </div>
          <CardDescription className="mt-0.5">v{framework.version}</CardDescription>
        </div>
        <ScoreCircle score={0} />
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between text-sm text-secondary">
          <span>{framework.control_count != null ? `${framework.control_count} ${t('vaktcomply.controlDetailPage.controlsCount')} · ` : ''}{t('vaktcomply.controlDetailPage.activatedOn')} {enabledDate}</span>
          <div className="flex items-center gap-1">
            {isDORA && onSwitchVariant && (
              <button
                onClick={() => { onSwitchVariant(framework); }}
                className="p-1.5 rounded text-secondary hover:text-brand hover:bg-brand/10 transition-colors text-xs"
                title="DORA-Rahmenversion wechseln"
              >
                ⇄
              </button>
            )}
            <button
              onClick={() => { onDelete(framework); }}
              className="p-1.5 rounded text-secondary hover:text-red-500 hover:bg-red-500/10 transition-colors"
              title={t('vaktcomply.frameworksPage.disableFramework')}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// Wizard localStorage key per framework
function wizardSeenKey(frameworkId: string) {
  return `vakt_wizard_seen_${frameworkId}`
}

export default function FrameworksPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [deleteTarget, setDeleteTarget] = useState<Framework | null>(null)
  const [switchVariantTarget, setSwitchVariantTarget] = useState<Framework | null>(null)
  // showDORAVariantModal: DORA just activated, ask user which variant they want
  const [showDORAVariantModal, setShowDORAVariantModal] = useState(false)
  const [wizardFramework, setWizardFramework] = useState<{
    id: string
    name: string
    description?: string
    controlCount?: number
  } | null>(null)
  const { data: frameworks, isLoading, isError } = useFrameworks()
  const enableFramework = useEnableFramework()
  const deleteFramework = useDeleteFramework()
  const switchDORAVariant = useSwitchDORAVariant()

  const enabledKeys = new Set((frameworks ?? []).map((f) => f.name.split(' ')[0].toUpperCase()))

  function handleExport() {
    void fetch('/api/v1/vaktcomply/export/audit-package', {
      credentials: 'include',
    })
      .then((r) => r.blob())
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `audit-paket-${new Date().toISOString().slice(0, 10)}.zip`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      })
  }

  function handleEnable(key: string) {
    const catalogueEntry = FRAMEWORK_CATALOGUE.find((fw) => fw.key === key)
    if (key === 'DORA') {
      // For DORA, show variant selection modal before enabling
      setShowDORAVariantModal(true)
      return
    }
    enableFramework.mutate({ name: key }, {
      onSuccess: (activatedFramework) => {
        if (localStorage.getItem(wizardSeenKey(activatedFramework.id)) !== '1') {
          setWizardFramework({
            id: activatedFramework.id,
            name: activatedFramework.name,
            description: catalogueEntry?.description,
            controlCount: activatedFramework.control_count ?? undefined,
          })
        }
      },
    })
  }

  function handleEnableDORAWithVariant(variant: 'full' | 'simplified') {
    setShowDORAVariantModal(false)
    const catalogueEntry = FRAMEWORK_CATALOGUE.find((fw) => fw.key === 'DORA')
    enableFramework.mutate({ name: 'DORA', variant }, {
      onSuccess: (activatedFramework) => {
        if (localStorage.getItem(wizardSeenKey(activatedFramework.id)) !== '1') {
          setWizardFramework({
            id: activatedFramework.id,
            name: activatedFramework.name,
            description: catalogueEntry?.description,
            controlCount: activatedFramework.control_count ?? undefined,
          })
        }
      },
    })
  }

  function handleSwitchVariantConfirm() {
    if (!switchVariantTarget) return
    const current = switchVariantTarget.framework_variant ?? 'full'
    const next: 'full' | 'simplified' = current === 'full' ? 'simplified' : 'full'
    switchDORAVariant.mutate(next, {
      onSuccess: () => { setSwitchVariantTarget(null); },
    })
  }

  function handleConfirmDelete() {
    if (!deleteTarget) return
    deleteFramework.mutate(deleteTarget.id, { onSuccess: () => { setDeleteTarget(null); } })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktcomply.frameworksPage.title')}
        description={t('vaktcomply.frameworksPage.description')}
        actions={
          <div className="flex items-center gap-2">
            <ExportButton
              endpoint="/api/v1/vaktcomply/controls/export/xlsx"
              filename={`controls-${new Date().toISOString().slice(0, 10)}`}
              label={t('common.exportControls')}
              format="xlsx"
            />
            <Button variant="outline" size="sm" onClick={handleExport}>
              <Download className="w-3.5 h-3.5 mr-1" />
              {t('vaktcomply.frameworksPage.exportAuditPackage')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => { navigate('/vaktcomply'); }}>
              {t('vaktcomply.frameworksPage.backToOverview')}
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-8">
        {/* Enabled Frameworks */}
        <section>
          <h2 className="text-sm font-semibold text-secondary uppercase tracking-wider mb-3">
            {t('vaktcomply.frameworksPage.activatedFrameworks')}
          </h2>

          {isLoading && (
            <div className="flex items-center justify-center h-24">
              <Spinner size="md" />
            </div>
          )}
          {isError && (
            <p className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
              {t('vaktcomply.frameworksPage.loadError')}
            </p>
          )}
          {!isLoading && !isError && frameworks && frameworks.length === 0 && (
            <EmptyState
              icon={ShieldCheck}
              title="Noch kein Compliance-Framework aktiv"
              description="Starte mit ISO 27001 — dem Standard für KMU in der DACH-Region"
              action={
                <Button onClick={() => {
                  document.getElementById('framework-catalogue')?.scrollIntoView({ behavior: 'smooth' })
                }}>
                  <Plus className="w-4 h-4 mr-1" />
                  Framework hinzufügen
                </Button>
              }
            />
          )}
          {!isLoading && !isError && frameworks && frameworks.length > 0 && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {frameworks.map((fw) => (
                <EnabledFrameworkCard
                  key={fw.id}
                  framework={fw}
                  onDelete={setDeleteTarget}
                  onSwitchVariant={fw.name === 'DORA' ? setSwitchVariantTarget : undefined}
                />
              ))}
            </div>
          )}
        </section>

        {/* Framework Catalogue */}
        <section id="framework-catalogue">
          <div className="flex items-center gap-2 mb-3">
            <BookOpen className="w-4 h-4 text-secondary" />
            <h2 className="text-sm font-semibold text-secondary uppercase tracking-wider">
              {t('vaktcomply.frameworksPage.frameworkCatalogue')}
            </h2>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {FRAMEWORK_CATALOGUE.map((fw) => {
              const alreadyEnabled = enabledKeys.has(fw.key.toUpperCase()) ||
                enabledKeys.has(fw.name.toUpperCase())
              return (
                <div
                  key={fw.key}
                  className="flex flex-col gap-3 p-5 bg-surface border border-border rounded-xl"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-semibold text-primary">{fw.name}</span>
                        <Badge variant="secondary" className="text-[10px]">{fw.category}</Badge>
                        {alreadyEnabled && <Badge variant="success" className="text-[10px]">{t('vaktcomply.frameworksPage.activated')}</Badge>}
                        {fw.draft && <Badge variant="outline" className="text-[10px] text-amber-500 border-amber-500/40">{t('vaktcomply.frameworksPage.frameworkStatusDraft')}</Badge>}
                      </div>
                      <p className="text-xs text-secondary mt-0.5">{fw.fullName}</p>
                    </div>
                  </div>
                  <p className="text-xs text-secondary leading-relaxed line-clamp-2">{fw.description}</p>
                  <div className="flex items-center justify-between mt-1">
                    <span className="text-xs text-secondary">{fw.controls}</span>
                    {alreadyEnabled ? (
                      <div className="flex items-center gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            const match = frameworks?.find(
                              (f) => f.name.toUpperCase().startsWith(fw.key.toUpperCase()),
                            )
                            if (match) navigate(`/vaktcomply/frameworks/${match.id}`)
                          }}
                        >
                          {t('vaktcomply.frameworksPage.view')}
                        </Button>
                        {fw.key === 'DORA' && (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              const match = frameworks?.find(
                                (f) => f.name.toUpperCase().startsWith(fw.key.toUpperCase()),
                              )
                              if (match) navigate(`/vaktcomply/dora/${match.id}`)
                            }}
                          >
                            {t('vaktcomply.frameworksPage.doraArticles')}
                          </Button>
                        )}
                      </div>
                    ) : fw.draft ? (
                      <Button
                        size="sm"
                        disabled
                        title={t('vaktcomply.frameworksPage.frameworkStatusDraft')}
                      >
                        <Plus className="w-3.5 h-3.5 mr-1" />
                        {t('vaktcomply.frameworksPage.activate')}
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        onClick={() => { handleEnable(fw.key); }}
                        disabled={enableFramework.isPending}
                      >
                        <Plus className="w-3.5 h-3.5 mr-1" />
                        {t('vaktcomply.frameworksPage.activate')}
                      </Button>
                    )}
                  </div>
                </div>
              )
            })}
          </div>

          <p className="text-xs text-secondary mt-4">
            {t('vaktcomply.frameworksPage.moreFrameworks')}
          </p>
        </section>
      </div>

      {/* Framework Setup Wizard — shown once per framework after activation */}
      {wizardFramework && (
        <FrameworkSetupWizard
          framework={wizardFramework}
          onClose={() => {
            localStorage.setItem(wizardSeenKey(wizardFramework.id), '1')
            setWizardFramework(null)
          }}
        />
      )}

      {/* DORA Variant Selection Modal — shown on first DORA activation */}
      <Dialog open={showDORAVariantModal} onOpenChange={(open) => { if (!open) setShowDORAVariantModal(false) }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>DORA-Rahmenwerk aktivieren</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-secondary py-2">
            Wähle den für dein Institut zutreffenden DORA-Anwendungsrahmen.
          </p>
          <div className="space-y-3">
            <button
              className="w-full text-left p-4 rounded-lg border border-border hover:border-brand hover:bg-brand/5 transition-colors"
              onClick={() => { handleEnableDORAWithVariant('full'); }}
            >
              <p className="font-semibold text-sm text-primary">Vollständiger Rahmen (Art. 5–15)</p>
              <p className="text-xs text-secondary mt-1">Für bedeutende Banken, große Versicherungen und systemrelevante Finanzinstitute. 23 Controls mit TLPT-Anforderungen und vollständigem Drittparteien-Register.</p>
            </button>
            <button
              className="w-full text-left p-4 rounded-lg border border-border hover:border-brand hover:bg-brand/5 transition-colors"
              onClick={() => { handleEnableDORAWithVariant('simplified'); }}
            >
              <p className="font-semibold text-sm text-primary">Vereinfachter Rahmen (Art. 16)</p>
              <p className="text-xs text-secondary mt-1">Für kleine und nicht-verflochtene Finanzinstitute gem. DORA Art. 16 (RTS EU 2024/1774 Kap. II). 15 Controls, kein TLPT erforderlich. Du kannst später zum vollständigen Rahmen wechseln.</p>
            </button>
          </div>
          <DialogFooter className="mt-2">
            <Button variant="outline" onClick={() => { setShowDORAVariantModal(false); }}>{t('common.cancel')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* DORA Variant Switch Confirmation */}
      <Dialog open={!!switchVariantTarget} onOpenChange={(open) => { if (!open) setSwitchVariantTarget(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>DORA-Rahmenversion wechseln</DialogTitle>
          </DialogHeader>
          {switchVariantTarget && (
            <p className="text-sm text-secondary py-2">
              {switchVariantTarget.framework_variant === 'full'
                ? 'Wechsel vom vollständigen zum vereinfachten Rahmen (Art. 16): Die vollständigen Controls (DORA-1.x–5.x) werden als „nicht anwendbar" markiert. Vorhandene Nachweise bleiben erhalten. 15 vereinfachte Controls (DORA-S.*) werden angelegt.'
                : 'Wechsel vom vereinfachten zum vollständigen Rahmen (Art. 5–15): Die vereinfachten Controls (DORA-S.*) werden als „nicht anwendbar" markiert. Vorhandene Nachweise bleiben erhalten. Vollständige DORA-Controls werden als „nicht implementiert" angelegt.'}
            </p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => { setSwitchVariantTarget(null); }}>{t('common.cancel')}</Button>
            <Button
              onClick={handleSwitchVariantConfirm}
              disabled={switchDORAVariant.isPending}
            >
              {switchDORAVariant.isPending ? 'Wird gewechselt…' : 'Rahmenversion wechseln'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <Dialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.frameworksPage.disableFramework')}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-secondary py-2">
            {t('vaktcomply.frameworksPage.disableConfirm', { name: deleteTarget?.name ?? '' })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteTarget(null); }}>{t('common.cancel')}</Button>
            <Button
              variant="destructive"
              onClick={handleConfirmDelete}
              disabled={deleteFramework.isPending}
            >
              {deleteFramework.isPending ? t('vaktcomply.frameworksPage.disabling') : t('vaktcomply.frameworksPage.confirmDisable')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
