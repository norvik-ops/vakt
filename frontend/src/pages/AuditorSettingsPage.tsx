import { useState, useEffect } from 'react'
import { Copy, Trash2, Plus, UserCheck } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Badge } from '../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../components/ui/alert-dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../components/ui/table'
import {
  useAuditorInvites,
  useCreateAuditorInvite,
  useRevokeAuditorInvite,
  type AuditorInvite,
  type CreateInviteInput,
} from '../hooks/useAuditor'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatDate(iso: string) {
  return new Date(iso).toLocaleString('de-DE', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function inviteStatus(invite: AuditorInvite): { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' } {
  if (invite.accepted_at) return { label: 'Aktiviert', variant: 'default' }
  if (new Date(invite.expires_at) < new Date()) return { label: 'Abgelaufen', variant: 'destructive' }
  return { label: 'Ausstehend', variant: 'secondary' }
}

// ---------------------------------------------------------------------------
// Create Invite Dialog
// ---------------------------------------------------------------------------

interface CreateDialogProps {
  open: boolean
  onClose: () => void
}

function CreateInviteDialog({ open, onClose }: CreateDialogProps) {
  const [email, setEmail] = useState('')
  const [expiresIn, setExpiresIn] = useState('30')
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [createdUrl, setCreatedUrl] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const create = useCreateAuditorInvite()

  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => setCopied(false), 2000)
    return () => clearTimeout(id)
  }, [copied])

  function handleSave() {
    if (!email.trim()) return
    const input: CreateInviteInput = { email: email.trim(), expires_in: parseInt(expiresIn, 10) }
    create.mutate(input, {
      onSuccess: (data) => {
        setCreatedToken(data.token)
        setCreatedUrl(window.location.origin + data.invite_url)
      },
    })
  }

  function handleCopy() {
    const link = createdUrl ?? ''
    void navigator.clipboard.writeText(link).then(() => {
      setCopied(true)
    })
  }

  function handleClose() {
    setEmail('')
    setExpiresIn('30')
    setCreatedToken(null)
    setCreatedUrl(null)
    setCopied(false)
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Auditor einladen</DialogTitle>
        </DialogHeader>

        {createdToken ? (
          <div className="space-y-4">
            <p className="text-sm text-secondary">
              Invite-Link erstellt. Sende diesen Link an den Auditor — er ist einmalig sichtbar.
            </p>
            <div className="flex items-center gap-2">
              <Input
                readOnly
                value={createdUrl ?? ''}
                className="font-mono text-xs"
              />
              <Button variant="outline" size="sm" onClick={handleCopy}>
                <Copy className="w-4 h-4 mr-1" />
                {copied ? 'Kopiert!' : 'Kopieren'}
              </Button>
            </div>
            <DialogFooter>
              <Button onClick={handleClose}>Schliessen</Button>
            </DialogFooter>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="space-y-1">
              <Label htmlFor="auditor-email">E-Mail-Adresse</Label>
              <Input
                id="auditor-email"
                type="email"
                placeholder="auditor@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="space-y-1">
              <Label>Gültigkeitsdauer</Label>
              <Select value={expiresIn} onValueChange={setExpiresIn}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="7">7 Tage</SelectItem>
                  <SelectItem value="14">14 Tage</SelectItem>
                  <SelectItem value="30">30 Tage</SelectItem>
                  <SelectItem value="60">60 Tage</SelectItem>
                  <SelectItem value="90">90 Tage</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={handleClose}>Abbrechen</Button>
              <Button
                onClick={handleSave}
                disabled={!email.trim() || create.isPending}
              >
                {create.isPending ? 'Erstelle...' : 'Einladen'}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function AuditorSettingsPage() {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [revokeTarget, setRevokeTarget] = useState<{ id: string; email: string } | null>(null)
  const { data: invites = [], isLoading } = useAuditorInvites()
  const revoke = useRevokeAuditorInvite()

  function handleRevoke(id: string, email: string) {
    setRevokeTarget({ id, email })
  }

  function confirmRevoke() {
    if (!revokeTarget) return
    revoke.mutate(revokeTarget.id)
    setRevokeTarget(null)
  }

  return (
    <div className="p-6 space-y-6 max-w-4xl">
      <PageHeader
        title="Auditoren"
        description="Erteile externen Auditoren zeitlich begrenzten Read-only-Zugang zu Frameworks, Controls und Nachweisen."
        actions={
          <Button onClick={() => setDialogOpen(true)}>
            <Plus className="w-4 h-4 mr-2" />
            Auditor einladen
          </Button>
        }
      />

      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>E-Mail</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Erstellt am</TableHead>
              <TableHead>Laeuft ab</TableHead>
              <TableHead>Aktiviert am</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-secondary py-8">
                  Laden...
                </TableCell>
              </TableRow>
            )}
            {!isLoading && invites.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="text-center py-12">
                  <div className="flex flex-col items-center gap-2 text-secondary">
                    <UserCheck className="w-8 h-8 opacity-40" />
                    <p className="text-sm">Noch keine Auditoren eingeladen</p>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {invites.map((invite) => {
              const { label, variant } = inviteStatus(invite)
              return (
                <TableRow key={invite.id}>
                  <TableCell className="font-medium">{invite.email}</TableCell>
                  <TableCell>
                    <Badge variant={variant}>{label}</Badge>
                  </TableCell>
                  <TableCell className="text-secondary text-sm">
                    {formatDate(invite.created_at)}
                  </TableCell>
                  <TableCell className="text-secondary text-sm">
                    {formatDate(invite.expires_at)}
                  </TableCell>
                  <TableCell className="text-secondary text-sm">
                    {invite.accepted_at ? formatDate(invite.accepted_at) : '—'}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleRevoke(invite.id, invite.email)}
                      disabled={revoke.isPending}
                      className="text-red-500 hover:text-red-600 hover:bg-red-50"
                    >
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>

      <CreateInviteDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />

      <AlertDialog open={revokeTarget !== null} onOpenChange={(open) => { if (!open) setRevokeTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Einladung widerrufen?</AlertDialogTitle>
            <AlertDialogDescription>
              Die Einladung von <strong>{revokeTarget?.email}</strong> wird widerrufen und der Zugang
              wird deaktiviert. Diese Aktion kann nicht rückgängig gemacht werden.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={confirmRevoke} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              Widerrufen
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
