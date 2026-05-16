import { useState } from 'react'
import { Plus, Trash2, Users } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Badge } from '../components/ui/badge'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../components/ui/alert-dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../components/ui/table'
import { useAuthStore } from '../shared/stores/auth'
import {
  useTeamMembers,
  useUpdateRole,
  useRemoveUser,
  useInvitations,
  useCreateInvitation,
  useRevokeInvitation,
  type TeamMember,
  type TeamInvitation,
} from '../hooks/useTeam'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type Role = 'admin' | 'editor' | 'viewer'

function roleBadge(role: Role) {
  switch (role) {
    case 'admin':
      return <Badge className="bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300 border-0">Admin</Badge>
    case 'editor':
      return <Badge className="bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0">Editor</Badge>
    case 'viewer':
      return <Badge variant="secondary">Viewer</Badge>
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

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('de-DE', {
    day: '2-digit', month: '2-digit', year: 'numeric',
  })
}

// ---------------------------------------------------------------------------
// Invite Dialog
// ---------------------------------------------------------------------------

interface InviteDialogProps {
  open: boolean
  onClose: () => void
}

function InviteDialog({ open, onClose }: InviteDialogProps) {
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('editor')
  const create = useCreateInvitation()

  function handleSend() {
    if (!email.trim()) return
    create.mutate({ email: email.trim(), role }, { onSuccess: handleClose })
  }

  function handleClose() {
    setEmail('')
    setRole('editor')
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Mitglied einladen</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1">
            <Label htmlFor="invite-email">E-Mail-Adresse</Label>
            <Input
              id="invite-email"
              type="email"
              placeholder="kollegin@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label>Rolle</Label>
            <Select value={role} onValueChange={(v) => setRole(v as Role)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="admin">Admin — Vollzugriff inkl. Einstellungen</SelectItem>
                <SelectItem value="editor">Editor — Lesen und Schreiben</SelectItem>
                <SelectItem value="viewer">Viewer — Nur lesen</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {create.error && (
            <p className="text-sm text-destructive">{create.error.message}</p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>Abbrechen</Button>
          <Button onClick={handleSend} disabled={!email.trim() || create.isPending}>
            {create.isPending ? 'Sende...' : 'Einladung senden'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------------------
// Members table
// ---------------------------------------------------------------------------

function MembersTable({ members, currentUserID }: { members: TeamMember[]; currentUserID: string }) {
  const updateRole = useUpdateRole()
  const removeUser = useRemoveUser()
  const [removeTarget, setRemoveTarget] = useState<TeamMember | null>(null)

  const adminCount = members.filter((m) => m.role === 'admin').length

  function handleRoleChange(member: TeamMember, newRole: Role) {
    updateRole.mutate({ id: member.id, role: newRole })
  }

  function handleRemove(member: TeamMember) {
    setRemoveTarget(member)
  }

  function confirmRemove() {
    if (removeTarget) removeUser.mutate(removeTarget.id)
    setRemoveTarget(null)
  }

  return (
    <div className="rounded-lg border border-border overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Mitglied</TableHead>
            <TableHead>E-Mail</TableHead>
            <TableHead>Rolle</TableHead>
            <TableHead>Mitglied seit</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {members.length === 0 && (
            <TableRow>
              <TableCell colSpan={5} className="text-center py-10 text-secondary">
                Keine Mitglieder gefunden.
              </TableCell>
            </TableRow>
          )}
          {members.map((member) => {
            const isSelf = member.id === currentUserID
            const isLastAdmin = member.role === 'admin' && adminCount <= 1

            return (
              <TableRow key={member.id}>
                <TableCell>
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-full bg-brand/10 flex items-center justify-center text-[11px] font-semibold text-brand shrink-0">
                      {initials(member.name, member.email)}
                    </div>
                    <span className="font-medium text-sm">
                      {member.name || member.email.split('@')[0]}
                      {isSelf && <span className="ml-1 text-secondary text-xs">(Du)</span>}
                    </span>
                  </div>
                </TableCell>
                <TableCell className="text-secondary text-sm">{member.email}</TableCell>
                <TableCell>
                  {isSelf || isLastAdmin ? (
                    roleBadge(member.role)
                  ) : (
                    <Select
                      value={member.role}
                      onValueChange={(v) => handleRoleChange(member, v as Role)}
                      disabled={updateRole.isPending}
                    >
                      <SelectTrigger className="h-7 w-28 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="admin">Admin</SelectItem>
                        <SelectItem value="editor">Editor</SelectItem>
                        <SelectItem value="viewer">Viewer</SelectItem>
                      </SelectContent>
                    </Select>
                  )}
                </TableCell>
                <TableCell className="text-secondary text-sm">{formatDate(member.created_at)}</TableCell>
                <TableCell>
                  {!isSelf && !isLastAdmin && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleRemove(member)}
                      disabled={removeUser.isPending}
                      className="text-destructive hover:text-destructive hover:bg-destructive/10"
                    >
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>

      <AlertDialog open={removeTarget !== null} onOpenChange={(open) => !open && setRemoveTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Mitglied entfernen?</AlertDialogTitle>
            <AlertDialogDescription>
              {removeTarget?.email} wird aus der Organisation entfernt. Diese Aktion kann nicht rückgängig gemacht werden.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setRemoveTarget(null)}>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={confirmRemove} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">Entfernen</AlertDialogAction>
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
  const revoke = useRevokeInvitation()
  const [revokeTarget, setRevokeTarget] = useState<TeamInvitation | null>(null)

  const pending = invitations.filter((inv) => !inv.accepted_at)

  function handleRevoke(inv: TeamInvitation) {
    setRevokeTarget(inv)
  }

  function confirmRevoke() {
    if (revokeTarget) revoke.mutate(revokeTarget.id)
    setRevokeTarget(null)
  }

  if (pending.length === 0) return null

  return (
    <div className="space-y-3">
      <h2 className="text-sm font-semibold text-secondary uppercase tracking-wide">
        Ausstehende Einladungen
      </h2>
      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>E-Mail</TableHead>
              <TableHead>Rolle</TableHead>
              <TableHead>Eingeladen von</TableHead>
              <TableHead>Laeuft ab in</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {pending.map((inv) => (
              <TableRow key={inv.id}>
                <TableCell className="font-medium text-sm">{inv.email}</TableCell>
                <TableCell>{roleBadge(inv.role)}</TableCell>
                <TableCell className="text-secondary text-sm">{inv.invited_by || '—'}</TableCell>
                <TableCell className="text-secondary text-sm">
                  {daysUntil(inv.expires_at)} Tage
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleRevoke(inv)}
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

      <AlertDialog open={revokeTarget !== null} onOpenChange={(open) => !open && setRevokeTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Einladung widerrufen?</AlertDialogTitle>
            <AlertDialogDescription>
              Die Einladung an {revokeTarget?.email} wird widerrufen.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setRevokeTarget(null)}>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={confirmRevoke} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">Widerrufen</AlertDialogAction>
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
  const [dialogOpen, setDialogOpen] = useState(false)
  const { user } = useAuthStore()
  const { data: members = [], isLoading: membersLoading } = useTeamMembers()
  const { data: invitations = [], isLoading: invLoading } = useInvitations()

  const isLoading = membersLoading || invLoading

  return (
    <div className="p-6 space-y-8 max-w-5xl">
      <PageHeader
        title="Team"
        description="Verwalte Teammitglieder, vergib Rollen und lade neue Kolleginnen ein."
        actions={
          <Button onClick={() => setDialogOpen(true)}>
            <Plus className="w-4 h-4 mr-2" />
            Mitglied einladen
          </Button>
        }
      />

      {isLoading ? (
        <div className="flex items-center justify-center h-32 text-secondary text-sm">
          Laden...
        </div>
      ) : (
        <>
          <div className="space-y-3">
            <h2 className="text-sm font-semibold text-secondary uppercase tracking-wide">
              Teammitglieder
            </h2>
            <MembersTable members={members} currentUserID={user?.id ?? ''} />
          </div>

          <InvitationsTable invitations={invitations} />
        </>
      )}

      {members.length === 0 && !isLoading && (
        <div className="flex flex-col items-center gap-3 py-16 text-secondary">
          <Users className="w-10 h-10 opacity-30" />
          <p className="text-sm">Noch keine Teammitglieder</p>
        </div>
      )}

      <InviteDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </div>
  )
}
