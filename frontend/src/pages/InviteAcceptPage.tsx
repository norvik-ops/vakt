import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Spinner } from '../components/Spinner'
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
  const { t } = useTranslation()
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
      setFormError(t('invite.errorNoToken'))
      return
    }
    if (password !== confirm) {
      setFormError(t('invite.errorPasswordMismatch'))
      return
    }
    if (password.length < 10) {
      setFormError(t('invite.errorPasswordTooShort'))
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
      setFormError(err instanceof Error ? err.message : t('invite.errorCreatingAccount'))
    } finally {
      setSubmitting(false)
    }
  }

  if (!token) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <div className="text-center space-y-2">
          <p className="text-lg font-semibold text-primary">{t('invite.invalidLink')}</p>
          <p className="text-sm text-secondary">{t('invite.invalidLinkHint')}</p>
        </div>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <Spinner size="lg" />
      </div>
    )
  }

  if (isError || !invite) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <div className="text-center space-y-2 max-w-sm">
          <p className="text-lg font-semibold text-primary">{t('invite.expiredTitle')}</p>
          <p className="text-sm text-secondary">{t('invite.expiredHint')}</p>
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
          <h1 className="text-xl font-semibold text-primary">{t('invite.createAccount')}</h1>
          <p className="text-sm text-secondary">
            {t('invite.invitedBy', { inviter: invite.invited_by || 'deinem Team', role: roleName(invite.role) })}
          </p>
          <p className="text-sm text-secondary">
            {t('invite.email')} <strong>{invite.email}</strong>
          </p>
        </div>

        {/* Form */}
        <form onSubmit={(e) => { void handleSubmit(e) }} className="space-y-4">
          <div className="space-y-1">
            <Label htmlFor="name">{t('invite.nameLabel')}</Label>
            <Input
              id="name"
              type="text"
              placeholder={t('invite.namePlaceholder')}
              value={name}
              onChange={(e) => { setName(e.target.value); }}
              required
              minLength={2}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="password">{t('invite.passwordLabel')}</Label>
            <Input
              id="password"
              type="password"
              placeholder={t('invite.passwordPlaceholder')}
              value={password}
              onChange={(e) => { setPassword(e.target.value); }}
              required
              minLength={10}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="confirm">{t('invite.confirmLabel')}</Label>
            <Input
              id="confirm"
              type="password"
              placeholder={t('invite.confirmPlaceholder')}
              value={confirm}
              onChange={(e) => { setConfirm(e.target.value); }}
              required
            />
          </div>

          {formError && (
            <p className="text-sm text-destructive">{formError}</p>
          )}

          <Button type="submit" className="w-full" disabled={submitting}>
            {submitting ? t('invite.submitting') : t('invite.submit')}
          </Button>
        </form>

        <p className="text-center text-xs text-secondary">
          {t('invite.alreadyHaveAccount')}{' '}
          <a href="/login" className="text-brand hover:underline">
            {t('invite.login')}
          </a>
        </p>
      </div>
    </div>
  )
}
