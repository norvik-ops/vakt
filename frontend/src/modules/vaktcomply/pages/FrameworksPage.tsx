import { useState } from 'react'
import { ReadOnlyHint } from '../../../shared/components/ReadOnlyHint'
import { useCanWrite } from '../../../shared/hooks/useCanWrite'
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
import { isUnsoldFeature } from '../../../shared/featureTiers'
import { FeatureLockedError } from '../../../api/client'
import { FeatureLock, FeatureLockHint } from '../../../shared/components/ProGate'
import { useLicenseStatus } from '../../../shared/hooks/useFeature'
import { downloadBlob } from '../../../shared/utils/downloadBlob'
import { FRAMEWORK_CATALOGUE, frameworkDisplayName, isNotOffered, type FrameworkCatalogueEntry } from '../utils/frameworkCatalogue'
import { ReadinessBreakdown } from '../components/ReadinessBreakdown'
import { roundPercent } from '../../../shared/utils/percent'

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
  const canWrite = useCanWrite()
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
            <CardTitle className="text-base">{frameworkDisplayName(framework.name)}</CardTitle>
            {isDORA && (
              <Badge variant={variant === 'simplified' ? 'outline' : 'secondary'} className="text-[10px]">
                {variant === 'simplified' ? 'Vereinfacht (Art. 16)' : 'Vollständig (Art. 5–15)'}
              </Badge>
            )}
          </div>
          <CardDescription className="mt-0.5">v{framework.version}</CardDescription>
        </div>
        {/* R1-KW-6: stand fest auf 0. readiness_score ist der kanonische Wert
            (ADR-0086), gerundet wie im Framework-Detail und im Dashboard. */}
        <ScoreCircle score={roundPercent(framework.readiness_score ?? 0)} />
      </CardHeader>
      <CardContent>
        {/* UX-15: der Ring allein verschwieg, dass „in Bearbeitung" halb zählt. */}
        {framework.applicable_controls !== undefined && (
          <ReadinessBreakdown
            className="block text-xs text-secondary mb-2"
            counts={{
              covered: framework.implemented_controls ?? 0,
              partial: framework.partial_controls ?? 0,
              applicable: framework.applicable_controls,
              scorePct: framework.readiness_score ?? 0,
            }}
          />
        )}
        <div className="flex items-center justify-between text-sm text-secondary">
          <span>{t('vaktcomply.controlDetailPage.activatedOn')} {enabledDate}</span>
          <div className="flex items-center gap-1">
            <ExportButton
              endpoint={`/api/v1/vaktcomply/controls/export/xlsx?framework_id=${framework.id}`}
              filename={`controls-${framework.name}-${new Date().toISOString().slice(0, 10)}`}
              label={t('common.exportControls')}
              format="xlsx"
            />
            {canWrite && isDORA && onSwitchVariant && (
              <button
                onClick={() => { onSwitchVariant(framework); }}
                className="p-1.5 rounded text-secondary hover:text-brand hover:bg-brand/10 transition-colors text-xs"
                title="DORA-Rahmenversion wechseln"
              >
                ⇄
              </button>
            )}
            {canWrite && (
              <button
                onClick={() => { onDelete(framework); }}
                className="p-1.5 rounded text-secondary hover:text-red-500 hover:bg-red-500/10 transition-colors"
                title={t('vaktcomply.frameworksPage.disableFramework')}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
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
  const canWrite = useCanWrite()
  const [deleteTarget, setDeleteTarget] = useState<Framework | null>(null)
  const [switchVariantTarget, setSwitchVariantTarget] = useState<Framework | null>(null)
  // showDORAVariantModal: DORA just activated, ask user which variant they want
  const [showDORAVariantModal, setShowDORAVariantModal] = useState(false)
  const [wizardFramework, setWizardFramework] = useState<{
    id: string
    name: string
    description?: string
  } | null>(null)
  const { data: frameworks, isLoading, isError } = useFrameworks()
  const enableFramework = useEnableFramework()
  const { data: license } = useLicenseStatus()
  const deleteFramework = useDeleteFramework()
  const switchDORAVariant = useSwitchDORAVariant()

  const enabledKeys = new Set((frameworks ?? []).map((f) => f.name.split(' ')[0].toUpperCase()))
  const isEnabled = (fw: FrameworkCatalogueEntry) =>
    enabledKeys.has(fw.key.toUpperCase()) || enabledKeys.has(fw.name.toUpperCase())
  // R1-KW-84 (Entscheid 2026-11-18): Frameworks, die Vakt in keinem Tarif verkauft
  // (TISAX, DORA, ISO 42001), stehen nicht mehr im Katalog — eine Karte, die man
  // nie aktivieren kann, ist nur Rauschen. Ist eins davon schon aktiv (Bestandsdaten),
  // bleibt die Karte stehen: Was ein Kunde aktiv hat, darf nicht verschwinden.
  const visibleCatalogue = FRAMEWORK_CATALOGUE.filter((fw) => isEnabled(fw) || !isNotOffered(fw))

  function handleExport() {
    // S131-D1 (D18-04): downloadBlob checks res.ok before saving — otherwise a
    // 500 error body was downloaded as a corrupt .zip with no error shown.
    void downloadBlob(
      '/api/v1/vaktcomply/export/audit-package',
      `audit-paket-${new Date().toISOString().slice(0, 10)}.zip`,
    )
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
        <ReadOnlyHint />
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
              action={canWrite ? (
                <Button onClick={() => {
                  document.getElementById('framework-catalogue')?.scrollIntoView({ behavior: 'smooth' })
                }}>
                  <Plus className="w-4 h-4 mr-1" />
                  Framework hinzufügen
                </Button>
              ) : undefined}
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
          {/* Aktivierung eines Pro-Frameworks ohne Lizenz (402): dieselbe Sperre wie überall. */}
          {enableFramework.error instanceof FeatureLockedError && (
            <FeatureLock kind={isUnsoldFeature(enableFramework.error.feature) ? 'unsold' : 'pro'} className="mb-4" />
          )}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {visibleCatalogue.map((fw) => {
              const alreadyEnabled = isEnabled(fw)
              // UX-12: Warum ist die Karte gesperrt? Vorher stand das nur im Tooltip
              // eines ausgegrauten Knopfs. Pro-Hinweis erst, wenn die Lizenz geladen
              // ist und das Merkmal wirklich fehlt — kein Flackern für Pro-Kunden.
              // Unverkaufte Karten erscheinen hier nur noch, wenn sie schon aktiv sind.
              const lockKind: 'pro' | 'draft' | null = alreadyEnabled
                ? null
                : fw.draft
                  ? 'draft'
                  : fw.feature && license && !(license.is_pro && license.features.includes(fw.feature))
                    ? 'pro'
                    : null
              return (
                <div
                  key={fw.key}
                  data-framework-key={fw.key}
                  className="flex flex-col gap-3 p-5 bg-surface border border-border rounded-xl"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-semibold text-primary">{fw.name}</span>
                        <Badge variant="secondary" className="text-[10px]">{fw.category}</Badge>
                        {alreadyEnabled && <Badge variant="success" className="text-[10px]">{t('vaktcomply.frameworksPage.activated')}</Badge>}
                        {fw.feature && !isUnsoldFeature(fw.feature) && (
                          <Badge variant="outline" data-testid="tier-pro" className="text-[10px] text-amber-600 border-amber-300 dark:text-amber-400 dark:border-amber-700">Pro</Badge>
                        )}
                        {fw.draft && <Badge variant="outline" className="text-[10px] text-amber-500 border-amber-500/40">{t('vaktcomply.frameworksPage.frameworkStatusDraft')}</Badge>}
                        {fw.notOffered && <Badge variant="outline" className="text-[10px] text-secondary border-border">{t('vaktcomply.frameworksPage.frameworkStatusNotOffered')}</Badge>}
                      </div>
                      <p className="text-xs text-secondary mt-0.5">{fw.fullName}</p>
                    </div>
                  </div>
                  <p className="text-xs text-secondary leading-relaxed line-clamp-2">{fw.description}</p>
                  {lockKind && <FeatureLockHint kind={lockKind} />}
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
                    ) : !canWrite ? null : fw.draft ? (
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
            {t('vaktcomply.frameworksPage.disableConfirm', { name: frameworkDisplayName(deleteTarget?.name) })}
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
