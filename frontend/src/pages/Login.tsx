import { useState, useEffect, type FormEvent } from 'react'
import { useNavigate, useSearchParams, Link } from 'react-router-dom'
import { Spinner } from '../components/Spinner'
import { Building2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { apiFetch, setSessionId } from '../api/client'
import { useAuthStore } from '../shared/stores/auth'
import { useDemoMode } from '../shared/hooks/useDemoMode'
import { toast } from '../shared/hooks/useToast'
import { useFieldValidation, required, minLength, email as emailRule } from '../shared/hooks/useFieldValidation'
import { FieldError } from '../shared/components/FieldError'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'

// Sprint 16 S16-2: Manual-Types ersetzt durch generierte Types aus
// backend/internal/shared/apidocs/openapi.yaml. Regenerieren via
// `npm run api-types` — Drift wird in CI via `npm run api-types:check`
// detektiert (siehe ADR-0017).
import type { components } from '../api/generated'

type LoginResponse = components['schemas']['LoginResponse']
type HealthResponse = components['schemas']['HealthResponse']

type DemoRole = { label: string; role: 'admin' | 'analyst' }

export default function Login() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)
  const isDemo = useDemoMode()
  const [email, setEmail] = useState('')
  const [ssoEnabled, setSsoEnabled] = useState(false)
  const [demoStarting, setDemoStarting] = useState<DemoRole['role'] | null>(null)
  const [demoError, setDemoError] = useState(false)

  // Demo roles are static — no need to fetch /demo/start on mount any more.
  // The actual session is created server-side when the user clicks the role
  // button (audit F041: password never enters the DOM).
  const demoRoles: DemoRole[] = [
    { label: 'Admin', role: 'admin' },
    { label: 'Analyst', role: 'analyst' },
  ]

  useEffect(() => {
    document.title = isDemo ? 'Vakt Demo' : 'Vakt'
  }, [isDemo])

  useEffect(() => {
    fetch('/health')
      .then((r) => r.json() as Promise<HealthResponse>)
      .then((data) => { setSsoEnabled(data.sso_enabled === true) })
      .catch(() => { /* SSO-Button bleibt ausgeblendet wenn Health-Check fehlschlägt */ })
  }, [])

  async function handleDemoLogin(role: DemoRole['role']) {
    setDemoStarting(role)
    setDemoError(false)
    try {
      const data = await apiFetch<LoginResponse>('/demo/login', {
        method: 'POST',
        body: JSON.stringify({ role }),
      })
      setAuth(data.user)
      if ('session_id' in data && typeof data.session_id === 'string') {
        setSessionId(data.session_id)
      }
      navigate('/')
    } catch {
      setDemoError(true)
      toast(t('auth.demoUnavailable'), { variant: 'error', duration: 10000 })
    } finally {
      setDemoStarting(null)
    }
  }

  const [searchParams] = useSearchParams()

  useEffect(() => {
    const samlError = searchParams.get('error')
    if (!samlError) return
    const msg: Record<string, string> = {
      saml_assertion_invalid: t('auth.samlAssertionInvalid'),
      saml_missing_email:     t('auth.samlMissingEmail'),
      saml_user_not_provisioned: t('auth.samlUserNotProvisioned'),
      saml_provision_failed:  t('auth.samlProvisionFailed'),
    }
    toast(msg[samlError] ?? t('auth.samlLoginFailed'), { variant: 'error', duration: 10000 })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const emailValidation = useFieldValidation(email, [required, emailRule])
  const passwordValidation = useFieldValidation(password, [required, minLength(10)])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (emailValidation.error || passwordValidation.error || !email || !password) return
    setError(null)
    setLoading(true)
    try {
      const data = await apiFetch<LoginResponse>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      setAuth(data.user)
      if ('session_id' in data && typeof data.session_id === 'string') {
        setSessionId(data.session_id)
      }
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : t('auth.loginFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-bg p-4">
      <a
        href="#login-main"
        className="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 z-50 bg-background px-4 py-2 rounded-lg border font-medium"
      >
        {t('nav.skipToContent')}
      </a>
      <main id="login-main" className="w-full max-w-sm space-y-4">
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2.5 mb-2">
              <img src="/logo.svg" alt="Vakt" className="w-9 h-9 shrink-0" />
              <div>
                <span className="font-semibold text-[16px] text-brand">Vakt</span>
                <p className="text-xs text-secondary mt-0.5">Security & Compliance Platform</p>
              </div>
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
                  onChange={(e) => { setEmail(e.target.value); }}
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
                  onChange={(e) => { setPassword(e.target.value); }}
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
            <Card className="border-brand/30 bg-brand/5">
              <CardContent className="pt-4 pb-4 space-y-3">
                <p className="text-xs font-semibold text-brand uppercase tracking-wide">{t('auth.demoCredentials')}</p>
                <p className="text-xs text-secondary">{t('auth.demoHint')}</p>
                {demoRoles.map((r) => (
                  <button
                    key={r.role}
                    type="button"
                    onClick={() => { void handleDemoLogin(r.role) }}
                    disabled={demoStarting !== null}
                    className="w-full text-left rounded-md border border-border bg-surface px-3 py-2 hover:bg-muted transition-colors disabled:opacity-60 disabled:cursor-wait flex items-center justify-between"
                  >
                    <span className="text-xs font-medium">{r.label}</span>
                    {demoStarting === r.role && <Spinner size="sm" className="w-3.5 h-3.5" />}
                  </button>
                ))}
                {demoError && (
                  <div className="space-y-1.5">
                    <p className="text-xs text-red-500">{t('auth.demoUnavailable')}</p>
                    <a
                      href="https://github.com/norvik-ops/vatk/blob/main/docs/guides/getting-started.md"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs text-brand underline hover:text-brand/80"
                    >
                      {t('auth.demoUnavailableInstall')}
                    </a>
                  </div>
                )}
              </CardContent>
            </Card>

            <div className="rounded-lg border border-amber-500/50 bg-amber-500/20 px-3 py-2.5 text-center">
              <p className="text-xs font-semibold text-amber-400 uppercase tracking-wide">Demo-Umgebung</p>
              <p className="text-[11px] text-amber-300/80 mt-1">
                {t('auth.demoDisclaimer')}
              </p>
            </div>
          </>
        )}
      </main>
    </div>
  )
}
