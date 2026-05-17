import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch } from '../api/client'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await apiFetch('/auth/password-reset/request', {
        method: 'POST',
        body: JSON.stringify({ email }),
      })
    } catch {
      // Ignore errors — we always show success to avoid email enumeration.
    } finally {
      setLoading(false)
      setSubmitted(true)
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
            <CardTitle>Passwort zurücksetzen</CardTitle>
          </CardHeader>
          <CardContent>
            {submitted ? (
              <div className="space-y-4">
                <p className="text-sm text-secondary">
                  Falls ein Konto mit dieser E-Mail-Adresse existiert, wurde eine E-Mail mit einem
                  Link zum Zurücksetzen des Passworts gesendet. Bitte prüfen Sie auch Ihren
                  Spam-Ordner.
                </p>
                <Link
                  to="/login"
                  className="text-sm text-brand hover:underline block text-center"
                >
                  Zurück zum Login
                </Link>
              </div>
            ) : (
              <form onSubmit={(e) => { void handleSubmit(e) }} className="space-y-4">
                <p className="text-sm text-secondary">
                  Geben Sie Ihre E-Mail-Adresse ein. Wir senden Ihnen einen Link zum Zurücksetzen
                  Ihres Passworts.
                </p>
                <div className="space-y-1">
                  <Label htmlFor="email">E-Mail-Adresse</Label>
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
                <Button type="submit" className="w-full" disabled={loading}>
                  {loading ? 'Wird gesendet…' : 'Link anfordern'}
                </Button>
                <div className="text-center">
                  <Link to="/login" className="text-sm text-secondary hover:text-primary hover:underline">
                    Zurück zum Login
                  </Link>
                </div>
              </form>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
