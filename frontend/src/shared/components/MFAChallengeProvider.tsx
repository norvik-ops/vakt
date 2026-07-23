import { useEffect, useRef, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '../../components/ui/dialog'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import { registerMFAChallengeHandler } from '../../api/client'

// MFAChallengeProvider renders the step-up TOTP prompt that apiFetch drives when
// a sensitive write returns MFA_TOKEN_REQUIRED (org opted into
// require_mfa_sensitive_calls, S131-R-H24). It registers a single global handler
// that opens the dialog and resolves with the entered code (or null on cancel);
// apiFetch then retries the original request with the X-MFA-Token header. Mount
// it once, high in the tree — it renders nothing until challenged.
export function MFAChallengeProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  const [invalid, setInvalid] = useState(false)
  const [code, setCode] = useState('')
  const resolverRef = useRef<((code: string | null) => void) | null>(null)

  useEffect(() => {
    registerMFAChallengeHandler((wasInvalid: boolean) => {
      // If a challenge is already pending (two sensitive writes raced), settle
      // the older one as cancelled so its apiFetch await cannot hang forever.
      resolverRef.current?.(null)
      setInvalid(wasInvalid)
      setCode('')
      setOpen(true)
      return new Promise<string | null>(resolve => {
        resolverRef.current = resolve
      })
    })
  }, [])

  function resolve(value: string | null) {
    const r = resolverRef.current
    resolverRef.current = null
    setOpen(false)
    if (r) r(value)
  }

  function handleOpenChange(next: boolean) {
    // Closing the dialog by any means (Esc, backdrop) counts as cancel.
    if (!next) resolve(null)
  }

  function submit() {
    if (code.trim().length < 6) return
    resolve(code.trim())
  }

  return (
    <>
      {children}
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Bestätigung mit 2FA erforderlich</DialogTitle>
            <DialogDescription>
              Für diese sicherheitsrelevante Aktion verlangt deine Organisation einen aktuellen
              Code aus deiner Authenticator-App.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="mfa-stepup-code">6-stelliger Code</Label>
            <Input
              id="mfa-stepup-code"
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={6}
              placeholder="123456"
              value={code}
              autoFocus
              onChange={e => {
                setCode(e.target.value.replace(/\D/g, ''))
              }}
              onKeyDown={e => {
                if (e.key === 'Enter') submit()
              }}
            />
            {invalid && (
              <p className="text-sm text-destructive">
                Code ungültig oder abgelaufen — bitte erneut versuchen.
              </p>
            )}
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                resolve(null)
              }}
            >
              Abbrechen
            </Button>
            <Button
              onClick={() => {
                submit()
              }}
              disabled={code.trim().length < 6}
            >
              Bestätigen
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
