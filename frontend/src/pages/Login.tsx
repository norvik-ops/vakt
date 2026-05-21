import { useState, useEffect, type FormEvent } from 'react'
import { useNavigate, useLocation, Link } from 'react-router-dom'
import { Building2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../api/client'
import { useAuthStore } from '../shared/stores/auth'
import { useDemoMode } from '../shared/hooks/useDemoMode'
import { toast } from '../shared/hooks/useToast'
import { useFieldValidation, required, minLength, email as emailRule } from '../shared/hooks/useFieldValidation'
import { FieldError } from '../shared/components/FieldError'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'

interface LoginResponse {
  access_token: string
  user: {
    id: string
    email: string
    display_name: string
    roles: string[]
  }
}

interface HealthResponse {
  status: string
  version: string
  demo: boolean
  sso_enabled: boolean
}

interface DemoStartResponse {
  admin_email: string
  admin_password: string
  analyst_email: string
  analyst_password: string
  expires_in: number
}

type DemoUser = { label: string; email: string; password: string }

export default function Login() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const setAuth = useAuthStore((s) => s.setAuth)
  const isDemo = useDemoMode()
  const [email, setEmail] = useState('')
  const [ssoEnabled, setSsoEnabled] = useState(false)
  const [demoUsers, setDemoUsers] = useState<DemoUser[] | null>(null)
  const [demoStarting, setDemoStarting] = useState(false)

  useEffect(() => {
    document.title = isDemo ? 'Vakt Demo' : 'Vakt'
  }, [isDemo])

  useEffect(() => {
    fetch('/health')
      .then((r) => r.json() as Promise<HealthResponse>)
      .then((data) => { setSsoEnabled(data.sso_enabled === true) })
      .catch(() => { /* SSO-Button bleibt ausgeblendet wenn Health-Check fehlschlägt */ })
  }, [])

  // Auto-start ephemeral demo session when in demo mode.
  // Jeder Visitor bekommt eine eigene Demo-Org mit Random-Slug + Random-
  // Passwörtern (16 hex chars). Die Klartext-Passwörter kommen einmalig
  // aus /api/v1/demo/start zurück und werden hier zur Anzeige/Vorbelegung
  // verwendet — keine hardcoded Defaults mehr (Min-Length-Validierung
  // 10 Zeichen würde "admin1234" ohnehin ablehnen). Cleanup-Job löscht
  // die Org nach 4 Stunden (siehe demo/cleanup.go).
  useEffect(() => {
    if (!isDemo) return
    const passed = (location.state as { demoCreds?: { admin: { email: string; password: string }; analyst: { email: string; password: string } } } | null)?.demoCreds
    if (passed) {
      setDemoUsers([
        { label: 'Admin', email: passed.admin.email, password: passed.admin.password },
        { label: 'Analyst', email: passed.analyst.email, password: passed.analyst.password },
      ])
      return
    }
    setDemoStarting(true)
    fetch('/api/v1/demo/start', { method: 'POST' })
      .then((r) => {
        if (!r.ok) throw new Error(`demo/start ${r.status}`)
        return r.json() as Promise<DemoStartResponse>
      })
      .then((d) => {
        setDemoUsers([
          { label: 'Admin', email: d.admin_email, password: d.admin_password },
          { label: 'Analyst', email: d.analyst_email, password: d.analyst_password },
        ])
      })
      .catch(() => {
        // S13-26: Demo-Start fehlgeschlagen — kein Fallback auf Pseudo-Credentials,
        // weil die nirgends echt funktionieren wuerden. Stattdessen sichtbarer
        // Toast statt stillem UI-Zerfall + setDemoUsers(null) als visuelles Signal.
        setDemoUsers(null)
        toast(t('auth.demoUnavailable'), { variant: 'error', duration: 10000 })
      })
      .finally(() => setDemoStarting(false))
  }, [isDemo, t])

  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const emailValidation = useFieldValidation(email, [required, emailRule])
  const passwordValidation = useFieldValidation(password, [required, minLength(8)])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const data = await apiFetch<LoginResponse>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      setAuth(data.user)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : t('auth.loginFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-bg p-4">
      <div className="w-full max-w-sm space-y-4">
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2.5 mb-2">
              <img src="/logo.svg" alt="Vakt" className="w-9 h-9 shrink-0" />
              <span className="font-semibold text-[16px] text-brand">Vakt</span>
            </div>
            <CardTitle>{t('auth.signIn')}</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={(e) => { void handleSubmit(e) }} className="space-y-4">
              {/* WCAG 3.3.2: required attribute + aria-required communicates required fields */}
              <div className="space-y-1">
                <Label htmlFor="email">{t('auth.emailLabel')}</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t('auth.emailPlaceholder')}
                  required
                  aria-required="true"
                  aria-describedby={error ? 'login-error' : undefined}
                  aria-invalid={!!error || !!emailValidation.error}
                  autoFocus
                />
                <FieldError error={emailValidation.error} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="password">{t('auth.passwordLabel')}</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  aria-required="true"
                  aria-describedby={error ? 'login-error' : undefined}
                  aria-invalid={!!error || !!passwordValidation.error}
                />
                <FieldError error={passwordValidation.error} />
                <div className="text-right">
                  <Link
                    to="/auth/forgot-password"
                    className="text-xs text-secondary hover:text-primary hover:underline"
                  >
                    {t('auth.forgotPassword')}
                  </Link>
                </div>
              </div>
              {/* WCAG 3.3.1 + 4.1.3: role="alert" announces errors immediately to screen readers */}
              {error && <p id="login-error" role="alert" aria-live="assertive" className="text-sm text-red-600">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? t('auth.signingIn') : t('auth.signIn')}
              </Button>
            </form>

            {ssoEnabled && (
              <>
                <div className="relative my-4">
                  <div className="absolute inset-0 flex items-center">
                    <span className="w-full border-t border-border" />
                  </div>
                  <div className="relative flex justify-center text-xs">
                    <span className="bg-card px-2 text-secondary">{t('auth.orSeparator')}</span>
                  </div>
                </div>
                <a
                  href="/auth/sso"
                  className="flex items-center justify-center gap-2 w-full rounded-md border border-border bg-surface px-4 py-2 text-sm font-medium text-primary hover:bg-muted transition-colors"
                >
                  {/* WCAG 1.1.1: icon is decorative, link text names the element */}
                  <Building2 className="w-4 h-4 shrink-0" aria-hidden="true" />
                  {t('auth.ssoButton')}
                </a>
              </>
            )}
          </CardContent>
        </Card>

        {isDemo && (
          <>
            <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-center">
              <p className="text-xs font-semibold text-amber-400 uppercase tracking-wide">Demo-Umgebung</p>
              <p className="text-[11px] text-amber-300/80 mt-0.5">
                Alle Daten werden nach 4 Stunden automatisch zurückgesetzt.
              </p>
            </div>

            <Card className="border-brand/30 bg-brand/5">
              <CardContent className="pt-4 pb-4 space-y-3">
                <p className="text-xs font-semibold text-brand uppercase tracking-wide">{t('auth.demoCredentials')}</p>
                <p className="text-xs text-secondary">{t('auth.demoHint')}</p>
                {demoStarting && (
                  <div className="flex items-center gap-2 py-2">
                    <span className="w-3.5 h-3.5 border-2 border-brand border-t-transparent rounded-full animate-spin inline-block shrink-0" />
                    <span className="text-xs text-secondary">{t('auth.demoPreparing')}</span>
                  </div>
                )}
                {!demoStarting && demoUsers && demoUsers.map((u) => (
                  <button
                    key={u.email}
                    type="button"
                    onClick={() => { setEmail(u.email); setPassword(u.password) }}
                    className="w-full text-left rounded-md border border-border bg-surface px-3 py-2 hover:bg-muted transition-colors"
                  >
                    <span className="text-xs font-medium block">{u.label}</span>
                    <span className="text-xs text-secondary font-mono">{u.email}</span>
                  </button>
                ))}
              </CardContent>
            </Card>

            <p className="text-xs text-secondary text-center px-2">
              {t('auth.demoDisclaimer')}{' '}
              <a href="https://vakt.io" className="underline hover:text-primary">vakt.io</a>
            </p>
          </>
        )}
      </div>
    </div>
  )
}
