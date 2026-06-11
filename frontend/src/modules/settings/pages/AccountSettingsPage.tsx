import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ShieldCheck, ShieldOff, Copy, Check, RefreshCw, Monitor, ExternalLink } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../../../shared/components/PageHeader'
import { RecoveryCodesDialog } from '../../../shared/components/RecoveryCodesDialog'
import { LoginHistorySection } from '../../../shared/components/LoginHistorySection'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Badge } from '../../../components/ui/badge'
import { Card } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { apiFetch } from '../../../api/client'
import { useAuthStore } from '../../../shared/stores/auth'

// ─── Types ────────────────────────────────────────────────────────────────────

interface TOTPStatus {
  enabled: boolean
}

interface SetupResponse {
  secret: string
  uri: string
}

interface ConfirmResponse {
  backup_codes: string[]
  recovery_codes: string[]
}

interface RegenerateResponse {
  recovery_codes: string[]
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

function useTOTPStatus() {
  return useQuery<TOTPStatus>({
    queryKey: ['auth', '2fa', 'status'],
    queryFn: () => apiFetch<TOTPStatus>('/auth/2fa/status'),
    retry: false,
  })
}

function useSetup2FA() {
  return useMutation<SetupResponse>({
    mutationFn: () =>
      apiFetch<SetupResponse>('/auth/2fa/setup', { method: 'POST' }),
  })
}

function useConfirm2FA() {
  const qc = useQueryClient()
  return useMutation<ConfirmResponse, Error, { code: string }>({
    mutationFn: (body) =>
      apiFetch<ConfirmResponse>('/auth/2fa/confirm', {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['auth', '2fa', 'status'] }),
  })
}

function useDisable2FA() {
  const qc = useQueryClient()
  return useMutation<unknown, Error, { code: string }>({
    mutationFn: (body) =>
      apiFetch<unknown>('/auth/2fa/disable', {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['auth', '2fa', 'status'] }),
  })
}

function useRegenerateRecoveryCodes() {
  return useMutation<RegenerateResponse>({
    mutationFn: () =>
      apiFetch<RegenerateResponse>('/auth/2fa/recovery-codes/regenerate', {
        method: 'POST',
      }),
  })
}

// ─── Setup Dialog ─────────────────────────────────────────────────────────────

type SetupStep = 'qr' | 'confirm' | 'backup'

function SetupDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const [step, setStep] = useState<SetupStep>('qr')
  const [code, setCode] = useState('')
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const [recoveryDialogOpen, setRecoveryDialogOpen] = useState(false)
  const [copied, setCopied] = useState(false)
  const [setupData, setSetupData] = useState<SetupResponse | null>(null)
  const [codeError, setCodeError] = useState('')

  const setup = useSetup2FA()
  const confirm = useConfirm2FA()

  function handleOpen() {
    if (step === 'qr' && !setupData) {
      setup.mutate(undefined, {
        onSuccess: (data) => { setSetupData(data); },
      })
    }
  }

  function handleConfirm() {
    setCodeError('')
    confirm.mutate(
      { code },
      {
        onSuccess: (data) => {
          setBackupCodes(data.backup_codes)
          if (data.recovery_codes.length > 0) {
            setRecoveryCodes(data.recovery_codes)
          }
          setStep('backup')
        },
        onError: (err) => {
          setCodeError(err.message)
        },
      },
    )
  }

  function handleClose() {
    setStep('qr')
    setCode('')
    setBackupCodes([])
    setRecoveryCodes([])
    setRecoveryDialogOpen(false)
    setCopied(false)
    setSetupData(null)
    setCodeError('')
    onClose()
  }

  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => { setCopied(false); }, 2000)
    return () => { clearTimeout(id); }
  }, [copied])

  function copyBackupCodes() {
    void navigator.clipboard.writeText(backupCodes.join('\n'))
    setCopied(true)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) handleClose()
        else handleOpen()
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('accountSettingsPage.setupDialogTitle')}</DialogTitle>
        </DialogHeader>

        {/* Step 1: QR / URI */}
        {step === 'qr' && (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              {t('accountSettingsPage.setupStep1Desc')}
            </p>
            {setup.isPending && (
              <p className="text-sm text-muted-foreground animate-pulse">
                {t('accountSettingsPage.generatingSecret')}
              </p>
            )}
            {setupData && (
              <div className="space-y-3">
                <div>
                  <Label className="text-xs text-muted-foreground mb-1 block">
                    {t('accountSettingsPage.secretKeyLabel')}
                  </Label>
                  <code className="block p-2 rounded bg-muted text-xs break-all select-all font-mono">
                    {setupData.secret}
                  </code>
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground mb-1 block">
                    {t('accountSettingsPage.otpLinkLabel')}
                  </Label>
                  <a
                    href={setupData.uri}
                    className="block p-2 rounded bg-muted text-xs break-all text-blue-400 hover:underline font-mono"
                    title={t('accountSettingsPage.otpLinkLabel')}
                  >
                    {setupData.uri}
                  </a>
                </div>
              </div>
            )}
            {setup.isError && (
              <p className="text-sm text-destructive">{setup.error.message}</p>
            )}
            <DialogFooter>
              <Button variant="outline" onClick={handleClose}>
                {t('common.cancel')}
              </Button>
              <Button
                onClick={() => { setStep('confirm'); }}
                disabled={!setupData || setup.isPending}
              >
                {t('accountSettingsPage.continue')}
              </Button>
            </DialogFooter>
          </div>
        )}

        {/* Step 2: Confirm code */}
        {step === 'confirm' && (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              {t('accountSettingsPage.step2Desc')}
            </p>
            <div className="space-y-1">
              <Label htmlFor="totp-code">{t('accountSettingsPage.authCodeLabel')}</Label>
              <Input
                id="totp-code"
                value={code}
                onChange={(e) => { setCode(e.target.value); }}
                placeholder="123456"
                maxLength={6}
                inputMode="numeric"
                autoComplete="one-time-code"
              />
              {codeError && (
                <p className="text-xs text-destructive">{codeError}</p>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => { setStep('qr'); }}>
                {t('accountSettingsPage.backStep')}
              </Button>
              <Button
                onClick={handleConfirm}
                disabled={code.length !== 6 || confirm.isPending}
              >
                {confirm.isPending ? t('accountSettingsPage.verifying') : t('accountSettingsPage.confirm')}
              </Button>
            </DialogFooter>
          </div>
        )}

        {/* Step 3: Backup codes */}
        {step === 'backup' && (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              <strong>{t('accountSettingsPage.backupCodesTitle')}:</strong> {t('accountSettingsPage.backupCodesDesc')}
            </p>
            <div className="p-3 rounded bg-muted font-mono text-sm grid grid-cols-2 gap-1">
              {backupCodes.map((c) => (
                <span key={c} className="select-all">
                  {c}
                </span>
              ))}
            </div>
            <Button variant="outline" className="w-full" onClick={copyBackupCodes}>
              {copied ? (
                <>
                  <Check className="mr-2 h-4 w-4 text-green-500" /> {t('accountSettingsPage.copied')}
                </>
              ) : (
                <>
                  <Copy className="mr-2 h-4 w-4" /> {t('accountSettingsPage.copyCodes')}
                </>
              )}
            </Button>
            {recoveryCodes.length > 0 && (
              <Button
                variant="outline"
                className="w-full"
                onClick={() => { setRecoveryDialogOpen(true); }}
              >
                {t('accountSettingsPage.showRecoveryCodes')}
              </Button>
            )}
            <DialogFooter>
              <Button onClick={handleClose} className="w-full">
                {t('accountSettingsPage.twoFANowActive')}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>

      <RecoveryCodesDialog
        open={recoveryDialogOpen}
        codes={recoveryCodes}
        onClose={() => { setRecoveryDialogOpen(false); }}
      />
    </Dialog>
  )
}

// ─── Disable Dialog ───────────────────────────────────────────────────────────

function DisableDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const [code, setCode] = useState('')
  const [codeError, setCodeError] = useState('')
  const disable = useDisable2FA()

  function handleDisable() {
    setCodeError('')
    disable.mutate(
      { code },
      {
        onSuccess: () => {
          setCode('')
          onClose()
        },
        onError: (err) => {
          setCodeError(err.message)
        },
      },
    )
  }

  function handleClose() {
    setCode('')
    setCodeError('')
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) handleClose() }}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{t('accountSettingsPage.disableDialogTitle')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            {t('accountSettingsPage.disableDesc')}
          </p>
          <div className="space-y-1">
            <Label htmlFor="disable-code">{t('accountSettingsPage.authCodeLabel')}</Label>
            <Input
              id="disable-code"
              value={code}
              onChange={(e) => { setCode(e.target.value); }}
              placeholder="123456"
              maxLength={6}
              inputMode="numeric"
              autoComplete="one-time-code"
            />
            {codeError && (
              <p className="text-xs text-destructive">{codeError}</p>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="destructive"
            onClick={handleDisable}
            disabled={code.length !== 6 || disable.isPending}
          >
            {disable.isPending ? t('accountSettingsPage.disabling') : t('accountSettingsPage.disableDialogTitle')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function AccountSettingsPage() {
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const { data: totpStatus, isLoading } = useTOTPStatus()
  const [setupOpen, setSetupOpen] = useState(false)
  const [disableOpen, setDisableOpen] = useState(false)
  const [regeneratedCodes, setRegeneratedCodes] = useState<string[]>([])
  const [regenerateDialogOpen, setRegenerateDialogOpen] = useState(false)

  const regenerate = useRegenerateRecoveryCodes()

  function handleRegenerate() {
    regenerate.mutate(undefined, {
      onSuccess: (data) => {
        setRegeneratedCodes(data.recovery_codes)
        setRegenerateDialogOpen(true)
      },
    })
  }

  const is2FAEnabled = totpStatus?.enabled ?? false

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title={t('accountSettingsPage.title')}
        description={t('accountSettingsPage.description')}
      />

      {/* ── Profil ────────────────────────────────────────────────────────── */}
      <Card className="p-6 space-y-4">
        <h2 className="text-base font-semibold">{t('accountSettingsPage.profileTitle')}</h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1">
            <Label className="text-xs text-muted-foreground">{t('accountSettingsPage.labelEmail')}</Label>
            <p className="text-sm font-medium">{user?.email ?? '–'}</p>
          </div>
          <div className="space-y-1">
            <Label className="text-xs text-muted-foreground">{t('accountSettingsPage.labelDisplayName')}</Label>
            <p className="text-sm font-medium">{user?.display_name ?? '–'}</p>
          </div>
        </div>
      </Card>

      {/* ── Zwei-Faktor-Authentifizierung ────────────────────────────────── */}
      <Card className="p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <h2 className="text-base font-semibold flex items-center gap-2">
              {t('accountSettingsPage.twoFATitle')}
              {!isLoading && (
                is2FAEnabled ? (
                  <Badge className="bg-green-900/40 text-green-300 border-green-800/40 text-xs">
                    <ShieldCheck className="mr-1 h-3 w-3" />
                    {t('accountSettingsPage.twoFAActive')}
                  </Badge>
                ) : (
                  <Badge variant="outline" className="text-xs text-muted-foreground">
                    <ShieldOff className="mr-1 h-3 w-3" />
                    {t('accountSettingsPage.twoFAInactive')}
                  </Badge>
                )
              )}
            </h2>
            <p className="text-sm text-muted-foreground">
              {is2FAEnabled
                ? t('accountSettingsPage.twoFAEnabled')
                : t('accountSettingsPage.twoFADisabled')}
            </p>
          </div>
        </div>

        <div className="flex gap-2 flex-wrap">
          {!is2FAEnabled && (
            <Button onClick={() => { setSetupOpen(true); }} disabled={isLoading}>
              <ShieldCheck className="mr-2 h-4 w-4" />
              {t('accountSettingsPage.enable2FA')}
            </Button>
          )}
          {is2FAEnabled && (
            <Button
              variant="destructive"
              onClick={() => { setDisableOpen(true); }}
            >
              <ShieldOff className="mr-2 h-4 w-4" />
              {t('accountSettingsPage.disable2FA')}
            </Button>
          )}
        </div>
      </Card>

      {/* ── Wiederherstellungscodes ───────────────────────────────────────── */}
      {is2FAEnabled && (
        <Card className="p-6 space-y-4">
          <div className="space-y-1">
            <h2 className="text-base font-semibold flex items-center gap-2">
              <RefreshCw className="h-4 w-4" />
              {t('accountSettingsPage.recoveryCodesTitle')}
            </h2>
            <p className="text-sm text-muted-foreground">
              {t('accountSettingsPage.recoveryCodesDesc')}
            </p>
          </div>
          <Button
            variant="outline"
            onClick={handleRegenerate}
            disabled={regenerate.isPending}
          >
            <RefreshCw className="mr-2 h-4 w-4" />
            {regenerate.isPending ? t('accountSettingsPage.generating') : t('accountSettingsPage.regenerateCodes')}
          </Button>
          {regenerate.isError && (
            <p className="text-xs text-destructive">{regenerate.error.message}</p>
          )}
        </Card>
      )}

      {/* ── Aktive Sitzungen ─────────────────────────────────────────────── */}
      <Card className="p-6 space-y-3">
        <div className="flex items-center gap-2">
          <Monitor className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-base font-semibold">{t('settings.sessionsPage.title')}</h2>
        </div>
        <p className="text-sm text-muted-foreground">
          {t('settings.sessionsPage.description')}
        </p>
        <Link
          to="/account/sessions"
          className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline"
        >
          {t('settings.sessionsPage.title')} <ExternalLink className="h-3.5 w-3.5" />
        </Link>
      </Card>

      {/* Sprint 22 S22-11: Login-History */}
      <LoginHistorySection />

      <SetupDialog open={setupOpen} onClose={() => { setSetupOpen(false); }} />
      <DisableDialog open={disableOpen} onClose={() => { setDisableOpen(false); }} />
      <RecoveryCodesDialog
        open={regenerateDialogOpen}
        codes={regeneratedCodes}
        onClose={() => {
          setRegenerateDialogOpen(false)
          setRegeneratedCodes([])
        }}
      />
    </div>
  )
}
