import { useState, useEffect, type FormEvent } from 'react'
import { useNavigate, useLocation, Link } from 'react-router-dom'
import { apiFetch } from '../api/client'
import { useAuthStore } from '../shared/stores/auth'
import { useDemoMode } from '../shared/hooks/useDemoMode'
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

const DEMO_USERS = [
  { label: 'Admin', email: 'admin@vakt.local', password: 'admin1234' },
  { label: 'Analyst', email: 'analyst@vakt.local', password: 'analyst1234' },
]

export default function Login() {
  const navigate = useNavigate()
  const location = useLocation()
  const setAuth = useAuthStore((s) => s.setAuth)
  const isDemo = useDemoMode()
  const [email, setEmail] = useState('')

  useEffect(() => {
    document.title = isDemo ? 'Vakt Demo' : 'Vakt'
  }, [isDemo])
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const data = await apiFetch<LoginResponse>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      setAuth(data.access_token, data.user)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
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
            <CardTitle>Sign in</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={(e) => { void handleSubmit(e) }} className="space-y-4">
              <div className="space-y-1">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="admin@example.com"
                  required
                  autoFocus
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
                <div className="text-right">
                  <Link
                    to="/auth/forgot-password"
                    className="text-xs text-secondary hover:text-primary hover:underline"
                  >
                    Passwort vergessen?
                  </Link>
                </div>
              </div>
              {error && <p className="text-sm text-red-600">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? 'Signing in…' : 'Sign in'}
              </Button>
            </form>
          </CardContent>
        </Card>

        {isDemo && (
          <>
            <Card className="border-brand/30 bg-brand/5">
              <CardContent className="pt-4 pb-4 space-y-3">
                <p className="text-xs font-semibold text-brand uppercase tracking-wide">Demo-Zugangsdaten</p>
                <p className="text-xs text-secondary">Einfach auf einen Account klicken — das Formular wird automatisch ausgefüllt.</p>
                {((): { label: string; email: string; password: string }[] => {
                  const passed = (location.state as { demoEmails?: { admin: string; analyst: string } } | null)?.demoEmails
                  return passed
                    ? [
                        { label: 'Admin', email: passed.admin, password: 'admin1234' },
                        { label: 'Analyst', email: passed.analyst, password: 'analyst1234' },
                      ]
                    : DEMO_USERS
                })().map((u) => (
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
              Dies ist eine öffentliche Demo-Instanz. Bitte keine echten oder sensiblen Daten eingeben.
              NorvikOps übernimmt keine Haftung für eingegebene Daten.{' '}
              <a href="https://norvikops.de" className="underline hover:text-primary">norvikops.de</a>
            </p>
          </>
        )}
      </div>
    </div>
  )
}
