import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { KeyRound, Plus, Pencil, Trash2, Camera, ChevronDown, ChevronUp } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  useAccessConcepts,
  useAccessRoles,
  useCreateAccessConcept,
  useUpdateAccessConcept,
  useDeleteAccessConcept,
  useAddAccessRole,
  useUpdateAccessRole,
  useDeleteAccessRole,
  useSnapshotAccessConcept,
  useAccessConceptVersions,
} from '../hooks/useAccessConcepts'
import type {
  AccessConcept,
  AccessRole,
  AccessLevel,
  CreateAccessConceptInput,
  CreateAccessRoleInput,
} from '../types'

// ─── Constants ────────────────────────────────────────────────────────────────

const LEVEL_CLASS: Record<AccessLevel, string> = {
  read: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  write: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  admin: 'bg-red-500/20 text-red-400 border-red-500/30',
  no_access: 'bg-muted text-muted-foreground',
}

const ACCESS_LEVELS: AccessLevel[] = ['read', 'write', 'admin', 'no_access']

function emptyConceptForm(): CreateAccessConceptInput {
  return { title: '', scope: '', owner: '' }
}

function conceptToForm(c: AccessConcept): CreateAccessConceptInput {
  return { title: c.title, scope: c.scope, owner: c.owner }
}

function emptyRoleForm(): CreateAccessRoleInput {
  return {
    role_name: '',
    system_name: '',
    access_level: 'read',
    justification: '',
    review_interval_months: 12,
  }
}

function roleToForm(r: AccessRole): CreateAccessRoleInput {
  return {
    role_name: r.role_name,
    system_name: r.system_name,
    access_level: r.access_level,
    justification: r.justification,
    review_interval_months: r.review_interval_months,
  }
}

// ─── Role matrix sub-component ────────────────────────────────────────────────

function RoleMatrix({ concept }: { concept: AccessConcept }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [roleDialogOpen, setRoleDialogOpen] = useState(false)
  const [editRoleId, setEditRoleId] = useState<string | null>(null)
  const [roleForm, setRoleForm] = useState<CreateAccessRoleInput>(emptyRoleForm())

  const { data: roles = [], isLoading } = useAccessRoles(open ? concept.id : '')
  const { data: versions = [] } = useAccessConceptVersions(open ? concept.id : '')
  const addRole = useAddAccessRole(concept.id)
  const updateRole = useUpdateAccessRole(concept.id, editRoleId ?? '')
  const deleteRole = useDeleteAccessRole(concept.id)
  const snapshot = useSnapshotAccessConcept(concept.id)

  function openAddRole() {
    setEditRoleId(null)
    setRoleForm(emptyRoleForm())
    setRoleDialogOpen(true)
  }

  function openEditRole(r: AccessRole) {
    setEditRoleId(r.id)
    setRoleForm(roleToForm(r))
    setRoleDialogOpen(true)
  }

  function handleRoleSubmit() {
    if (editRoleId) {
      updateRole.mutate(roleForm, { onSuccess: () => { setRoleDialogOpen(false); } })
    } else {
      addRole.mutate(roleForm, { onSuccess: () => { setRoleDialogOpen(false); } })
    }
  }

  return (
    <div className="mt-3 border-t pt-3">
      <button
        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
        onClick={() => { setOpen((v) => !v); }}
      >
        {open ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
        {t('accessConcepts.roles')}
        {concept.current_version > 0 && (
          <span className="ml-1 text-muted-foreground">
            v{concept.current_version}
          </span>
        )}
      </button>

      {open && (
        <div className="mt-2 space-y-2">
          {isLoading && <Spinner size="sm" color="primary" />}
          {roles.length > 0 && (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b text-muted-foreground">
                    <th className="text-left py-1 pr-2">{t('accessConcepts.roleName')}</th>
                    <th className="text-left py-1 pr-2">{t('accessConcepts.system')}</th>
                    <th className="text-left py-1 pr-2">{t('accessConcepts.level')}</th>
                    <th className="py-1" />
                  </tr>
                </thead>
                <tbody>
                  {roles.map((r) => (
                    <tr key={r.id} className="border-b border-border/30">
                      <td className="py-1 pr-2 font-medium">{r.role_name}</td>
                      <td className="py-1 pr-2 text-muted-foreground">{r.system_name}</td>
                      <td className="py-1 pr-2">
                        <Badge className={LEVEL_CLASS[r.access_level]} variant="outline">
                          {r.access_level}
                        </Badge>
                      </td>
                      <td className="py-1">
                        <div className="flex gap-0.5">
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-5 w-5"
                            onClick={() => { openEditRole(r); }}
                          >
                            <Pencil className="w-3 h-3" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-5 w-5 text-red-400"
                            onClick={() => {
                              if (confirm(t('accessConcepts.deleteRoleConfirm'))) {
                                deleteRole.mutate(r.id)
                              }
                            }}
                          >
                            <Trash2 className="w-3 h-3" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className="flex items-center gap-2 flex-wrap">
            <Button variant="outline" size="sm" className="h-6 text-xs" onClick={openAddRole}>
              <Plus className="w-3 h-3 mr-1" />
              {t('accessConcepts.addRole')}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="h-6 text-xs"
              disabled={snapshot.isPending}
              onClick={() => {
                if (confirm(t('accessConcepts.snapshotConfirm'))) {
                  snapshot.mutate()
                }
              }}
            >
              <Camera className="w-3 h-3 mr-1" />
              {snapshot.isPending ? t('common.saving') : t('accessConcepts.snapshot')}
            </Button>
          </div>

          {versions.length > 0 && (
            <div className="text-xs text-muted-foreground">
              {t('accessConcepts.versions')}: {versions.map((v) => `v${v.version_number}`).join(', ')}
            </div>
          )}
        </div>
      )}

      {/* Role dialog */}
      <Dialog open={roleDialogOpen} onOpenChange={setRoleDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {editRoleId ? t('accessConcepts.editRole') : t('accessConcepts.addRole')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('accessConcepts.roleName')} *</Label>
                <Input
                  placeholder="z.B. Administrator"
                  value={roleForm.role_name}
                  onChange={(e) => { setRoleForm((f) => ({ ...f, role_name: e.target.value })); }}
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('accessConcepts.system')} *</Label>
                <Input
                  placeholder="z.B. ERP-System"
                  value={roleForm.system_name}
                  onChange={(e) => { setRoleForm((f) => ({ ...f, system_name: e.target.value })); }}
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>{t('accessConcepts.level')}</Label>
              <Select
                value={roleForm.access_level}
                onValueChange={(v) => { setRoleForm((f) => ({ ...f, access_level: v as AccessLevel })); }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {ACCESS_LEVELS.map((l) => (
                    <SelectItem key={l} value={l}>
                      {t(`accessConcepts.accessLevel.${l}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>{t('accessConcepts.justification')}</Label>
              <Textarea
                rows={2}
                placeholder={t('accessConcepts.justificationPlaceholder')}
                value={roleForm.justification ?? ''}
                onChange={(e) => { setRoleForm((f) => ({ ...f, justification: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('accessConcepts.reviewInterval')}</Label>
              <Input
                type="number"
                min={1}
                max={36}
                value={roleForm.review_interval_months ?? 12}
                onChange={(e) => { setRoleForm((f) => ({ ...f, review_interval_months: parseInt(e.target.value, 10) })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setRoleDialogOpen(false); }}>
              {t('common.cancel')}
            </Button>
            <Button
              disabled={!roleForm.role_name || !roleForm.system_name || addRole.isPending || updateRole.isPending}
              onClick={handleRoleSubmit}
            >
              {(addRole.isPending || updateRole.isPending) ? t('common.saving') : editRoleId ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ─── Concept card ─────────────────────────────────────────────────────────────

function ConceptCard({
  concept,
  onEdit,
  onDelete,
}: {
  concept: AccessConcept
  onEdit: () => void
  onDelete: () => void
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="flex-1 space-y-0.5">
            <CardTitle className="text-base">{concept.title}</CardTitle>
            {concept.owner && (
              <p className="text-xs text-muted-foreground">{concept.owner}</p>
            )}
            {concept.scope && (
              <p className="text-xs text-muted-foreground line-clamp-2">{concept.scope}</p>
            )}
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit}>
              <Pencil className="w-3.5 h-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-red-400 hover:text-red-300"
              onClick={onDelete}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        <RoleMatrix concept={concept} />
      </CardContent>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function AccessConceptsPage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateAccessConceptInput>(emptyConceptForm())

  const { data: concepts = [], isLoading, isError } = useAccessConcepts()
  const createConcept = useCreateAccessConcept()
  const updateConcept = useUpdateAccessConcept(editId ?? '')
  const deleteConcept = useDeleteAccessConcept()

  function openCreate() {
    setEditId(null)
    setForm(emptyConceptForm())
    setDialogOpen(true)
  }

  function openEdit(c: AccessConcept) {
    setEditId(c.id)
    setForm(conceptToForm(c))
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm(t('accessConcepts.deleteConfirm'))) {
      deleteConcept.mutate(id)
    }
  }

  function handleSubmit() {
    if (editId) {
      updateConcept.mutate(form, { onSuccess: () => { setDialogOpen(false); } })
    } else {
      createConcept.mutate(form, { onSuccess: () => { setDialogOpen(false); } })
    }
  }

  const isPending = createConcept.isPending || updateConcept.isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('accessConcepts.title')}
        description={t('accessConcepts.description')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            {t('accessConcepts.new')}
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            {t('accessConcepts.loadError')}
          </div>
        )}
        {!isLoading && !isError && concepts.length === 0 && (
          <EmptyState
            icon={KeyRound}
            title={t('accessConcepts.emptyTitle')}
            description={t('accessConcepts.emptyDescription')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('accessConcepts.new')}
              </Button>
            }
          />
        )}
        {!isLoading && !isError && concepts.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {concepts.map((c) => (
              <ConceptCard
                key={c.id}
                concept={c}
                onEdit={() => { openEdit(c); }}
                onDelete={() => { handleDelete(c.id); }}
              />
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editId ? t('accessConcepts.edit') : t('accessConcepts.new')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('accessConcepts.conceptTitle')} *</Label>
              <Input
                placeholder={t('accessConcepts.titlePlaceholder')}
                value={form.title}
                onChange={(e) => { setForm((f) => ({ ...f, title: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('accessConcepts.owner')}</Label>
              <Input
                placeholder={t('accessConcepts.ownerPlaceholder')}
                value={form.owner ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, owner: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('accessConcepts.scope')}</Label>
              <Textarea
                rows={2}
                placeholder={t('accessConcepts.scopePlaceholder')}
                value={form.scope ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, scope: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={!form.title || isPending}>
              {isPending ? t('common.saving') : editId ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
