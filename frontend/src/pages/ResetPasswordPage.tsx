import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { apiFetch } from '../api/client'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'

export default function ResetPasswordPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''

  const [password, setPassword] = useState('')
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)

    if (password.length < 8) {
      setError('Das Passwort muss mindestens 8 Zeichen lang sein.')
      return
    }
    if (password !== passwordConfirm) {
      setError('Die Passwörter stimmen nicht überein.')
      return
    }

    setLoading(true)
    try {
      await apiFetch('/auth/password-reset/confirm', {
        method: 'POST',
        body: JSON.stringify({ token, password }),
      })
      navigate('/login', {
        state: { successMessage: 'Passwort erfolgreich zurückgesetzt. Sie können sich jetzt anmelden.' },
        replace: true,
      })
    } catch {
      setError('Link ungültig oder abgelaufen. Bitte fordern Sie einen neuen Link an.')
    } finally {
      setLoading(false)
    }
  }

  if (!token) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg p-4">
        <div className="w-full max-w-sm">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2.5 mb-2">
                <img src="/logo.svg" alt="Vakt" className="w-9 h-9 shrink-0" />
                <span className="font-semibold text-[16px] text-brand">Vakt</span>
              </div>
              <CardTitle>Link ungültig</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-sm text-secondary">
                Dieser Link zum Zurücksetzen des Passworts ist ungültig oder abgelaufen.
              </p>
              <Link
                to="/auth/forgot-password"
                className="text-sm text-brand hover:underline block text-center"
              >
                Neuen Link anfordern
              </Link>
            </CardContent>
          </Card>
        </div>
      </div>
    )
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
            <CardTitle>Neues Passwort festlegen</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={(e) => { void handleSubmit(e) }} className="space-y-4">
              <div className="space-y-1">
                <Label htmlFor="password">Neues Passwort</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Mindestens 8 Zeichen"
                  required
                  autoFocus
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="password-confirm">Passwort bestätigen</Label>
                <Input
                  id="password-confirm"
                  type="password"
                  value={passwordConfirm}
                  onChange={(e) => setPasswordConfirm(e.target.value)}
                  placeholder="Passwort wiederholen"
                  required
                />
              </div>
              {error && <p className="text-sm text-red-600">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? 'Wird gespeichert…' : 'Passwort zurücksetzen'}
              </Button>
              <div className="text-center">
                <Link to="/login" className="text-sm text-secondary hover:text-primary hover:underline">
                  Zurück zum Login
                </Link>
              </div>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
