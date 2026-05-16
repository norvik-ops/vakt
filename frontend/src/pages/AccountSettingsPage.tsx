import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ShieldCheck, ShieldOff, Copy, Check, RefreshCw } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Badge } from '../components/ui/badge'
import { Card } from '../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../components/ui/dialog'
import { apiFetch } from '../api/client'
import { useAuthStore } from '../shared/stores/auth'

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
  return useMutation<SetupResponse, Error>({
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

// ─── Setup Dialog ─────────────────────────────────────────────────────────────

type SetupStep = 'qr' | 'confirm' | 'backup'

function SetupDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [step, setStep] = useState<SetupStep>('qr')
  const [code, setCode] = useState('')
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  const [copied, setCopied] = useState(false)
  const [setupData, setSetupData] = useState<SetupResponse | null>(null)
  const [codeError, setCodeError] = useState('')

  const setup = useSetup2FA()
  const confirm = useConfirm2FA()

  function handleOpen() {
    if (step === 'qr' && !setupData) {
      setup.mutate(undefined, {
        onSuccess: (data) => setSetupData(data),
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
    setCopied(false)
    setSetupData(null)
    setCodeError('')
    onClose()
  }

  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => setCopied(false), 2000)
    return () => clearTimeout(id)
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
          <DialogTitle>Zwei-Faktor-Authentifizierung einrichten</DialogTitle>
        </DialogHeader>

        {/* Step 1: QR / URI */}
        {step === 'qr' && (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Scanne diesen Link in deiner Authenticator-App (z.B. Aegis, Google
              Authenticator, 1Password) oder gib den Secret-Key manuell ein.
            </p>
            {setup.isPending && (
              <p className="text-sm text-muted-foreground animate-pulse">
                Generiere Secret...
              </p>
            )}
            {setupData && (
              <div className="space-y-3">
                <div>
                  <Label className="text-xs text-muted-foreground mb-1 block">
                    Secret-Key (manuell eingeben)
                  </Label>
                  <code className="block p-2 rounded bg-muted text-xs break-all select-all font-mono">
                    {setupData.secret}
                  </code>
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground mb-1 block">
                    OTP-Auth Link (in Authenticator-App scannen / importieren)
                  </Label>
                  <a
                    href={setupData.uri}
                    className="block p-2 rounded bg-muted text-xs break-all text-blue-400 hover:underline font-mono"
                    title="In Authenticator-App öffnen"
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
                Abbrechen
              </Button>
              <Button
                onClick={() => setStep('confirm')}
                disabled={!setupData || setup.isPending}
              >
                Weiter
              </Button>
            </DialogFooter>
          </div>
        )}

        {/* Step 2: Confirm code */}
        {step === 'confirm' && (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Gib den 6-stelligen Code aus deiner Authenticator-App ein, um die
              Einrichtung abzuschließen.
            </p>
            <div className="space-y-1">
              <Label htmlFor="totp-code">Authentifizierungscode</Label>
              <Input
                id="totp-code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
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
              <Button variant="outline" onClick={() => setStep('qr')}>
                Zurück
              </Button>
              <Button
                onClick={handleConfirm}
                disabled={code.length !== 6 || confirm.isPending}
              >
                {confirm.isPending ? 'Prüfe...' : 'Bestätigen'}
              </Button>
            </DialogFooter>
          </div>
        )}

        {/* Step 3: Backup codes */}
        {step === 'backup' && (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              <strong>Wichtig:</strong> Speichere diese Backup-Codes sicher. Jeder Code
              kann nur einmal verwendet werden, falls du keinen Zugriff auf deine
              Authenticator-App hast.
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
                  <Check className="mr-2 h-4 w-4 text-green-500" /> Kopiert
                </>
              ) : (
                <>
                  <Copy className="mr-2 h-4 w-4" /> Codes kopieren
                </>
              )}
            </Button>
            <DialogFooter>
              <Button onClick={handleClose} className="w-full">
                2FA ist jetzt aktiv
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

// ─── Disable Dialog ───────────────────────────────────────────────────────────

function DisableDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
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
          <DialogTitle>2FA deaktivieren</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Gib deinen aktuellen Authentifizierungscode ein, um die
            Zwei-Faktor-Authentifizierung zu deaktivieren.
          </p>
          <div className="space-y-1">
            <Label htmlFor="disable-code">Authentifizierungscode</Label>
            <Input
              id="disable-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
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
            Abbrechen
          </Button>
          <Button
            variant="destructive"
            onClick={handleDisable}
            disabled={code.length !== 6 || disable.isPending}
          >
            {disable.isPending ? 'Deaktiviere...' : '2FA deaktivieren'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function AccountSettingsPage() {
  const { user } = useAuthStore()
  const { data: totpStatus, isLoading } = useTOTPStatus()
  const [setupOpen, setSetupOpen] = useState(false)
  const [disableOpen, setDisableOpen] = useState(false)

  const is2FAEnabled = totpStatus?.enabled ?? false

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Konto-Einstellungen"
        description="Verwalte dein Profil und deine Sicherheitseinstellungen."
      />

      {/* ── Profil ────────────────────────────────────────────────────────── */}
      <Card className="p-6 space-y-4">
        <h2 className="text-base font-semibold">Profil</h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1">
            <Label className="text-xs text-muted-foreground">E-Mail</Label>
            <p className="text-sm font-medium">{user?.email ?? '–'}</p>
          </div>
          <div className="space-y-1">
            <Label className="text-xs text-muted-foreground">Anzeigename</Label>
            <p className="text-sm font-medium">{user?.display_name ?? '–'}</p>
          </div>
        </div>
      </Card>

      {/* ── Zwei-Faktor-Authentifizierung ────────────────────────────────── */}
      <Card className="p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <h2 className="text-base font-semibold flex items-center gap-2">
              Zwei-Faktor-Authentifizierung
              {!isLoading && (
                is2FAEnabled ? (
                  <Badge className="bg-green-900/40 text-green-300 border-green-800/40 text-xs">
                    <ShieldCheck className="mr-1 h-3 w-3" />
                    Aktiv
                  </Badge>
                ) : (
                  <Badge variant="outline" className="text-xs text-muted-foreground">
                    <ShieldOff className="mr-1 h-3 w-3" />
                    Inaktiv
                  </Badge>
                )
              )}
            </h2>
            <p className="text-sm text-muted-foreground">
              {is2FAEnabled
                ? 'Dein Konto ist mit einem TOTP-Authenticator geschützt.'
                : 'Aktiviere 2FA mit einer Authenticator-App für mehr Sicherheit.'}
            </p>
          </div>
        </div>

        <div className="flex gap-2 flex-wrap">
          {!is2FAEnabled && (
            <Button onClick={() => setSetupOpen(true)} disabled={isLoading}>
              <ShieldCheck className="mr-2 h-4 w-4" />
              2FA aktivieren
            </Button>
          )}
          {is2FAEnabled && (
            <Button
              variant="destructive"
              onClick={() => setDisableOpen(true)}
            >
              <ShieldOff className="mr-2 h-4 w-4" />
              2FA deaktivieren
            </Button>
          )}
        </div>
      </Card>

      {/* ── Backup-Codes ─────────────────────────────────────────────────── */}
      {is2FAEnabled && (
        <Card className="p-6 space-y-4">
          <div className="space-y-1">
            <h2 className="text-base font-semibold flex items-center gap-2">
              <RefreshCw className="h-4 w-4" />
              Backup-Codes
            </h2>
            <p className="text-sm text-muted-foreground">
              Falls du keinen Zugriff auf deine Authenticator-App hast, kannst du
              neue Backup-Codes generieren. Bestehende Codes werden dabei
              ungültig.
            </p>
          </div>
          <Button
            variant="outline"
            onClick={() => setSetupOpen(true)}
          >
            <RefreshCw className="mr-2 h-4 w-4" />
            Neue Backup-Codes generieren
          </Button>
        </Card>
      )}

      <SetupDialog open={setupOpen} onClose={() => setSetupOpen(false)} />
      <DisableDialog open={disableOpen} onClose={() => setDisableOpen(false)} />
    </div>
  )
}
