import { useState } from 'react'
import { Users, Plus, Trash2, ChevronDown, ChevronUp } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useTargetGroups, useCreateTargetGroup, useDeleteTargetGroup, useTargets, useAddTarget } from '../hooks/useTargetGroups'
import type { TargetGroup } from '../types'

function TargetGroupRow({ group, onDelete }: { group: TargetGroup; onDelete: () => void }) {
  const [expanded, setExpanded] = useState(false)
  const [addOpen, setAddOpen] = useState(false)
  const { data: targets, isLoading } = useTargets(expanded ? group.id : '')
  const addTarget = useAddTarget(group.id)
  const [email, setEmail] = useState('')
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [department, setDepartment] = useState('')

  function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    addTarget.mutate(
      { email, first_name: firstName || undefined, last_name: lastName || undefined, department: department || undefined },
      {
        onSuccess: () => {
          setAddOpen(false)
          setEmail(''); setFirstName(''); setLastName(''); setDepartment('')
        },
      },
    )
  }

  return (
    <div className="border border-border rounded-lg bg-surface overflow-x-auto">
      <div
        className="flex items-center gap-4 p-4 cursor-pointer hover:bg-surface2"
        onClick={() => setExpanded(!expanded)}
      >
        <Users className="w-4 h-4 text-secondary shrink-0" />
        <span className="font-medium text-sm flex-1">{group.name}</span>
        <span className="text-xs text-secondary">{group.target_count ?? 0} targets</span>
        <span className="text-xs text-secondary">{group.source}</span>
        <Button
          variant="ghost"
          size="sm"
          className="text-red-500 hover:text-red-700 h-7 w-7 p-0"
          onClick={(e) => { e.stopPropagation(); onDelete() }}
        >
          <Trash2 className="w-3.5 h-3.5" />
        </Button>
        {expanded ? <ChevronUp className="w-4 h-4 text-secondary" /> : <ChevronDown className="w-4 h-4 text-secondary" />}
      </div>

      {expanded && (
        <div className="border-t border-border p-4 space-y-3">
          <div className="flex justify-end">
            <Button size="sm" onClick={(e) => { e.stopPropagation(); setAddOpen(true) }}>
              <Plus className="w-4 h-4 mr-1" />Add Target
            </Button>
          </div>
          {isLoading ? (
            <div className="flex justify-center py-4">
              <div className="w-4 h-4 border-2 border-brand border-t-transparent rounded-full animate-spin" />
            </div>
          ) : !targets || targets.length === 0 ? (
            <p className="text-sm text-secondary text-center py-4">No targets yet.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>E-Mail</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Abteilung</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {targets.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell className="font-mono text-xs">{t.email}</TableCell>
                    <TableCell className="text-sm">{[t.first_name, t.last_name].filter(Boolean).join(' ') || '—'}</TableCell>
                    <TableCell className="text-sm text-secondary">{t.department ?? '—'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      )}

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Ziel hinzufügen</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { void handleAdd(e) }}>
            <div className="py-4 space-y-3">
              <div className="space-y-1.5">
                <Label htmlFor="target-email">E-Mail</Label>
                <Input id="target-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="target-fname">Vorname</Label>
                  <Input id="target-fname" value={firstName} onChange={(e) => setFirstName(e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="target-lname">Nachname</Label>
                  <Input id="target-lname" value={lastName} onChange={(e) => setLastName(e.target.value)} />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="target-dept">Abteilung</Label>
                <Input id="target-dept" value={department} onChange={(e) => setDepartment(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>Abbrechen</Button>
              <Button type="submit" disabled={addTarget.isPending}>
                {addTarget.isPending ? 'Adding…' : 'Add Target'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function TargetGroupsPage() {
  const { data: groups, isLoading } = useTargetGroups()
  const createGroup = useCreateTargetGroup()
  const deleteGroup = useDeleteTargetGroup()

  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [source, setSource] = useState('')
  const [deleteId, setDeleteId] = useState<string | null>(null)

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    createGroup.mutate({ name: name.trim(), source: source.trim() || 'manual' }, {
      onSuccess: () => {
        setOpen(false)
        setName(''); setSource('')
      },
    })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Target Groups"
        description="Zielgruppen für Phishing-Simulationen verwalten."
        actions={
          <Button onClick={() => setOpen(true)}>
            <Plus className="w-4 h-4 mr-1" />New Group
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-3">
        {isLoading ? (
          <div className="flex justify-center py-16">
            <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        ) : !groups || groups.length === 0 ? (
          <EmptyState
            icon={Users}
            title="Noch keine Zielgruppen vorhanden"
            description="Erstellen Sie eine Zielgruppe, um festzulegen, wer Phishing-Simulationen erhält."
            action={<Button onClick={() => setOpen(true)}><Plus className="w-4 h-4 mr-1" />New Group</Button>}
          />
        ) : (
          groups.map((g) => (
            <TargetGroupRow
              key={g.id}
              group={g}
              onDelete={() => setDeleteId(g.id)}
            />
          ))
        )}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Neue Zielgruppe</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { void handleCreate(e) }}>
            <div className="py-4 space-y-3">
              <div className="space-y-1.5">
                <Label htmlFor="group-name">Group Name</Label>
                <Input id="group-name" placeholder="All Employees" value={name} onChange={(e) => setName(e.target.value)} required />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="group-source">Source (optional)</Label>
                <Input id="group-source" placeholder="manual, csv, ldap…" value={source} onChange={(e) => setSource(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>Abbrechen</Button>
              <Button type="submit" disabled={createGroup.isPending}>
                {createGroup.isPending ? 'Creating…' : 'Create Group'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleteId} onOpenChange={(open) => !open && setDeleteId(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>Zielgruppe löschen</DialogTitle></DialogHeader>
          <p className="text-sm text-secondary py-2">This will permanently delete the group and all its targets.</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteId(null)}>Abbrechen</Button>
            <Button
              variant="destructive"
              onClick={() => deleteGroup.mutate(deleteId!, { onSuccess: () => setDeleteId(null) })}
              disabled={deleteGroup.isPending}
            >
              {deleteGroup.isPending ? 'Deleting…' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
