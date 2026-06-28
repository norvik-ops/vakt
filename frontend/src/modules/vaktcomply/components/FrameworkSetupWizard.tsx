import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { ShieldCheck, ListChecks, Rocket, CheckCircle2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Button } from '../../../components/ui/button'

interface FrameworkSetupWizardProps {
  framework: { id: string; name: string; description?: string; controlCount?: number }
  onClose: () => void
}

// Per-framework recommended first controls to focus on
const RECOMMENDED_CONTROLS: Record<string, { label: string; hint: string }[]> = {
  ISO27001: [
    { label: 'Zugangskontrolle (A.8.2 / A.8.3)', hint: 'Benutzerrechte verwalten und prüfen' },
    { label: 'Asset-Inventar (A.5.9)', hint: 'Alle Informationswerte erfassen' },
    { label: 'Risikobewertung (A.6.1)', hint: 'Risiken identifizieren und bewerten' },
    { label: 'Vorfallmanagement (A.5.24)', hint: 'Prozess für Sicherheitsvorfälle definieren' },
  ],
  NIS2: [
    { label: 'Zugangskontrolle', hint: 'Art. 21 Abs. 2 lit. i — Authentifizierung & Zugriff' },
    { label: 'Asset-Inventar', hint: 'Art. 21 Abs. 2 lit. a — Risikoanalyse' },
    { label: 'Vorfallmeldung', hint: 'Art. 23 — Meldepflichten für erhebliche Vorfälle' },
    { label: 'Business Continuity', hint: 'Art. 21 Abs. 2 lit. c — Betriebskontinuität' },
  ],
  BSI: [
    { label: 'ORP.4 Identitäts- und Berechtigungsmanagement', hint: 'Zugriffsrechte strukturiert verwalten' },
    { label: 'ORP.1 Organisation', hint: 'Sicherheitsorganisation aufbauen' },
    { label: 'ORP.2 Personal', hint: 'Sicherheitsmaßnahmen für Mitarbeitende' },
    { label: 'DER.2.1 Vorfallmanagement', hint: 'Vorfälle erkennen und behandeln' },
  ],
  DORA: [
    { label: 'IKT-Risikomanagement (Art. 6–16)', hint: 'Rahmenwerk für IKT-Risiken aufbauen' },
    { label: 'Vorfallmeldung (Art. 17–23)', hint: 'Meldeprozesse für IKT-Vorfälle definieren' },
    { label: 'Resilienztests (Art. 24–27)', hint: 'TLPT- und digitale Resilienztests planen' },
    { label: 'Drittparteienrisiko (Art. 28–44)', hint: 'IKT-Dienstleister registrieren und überwachen' },
  ],
  EUAIACT: [
    { label: 'Risikoklassifizierung (Art. 6 / Annex III)', hint: 'KI-Systeme nach Risikostufe einordnen' },
    { label: 'Technische Dokumentation (Art. 11)', hint: 'Dokumentation für Hochrisiko-KI erstellen' },
    { label: 'Menschliche Aufsicht (Art. 14)', hint: 'Kontrollmechanismen für KI-Entscheidungen' },
    { label: 'Transparenz (Art. 13)', hint: 'Nutzerinformation und Erklärbarkeit sicherstellen' },
  ],
  TISAX: [
    { label: 'Informationssicherheitsrichtlinie (1.1)', hint: 'Richtlinien und Verantwortlichkeiten festlegen' },
    { label: 'Zugangskontrolle (1.3)', hint: 'Zugangsberechtigungen steuern' },
    { label: 'Asset-Management (5.2)', hint: 'Informationswerte klassifizieren' },
    { label: 'Vorfallmanagement (1.6)', hint: 'Sicherheitsvorfälle behandeln' },
  ],
  ISO42001: [
    { label: 'AI-Governance (A.2.2)', hint: 'Verantwortlichkeiten für KI-Systeme definieren' },
    { label: 'Risikobewertung KI (A.4.2)', hint: 'KI-spezifische Risiken identifizieren' },
    { label: 'Datenmanagement (A.8.2)', hint: 'Trainingsdaten qualitätssichern' },
    { label: 'Transparenz (A.6.2)', hint: 'Nachvollziehbarkeit von KI-Entscheidungen' },
  ],
  CRA: [
    { label: 'Sicherheitsanforderungen (Annex I)', hint: 'Mindestanforderungen für Produktsicherheit' },
    { label: 'SBOM (Art. 13 Abs. 3)', hint: 'Software-Stückliste erstellen und pflegen' },
    { label: 'Schwachstellenmanagement (Art. 13)', hint: 'Patches bereitstellen und dokumentieren' },
    { label: 'Vorfallmeldung (Art. 14)', hint: 'Aktiv ausgenutzte Schwachstellen melden' },
  ],
}

// Derive a framework key from the name for catalogue lookup
function deriveFrameworkKey(name: string): string {
  const n = name.toUpperCase()
  if (n.includes('ISO') && n.includes('42001')) return 'ISO42001'
  if (n.includes('ISO') && n.includes('27001')) return 'ISO27001'
  if (n.includes('NIS')) return 'NIS2'
  if (n.includes('BSI')) return 'BSI'
  if (n.includes('DORA')) return 'DORA'
  if (n.includes('AI ACT') || n.includes('EUAIACT') || n.includes('EU AI')) return 'EUAIACT'
  if (n.includes('TISAX')) return 'TISAX'
  if (n.includes('CRA') || n.includes('CYBER RESILIENCE')) return 'CRA'
  return ''
}

const DEFAULT_RECOMMENDED_CONTROLS = [
  { label: 'Zugangskontrolle', hint: 'Benutzerrechte und Authentifizierung' },
  { label: 'Asset-Inventar', hint: 'Alle Informationswerte erfassen' },
  { label: 'Risikobewertung', hint: 'Risiken identifizieren und bewerten' },
  { label: 'Vorfallmanagement', hint: 'Prozess für Sicherheitsvorfälle definieren' },
]

export function FrameworkSetupWizard({ framework, onClose }: FrameworkSetupWizardProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [step, setStep] = useState<1 | 2 | 3>(1)

  const fwKey = deriveFrameworkKey(framework.name)
  const recommendedControls = RECOMMENDED_CONTROLS[fwKey] ?? DEFAULT_RECOMMENDED_CONTROLS

  function handleDone() {
    onClose()
    navigate(`/vaktcomply/frameworks/${framework.id}`)
  }

  function handleSkip() {
    onClose()
  }

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent className="max-w-lg">
        {/* Step indicator */}
        <div className="flex items-center gap-1.5 mb-2" aria-label={`Schritt ${step} von 3`}>
          {([1, 2, 3] as const).map((s) => (
            <div
              key={s}
              className={`h-1 rounded-full flex-1 transition-colors duration-300 ${
                s <= step ? 'bg-brand' : 'bg-border'
              }`}
            />
          ))}
        </div>

        {/* ── Step 1: Welcome ── */}
        {step === 1 && (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2 text-xl">
                <ShieldCheck className="w-5 h-5 text-brand" aria-hidden="true" />
                Willkommen bei {framework.name}
              </DialogTitle>
            </DialogHeader>

            <div className="space-y-4 py-2">
              {framework.description && (
                <p className="text-sm text-secondary leading-relaxed">{framework.description}</p>
              )}

              {framework.controlCount != null && framework.controlCount > 0 && (
                <div className="flex items-center gap-3 p-3 rounded-lg bg-surface2 border border-border">
                  <ListChecks className="w-5 h-5 text-brand shrink-0" aria-hidden="true" />
                  <div>
                    <p className="text-sm font-medium text-primary">
                      {framework.controlCount} Controls enthalten
                    </p>
                    <p className="text-xs text-secondary mt-0.5">
                      Strukturiert nach Kategorien, bereit zur Bewertung
                    </p>
                  </div>
                </div>
              )}

              <p className="text-sm text-secondary">
                Dieser Assistent führt Sie durch die ersten Schritte — in unter 3 Minuten.
              </p>
            </div>

            <DialogFooter className="mt-4">
              <Button variant="ghost" size="sm" onClick={handleSkip}>
                Später einrichten
              </Button>
              <Button onClick={() => { setStep(2); }}>
                Erste Controls anzeigen
              </Button>
            </DialogFooter>
          </>
        )}

        {/* ── Step 2: Prioritisation ── */}
        {step === 2 && (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2 text-xl">
                <ListChecks className="w-5 h-5 text-brand" aria-hidden="true" />
                Priorisierung
              </DialogTitle>
            </DialogHeader>

            <div className="space-y-4 py-2">
              <p className="text-sm text-secondary">
                Starten Sie mit diesen Controls — sie decken die wichtigsten Sicherheitsbereiche ab:
              </p>

              <ul className="space-y-2" role="list">
                {recommendedControls.map((ctrl) => (
                  <li
                    key={ctrl.label}
                    className="flex items-start gap-2.5 p-2.5 rounded-lg bg-surface2 border border-border"
                  >
                    <CheckCircle2 className="w-4 h-4 text-brand shrink-0 mt-0.5" aria-hidden="true" />
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-primary">{ctrl.label}</p>
                      <p className="text-xs text-secondary mt-0.5">{ctrl.hint}</p>
                    </div>
                  </li>
                ))}
              </ul>

              <p className="text-sm text-secondary leading-relaxed">
                Bewerten Sie den Status jedes Controls:{' '}
                <span className="text-primary font-medium">{t('vaktcomply.frameworkSetupWizard.implemented')}</span>,{' '}
                <span className="text-primary font-medium">In Bearbeitung</span> oder{' '}
                <span className="text-primary font-medium">{t('vaktcomply.frameworkSetupWizard.notApplicable')}</span>.
              </p>
            </div>

            <DialogFooter className="mt-4">
              <Button variant="ghost" size="sm" onClick={() => { setStep(1); }}>
                Zurück
              </Button>
              <Button onClick={() => { setStep(3); }}>
                Verstanden, weiter
              </Button>
            </DialogFooter>
          </>
        )}

        {/* ── Step 3: Ready ── */}
        {step === 3 && (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2 text-xl">
                <Rocket className="w-5 h-5 text-brand" aria-hidden="true" />
                Los geht's!
              </DialogTitle>
            </DialogHeader>

            <div className="space-y-4 py-2">
              <p className="text-sm text-secondary">
                Ihr Framework <span className="text-primary font-medium">{framework.name}</span> ist
                bereit. Jetzt geht es um die erste Bewertung.
              </p>

              {/* Score progress indicator */}
              <div className="p-4 rounded-lg bg-surface2 border border-border space-y-2">
                <div className="flex items-center justify-between text-sm">
                  <span className="text-secondary">Compliance-Score</span>
                  <span className="font-semibold text-primary">0 %</span>
                </div>
                <div
                  role="progressbar"
                  aria-valuenow={0}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-label="Compliance-Fortschritt: 0 %"
                  className="h-2 rounded-full bg-border overflow-hidden"
                >
                  <div
                    className="h-full rounded-full bg-brand transition-all duration-700"
                    style={{ width: '0%' }}
                  />
                </div>
                <p className="text-xs text-secondary">
                  Beginnen Sie mit der Bewertung, um Ihren Score zu steigern.
                </p>
              </div>

              <div className="flex items-start gap-2.5 p-3 rounded-lg border border-brand/30 bg-brand/5">
                <span className="text-base" aria-hidden="true">💡</span>
                <p className="text-xs text-secondary leading-relaxed">
                  <span className="text-primary font-medium">{t('vaktcomply.frameworkSetupWizard.tip')}</span>{' '}
                  {t('vaktcomply.frameworkSetupWizard.tipText')}
                </p>
              </div>
            </div>

            <DialogFooter className="mt-4">
              <Button variant="ghost" size="sm" onClick={handleSkip}>
                Schließen
              </Button>
              <Button onClick={handleDone}>
                Controls jetzt bewerten
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
