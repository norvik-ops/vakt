import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../api/client'
import { useFieldValidation, required, email as emailRule } from '../shared/hooks/useFieldValidation'
import { FieldError } from '../shared/components/FieldError'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'

export default function ForgotPasswordPage() {
  const { t } = useTranslation()
  const [email, setEmail] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [loading, setLoading] = useState(false)
  const emailValidation = useFieldValidation(email, [required, emailRule])

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
            <CardTitle>{t('auth.forgotPasswordTitle')}</CardTitle>
          </CardHeader>
          <CardContent>
            {submitted ? (
              <div className="space-y-4">
                <p className="text-sm text-secondary">
                  {t('auth.forgotPasswordSuccess')}
                </p>
                <Link
                  to="/login"
                  className="text-sm text-brand hover:underline block text-center"
                >
                  {t('auth.backToLogin')}
                </Link>
              </div>
            ) : (
              <form onSubmit={(e) => { void handleSubmit(e) }} className="space-y-4">
                <p className="text-sm text-secondary">
                  {t('auth.forgotPasswordDesc')}
                </p>
                <div className="space-y-1">
                  <Label htmlFor="email">{t('auth.forgotPasswordEmailLabel')}</Label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder={t('auth.emailPlaceholder')}
                    required
                    autoFocus
                    aria-invalid={!!emailValidation.error}
                  />
                  <FieldError error={emailValidation.error} />
                </div>
                <Button type="submit" className="w-full" disabled={loading}>
                  {loading ? t('auth.requestingSending') : t('auth.requestLink')}
                </Button>
                <div className="text-center">
                  <Link to="/login" className="text-sm text-secondary hover:text-primary hover:underline">
                    {t('auth.backToLogin')}
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
