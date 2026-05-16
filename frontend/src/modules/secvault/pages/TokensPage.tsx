import { useState, useEffect } from 'react'
import { KeyRound, Plus, Trash2, Copy, Check } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Card, CardContent } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useTokens, useCreateToken, useDeleteToken } from '../hooks/useTokens'
import { ProGate } from '../../../shared/components/ProGate'

const AVAILABLE_SCOPES = ['secrets:read', 'secrets:write', 'scans:trigger', 'tokens:read']

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => setCopied(false), 2000)
    return () => clearTimeout(id)
  }, [copied])

  function handle() {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
    })
  }
  return (
    <Button variant="outline" size="sm" onClick={handle} className="gap-1">
      {copied ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
      {copied ? 'Copied' : 'Copy'}
    </Button>
  )
}

export default function TokensPage() {
  const { data: tokens, isLoading, error: tokensError } = useTokens()
  const createToken = useCreateToken()
  const deleteToken = useDeleteToken()

  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [selectedScopes, setSelectedScopes] = useState<Set<string>>(new Set(['secrets:read']))
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  function toggleScope(scope: string) {
    setSelectedScopes((prev) => {
      const next = new Set(prev)
      if (next.has(scope)) next.delete(scope)
      else next.add(scope)
      return next
    })
  }

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    createToken.mutate(
      { name: name.trim(), scopes: Array.from(selectedScopes) },
      {
        onSuccess: (data) => {
          setCreatedToken(data.token)
          setName('')
          setSelectedScopes(new Set(['secrets:read']))
        },
      },
    )
  }

  function handleDelete() {
    if (!deleteId) return
    deleteToken.mutate(deleteId, { onSuccess: () => setDeleteId(null) })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="API-Tokens"
        description="API-Tokens für programmatischen Zugriff verwalten."
        actions={
          <Button onClick={() => { setOpen(true); setCreatedToken(null) }}>
            <Plus className="w-4 h-4 mr-1" />
            Create Token
          </Button>
        }
      />

      <div className="flex-1 p-6">
        <ProGate error={tokensError}>
        {isLoading ? (
          <div className="flex justify-center py-16">
            <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        ) : !tokens || tokens.length === 0 ? (
          <EmptyState
            icon={KeyRound}
            title="Noch keine API-Tokens vorhanden"
            description="Erstellen Sie einen Access Token, um Secrets aus CI/CD oder anderen Systemen abzurufen."
            action={
              <Button onClick={() => setOpen(true)}>
                <Plus className="w-4 h-4 mr-1" />Create Token
              </Button>
            }
          />
        ) : (
          <div className="rounded-md border border-border bg-surface overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Scopes</TableHead>
                  <TableHead>Zuletzt verwendet</TableHead>
                  <TableHead>Erstellt</TableHead>
                  <TableHead></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tokens.map((token) => (
                  <TableRow key={token.id}>
                    <TableCell className="font-medium">{token.name}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {token.scopes.map((s) => (
                          <Badge key={s} variant="outline" className="text-xs font-mono">{s}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-secondary">
                      {token.last_used_at ? new Date(token.last_used_at).toLocaleDateString() : 'Never'}
                    </TableCell>
                    <TableCell className="text-sm text-secondary">
                      {new Date(token.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-red-500 hover:text-red-700"
                        onClick={() => setDeleteId(token.id)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
        </ProGate>
      </div>

      {/* Create dialog */}
      <Dialog open={open} onOpenChange={(v) => { setOpen(v); if (!v) setCreatedToken(null) }}>
        <DialogContent>
          <DialogHeader><DialogTitle>API-Token erstellen</DialogTitle></DialogHeader>
          {createdToken ? (
            <div className="py-4 space-y-3">
              <p className="text-sm text-secondary">
                Copy this token now — it won't be shown again.
              </p>
              <Card>
                <CardContent className="py-3 flex items-center gap-3">
                  <code className="font-mono text-xs text-primary flex-1 break-all">{createdToken}</code>
                  <CopyButton text={createdToken} />
                </CardContent>
              </Card>
              <DialogFooter>
                <Button onClick={() => { setOpen(false); setCreatedToken(null) }}>Fertig</Button>
              </DialogFooter>
            </div>
          ) : (
            <form onSubmit={(e) => { void handleCreate(e) }}>
              <div className="py-4 space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="token-name">Token-Name</Label>
                  <Input
                    id="token-name"
                    placeholder="github-actions-prod"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>Scopes</Label>
                  <div className="grid grid-cols-2 gap-2">
                    {AVAILABLE_SCOPES.map((scope) => (
                      <label key={scope} className="flex items-center gap-2 text-sm cursor-pointer">
                        <input
                          type="checkbox"
                          className="rounded"
                          checked={selectedScopes.has(scope)}
                          onChange={() => toggleScope(scope)}
                        />
                        <span className="font-mono text-xs">{scope}</span>
                      </label>
                    ))}
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setOpen(false)}>Abbrechen</Button>
                <Button type="submit" disabled={createToken.isPending || !name.trim() || selectedScopes.size === 0}>
                  {createToken.isPending ? 'Creating…' : 'Create Token'}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <Dialog open={!!deleteId} onOpenChange={(open) => !open && setDeleteId(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>Revoke Token</DialogTitle></DialogHeader>
          <p className="text-sm text-secondary py-2">This will permanently revoke the token. Any systems using it will lose access.</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteId(null)}>Abbrechen</Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleteToken.isPending}>
              {deleteToken.isPending ? 'Revoking…' : 'Revoke'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
