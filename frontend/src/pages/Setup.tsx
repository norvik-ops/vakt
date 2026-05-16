import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiFetch } from '../api/client'

// ── Types ─────────────────────────────────────────────────────────────────────

interface SetupPayload {
  org_name: string
  admin_email: string
  admin_password: string
  modules_enabled: string[]
  smtp_host?: string
  smtp_port?: string
}

interface SetupResponse {
  org_id: string
  user_id: string
  message: string
}

const ALL_MODULES = ['secpulse', 'secvitals', 'secvault', 'secreflex', 'secprivacy'] as const

// ── Step components ───────────────────────────────────────────────────────────

interface Step1Props {
  orgName: string
  onChange: (v: string) => void
  onNext: () => void
}

function Step1OrgName({ orgName, onChange, onNext }: Step1Props) {
  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Schritt 1 von 3 — Organisation</h2>
        <p className="text-secondary text-sm mt-1">
          Geben Sie den Namen Ihrer Organisation ein. Dieser wird nur zur Anzeige verwendet.
        </p>
      </div>
      <div className="space-y-1">
        <label htmlFor="org_name" className="block text-sm font-medium">
          Organisationsname
        </label>
        <input
          id="org_name"
          type="text"
          value={orgName}
          onChange={(e) => onChange(e.target.value)}
          placeholder="Muster GmbH"
          className="w-full border border-border rounded px-3 py-2 text-sm bg-surface2 text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand"
          autoFocus
        />
      </div>
      <button
        onClick={onNext}
        disabled={orgName.trim().length < 2}
        className="w-full bg-brand text-white py-2 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed"
      >
        Weiter
      </button>
    </div>
  )
}

interface Step2Props {
  email: string
  password: string
  onChangeEmail: (v: string) => void
  onChangePassword: (v: string) => void
  onBack: () => void
  onNext: () => void
}

function Step2AdminAccount({
  email,
  password,
  onChangeEmail,
  onChangePassword,
  onBack,
  onNext,
}: Step2Props) {
  const valid = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) && password.length >= 8

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Schritt 2 von 3 — Admin-Konto</h2>
        <p className="text-secondary text-sm mt-1">
          Erstellen Sie das initiale Administrator-Konto. Weitere Benutzer können nach der Einrichtung hinzugefügt werden.
        </p>
      </div>
      <div className="space-y-1">
        <label htmlFor="admin_email" className="block text-sm font-medium">
          E-Mail-Adresse
        </label>
        <input
          id="admin_email"
          type="email"
          value={email}
          onChange={(e) => onChangeEmail(e.target.value)}
          placeholder="admin@example.com"
          className="w-full border border-border rounded px-3 py-2 text-sm bg-surface2 text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand"
          autoFocus
        />
      </div>
      <div className="space-y-1">
        <label htmlFor="admin_password" className="block text-sm font-medium">
          Passwort <span className="text-secondary font-normal">(mind. 8 Zeichen)</span>
        </label>
        <input
          id="admin_password"
          type="password"
          value={password}
          onChange={(e) => onChangePassword(e.target.value)}
          placeholder="••••••••"
          className="w-full border border-border rounded px-3 py-2 text-sm bg-surface2 text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand"
        />
      </div>
      <div className="flex gap-2">
        <button
          onClick={onBack}
          className="flex-1 border border-border py-2 rounded text-sm font-medium text-primary hover:bg-surface2"
        >
          Zurück
        </button>
        <button
          onClick={onNext}
          disabled={!valid}
          className="flex-1 bg-brand text-white py-2 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Weiter
        </button>
      </div>
    </div>
  )
}

interface Step3Props {
  modules: string[]
  onToggle: (module: string) => void
  onBack: () => void
  onSubmit: () => void
  submitting: boolean
  error: string | null
}

function Step3Modules({ modules, onToggle, onBack, onSubmit, submitting, error }: Step3Props) {
  const labels: Record<string, { label: string; description: string }> = {
    secpulse:  {
      label: 'Vakt Scan — Schwachstellenmanagement',
      description: 'Orchestriert bestehende Scanner (Trivy, Nuclei, OpenVAS), dedupliziert Findings und priorisiert nach Risiko. Ergebnisse fließen automatisch als Compliance-Nachweis in Vakt Comply.',
    },
    secvitals:  {
      label: 'Vakt Comply — Compliance & Governance',
      description: 'Führt durch NIS2, ISO 27001 und BSI-Grundschutz. Verwaltet Controls, Lücken und Fortschritt. Speichert versionierte Nachweise und erstellt prüfungsfertige Dokumentation.',
    },
    secvault:  {
      label: 'Vakt Vault — Secrets & Git-Scanning',
      description: 'Sichere Secrets-Verwaltung mit AES-256-GCM-Verschlüsselung. Scannt Git-Repositories auf durchgesickerte Zugangsdaten und unterstützt automatische Rotation.',
    },
    secreflex: {
      label: 'Vakt Aware — Phishing-Simulationen',
      description: 'Interne Phishing-Simulationen und Micro-Trainings für Mitarbeiter. Betriebsratskonform durch anonymisierte Berichte. Abgeschlossene Trainings fließen als Nachweis in Vakt Comply.',
    },
    secprivacy: {
      label: 'Vakt Privacy — DSGVO-Dokumentation',
      description: 'Vollständige DSGVO-Dokumentation: Verzeichnis von Verarbeitungstätigkeiten (Art. 30), Datenschutz-Folgenabschätzungen (Art. 35), AV-Verträge (Art. 28) und Datenpannen-Register mit 72h-Meldepflicht (Art. 33/34).',
    },
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Schritt 3 von 3 — Module</h2>
        <p className="text-secondary text-sm mt-1">
          Wählen Sie die zu aktivierenden Module. Dies kann später über Umgebungsvariablen geändert werden.
        </p>
      </div>
      <div className="space-y-2">
        {ALL_MODULES.map((mod) => (
          <label
            key={mod}
            className="flex items-start gap-3 border border-border rounded p-3 cursor-pointer hover:bg-surface2 text-primary"
          >
            <input
              type="checkbox"
              checked={modules.includes(mod)}
              onChange={() => onToggle(mod)}
              className="mt-0.5"
            />
            <div>
              <div className="text-sm font-semibold">{labels[mod].label}</div>
              <div className="text-xs text-secondary mt-1 leading-relaxed">{labels[mod].description}</div>
            </div>
          </label>
        ))}
      </div>
      {error && (
        <div className="bg-red-500/10 border border-red-500/30 text-red-400 text-sm rounded p-3">
          {error}
        </div>
      )}
      <div className="flex gap-2">
        <button
          onClick={onBack}
          disabled={submitting}
          className="flex-1 border border-border py-2 rounded text-sm font-medium text-primary hover:bg-surface2 disabled:opacity-50"
        >
          Zurück
        </button>
        <button
          onClick={onSubmit}
          disabled={submitting || modules.length === 0}
          className="flex-1 bg-brand text-white py-2 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {submitting ? 'Wird eingerichtet…' : 'Einrichtung abschließen'}
        </button>
      </div>
    </div>
  )
}

// ── Main wizard ───────────────────────────────────────────────────────────────

export default function Setup() {
  const navigate = useNavigate()

  const [step, setStep] = useState(1)
  const [orgName, setOrgName] = useState('')
  const [adminEmail, setAdminEmail] = useState('')
  const [adminPassword, setAdminPassword] = useState('')
  const [modules, setModules] = useState<string[]>([...ALL_MODULES])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const toggleModule = (mod: string) => {
    setModules((prev) =>
      prev.includes(mod) ? prev.filter((m) => m !== mod) : [...prev, mod],
    )
  }

  const handleSubmit = async () => {
    setSubmitting(true)
    setError(null)
    try {
      const payload: SetupPayload = {
        org_name: orgName.trim(),
        admin_email: adminEmail.trim(),
        admin_password: adminPassword,
        modules_enabled: modules,
      }
      await apiFetch<SetupResponse>('/setup', {
        method: 'POST',
        body: JSON.stringify(payload),
      })
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Einrichtung fehlgeschlagen. Bitte versuchen Sie es erneut.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface2 p-4">
      <div className="w-full max-w-md bg-surface rounded-xl shadow-sm border border-border p-8 text-primary">
        <div className="mb-6">
          <h1 className="text-2xl font-bold tracking-tight">Vakt</h1>
          <p className="text-secondary text-sm">Ersteinrichtungs-Assistent</p>
        </div>

        {step === 1 && (
          <Step1OrgName
            orgName={orgName}
            onChange={setOrgName}
            onNext={() => setStep(2)}
          />
        )}
        {step === 2 && (
          <Step2AdminAccount
            email={adminEmail}
            password={adminPassword}
            onChangeEmail={setAdminEmail}
            onChangePassword={setAdminPassword}
            onBack={() => setStep(1)}
            onNext={() => setStep(3)}
          />
        )}
        {step === 3 && (
          <Step3Modules
            modules={modules}
            onToggle={toggleModule}
            onBack={() => setStep(2)}
            onSubmit={handleSubmit}
            submitting={submitting}
            error={error}
          />
        )}
      </div>
    </div>
  )
}
