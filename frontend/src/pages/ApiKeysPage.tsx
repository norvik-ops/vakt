import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Key, Plus, Trash2, Copy, Check, AlertTriangle, RotateCw, ScrollText } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card } from '../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../components/ui/dialog'
import { apiFetch } from '../api/client'
import { ProGate } from '../shared/components/ProGate'
import { toast } from '../shared/hooks/useToast'
import { SkeletonTable } from '../shared/components/SkeletonLoaders'
import { useFormatDate } from '../shared/hooks/useFormatDate'

// ─── Types ────────────────────────────────────────────────────────────────────

type APIKey = {
  id: string
  name: string
  key_prefix: string
  scopes: string[]
  last_used_at: string | null
  last_used_ip?: string | null
  expires_at: string | null
  created_at: string
  rotated_at?: string | null
}

// Scope-Liste passt zu den 6 Modulen aus CLAUDE.md. Wildcards `.*` geben dem
// Key vollen Zugriff auf das Modul; `*` ist Admin und sollte nur in seltenen
// Ausnahmefällen gesetzt werden.
const SCOPE_GROUPS: Array<{ label: string; description: string; scope: string }> = [
  { label: 'Vakt Comply', description: 'Controls, Evidence, NIS2/ISO/BSI', scope: 'secvitals.*' },
  { label: 'Vakt Scan', description: 'Scanner-Orchestrierung, Findings', scope: 'secpulse.*' },
  { label: 'Vakt Vault', description: 'Secrets, Git-Scan, Rotation', scope: 'secvault.*' },
  { label: 'Vakt Aware', description: 'Phishing-Sim, Trainings', scope: 'secreflex.*' },
  { label: 'Vakt Privacy', description: 'VVT, DPIA, AVV, Breaches', scope: 'secprivacy.*' },
  { label: 'Vakt HR', description: 'Onboarding / Offboarding', scope: 'hr.*' },
]

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

function useRotateAPIKey() {
  const qc = useQueryClient()
  return useMutation<CreateKeyResponse, Error, string>({
    mutationFn: (id) => apiFetch<CreateKeyResponse>(`/api-keys/${id}/rotate`, { method: 'POST' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  function handleCopy() {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => { setCopied(false); }, 2000)
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
        <><Check className="w-3.5 h-3.5 text-green-500" />{t('settings.apiKeysPage.copied')}</>
      ) : (
        <><Copy className="w-3.5 h-3.5" />{t('settings.apiKeysPage.copy')}</>
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
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [nameTouched, setNameTouched] = useState(false)
  const [selectedScopes, setSelectedScopes] = useState<Set<string>>(new Set())

  const createKey = useCreateAPIKey()

  function handleClose(isOpen: boolean) {
    if (!isOpen) {
      setName('')
      setExpiresAt('')
      setNameTouched(false)
      setSelectedScopes(new Set())
    }
    onOpenChange(isOpen)
  }

  function toggleScope(scope: string) {
    setSelectedScopes((prev) => {
      const next = new Set(prev)
      if (next.has(scope)) next.delete(scope)
      else next.add(scope)
      return next
    })
  }

  function handleCreate() {
    setNameTouched(true)
    if (!name.trim()) return

    const input: CreateKeyRequest = {
      name: name.trim(),
      scopes: Array.from(selectedScopes),
    }
    if (expiresAt) {
      input.expires_at = new Date(expiresAt).toISOString()
    }

    createKey.mutate(input, {
      onSuccess: (result) => {
        handleClose(false)
        onCreated(result.raw_key)
        toast(t('settings.apiKeysPage.toastCreated'), 'success')
      },
      onError: (err) => toast(`Fehler: ${err.message}`, 'error'),
    })
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('settings.apiKeysPage.createDialogTitle')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>{t('settings.apiKeysPage.labelName')} <span className="text-destructive">*</span></Label>
            <Input
              placeholder="z.B. GitHub Actions CI"
              value={name}
              onChange={(e) => { setName(e.target.value); }}
              onBlur={() => { setNameTouched(true); }}
              aria-invalid={nameTouched && !name.trim()}
            />
            {nameTouched && !name.trim() && (
              <p className="text-xs text-destructive">{t('settings.apiKeysPage.labelNameRequired')}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label>{t('settings.apiKeysPage.labelExpiry')} <span className="text-secondary text-xs">{t('settings.apiKeysPage.labelExpiryOptional')}</span></Label>
            <Input
              type="date"
              value={expiresAt}
              onChange={(e) => { setExpiresAt(e.target.value); }}
              min={new Date().toISOString().split('T')[0]}
            />
            <p className="text-[11px] text-secondary">{t('settings.apiKeysPage.labelExpiryHint')}</p>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Scopes</Label>
              <span className="text-[11px] text-secondary">
                {selectedScopes.size === 0 ? 'Personal-Key (Full Access)' : `${selectedScopes.size.toString()} ausgewählt`}
              </span>
            </div>
            <div className="space-y-1 rounded-lg border border-border p-2">
              {SCOPE_GROUPS.map((g) => {
                const checked = selectedScopes.has(g.scope)
                return (
                  <label
                    key={g.scope}
                    className="flex items-start gap-2.5 px-2 py-1.5 rounded hover:bg-muted/40 cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => { toggleScope(g.scope); }}
                      className="mt-0.5 h-4 w-4 rounded border-border"
                    />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-primary">{g.label}</span>
                        <code className="text-[10px] font-mono text-secondary bg-muted/30 px-1.5 py-0.5 rounded">{g.scope}</code>
                      </div>
                      <p className="text-[11px] text-secondary mt-0.5">{g.description}</p>
                    </div>
                  </label>
                )
              })}
            </div>
            <p className="text-[11px] text-secondary">
              Leer lassen → Personal-Key mit voller User-Berechtigung. Scopes setzen für CI-Bots
              mit Least-Privilege.
            </p>
          </div>
        </div>
        {createKey.isError && (
          <p className="text-xs text-destructive px-1">{createKey.error.message}</p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={() => { handleClose(false); }}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleCreate} disabled={createKey.isPending}>
            {createKey.isPending ? t('settings.apiKeysPage.creating') : t('settings.apiKeysPage.createSubmit')}
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
  const { t } = useTranslation()
  return (
    <Dialog open={rawKey !== null} onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('settings.apiKeysPage.newKeyTitle')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="flex items-start gap-3 p-3 rounded-lg bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800">
            <AlertTriangle className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
            <p className="text-sm text-amber-700 dark:text-amber-400 leading-relaxed">
              <strong>{t('settings.apiKeysPage.newKeyWarning')}</strong> {t('settings.apiKeysPage.newKeyWarningFull').replace(t('settings.apiKeysPage.newKeyWarning') + ' ', '')}
            </p>
          </div>
          <div className="space-y-2">
            <Label>{t('settings.apiKeysPage.newKeyLabel')}</Label>
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
          <Button onClick={onClose}>{t('settings.apiKeysPage.understood')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Rotate Dialog ────────────────────────────────────────────────────────────

type RotateDialogProps = {
  keyToRotate: APIKey | null
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
}

function RotateKeyDialog({ keyToRotate, onConfirm, onCancel, isPending }: RotateDialogProps) {
  return (
    <Dialog open={keyToRotate !== null} onOpenChange={(open) => { if (!open) onCancel() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Key rotieren</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-2">
          <p className="text-sm text-secondary">
            Der Key <strong className="text-primary">{keyToRotate?.name ?? ''}</strong> wird durch
            einen neuen ersetzt. Der alte Key bleibt für <strong>24 Stunden</strong> weiterhin
            gültig (Grace-Period), damit CI/CD-Pipelines Zeit zum Umstellen haben.
          </p>
          <div className="flex items-start gap-3 p-3 rounded-lg bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800">
            <AlertTriangle className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
            <div className="text-xs text-amber-700 dark:text-amber-400 leading-relaxed space-y-1">
              <p><strong>Während der Grace-Period:</strong></p>
              <ul className="list-disc list-inside space-y-0.5">
                <li>Alter Key liefert Header <code className="font-mono">X-Vakt-Key-Deprecated: true</code></li>
                <li>Pipelines sehen Warnung in Logs</li>
                <li>Nach 24h ist der alte Key endgültig ungültig</li>
              </ul>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={isPending}>
            Abbrechen
          </Button>
          <Button onClick={onConfirm} disabled={isPending}>
            <RotateCw className="mr-1.5 h-4 w-4" />
            {isPending ? 'Rotiere…' : 'Jetzt rotieren'}
          </Button>
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
  const { t } = useTranslation()
  return (
    <Dialog open={keyId !== null} onOpenChange={(open) => { if (!open) onCancel() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('settings.apiKeysPage.revokeDialogTitle')}</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-secondary py-2">
          Der Key <strong className="text-primary">{keyName}</strong> wird sofort ungültig.
          Alle Integrationen, die diesen Key verwenden, verlieren den Zugang.
          Dieser Vorgang kann nicht rückgängig gemacht werden.
        </p>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={isPending}>
            {t('common.cancel')}
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={isPending}>
            {isPending ? t('settings.apiKeysPage.revoking') : t('settings.apiKeysPage.revokeConfirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

function ApiKeysContent() {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const navigate = useNavigate()
  const [createOpen, setCreateOpen] = useState(false)
  const [newRawKey, setNewRawKey] = useState<string | null>(null)
  const [revokingKey, setRevokingKey] = useState<APIKey | null>(null)
  const [rotatingKey, setRotatingKey] = useState<APIKey | null>(null)

  const { data, isLoading, isError, error } = useAPIKeys()
  const revoke = useRevokeAPIKey()
  const rotate = useRotateAPIKey()

  const keys = data?.data ?? []

  function handleRevoke() {
    if (!revokingKey) return
    revoke.mutate(revokingKey.id, {
      onSuccess: () => {
        setRevokingKey(null)
        toast(t('settings.apiKeysPage.toastRevoked'), 'success')
      },
      onError: (err) => {
        setRevokingKey(null)
        toast(`Fehler: ${err.message}`, 'error')
      },
    })
  }

  function handleRotate() {
    if (!rotatingKey) return
    rotate.mutate(rotatingKey.id, {
      onSuccess: (result) => {
        setRotatingKey(null)
        setNewRawKey(result.raw_key)
        toast('Key rotiert — alter Key bleibt 24h gültig', 'success')
      },
      onError: (err) => {
        setRotatingKey(null)
        toast(`Fehler: ${err.message}`, 'error')
      },
    })
  }

  return (
    <ProGate error={isError ? error : null}>
      <div className="space-y-6 p-6">
        <div className="flex items-center justify-between">
          <PageHeader
            title={t('settings.apiKeysPage.title')}
            description={t('settings.apiKeysPage.description')}
          />
          <Button
            size="sm"
            className="gap-1.5 shrink-0"
            onClick={() => { setCreateOpen(true); }}
          >
            <Plus className="w-4 h-4" />
            {t('settings.apiKeysPage.createKey')}
          </Button>
        </div>

        <Card className="p-0 overflow-hidden">
          {/* Table header */}
          <div className="grid grid-cols-[2fr_1.5fr_1.5fr_1.5fr_auto] gap-x-4 px-4 py-2.5 border-b border-border bg-muted/30">
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.apiKeysPage.colNamePrefix')}</span>
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.apiKeysPage.colCreated')}</span>
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.apiKeysPage.colLastUsed')}</span>
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.apiKeysPage.colExpiry')}</span>
            <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.apiKeysPage.colActions')}</span>
          </div>

          {/* Loading */}
          {isLoading && (
            <div className="px-4 py-4">
              <SkeletonTable rows={3} cols={5} />
            </div>
          )}

          {/* Empty */}
          {!isLoading && !isError && keys.length === 0 && (
            <div className="px-4 py-10 flex flex-col items-center gap-3 text-center">
              <Key className="w-8 h-8 text-secondary opacity-40" />
              <div>
                <p className="text-sm font-medium text-primary">{t('settings.apiKeysPage.noKeys')}</p>
                <p className="text-xs text-secondary mt-1">
                  {t('settings.apiKeysPage.noKeysDesc')}
                </p>
              </div>
            </div>
          )}

          {/* Rows */}
          {keys.map((key) => {
            const inGrace = key.rotated_at && new Date(key.rotated_at).getTime() > Date.now() - 24 * 60 * 60 * 1000
            return (
              <div
                key={key.id}
                className="grid grid-cols-[2fr_1.5fr_1.5fr_1.5fr_auto] gap-x-4 items-start px-4 py-3 border-b border-border last:border-0"
              >
                <div className="min-w-0 space-y-1">
                  <div className="text-sm font-medium text-primary truncate">{key.name}</div>
                  <div className="flex items-center gap-1 text-xs font-mono text-secondary">
                    <span>{key.key_prefix}…</span>
                    <CopyButton text={key.key_prefix} />
                  </div>
                  {/* Scope-Tags */}
                  <div className="flex flex-wrap gap-1 pt-0.5">
                    {key.scopes.length === 0 ? (
                      <span className="text-[10px] uppercase font-semibold tracking-wide px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300">
                        Personal (Full Access)
                      </span>
                    ) : (
                      key.scopes.map((s) => (
                        <code
                          key={s}
                          className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-brand/10 text-brand"
                        >
                          {s}
                        </code>
                      ))
                    )}
                    {inGrace && (
                      <span className="text-[10px] uppercase font-semibold tracking-wide px-1.5 py-0.5 rounded bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                        Grace 24h aktiv
                      </span>
                    )}
                  </div>
                </div>
                <span className="text-sm text-secondary">{formatDate(key.created_at)}</span>
                <div className="text-sm text-secondary">
                  <div>{key.last_used_at ? formatDate(key.last_used_at) : '–'}</div>
                  {key.last_used_ip && (
                    <div className="text-[10px] font-mono text-muted mt-0.5">{key.last_used_ip}</div>
                  )}
                </div>
                <span className="text-sm text-secondary">
                  {key.expires_at ? formatDate(key.expires_at) : t('settings.apiKeysPage.never')}
                </span>
                <div className="flex items-center gap-0.5">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-secondary hover:text-primary hover:bg-muted/40"
                    onClick={() => { void navigate('/settings/audit-log'); }}
                    title="Audit-Trail anzeigen"
                  >
                    <ScrollText className="w-4 h-4" />
                    <span className="sr-only">Audit-Trail anzeigen</span>
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-secondary hover:text-primary hover:bg-muted/40"
                    onClick={() => { setRotatingKey(key); }}
                    title="Key rotieren (24h Grace-Period)"
                  >
                    <RotateCw className="w-4 h-4" />
                    <span className="sr-only">Rotieren</span>
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-destructive hover:text-destructive hover:bg-destructive/10"
                    onClick={() => { setRevokingKey(key); }}
                  >
                    <Trash2 className="w-4 h-4" />
                    <span className="sr-only">{t('settings.apiKeysPage.revokeKey')}</span>
                  </Button>
                </div>
              </div>
            )
          })}
        </Card>

        <p className="text-xs text-secondary">
          {t('settings.apiKeysPage.securityHint')}
        </p>

        <CreateKeyDialog
          open={createOpen}
          onOpenChange={setCreateOpen}
          onCreated={(raw) => { setNewRawKey(raw); }}
        />

        <NewKeyDialog
          rawKey={newRawKey}
          onClose={() => { setNewRawKey(null); }}
        />

        <ConfirmRevokeDialog
          keyId={revokingKey?.id ?? null}
          keyName={revokingKey?.name ?? ''}
          onConfirm={handleRevoke}
          onCancel={() => { setRevokingKey(null); }}
          isPending={revoke.isPending}
        />

        <RotateKeyDialog
          keyToRotate={rotatingKey}
          onConfirm={handleRotate}
          onCancel={() => { setRotatingKey(null); }}
          isPending={rotate.isPending}
        />
      </div>
    </ProGate>
  )
}

export default function ApiKeysPage() {
  return <ApiKeysContent />
}
