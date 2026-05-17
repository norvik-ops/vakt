import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Key, Plus, Trash2, Copy, Check, AlertTriangle } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card } from '../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../components/ui/dialog'
import { apiFetch } from '../api/client'
import { ProGate } from '../shared/components/ProGate'
import { toast } from '../shared/hooks/useToast'

// ─── Types ────────────────────────────────────────────────────────────────────

type APIKey = {
  id: string
  name: string
  key_prefix: string
  scopes: string[]
  last_used_at: string | null
  expires_at: string | null
  created_at: string
}

type APIKeyListResponse = {
  data: APIKey[]
}

type CreateKeyRequest = {
  name: string
  expires_at?: string
  scopes: string[]
}

type CreateKeyResponse = APIKey & {
  raw_key: string
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

function useAPIKeys() {
  return useQuery<APIKeyListResponse>({
    queryKey: ['api-keys'],
    queryFn: () => apiFetch<APIKeyListResponse>('/api-keys'),
    retry: false,
  })
}

function useCreateAPIKey() {
  const qc = useQueryClient()
  return useMutation<CreateKeyResponse, Error, CreateKeyRequest>({
    mutationFn: (input) =>
      apiFetch<CreateKeyResponse>('/api-keys', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })
}

function useRevokeAPIKey() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/api-keys/${id}`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatDate(iso: string | null): string {
  if (!iso) return '–'
  try {
    return new Intl.DateTimeFormat('de-DE', {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(iso))
  } catch {
    return iso
  }
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  function handleCopy() {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <Button
      variant="outline"
      size="sm"
      className="h-8 gap-1.5"
      onClick={handleCopy}
    >
      {copied ? (
        <><Check className="w-3.5 h-3.5 text-green-500" />Kopiert</>
      ) : (
        <><Copy className="w-3.5 h-3.5" />Kopieren</>
      )}
    </Button>
  )
}

// ─── Create Dialog ─────────────────────────────────────────────────────────────

type CreateDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (rawKey: string) => void
}

function CreateKeyDialog({ open, onOpenChange, onCreated }: CreateDialogProps) {
  const [name, setName] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [nameTouched, setNameTouched] = useState(false)

  const createKey = useCreateAPIKey()

  function handleClose(isOpen: boolean) {
    if (!isOpen) {
      setName('')
      setExpiresAt('')
      setNameTouched(false)
    }
    onOpenChange(isOpen)
  }

  function handleCreate() {
    setNameTouched(true)
    if (!name.trim()) return

    const input: CreateKeyRequest = {
      name: name.trim(),
      scopes: [],
    }
    if (expiresAt) {
      input.expires_at = new Date(expiresAt).toISOString()
    }

    createKey.mutate(input, {
      onSuccess: (result) => {
        handleClose(false)
        onCreated(result.raw_key)
        toast('API-Key erstellt', 'success')
      },
      onError: (err) => toast(`Fehler: ${err.message}`, 'error'),
    })
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Neuen API-Key erstellen</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>Name <span className="text-destructive">*</span></Label>
            <Input
              placeholder="z.B. GitHub Actions CI"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onBlur={() => setNameTouched(true)}
              aria-invalid={nameTouched && !name.trim()}
            />
            {nameTouched && !name.trim() && (
              <p className="text-xs text-destructive">Name ist erforderlich.</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label>Ablaufdatum <span className="text-secondary text-xs">(optional)</span></Label>
            <Input
              type="date"
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
              min={new Date().toISOString().split('T')[0]}
            />
            <p className="text-[11px] text-secondary">Leer lassen für unbegrenzte Gültigkeit.</p>
          </div>
        </div>
        {createKey.isError && (
          <p className="text-xs text-destructive px-1">{createKey.error.message}</p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={() => handleClose(false)}>
            Abbrechen
          </Button>
          <Button onClick={handleCreate} disabled={createKey.isPending}>
            {createKey.isPending ? 'Wird erstellt…' : 'Key erstellen'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── New Key Display Dialog ────────────────────────────────────────────────────

type NewKeyDialogProps = {
  rawKey: string | null
  onClose: () => void
}

function NewKeyDialog({ rawKey, onClose }: NewKeyDialogProps) {
  return (
    <Dialog open={rawKey !== null} onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>API-Key erstellt</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="flex items-start gap-3 p-3 rounded-lg bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800">
            <AlertTriangle className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
            <p className="text-sm text-amber-700 dark:text-amber-400 leading-relaxed">
              <strong>Dieser Key wird nur einmal angezeigt.</strong> Kopiere ihn jetzt — er kann danach nicht mehr eingesehen werden.
            </p>
          </div>
          <div className="space-y-2">
            <Label>Dein API-Key</Label>
            <div className="flex gap-2">
              <Input
                value={rawKey ?? ''}
                readOnly
                className="font-mono text-xs bg-surface2 flex-1"
              />
              <CopyButton text={rawKey ?? ''} />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button onClick={onClose}>Verstanden, schließen</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Confirm Revoke Dialog ────────────────────────────────────────────────────

type ConfirmRevokeProps = {
  keyId: string | null
  keyName: string
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
}

function ConfirmRevokeDialog({ keyId, keyName, onConfirm, onCancel, isPending }: ConfirmRevokeProps) {
  return (
    <Dialog open={keyId !== null} onOpenChange={(open) => { if (!open) onCancel() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>API-Key widerrufen?</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-secondary py-2">
          Der Key <strong className="text-primary">{keyName}</strong> wird sofort ungültig.
          Alle Integrationen, die diesen Key verwenden, verlieren den Zugang.
          Dieser Vorgang kann nicht rückgängig gemacht werden.
        </p>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={isPending}>
            Abbrechen
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={isPending}>
            {isPending ? 'Wird widerrufen…' : 'Widerrufen'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

function ApiKeysContent() {
  const [createOpen, setCreateOpen] = useState(false)
  const [newRawKey, setNewRawKey] = useState<string | null>(null)
  const [revokingKey, setRevokingKey] = useState<APIKey | null>(null)

  const { data, isLoading, isError, error } = useAPIKeys()
  const revoke = useRevokeAPIKey()

  const keys = data?.data ?? []

  function handleRevoke() {
    if (!revokingKey) return
    revoke.mutate(revokingKey.id, {
      onSuccess: () => {
        setRevokingKey(null)
        toast('API-Key widerrufen', 'success')
      },
      onError: (err) => {
        setRevokingKey(null)
        toast(`Fehler: ${err.message}`, 'error')
      },
    })
  }

  return (
    <ProGate error={isError ? error : null}>
      <div className="space-y-6 p-6">
        <div className="flex items-center justify-between">
          <PageHeader
            title="API-Keys"
            description="Persönliche API-Keys für programmatischen Zugang (CI/CD, Integrationen)."
          />
          <Button
            size="sm"
            className="gap-1.5 shrink-0"
            onClick={() => setCreateOpen(true)}
          >
            <Plus className="w-4 h-4" />
            Neuen API-Key erstellen
          </Button>
        </div>

        <Card className="p-0 overflow-hidden">
          {/* Table header */}
          <div className="grid grid-cols-[2fr_1.5fr_1.5fr_1.5fr_auto] gap-x-4 px-4 py-2.5 border-b border-border bg-muted/30">
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Name / Prefix</span>
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Erstellt am</span>
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Zuletzt verwendet</span>
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Ablauf</span>
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Aktionen</span>
          </div>

          {/* Loading */}
          {isLoading && (
            <div className="px-4 py-8 text-center text-sm text-secondary">
              Lade API-Keys…
            </div>
          )}

          {/* Empty */}
          {!isLoading && !isError && keys.length === 0 && (
            <div className="px-4 py-10 flex flex-col items-center gap-3 text-center">
              <Key className="w-8 h-8 text-secondary opacity-40" />
              <div>
                <p className="text-sm font-medium text-primary">Keine API-Keys vorhanden</p>
                <p className="text-xs text-secondary mt-1">
                  Erstelle einen Key für CI/CD-Pipelines oder externe Integrationen.
                </p>
              </div>
            </div>
          )}

          {/* Rows */}
          {keys.map((key) => (
            <div
              key={key.id}
              className="grid grid-cols-[2fr_1.5fr_1.5fr_1.5fr_auto] gap-x-4 items-center px-4 py-3 border-b border-border last:border-0"
            >
              <div className="min-w-0">
                <div className="text-sm font-medium text-primary truncate">{key.name}</div>
                <div className="text-xs font-mono text-secondary mt-0.5">
                  {key.key_prefix}…
                </div>
              </div>
              <span className="text-sm text-secondary">{formatDate(key.created_at)}</span>
              <span className="text-sm text-secondary">{formatDate(key.last_used_at)}</span>
              <span className="text-sm text-secondary">
                {key.expires_at ? formatDate(key.expires_at) : 'Nie'}
              </span>
              <Button
                variant="ghost"
                size="sm"
                className="text-destructive hover:text-destructive hover:bg-destructive/10"
                onClick={() => setRevokingKey(key)}
              >
                <Trash2 className="w-4 h-4" />
                <span className="sr-only">Widerrufen</span>
              </Button>
            </div>
          ))}
        </Card>

        <p className="text-xs text-secondary">
          API-Keys gewähren den gleichen Zugang wie dein Benutzerkonto.
          Teile sie nicht und widerrufe sie, wenn sie nicht mehr benötigt werden.
        </p>

        <CreateKeyDialog
          open={createOpen}
          onOpenChange={setCreateOpen}
          onCreated={(raw) => setNewRawKey(raw)}
        />

        <NewKeyDialog
          rawKey={newRawKey}
          onClose={() => setNewRawKey(null)}
        />

        <ConfirmRevokeDialog
          keyId={revokingKey?.id ?? null}
          keyName={revokingKey?.name ?? ''}
          onConfirm={handleRevoke}
          onCancel={() => setRevokingKey(null)}
          isPending={revoke.isPending}
        />
      </div>
    </ProGate>
  )
}

export default function ApiKeysPage() {
  return <ApiKeysContent />
}
