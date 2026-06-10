import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Copy, Trash2, Plus, UserCheck } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../../../components/ui/alert-dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import {
  useAuditorInvites,
  useCreateAuditorInvite,
  useRevokeAuditorInvite,
  type AuditorInvite,
  type CreateInviteInput,
} from '../../../hooks/useAuditor'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function inviteStatus(invite: AuditorInvite, t: (key: string) => string): { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' } {
  if (invite.accepted_at) return { label: t('settings.auditor.statusActive'), variant: 'default' }
  if (new Date(invite.expires_at) < new Date()) return { label: t('settings.auditor.statusExpired'), variant: 'destructive' }
  return { label: t('settings.auditor.statusPending'), variant: 'secondary' }
}

// ---------------------------------------------------------------------------
// Create Invite Dialog
// ---------------------------------------------------------------------------

interface CreateDialogProps {
  open: boolean
  onClose: () => void
}

function CreateInviteDialog({ open, onClose }: CreateDialogProps) {
  const { t } = useTranslation()
  const [email, setEmail] = useState('')
  const [expiresIn, setExpiresIn] = useState('30')
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [createdUrl, setCreatedUrl] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const create = useCreateAuditorInvite()

  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => { setCopied(false); }, 2000)
    return () => { clearTimeout(id); }
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
    <Dialog open={open} onOpenChange={(v) => { if (!v) { handleClose(); } }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('settings.auditor.invite')}</DialogTitle>
        </DialogHeader>

        {createdToken ? (
          <div className="space-y-4">
            <p className="text-sm text-secondary">
              {t('settings.auditor.inviteLinkCreated')}
            </p>
            <div className="flex items-center gap-2">
              <Input
                readOnly
                value={createdUrl ?? ''}
                className="font-mono text-xs"
              />
              <Button variant="outline" size="sm" onClick={handleCopy}>
                <Copy className="w-4 h-4 mr-1" />
                {copied ? t('settings.auditor.copied') : t('settings.auditor.copy')}
              </Button>
            </div>
            <DialogFooter>
              <Button onClick={handleClose}>{t('settings.auditor.close')}</Button>
            </DialogFooter>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="space-y-1">
              <Label htmlFor="auditor-email">{t('settings.auditor.emailAddress')}</Label>
              <Input
                id="auditor-email"
                type="email"
                placeholder="auditor@example.com"
                value={email}
                onChange={(e) => { setEmail(e.target.value); }}
              />
            </div>
            <div className="space-y-1">
              <Label>{t('settings.auditor.validity')}</Label>
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
              <Button variant="outline" onClick={handleClose}>{t('settings.auditor.cancel')}</Button>
              <Button
                onClick={handleSave}
                disabled={!email.trim() || create.isPending}
              >
                {create.isPending ? t('settings.auditor.creating') : t('settings.auditor.inviteAction')}
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
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [revokeTarget, setRevokeTarget] = useState<{ id: string; email: string } | null>(null)
  const { data: invites = [], isLoading } = useAuditorInvites()
  const revoke = useRevokeAuditorInvite()
  const { formatDateTime } = useFormatDate()

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
        title={t('settings.auditor.title')}
        description={t('settings.auditor.description')}
        actions={
          <Button onClick={() => { setDialogOpen(true); }}>
            <Plus className="w-4 h-4 mr-2" />
            {t('settings.auditor.invite')}
          </Button>
        }
      />

      <div className="rounded-lg border border-border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('settings.auditor.email')}</TableHead>
              <TableHead>{t('settings.auditor.status')}</TableHead>
              <TableHead>{t('settings.auditor.createdAt')}</TableHead>
              <TableHead>{t('settings.auditor.expiresAt')}</TableHead>
              <TableHead>{t('settings.auditor.activatedAt')}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-secondary py-8">
                  {t('settings.auditor.loading')}
                </TableCell>
              </TableRow>
            )}
            {!isLoading && invites.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="text-center py-12">
                  <div className="flex flex-col items-center gap-2 text-secondary">
                    <UserCheck className="w-8 h-8 opacity-40" />
                    <p className="text-sm">{t('settings.auditor.noAuditors')}</p>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {invites.map((invite) => {
              const { label, variant } = inviteStatus(invite, t)
              return (
                <TableRow key={invite.id}>
                  <TableCell className="font-medium">{invite.email}</TableCell>
                  <TableCell>
                    <Badge variant={variant}>{label}</Badge>
                  </TableCell>
                  <TableCell className="text-secondary text-sm">
                    {formatDateTime(invite.created_at, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })}
                  </TableCell>
                  <TableCell className="text-secondary text-sm">
                    {formatDateTime(invite.expires_at, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })}
                  </TableCell>
                  <TableCell className="text-secondary text-sm">
                    {invite.accepted_at ? formatDateTime(invite.accepted_at, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : '—'}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => { handleRevoke(invite.id, invite.email); }}
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

      <CreateInviteDialog open={dialogOpen} onClose={() => { setDialogOpen(false); }} />

      <AlertDialog open={revokeTarget !== null} onOpenChange={(open) => { if (!open) setRevokeTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('settings.auditor.revokeConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('settings.auditor.revokeConfirmDesc')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('settings.auditor.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmRevoke} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              {t('settings.auditor.revoke')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
