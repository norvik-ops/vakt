import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'
import { Shield, ChevronRight, ChevronLeft, Printer, ExternalLink, CheckSquare, Square } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { PageHeader } from '../../../shared/components/PageHeader'
import { TermTooltip } from '../../../shared/components/TermTooltip'

// ─── Types ────────────────────────────────────────────────────────────────────

type SectorType = 'essential' | 'important' | 'none'

interface Sector {
  id: string
  label: string
  type: SectorType
  /** True = size thresholds do NOT apply; always in scope */
  alwaysInScope?: boolean
}

type NIS2Classification = 'essential' | 'important' | 'not-applicable'

interface WizardState {
  step: number
  selectedSectorId: string | null
  employees: string
  revenue: string
  checkedItems: Set<string>
}

// ─── Static Data ──────────────────────────────────────────────────────────────

const ESSENTIAL_SECTORS: Sector[] = [
  { id: 'energy',        label: 'Energie (Strom, Gas, Öl, Fernwärme, Wasserstoff)',            type: 'essential', alwaysInScope: true },
  { id: 'transport',     label: 'Transport (Luft, Schiene, Wasser, Straße)',                    type: 'essential' },
  { id: 'banking',       label: 'Bankwesen',                                                    type: 'essential' },
  { id: 'finance',       label: 'Finanzmarktinfrastrukturen',                                   type: 'essential' },
  { id: 'health',        label: 'Gesundheitswesen (Krankenhäuser, Labore, Pharma)',              type: 'essential', alwaysInScope: true },
  { id: 'water',         label: 'Trinkwasser',                                                  type: 'essential', alwaysInScope: true },
  { id: 'wastewater',    label: 'Abwasser',                                                     type: 'essential', alwaysInScope: true },
  { id: 'digital-infra', label: 'Digitale Infrastruktur (Cloud, Rechenzentren, DNS, TLD, IXP, Trust Services)', type: 'essential', alwaysInScope: true },
  { id: 'ict-services',  label: 'IKT-Dienste (B2B Managed Services)',                          type: 'essential' },
  { id: 'public-admin',  label: 'Öffentliche Verwaltung',                                      type: 'essential', alwaysInScope: true },
]

const IMPORTANT_SECTORS: Sector[] = [
  { id: 'postal',        label: 'Post- und Kurierdienste',                                     type: 'important' },
  { id: 'waste',         label: 'Abfallbewirtschaftung',                                       type: 'important' },
  { id: 'chemicals',     label: 'Chemische Stoffe',                                            type: 'important' },
  { id: 'food',          label: 'Lebensmittel',                                                type: 'important' },
  { id: 'manufacturing', label: 'Verarbeitendes Gewerbe (Medizinprodukte, DV-Erzeugnisse, Fahrzeuge, Maschinen)', type: 'important' },
  { id: 'digital-prov',  label: 'Digitale Anbieter (Suchmaschinen, Social Media, Online-Marktplätze)', type: 'important' },
  { id: 'research',      label: 'Forschung',                                                   type: 'important' },
]

const ALL_SECTORS: Sector[] = [...ESSENTIAL_SECTORS, ...IMPORTANT_SECTORS]

interface ChecklistItem {
  id: string
  text: string
  ref?: string
}

interface ChecklistSection {
  title: string
  items: ChecklistItem[]
}

const CHECKLIST: ChecklistSection[] = [
  {
    title: 'Organisatorisch',
    items: [
      { id: 'org-1', text: 'BSI-Registrierung abgeschlossen',                                            ref: 'BSIG § 33' },
      { id: 'org-2', text: 'Sicherheitsbeauftragter benannt',                                            ref: '' },
      { id: 'org-3', text: 'Managementsystem für Informationssicherheit (ISMS) implementiert',            ref: '' },
      { id: 'org-4', text: 'Risikoanalyse und -bewertung durchgeführt',                                   ref: 'Art. 21 Abs. 2a' },
    ],
  },
  {
    title: 'Technisch',
    items: [
      { id: 'tech-1',  text: 'Incident Response Plan vorhanden',                                          ref: 'Art. 21 Abs. 2b' },
      { id: 'tech-2',  text: 'Business Continuity / Notfallkonzept',                                      ref: 'Art. 21 Abs. 2c' },
      { id: 'tech-3',  text: 'Lieferkettensicherheit dokumentiert',                                       ref: 'Art. 21 Abs. 2d' },
      { id: 'tech-4',  text: 'Sicherheit in Beschaffung und Entwicklung',                                 ref: 'Art. 21 Abs. 2e' },
      { id: 'tech-5',  text: 'Schwachstellenmanagement implementiert',                                    ref: 'Art. 21 Abs. 2f' },
      { id: 'tech-6',  text: 'Wirksamkeit der Sicherheitsmaßnahmen wird geprüft',                        ref: 'Art. 21 Abs. 2g' },
      { id: 'tech-7',  text: 'Kryptographie-Richtlinie vorhanden',                                       ref: 'Art. 21 Abs. 2h' },
      { id: 'tech-8',  text: 'Zugangskontrollen und Asset Management',                                    ref: 'Art. 21 Abs. 2i' },
      { id: 'tech-9',  text: 'Multi-Faktor-Authentifizierung implementiert',                              ref: 'Art. 21 Abs. 2j' },
    ],
  },
  {
    title: 'Meldepflichten',
    items: [
      { id: 'rep-1', text: 'Meldeverfahren für erhebliche Vorfälle eingerichtet',                        ref: '' },
      { id: 'rep-2', text: '24h-Frühwarnung an BSI bei erheblichen Vorfällen bekannt',                   ref: 'Art. 23 Abs. 3' },
      { id: 'rep-3', text: '72h-Meldung mit Erstbewertung bekannt',                                       ref: 'Art. 23 Abs. 4' },
      { id: 'rep-4', text: 'Abschlussbericht innerhalb 1 Monat bekannt',                                  ref: 'Art. 23 Abs. 4' },
    ],
  },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

function getSector(id: string | null): Sector | undefined {
  if (!id) return undefined
  return ALL_SECTORS.find((s) => s.id === id)
}

function classify(
  sector: Sector | undefined,
  employees: number,
  revenue: number,
): NIS2Classification {
  if (!sector) return 'not-applicable'

  // Always-in-scope sectors ignore size thresholds
  if (sector.alwaysInScope) {
    return sector.type === 'essential' ? 'essential' : 'important'
  }

  if (sector.type === 'essential') {
    // ≥250 employees OR ≥50 M€ revenue
    if (employees >= 250 || revenue >= 50) return 'essential'
    return 'not-applicable'
  }

  // important sector: ≥50 employees OR ≥10 M€ revenue
  if (employees >= 50 || revenue >= 10) return 'important'
  return 'not-applicable'
}

// ─── Step Components ──────────────────────────────────────────────────────────

interface StepHeaderProps {
  current: number
  total: number
}

function StepProgress({ current, total }: StepHeaderProps) {
  const { t } = useTranslation()
  const pct = Math.round((current / total) * 100)
  return (
    <div className="mb-6">
      <div className="flex items-center justify-between mb-1.5">
        <span className="text-xs text-secondary">{t('nis2Assistant.stepProgress', { current, total })}</span>
        <span className="text-xs text-secondary">{pct}%</span>
      </div>
      <div className="w-full bg-border rounded-full h-1.5">
        <div
          className="bg-brand h-1.5 rounded-full transition-all duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}

interface SectorButtonProps {
  sector: Sector
  selected: boolean
  onSelect: (id: string) => void
}

function SectorButton({ sector, selected, onSelect }: SectorButtonProps) {
  return (
    <button
      type="button"
      onClick={() => { onSelect(sector.id); }}
      className={`w-full text-left px-3 py-2.5 rounded-lg border text-[13px] transition-all duration-150 flex items-center gap-2 ${
        selected
          ? 'border-brand bg-brand/10 text-brand font-medium'
          : 'border-border bg-surface text-secondary hover:border-brand/40 hover:text-primary'
      }`}
    >
      <div className={`w-3.5 h-3.5 rounded-full border-2 shrink-0 flex items-center justify-center ${
        selected ? 'border-brand' : 'border-border'
      }`}>
        {selected && <div className="w-1.5 h-1.5 rounded-full bg-brand" />}
      </div>
      {sector.label}
    </button>
  )
}

// ─── Step 1 — Sector Selection ────────────────────────────────────────────────

interface Step1Props {
  selectedSectorId: string | null
  onSelect: (id: string) => void
  onNext: () => void
}

function Step1SectorSelection({ selectedSectorId, onSelect, onNext }: Step1Props) {
  const { t } = useTranslation()
  return (
    <div>
      <h2 className="text-base font-semibold text-primary mb-1">{t('nis2Assistant.step1Title')}</h2>
      <p className="text-[12px] text-secondary mb-5">
        <TermTooltip term="NIS2" glossaryKey="NIS2">NIS2</TermTooltip> {t('nis2Assistant.step1Desc')}
      </p>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Essential sectors */}
        <div>
          <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-2">
            {t('nis2Assistant.essentialTitle')} <span className="normal-case">{t('nis2Assistant.essentialArt')}</span>
          </h3>
          <p className="text-[11px] text-secondary mb-3">{t('nis2Assistant.essentialHint')}</p>
          <div className="space-y-1.5">
            {ESSENTIAL_SECTORS.map((s) => (
              <SectorButton
                key={s.id}
                sector={s}
                selected={selectedSectorId === s.id}
                onSelect={onSelect}
              />
            ))}
          </div>
        </div>

        {/* Important sectors */}
        <div>
          <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-2">
            {t('nis2Assistant.importantTitle')} <span className="normal-case">{t('nis2Assistant.importantArt')}</span>
          </h3>
          <p className="text-[11px] text-secondary mb-3">{t('nis2Assistant.importantHint')}</p>
          <div className="space-y-1.5">
            {IMPORTANT_SECTORS.map((s) => (
              <SectorButton
                key={s.id}
                sector={s}
                selected={selectedSectorId === s.id}
                onSelect={onSelect}
              />
            ))}
            {/* None option */}
            <button
              type="button"
              onClick={() => { onSelect('none'); }}
              className={`w-full text-left px-3 py-2.5 rounded-lg border text-[13px] transition-all duration-150 flex items-center gap-2 ${
                selectedSectorId === 'none'
                  ? 'border-brand bg-brand/10 text-brand font-medium'
                  : 'border-border bg-surface text-secondary hover:border-brand/40 hover:text-primary'
              }`}
            >
              <div className={`w-3.5 h-3.5 rounded-full border-2 shrink-0 flex items-center justify-center ${
                selectedSectorId === 'none' ? 'border-brand' : 'border-border'
              }`}>
                {selectedSectorId === 'none' && <div className="w-1.5 h-1.5 rounded-full bg-brand" />}
              </div>
              {t('nis2Assistant.noneOption')}
            </button>
          </div>
        </div>
      </div>

      <div className="mt-6 flex justify-end">
        <Button onClick={onNext} disabled={!selectedSectorId}>
          {t('nis2Assistant.btnNext')}
          <ChevronRight className="w-4 h-4 ml-1" />
        </Button>
      </div>
    </div>
  )
}

// ─── Step 2 — Size Thresholds ─────────────────────────────────────────────────

interface Step2Props {
  sector: Sector | undefined
  employees: string
  revenue: string
  onEmployeesChange: (v: string) => void
  onRevenueChange: (v: string) => void
  onNext: () => void
  onBack: () => void
}

function Step2SizeClass({ sector, employees, revenue, onEmployeesChange, onRevenueChange, onNext, onBack }: Step2Props) {
  const { t } = useTranslation()
  const canProceed = employees !== '' && revenue !== ''

  return (
    <div>
      <h2 className="text-base font-semibold text-primary mb-1">{t('nis2Assistant.step2Title')}</h2>
      <p className="text-[12px] text-secondary mb-5">
        {t('nis2Assistant.step2Desc')}
      </p>

      {sector?.alwaysInScope && (
        <div className="mb-5 p-3 bg-amber-500/10 border border-amber-500/30 rounded-lg text-[12px] text-amber-400">
          {t('nis2Assistant.alwaysInScopeHint', { sector: sector.label })}
        </div>
      )}

      <div className="p-3 bg-surface border border-border rounded-lg text-[12px] text-secondary mb-5">
        {t('nis2Assistant.sizeThresholdNote')}
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-5 max-w-md">
        <div className="space-y-1.5">
          <Label htmlFor="employees" className="text-[13px]">{t('nis2Assistant.employeesLabel')}</Label>
          <Input
            id="employees"
            type="number"
            min="0"
            placeholder={t('nis2Assistant.employeesPlaceholder')}
            value={employees}
            onChange={(e) => { onEmployeesChange(e.target.value); }}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="revenue" className="text-[13px]">{t('nis2Assistant.revenueLabel')}</Label>
          <Input
            id="revenue"
            type="number"
            min="0"
            step="0.1"
            placeholder={t('nis2Assistant.revenuePlaceholder')}
            value={revenue}
            onChange={(e) => { onRevenueChange(e.target.value); }}
          />
        </div>
      </div>

      <div className="mt-5 space-y-2 text-[12px] text-secondary">
        <p className="font-medium text-primary text-[13px]">{t('nis2Assistant.thresholdsTitle')}</p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <div className="p-2.5 bg-surface border border-border rounded-lg">
            <p className="font-semibold text-primary mb-0.5">{t('nis2Assistant.essentialThresholdTitle')}</p>
            <p>{t('nis2Assistant.essentialThresholdText')}</p>
          </div>
          <div className="p-2.5 bg-surface border border-border rounded-lg">
            <p className="font-semibold text-primary mb-0.5">{t('nis2Assistant.importantThresholdTitle')}</p>
            <p>{t('nis2Assistant.importantThresholdText')}</p>
          </div>
        </div>
      </div>

      <div className="mt-6 flex items-center justify-between">
        <Button variant="outline" onClick={onBack}>
          <ChevronLeft className="w-4 h-4 mr-1" />
          {t('nis2Assistant.btnBack')}
        </Button>
        <Button onClick={onNext} disabled={!canProceed}>
          {t('nis2Assistant.btnNext')}
          <ChevronRight className="w-4 h-4 ml-1" />
        </Button>
      </div>
    </div>
  )
}

// ─── Step 3 — Result ──────────────────────────────────────────────────────────

interface Step3Props {
  classification: NIS2Classification
  sector: Sector | undefined
  onNext: () => void
  onBack: () => void
}

function Step3Result({ classification, sector, onNext, onBack }: Step3Props) {
  const { t } = useTranslation()
  const isApplicable = classification !== 'not-applicable'

  const config = {
    essential: {
      badge: <Badge variant="destructive" className="text-sm px-3 py-1">{t('nis2Assistant.essentialBadge')}</Badge>,
      title: t('nis2Assistant.essentialResultTitle'),
      description: t('nis2Assistant.essentialResultDesc'),
    },
    important: {
      badge: <Badge variant="warning" className="text-sm px-3 py-1">{t('nis2Assistant.importantBadge')}</Badge>,
      title: t('nis2Assistant.importantResultTitle'),
      description: t('nis2Assistant.importantResultDesc'),
    },
    'not-applicable': {
      badge: <Badge variant="success" className="text-sm px-3 py-1">{t('nis2Assistant.notApplicableBadge')}</Badge>,
      title: t('nis2Assistant.notApplicableResultTitle'),
      description: t('nis2Assistant.notApplicableResultDesc'),
    },
  } as const

  const { badge, title, description } = config[classification]

  return (
    <div>
      <h2 className="text-base font-semibold text-primary mb-5">{t('nis2Assistant.step3Title')}</h2>

      <div className="flex flex-col items-start gap-3 mb-5">
        {badge}
        <p className="text-[14px] font-semibold text-primary">{title}</p>
        <p className="text-[13px] text-secondary leading-relaxed">{description}</p>
      </div>

      {sector && (
        <div className="mb-5 p-3 bg-surface border border-border rounded-lg text-[12px] text-secondary">
          <span className="font-medium text-primary">{t('nis2Assistant.selectedSector')}</span> {sector.label}
        </div>
      )}

      <div className={`p-4 rounded-lg border text-[13px] ${isApplicable ? 'bg-amber-500/10 border-amber-500/30 text-amber-400' : 'bg-surface border-border text-secondary'}`}>
        <p className="font-semibold mb-1">{t('nis2Assistant.registrationTitle')}</p>
        <p>
          {t('nis2Assistant.registrationText')}{' '}
          {isApplicable && t('nis2Assistant.registrationOverdue')}
        </p>
      </div>

      <div className="mt-6 flex items-center justify-between">
        <Button variant="outline" onClick={onBack}>
          <ChevronLeft className="w-4 h-4 mr-1" />
          {t('nis2Assistant.btnBack')}
        </Button>
        {isApplicable ? (
          <Button onClick={onNext}>
            {t('nis2Assistant.btnShowChecklist')}
            <ChevronRight className="w-4 h-4 ml-1" />
          </Button>
        ) : (
          <Button variant="outline" onClick={onNext}>
            {t('nis2Assistant.btnShowChecklistOptional')}
            <ChevronRight className="w-4 h-4 ml-1" />
          </Button>
        )}
      </div>
    </div>
  )
}

// ─── Step 4 — Checklist ───────────────────────────────────────────────────────

interface Step4Props {
  classification: NIS2Classification
  checkedItems: Set<string>
  onToggle: (id: string) => void
  onBack: () => void
}

function Step4Checklist({ classification, checkedItems, onToggle, onBack }: Step4Props) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const totalItems = CHECKLIST.reduce((sum, s) => sum + s.items.length, 0)
  const checkedCount = checkedItems.size
  const pct = Math.round((checkedCount / totalItems) * 100)

  const classificationLabel =
    classification === 'essential'
      ? t('nis2Assistant.classEssential')
      : classification === 'important'
        ? t('nis2Assistant.classImportant')
        : t('nis2Assistant.classNotApplicable')

  return (
    <div>
      <div className="flex items-start justify-between mb-5">
        <div>
          <h2 className="text-base font-semibold text-primary mb-1">{t('nis2Assistant.step4Title')}</h2>
          <p className="text-[12px] text-secondary">
            {t('nis2Assistant.classificationLabel')} <span className="font-medium text-primary">{classificationLabel}</span>
          </p>
        </div>
        <div className="text-right">
          <p className="text-[22px] font-bold text-primary leading-none">{checkedCount}<span className="text-[14px] text-secondary font-normal">/{totalItems}</span></p>
          <p className="text-[11px] text-secondary">{t('nis2Assistant.checkedOf')}</p>
        </div>
      </div>

      {/* Progress bar */}
      <div className="mb-5">
        <div className="w-full bg-border rounded-full h-2">
          <div
            className={`h-2 rounded-full transition-all duration-300 ${pct === 100 ? 'bg-green-500' : 'bg-brand'}`}
            style={{ width: `${pct}%` }}
          />
        </div>
        <p className="text-[11px] text-secondary mt-1">{t('nis2Assistant.progressDone', { pct })}</p>
      </div>

      {classification === 'not-applicable' && (
        <div className="mb-4 p-3 bg-surface border border-border rounded-lg text-[12px] text-secondary">
          {t('nis2Assistant.notApplicableNote')}
        </div>
      )}

      <div className="space-y-5 print:space-y-4">
        {CHECKLIST.map((section) => (
          <div key={section.title}>
            <h3 className="text-[11px] font-semibold text-secondary uppercase tracking-wider mb-2">
              {section.title}
            </h3>
            <div className="space-y-1.5">
              {section.items.map((item) => {
                const checked = checkedItems.has(item.id)
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => { onToggle(item.id); }}
                    className="w-full flex items-start gap-3 px-3 py-2.5 rounded-lg border border-border bg-surface hover:border-brand/40 text-left transition-all duration-150 group"
                  >
                    <div className="mt-0.5 shrink-0">
                      {checked
                        ? <CheckSquare className="w-4 h-4 text-green-500" />
                        : <Square className="w-4 h-4 text-secondary group-hover:text-primary transition-colors" />
                      }
                    </div>
                    <div className="flex-1 min-w-0">
                      <span className={`text-[13px] ${checked ? 'text-secondary line-through' : 'text-primary'}`}>
                        {item.text}
                      </span>
                      {item.ref && (
                        <span className="ml-1.5 text-[11px] text-secondary">({item.ref})</span>
                      )}
                    </div>
                  </button>
                )
              })}
            </div>
          </div>
        ))}
      </div>

      <p className="mt-4 text-[11px] text-secondary italic">
        {t('nis2Assistant.checklistDisclaimer')}
      </p>

      <div className="mt-6 flex flex-wrap items-center gap-3 justify-between">
        <Button variant="outline" onClick={onBack}>
          <ChevronLeft className="w-4 h-4 mr-1" />
          {t('nis2Assistant.btnBack')}
        </Button>
        <div className="flex gap-2">
          <Button
            variant="outline"
            onClick={() => { window.print(); }}
          >
            <Printer className="w-4 h-4 mr-1.5" />
            {t('nis2Assistant.btnPrint')}
          </Button>
          <Button onClick={() => { navigate('/vaktcomply'); }}>
            {t('nis2Assistant.btnOpenComply')}
            <ExternalLink className="w-4 h-4 ml-1.5" />
          </Button>
        </div>
      </div>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function NIS2AssistantPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const { error: licenseError } = useQuery({
    queryKey: ['nis2', 'enabled'],
    queryFn: () => apiFetch<{ enabled: boolean }>('/vaktcomply/nis2/enabled'),
    retry: false,
  })

  const [state, setState] = useState<WizardState>({
    step: 1,
    selectedSectorId: null,
    employees: '',
    revenue: '',
    checkedItems: new Set(),
  })

  function goTo(step: number) {
    setState((prev) => ({ ...prev, step }))
  }

  function handleSectorSelect(id: string) {
    setState((prev) => ({ ...prev, selectedSectorId: id }))
  }

  function handleStep1Next() {
    if (!state.selectedSectorId) return
    // "none" selected → skip to result directly (step 3), skip size step
    if (state.selectedSectorId === 'none') {
      goTo(3)
      return
    }
    goTo(2)
  }

  function handleStep2Next() {
    goTo(3)
  }

  function handleStep3Next() {
    goTo(4)
  }

  function handleToggleCheck(id: string) {
    setState((prev) => {
      const next = new Set(prev.checkedItems)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return { ...prev, checkedItems: next }
    })
  }

  const sector = getSector(state.selectedSectorId)
  const employeesNum = parseFloat(state.employees) || 0
  const revenueNum = parseFloat(state.revenue) || 0

  const classification: NIS2Classification =
    state.selectedSectorId === 'none' || !state.selectedSectorId
      ? 'not-applicable'
      : classify(sector, employeesNum, revenueNum)

  // How many real steps does this flow have?
  // If sector is 'none', we skip step 2 → show steps 1, 3, 4 but label as 1-of-3
  const isShortFlow = state.selectedSectorId === 'none'
  const displayTotal = isShortFlow ? 3 : 4

  function displayStep() {
    if (state.step === 1) return 1
    if (state.step === 2) return 2
    if (state.step === 3) return isShortFlow ? 2 : 3
    return isShortFlow ? 3 : 4
  }

  return (
    <ProGate error={licenseError ?? null}>
    <div className="flex flex-col h-full">
      {/* Print styles injected inline */}
      <style>{`
        @media print {
          aside, header, .no-print { display: none !important; }
          body { background: white !important; color: black !important; }
          main { overflow: visible !important; }
        }
      `}</style>

      <PageHeader
        title={t('nis2Assistant.pageTitle')}
        description={t('nis2Assistant.pageDescription')}
        actions={
          <Button variant="outline" size="sm" onClick={() => { navigate('/vaktcomply'); }}>
            <Shield className="w-3.5 h-3.5 mr-1" />
            {t('nis2Assistant.backToComply')}
          </Button>
        }
      />

      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-4xl mx-auto">
          <Card>
            <CardHeader>
              <CardTitle className="text-[15px] flex items-center gap-2">
                <Shield className="w-4 h-4 text-brand" />
                {t('nis2Assistant.cardTitle')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <StepProgress current={displayStep()} total={displayTotal} />

              {state.step === 1 && (
                <Step1SectorSelection
                  selectedSectorId={state.selectedSectorId}
                  onSelect={handleSectorSelect}
                  onNext={handleStep1Next}
                />
              )}

              {state.step === 2 && (
                <Step2SizeClass
                  sector={sector}
                  employees={state.employees}
                  revenue={state.revenue}
                  onEmployeesChange={(v) => { setState((prev) => ({ ...prev, employees: v })); }}
                  onRevenueChange={(v) => { setState((prev) => ({ ...prev, revenue: v })); }}
                  onNext={handleStep2Next}
                  onBack={() => { goTo(1); }}
                />
              )}

              {state.step === 3 && (
                <Step3Result
                  classification={classification}
                  sector={sector}
                  onNext={handleStep3Next}
                  onBack={() => { goTo(state.selectedSectorId === 'none' ? 1 : 2); }}
                />
              )}

              {state.step === 4 && (
                <Step4Checklist
                  classification={classification}
                  checkedItems={state.checkedItems}
                  onToggle={handleToggleCheck}
                  onBack={() => { goTo(3); }}
                />
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
    </ProGate>
  )
}
