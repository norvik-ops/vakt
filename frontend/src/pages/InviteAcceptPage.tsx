import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { apiFetch } from '../api/client'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { useInviteInfo } from '../hooks/useTeam'

function roleName(role: string) {
  switch (role) {
    case 'admin':  return 'Admin'
    case 'editor': return 'Editor'
    case 'viewer': return 'Viewer'
    default: return role
  }
}

export default function InviteAcceptPage() {
  const [params] = useSearchParams()
  const token = params.get('token')
  const navigate = useNavigate()

  const { data: invite, isLoading, isError } = useInviteInfo(token)

  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)

    if (!token) {
      setFormError('Kein Einladungstoken gefunden.')
      return
    }
    if (password !== confirm) {
      setFormError('Passwörter stimmen nicht überein.')
      return
    }
    if (password.length < 8) {
      setFormError('Das Passwort muss mindestens 8 Zeichen lang sein.')
      return
    }

    setSubmitting(true)
    try {
      await apiFetch('/invite/accept', {
        method: 'POST',
        body: JSON.stringify({ token, name, password }),
      })
      navigate('/login?message=account-created', { replace: true })
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'Fehler beim Erstellen des Kontos.')
    } finally {
      setSubmitting(false)
    }
  }

  if (!token) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <div className="text-center space-y-2">
          <p className="text-lg font-semibold text-primary">Ungültiger Link</p>
          <p className="text-sm text-secondary">Der Einladungslink ist unvollständig oder fehlerhaft.</p>
        </div>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <div className="w-6 h-6 border-2 border-brand border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (isError || !invite) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <div className="text-center space-y-2 max-w-sm">
          <p className="text-lg font-semibold text-primary">Einladung abgelaufen</p>
          <p className="text-sm text-secondary">
            Dieser Einladungslink ist nicht mehr gültig oder wurde bereits verwendet.
            Bitte bitte die einladende Person, eine neue Einladung zu senden.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-bg px-4">
      <div className="w-full max-w-sm space-y-6">
        {/* Header */}
        <div className="text-center space-y-1">
          <div className="flex items-center justify-center gap-2 mb-4">
            <img src="/logo.svg" alt="Vakt" className="w-8 h-8" />
            <span className="font-bold text-xl text-brand">Vakt</span>
          </div>
          <h1 className="text-xl font-semibold text-primary">Konto erstellen</h1>
          <p className="text-sm text-secondary">
            Du wurdest von <strong>{invite.invited_by || 'deinem Team'}</strong> als{' '}
            <strong>{roleName(invite.role)}</strong> eingeladen.
          </p>
          <p className="text-sm text-secondary">
            E-Mail: <strong>{invite.email}</strong>
          </p>
        </div>

        {/* Form */}
        <form onSubmit={(e) => { void handleSubmit(e) }} className="space-y-4">
          <div className="space-y-1">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              type="text"
              placeholder="Vor- und Nachname"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              minLength={2}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="password">Passwort</Label>
            <Input
              id="password"
              type="password"
              placeholder="Mindestens 8 Zeichen"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="confirm">Passwort wiederholen</Label>
            <Input
              id="confirm"
              type="password"
              placeholder="Passwort bestätigen"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
            />
          </div>

          {formError && (
            <p className="text-sm text-destructive">{formError}</p>
          )}

          <Button type="submit" className="w-full" disabled={submitting}>
            {submitting ? 'Erstelle Konto...' : 'Konto erstellen'}
          </Button>
        </form>

        <p className="text-center text-xs text-secondary">
          Bereits ein Konto?{' '}
          <a href="/login" className="text-brand hover:underline">
            Einloggen
          </a>
        </p>
      </div>
    </div>
  )
}
