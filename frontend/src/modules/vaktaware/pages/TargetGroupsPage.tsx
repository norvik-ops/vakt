import { useState } from 'react'
import { Users, Plus, Trash2, ChevronDown, ChevronUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Spinner } from '../../../components/Spinner'
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
  const { t } = useTranslation()
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
        onClick={() => { setExpanded(!expanded); }}
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
              <Spinner size="sm" />
            </div>
          ) : !targets || targets.length === 0 ? (
            <p className="text-sm text-secondary text-center py-4">{t('vaktaware.targetGroups.noTargets')}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>E-Mail</TableHead>
                  <TableHead>{t('common.name')}</TableHead>
                  <TableHead>{t('vaktaware.targetGroups.colDepartment')}</TableHead>
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
          <DialogHeader><DialogTitle>{t('vaktaware.targetGroups.addTargetTitle')}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { handleAdd(e) }}>
            <div className="py-4 space-y-3">
              <div className="space-y-1.5">
                <Label htmlFor="target-email">E-Mail</Label>
                <Input id="target-email" type="email" value={email} onChange={(e) => { setEmail(e.target.value); }} required />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="target-fname">{t('vaktaware.targetGroups.labelFirstName')}</Label>
                  <Input id="target-fname" value={firstName} onChange={(e) => { setFirstName(e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="target-lname">{t('vaktaware.targetGroups.labelLastName')}</Label>
                  <Input id="target-lname" value={lastName} onChange={(e) => { setLastName(e.target.value); }} />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="target-dept">{t('vaktaware.targetGroups.labelDepartment')}</Label>
                <Input id="target-dept" value={department} onChange={(e) => { setDepartment(e.target.value); }} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setAddOpen(false); }}>{t('common.cancel')}</Button>
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
  const { t } = useTranslation()
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
        title={t('vaktaware.targetGroups.title')}
        description={t('vaktaware.targetGroups.description')}
        tier="pro"
        actions={
          <Button onClick={() => { setOpen(true); }}>
            <Plus className="w-4 h-4 mr-1" />New Group
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-3">
        {isLoading ? (
          <div className="flex justify-center py-16">
            <Spinner size="md" />
          </div>
        ) : !groups || groups.length === 0 ? (
          <EmptyState
            icon={Users}
            title={t('vaktaware.targetGroups.noGroups')}
            description={t('vaktaware.targetGroups.noGroupsDesc')}
            action={<Button onClick={() => { setOpen(true); }}><Plus className="w-4 h-4 mr-1" />New Group</Button>}
          />
        ) : (
          groups.map((g) => (
            <TargetGroupRow
              key={g.id}
              group={g}
              onDelete={() => { setDeleteId(g.id); }}
            />
          ))
        )}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('vaktaware.targetGroups.createDialogTitle')}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { handleCreate(e) }}>
            <div className="py-4 space-y-3">
              <div className="space-y-1.5">
                <Label htmlFor="group-name">Group Name</Label>
                <Input id="group-name" placeholder="All Employees" value={name} onChange={(e) => { setName(e.target.value); }} required />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="group-source">Source (optional)</Label>
                <Input id="group-source" placeholder="manual, csv, ldap…" value={source} onChange={(e) => { setSource(e.target.value); }} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setOpen(false); }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={createGroup.isPending}>
                {createGroup.isPending ? 'Creating…' : 'Create Group'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleteId} onOpenChange={(open) => { if (!open) { setDeleteId(null); } }}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('vaktaware.targetGroups.deleteDialogTitle')}</DialogTitle></DialogHeader>
          <p className="text-sm text-secondary py-2">{t('vaktaware.targetGroups.deleteGroupDesc')}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteId(null); }}>{t('common.cancel')}</Button>
            <Button
              variant="destructive"
              onClick={() => { deleteGroup.mutate(deleteId!, { onSuccess: () => { setDeleteId(null); } }); }}
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
