import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../api/client'
import { toast } from '../shared/hooks/useToast'

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

const ALL_MODULES = ['vaktscan', 'vaktcomply', 'vaktvault', 'vaktaware', 'vaktprivacy'] as const

// ── Step components ───────────────────────────────────────────────────────────

interface Step1Props {
  orgName: string
  onChange: (v: string) => void
  onNext: () => void
}

function Step1OrgName({ orgName, onChange, onNext }: Step1Props) {
  const { t } = useTranslation()
  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">{t('setup.step1Title')}</h2>
        <p className="text-secondary text-sm mt-1">
          {t('setup.step1Desc')}
        </p>
      </div>
      <div className="space-y-1">
        <label htmlFor="org_name" className="block text-sm font-medium">
          {t('setup.orgNameLabel')}
        </label>
        <input
          id="org_name"
          type="text"
          value={orgName}
          onChange={(e) => { onChange(e.target.value); }}
          placeholder={t('setup.orgNamePlaceholder')}
          className="w-full border border-border rounded px-3 py-2 text-sm bg-surface2 text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand"
          autoFocus
        />
      </div>
      <button
        onClick={onNext}
        disabled={orgName.trim().length < 2}
        className="w-full bg-brand text-white py-2 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {t('setup.next')}
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
  const { t } = useTranslation()
  const valid = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) && password.length >= 10

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">{t('setup.step2Title')}</h2>
        <p className="text-secondary text-sm mt-1">
          {t('setup.step2Desc')}
        </p>
      </div>
      <div className="space-y-1">
        <label htmlFor="admin_email" className="block text-sm font-medium">
          {t('setup.adminEmailLabel')}
        </label>
        <input
          id="admin_email"
          type="email"
          value={email}
          onChange={(e) => { onChangeEmail(e.target.value); }}
          placeholder={t('setup.adminEmailPlaceholder')}
          className="w-full border border-border rounded px-3 py-2 text-sm bg-surface2 text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand"
          autoFocus
        />
      </div>
      <div className="space-y-1">
        <label htmlFor="admin_password" className="block text-sm font-medium">
          {t('setup.adminPasswordLabel')} <span className="text-secondary font-normal">{t('setup.adminPasswordHint')}</span>
        </label>
        <input
          id="admin_password"
          type="password"
          value={password}
          onChange={(e) => { onChangePassword(e.target.value); }}
          placeholder={t('setup.adminPasswordPlaceholder')}
          className="w-full border border-border rounded px-3 py-2 text-sm bg-surface2 text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand"
        />
      </div>
      <div className="flex gap-2">
        <button
          onClick={onBack}
          className="flex-1 border border-border py-2 rounded text-sm font-medium text-primary hover:bg-surface2"
        >
          {t('setup.back')}
        </button>
        <button
          onClick={onNext}
          disabled={!valid}
          className="flex-1 bg-brand text-white py-2 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {t('setup.next')}
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
  const { t } = useTranslation()

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">{t('setup.step3Title')}</h2>
        <p className="text-secondary text-sm mt-1">
          {t('setup.step3Desc')}
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
              onChange={() => { onToggle(mod); }}
              className="mt-0.5"
            />
            <div>
              <div className="text-sm font-semibold">{t(`setup.modules.${mod}.label`)}</div>
              <div className="text-xs text-secondary mt-1 leading-relaxed">{t(`setup.modules.${mod}.description`)}</div>
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
          {t('setup.back')}
        </button>
        <button
          onClick={onSubmit}
          disabled={submitting || modules.length === 0}
          className="flex-1 bg-brand text-white py-2 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {submitting ? t('setup.finishing') : t('setup.finish')}
        </button>
      </div>
    </div>
  )
}

// ── Main wizard ───────────────────────────────────────────────────────────────

export default function Setup() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [step, setStep] = useState(1)
  const [orgName, setOrgName] = useState('')
  const [adminEmail, setAdminEmail] = useState('')
  const [adminPassword, setAdminPassword] = useState('')
  const [modules, setModules] = useState<string[]>([...ALL_MODULES])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Sprint 22 S22-4: bei Sign-up mit ?nis2_token= aus dem Public-Wizard
  // wird der Token nach erfolgreichem Setup an den Migrate-Endpoint
  // gesendet. Token kommt entweder per Query-Param ODER aus localStorage
  // (Wizard speichert ihn dort).
  const nis2Token = (() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('nis2_token') || localStorage.getItem('vakt_nis2_token') || ''
  })()

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
      // Sprint 22 S22-4: NIS2-Wizard-Token migrieren falls vorhanden.
      // Setup-Endpoint logged den User schon ein (Cookie gesetzt), also
      // hat der nachfolgende authentifizierte Call Auth-Context.
      if (nis2Token) {
        try {
          const migrateResp = await apiFetch<{ assessment_id: string; controls_mapped: number }>(
            '/vaktcomply/nis2-assessment/migrate-from-anonymous',
            { method: 'POST', body: JSON.stringify({ token: nis2Token }) },
          )
          localStorage.removeItem('vakt_nis2_token')
          toast(
            `NIS2-Wizard-Ergebnis übernommen: ${migrateResp.controls_mapped.toString()} Controls vorbelegt.`,
            'success',
          )
        } catch {
          // Migration ist Bonus — Setup-Erfolg nicht blockieren.
        }
      }
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : t('setup.setupFailed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface2 p-4">
      <div className="w-full max-w-md bg-surface rounded-xl shadow-sm border border-border p-8 text-primary">
        <div className="mb-6">
          <h1 className="text-2xl font-bold tracking-tight">{t('setup.title')}</h1>
          <p className="text-secondary text-sm">{t('setup.subtitle')}</p>
        </div>

        {step === 1 && (
          <Step1OrgName
            orgName={orgName}
            onChange={setOrgName}
            onNext={() => { setStep(2); }}
          />
        )}
        {step === 2 && (
          <Step2AdminAccount
            email={adminEmail}
            password={adminPassword}
            onChangeEmail={setAdminEmail}
            onChangePassword={setAdminPassword}
            onBack={() => { setStep(1); }}
            onNext={() => { setStep(3); }}
          />
        )}
        {step === 3 && (
          <Step3Modules
            modules={modules}
            onToggle={toggleModule}
            onBack={() => { setStep(2); }}
            onSubmit={() => { void handleSubmit(); }}
            submitting={submitting}
            error={error}
          />
        )}
      </div>
    </div>
  )
}
