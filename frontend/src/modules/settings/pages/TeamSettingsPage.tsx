import { useState } from 'react'
import { Plus, ShieldCheck, Trash2, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { useFieldValidation, required, email as emailRule } from '../../../shared/hooks/useFieldValidation'
import { FieldError } from '../../../shared/components/FieldError'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from '../../../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../../../components/ui/alert-dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useAuthStore } from '../../../shared/stores/auth'
import {
  useTeamMembers,
  useUpdateRole,
  useRemoveUser,
  useInvitations,
  useCreateInvitation,
  useRevokeInvitation,
  useCreateUser,
  type TeamMember,
  type TeamInvitation,
  type PlatformRole,
} from '../../../hooks/useTeam'
import { UserPermissionsEditor } from '../../../components/UserPermissionsEditor'
import { toast } from '../../../shared/hooks/useToast'
import { ErrorState } from '../../../shared/components/ErrorState'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type Role = 'admin' | 'editor' | 'viewer'

// The roles a member can be changed to, in the order the select shows them.
// These are the platform role names (roles.name) — the same strings the backend
// puts into an insufficient-role 403 ("requires role InternalAuditor"), so an
// operator who reads that message finds the identical word in this dropdown.
//
// InternalAuditor is here because ADR-0055 makes it mandatory from migration 202
// and expects an admin to assign it explicitly, while the platform offered no way
// to do so: org_members.role_id had nine INSERT sites in non-test code (seven
// outside the two seeders) and no UPDATE site, and this select only ever offered
// admin/editor/viewer, which are not the roles the guards check. A single-admin
// org therefore could not complete an audit (ISO 27001 9.2) without an UPDATE in
// the database (ESK-13).
//
// The two audit roles carry a descriptionKey. Admin/SecurityAnalyst/Viewer are
// the vocabulary this product has always used and an operator knows them; nobody
// can be expected to know from the name alone that InternalAuditor is the role
// that completes audits and that ADR-0055 forbids the same person from doing both
// (REV-ESK13 §2.3 — these strings existed in all four locales but were rendered
// nowhere).
const ASSIGNABLE_ROLES: { value: PlatformRole; labelKey: string; descriptionKey?: string }[] = [
  { value: 'Admin', labelKey: 'settings.team.roleAdmin' },
  { value: 'SecurityAnalyst', labelKey: 'settings.team.roleEditor' },
  { value: 'Viewer', labelKey: 'settings.team.roleViewer' },
  {
    value: 'AuditorReadOnly',
    labelKey: 'settings.team.roleAuditorReadOnly',
    descriptionKey: 'teamSettingsPage.roleAuditorReadOnly',
  },
  {
    value: 'InternalAuditor',
    labelKey: 'settings.team.roleInternalAuditor',
    descriptionKey: 'teamSettingsPage.roleInternalAuditor',
  },
]

function roleBadge(role: PlatformRole, t: (key: string) => string) {
  switch (role) {
    case 'Admin':
      return <Badge className="bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300 border-0">{t('settings.team.roleAdmin')}</Badge>
    case 'SecurityAnalyst':
      return <Badge className="bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0">{t('settings.team.roleEditor')}</Badge>
    case 'AuditorReadOnly':
      return <Badge variant="secondary">{t('settings.team.roleAuditorReadOnly')}</Badge>
    case 'InternalAuditor':
      return <Badge className="bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300 border-0">{t('settings.team.roleInternalAuditor')}</Badge>
    case 'Viewer':
      return <Badge variant="secondary">{t('settings.team.roleViewer')}</Badge>
  }
}

// inviteRoleBadge renders the invitation vocabulary, which is a separate,
// narrower set (POST /admin/invitations still takes admin/editor/viewer only).
function inviteRoleBadge(role: Role, t: (key: string) => string) {
  switch (role) {
    case 'admin':
      return roleBadge('Admin', t)
    case 'editor':
      return roleBadge('SecurityAnalyst', t)
    case 'viewer':
      return roleBadge('Viewer', t)
  }
}

function initials(name: string, email: string) {
  const src = name.trim() || email
  const parts = src.split(/[\s@]/).filter(Boolean)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return src.slice(0, 2).toUpperCase()
}

function daysUntil(iso: string) {
  const diff = new Date(iso).getTime() - Date.now()
  return Math.max(0, Math.round(diff / 86_400_000))
}

// ---------------------------------------------------------------------------
// Invite Dialog
// ---------------------------------------------------------------------------

interface InviteDialogProps {
  open: boolean
  onClose: () => void
}

function InviteDialog({ open, onClose }: InviteDialogProps) {
  const { t } = useTranslation()
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('editor')
  const create = useCreateInvitation()
  const emailValidation = useFieldValidation(email, [required, emailRule])

  function handleSend() {
    if (!email.trim() || emailValidation.error) return
    create.mutate({ email: email.trim(), role }, {
      onSuccess: () => {
        handleClose()
        toast(t('teamSettingsPage.invitationSent'), 'success')
      },
      onError: (err) => toast(`${t('common.error')}: ${err.message}`, 'error'),
    })
  }

  function handleClose() {
    setEmail('')
    setRole('editor')
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) { handleClose(); } }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('teamSettingsPage.inviteDialogTitle')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1">
            <Label htmlFor="invite-email">{t('teamSettingsPage.labelEmail')}</Label>
            <Input
              id="invite-email"
              type="email"
              placeholder={t('teamSettingsPage.placeholderEmail')}
              value={email}
              onChange={(e) => { setEmail(e.target.value); }}
              aria-invalid={!!emailValidation.error}
            />
            <FieldError error={emailValidation.error} />
          </div>
          <div className="space-y-1">
            <Label>{t('teamSettingsPage.labelRole')}</Label>
            <Select value={role} onValueChange={(v) => { setRole(v as Role); }}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="admin">{t('teamSettingsPage.roleAdmin')}</SelectItem>
                <SelectItem value="editor">{t('teamSettingsPage.roleEditor')}</SelectItem>
                <SelectItem value="viewer">{t('teamSettingsPage.roleViewer')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {create.error && (
            <p className="text-sm text-destructive">{create.error.message}</p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>{t('common.cancel')}</Button>
          <Button onClick={handleSend} disabled={!email.trim() || !!emailValidation.error || create.isPending}>
            {create.isPending ? t('teamSettingsPage.sending') : t('teamSettingsPage.sendInvitation')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------------------
// Create User Dialog (S105-1 — direct creation, no SMTP required)
// ---------------------------------------------------------------------------

type BackendRole = 'Admin' | 'SecurityAnalyst' | 'Viewer' | 'AuditorReadOnly'

interface CreateUserDialogProps {
  open: boolean
  onClose: () => void
}

function CreateUserDialog({ open, onClose }: CreateUserDialogProps) {
  const { t } = useTranslation()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<BackendRole>('Viewer')
  const create = useCreateUser()
  const emailValidation = useFieldValidation(email, [required, emailRule])

  function handleCreate() {
    if (!email.trim() || emailValidation.error || password.length < 10) return
    create.mutate({ email: email.trim(), password, role }, {
      onSuccess: () => {
        handleClose()
        toast(t('teamSettingsPage.userCreated'), 'success')
      },
      onError: (err) => toast(`${t('common.error')}: ${err.message}`, 'error'),
    })
  }

  function handleClose() {
    setEmail('')
    setPassword('')
    setRole('Viewer')
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) { handleClose(); } }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('teamSettingsPage.createUserDialogTitle')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1">
            <Label htmlFor="create-email">{t('teamSettingsPage.labelEmail')}</Label>
            <Input
              id="create-email"
              type="email"
              placeholder={t('teamSettingsPage.placeholderEmail')}
              value={email}
              onChange={(e) => { setEmail(e.target.value); }}
              aria-invalid={!!emailValidation.error}
            />
            <FieldError error={emailValidation.error} />
          </div>
          <div className="space-y-1">
            <Label htmlFor="create-password">{t('teamSettingsPage.labelPassword')}</Label>
            <Input
              id="create-password"
              type="password"
              placeholder={t('teamSettingsPage.placeholderPassword')}
              value={password}
              onChange={(e) => { setPassword(e.target.value); }}
              aria-invalid={password.length > 0 && password.length < 10}
            />
            {password.length > 0 && password.length < 10 && (
              <p className="text-xs text-destructive">{t('teamSettingsPage.passwordMinLen')}</p>
            )}
          </div>
          <div className="space-y-1">
            <Label>{t('teamSettingsPage.labelRole')}</Label>
            <Select value={role} onValueChange={(v) => { setRole(v as BackendRole); }}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="Admin">{t('teamSettingsPage.roleAdmin')}</SelectItem>
                <SelectItem value="SecurityAnalyst">{t('teamSettingsPage.roleEditor')}</SelectItem>
                <SelectItem value="Viewer">{t('teamSettingsPage.roleViewer')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {create.error && (
            <p className="text-sm text-destructive">{create.error.message}</p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>{t('common.cancel')}</Button>
          <Button
            onClick={handleCreate}
            disabled={!email.trim() || !!emailValidation.error || password.length < 10 || create.isPending}
          >
            {create.isPending ? t('teamSettingsPage.creating') : t('teamSettingsPage.createUser')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------------------
// Permissions Dialog
// ---------------------------------------------------------------------------

interface PermissionsDialogProps {
  member: TeamMember | null
  onClose: () => void
}

function PermissionsDialog({ member, onClose }: PermissionsDialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog open={member !== null} onOpenChange={(open) => { if (!open) { onClose(); } }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('teamSettingsPage.permissionsDialogTitle')}</DialogTitle>
          <DialogDescription>
            {member ? (member.name || member.email) : ''} — {t('teamSettingsPage.permissionsDialogDesc')}
          </DialogDescription>
        </DialogHeader>
        {member && <UserPermissionsEditor userId={member.id} />}
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------------------
// Members table
// ---------------------------------------------------------------------------

function MembersTable({ members, currentUserID }: { members: TeamMember[]; currentUserID: string }) {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const updateRole = useUpdateRole()
  const removeUser = useRemoveUser()
  const [removeTarget, setRemoveTarget] = useState<TeamMember | null>(null)
  const [permTarget, setPermTarget] = useState<TeamMember | null>(null)

  // Counted over platform_role, the role the backend's last-admin guard counts.
  const adminCount = members.filter((m) => m.platform_role === 'Admin').length

  function handleRoleChange(member: TeamMember, newRole: PlatformRole) {
    updateRole.mutate({ id: member.id, role: newRole }, {
      onSuccess: () => toast(t('teamSettingsPage.roleSaved'), 'success'),
      onError: (err) => toast(`${t('common.error')}: ${err.message}`, 'error'),
    })
  }

  function handleRemove(member: TeamMember) {
    setRemoveTarget(member)
  }

  function confirmRemove() {
    if (removeTarget) {
      removeUser.mutate(removeTarget.id, {
        onSuccess: () => toast(t('teamSettingsPage.deleted'), 'success'),
        onError: (err) => toast(`${t('common.error')}: ${err.message}`, 'error'),
      })
    }
    setRemoveTarget(null)
  }

  return (
    <div className="rounded-lg border border-border overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('teamSettingsPage.colMember')}</TableHead>
            <TableHead>{t('teamSettingsPage.colEmail')}</TableHead>
            <TableHead>{t('teamSettingsPage.colRole')}</TableHead>
            <TableHead>{t('teamSettingsPage.colMemberSince')}</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {members.length === 0 && (
            <TableRow>
              <TableCell colSpan={5} className="text-center py-10 text-secondary">
                {t('teamSettingsPage.noMembersFound')}
              </TableCell>
            </TableRow>
          )}
          {members.map((member) => {
            const isSelf = member.id === currentUserID
            const isLastAdmin = member.platform_role === 'Admin' && adminCount <= 1

            return (
              <TableRow key={member.id}>
                <TableCell>
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-full bg-brand/10 flex items-center justify-center text-[11px] font-semibold text-brand shrink-0">
                      {initials(member.name, member.email)}
                    </div>
                    <span className="font-medium text-sm">
                      {member.name || member.email.split('@')[0]}
                      {isSelf && <span className="ml-1 text-secondary text-xs">({t('teamSettingsPage.you')})</span>}
                    </span>
                  </div>
                </TableCell>
                <TableCell className="text-secondary text-sm">{member.email}</TableCell>
                <TableCell>
                  {isSelf || isLastAdmin ? (
                    roleBadge(member.platform_role, t)
                  ) : (
                    <Select
                      value={member.platform_role}
                      onValueChange={(v) => { handleRoleChange(member, v as PlatformRole); }}
                      disabled={updateRole.isPending}
                    >
                      <SelectTrigger className="h-7 w-40 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {ASSIGNABLE_ROLES.map((r) => (
                          <SelectItem key={r.value} value={r.value}>{t(r.labelKey)}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                </TableCell>
                <TableCell className="text-secondary text-sm">{formatDate(member.created_at)}</TableCell>
                <TableCell>
                  <div className="flex items-center justify-end gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => { setPermTarget(member); }}
                      title={t('teamSettingsPage.permissionsDialogTitle')}
                    >
                      <ShieldCheck className="w-4 h-4 text-secondary" />
                    </Button>
                    {!isSelf && !isLastAdmin && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => { handleRemove(member); }}
                        disabled={removeUser.isPending}
                        className="text-destructive hover:text-destructive hover:bg-destructive/10"
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>

      {/* What the two audit roles mean. The select can only show their names —
          Radix mirrors an item's content into the trigger, so a second line there
          would end up truncated in a 10rem-wide control. An operator picking
          between "Viewer", "AuditorReadOnly" and "InternalAuditor" cannot tell
          from the names which one completes audits, and that is the choice
          ADR-0055's segregation of duties turns on. Driven off ASSIGNABLE_ROLES,
          so a role that gains a description is documented here by adding it
          there. */}
      <dl className="border-t border-border px-4 py-3 space-y-1">
        {ASSIGNABLE_ROLES.filter((r) => r.descriptionKey).map((r) => (
          <div key={r.value} className="text-xs text-secondary">
            <dt className="sr-only">{t(r.labelKey)}</dt>
            <dd>{t(r.descriptionKey as string)}</dd>
          </div>
        ))}
      </dl>

      <PermissionsDialog member={permTarget} onClose={() => { setPermTarget(null); }} />

      <AlertDialog open={removeTarget !== null} onOpenChange={(open) => { if (!open) { setRemoveTarget(null); } }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('teamSettingsPage.removeDialogTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('teamSettingsPage.removeDialogDesc', { email: removeTarget?.email ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => { setRemoveTarget(null); }}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmRemove} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Invitations table
// ---------------------------------------------------------------------------

function InvitationsTable({ invitations }: { invitations: TeamInvitation[] }) {
  const { t } = useTranslation()
  const revoke = useRevokeInvitation()
  const [revokeTarget, setRevokeTarget] = useState<TeamInvitation | null>(null)

  const pending = invitations.filter((inv) => !inv.accepted_at)

  function handleRevoke(inv: TeamInvitation) {
    setRevokeTarget(inv)
  }

  function confirmRevoke() {
    if (revokeTarget) {
      revoke.mutate(revokeTarget.id, {
        onSuccess: () => toast(t('teamSettingsPage.revokeDialogTitle'), 'success'),
        onError: (err) => toast(`${t('common.error')}: ${err.message}`, 'error'),
      })
    }
    setRevokeTarget(null)
  }

  if (pending.length === 0) return null

  return (
    <div className="space-y-3">
      <h2 className="text-sm font-semibold text-secondary uppercase tracking-wide">
        {t('teamSettingsPage.pendingInvitations')}
      </h2>
      <div className="rounded-lg border border-border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('teamSettingsPage.colEmail')}</TableHead>
              <TableHead>{t('teamSettingsPage.colRole')}</TableHead>
              <TableHead>{t('teamSettingsPage.colInvitedBy')}</TableHead>
              <TableHead>{t('teamSettingsPage.colExpiresIn')}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {pending.map((inv) => (
              <TableRow key={inv.id}>
                <TableCell className="font-medium text-sm">{inv.email}</TableCell>
                <TableCell>{inviteRoleBadge(inv.role, t)}</TableCell>
                <TableCell className="text-secondary text-sm">{inv.invited_by || '—'}</TableCell>
                <TableCell className="text-secondary text-sm">
                  {daysUntil(inv.expires_at)} {t('teamSettingsPage.days')}
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => { handleRevoke(inv); }}
                    disabled={revoke.isPending}
                    className="text-destructive hover:text-destructive hover:bg-destructive/10"
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <AlertDialog open={revokeTarget !== null} onOpenChange={(open) => { if (!open) { setRevokeTarget(null); } }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('teamSettingsPage.revokeDialogTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('teamSettingsPage.revokeDialogDesc', { email: revokeTarget?.email ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => { setRevokeTarget(null); }}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmRevoke} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">{t('teamSettingsPage.revokeDialogTitle')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function TeamSettingsPage() {
  const { t } = useTranslation()
  const [inviteOpen, setInviteOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const { user } = useAuthStore()
  const { data: members = [], isLoading: membersLoading, isError: membersError, refetch: refetchMembers } = useTeamMembers()
  const { data: invitations = [], isLoading: invLoading, isError: invError, refetch: refetchInv } = useInvitations()

  const isLoading = membersLoading || invLoading
  const isError = membersError || invError

  return (
    <div className="p-6 space-y-8 max-w-5xl">
      <PageHeader
        title={t('teamSettingsPage.title')}
        description={t('teamSettingsPage.description')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { setCreateOpen(true); }}>
              <Plus className="w-4 h-4 mr-2" />
              {t('teamSettingsPage.createUserAction')}
            </Button>
            <Button onClick={() => { setInviteOpen(true); }}>
              <Plus className="w-4 h-4 mr-2" />
              {t('teamSettingsPage.inviteMember')}
            </Button>
          </div>
        }
      />

      {isError ? (
        <ErrorState
          message={t('teamSettingsPage.loadError')}
          onRetry={() => { void refetchMembers(); void refetchInv() }}
        />
      ) : isLoading ? (
        <div className="flex items-center justify-center h-32 text-secondary text-sm">
          {t('teamSettingsPage.loading')}
        </div>
      ) : (
        <>
          <div className="space-y-3">
            <h2 className="text-sm font-semibold text-secondary uppercase tracking-wide">
              {t('teamSettingsPage.teamMembersTitle')}
            </h2>
            <MembersTable members={members} currentUserID={user?.id ?? ''} />
          </div>

          <InvitationsTable invitations={invitations} />
        </>
      )}

      {members.length === 0 && !isLoading && (
        <div className="flex flex-col items-center gap-3 py-16 text-secondary">
          <Users className="w-10 h-10 opacity-30" />
          <p className="text-sm">{t('teamSettingsPage.noMembers')}</p>
        </div>
      )}

      <InviteDialog open={inviteOpen} onClose={() => { setInviteOpen(false); }} />
      <CreateUserDialog open={createOpen} onClose={() => { setCreateOpen(false); }} />
    </div>
  )
}
