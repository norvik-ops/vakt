import { useState } from 'react'
import { UserPlus, Pencil, Trash2, Users } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '../../../components/ui/alert-dialog'
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
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import {
  useEmployees,
  useCreateEmployee,
  useUpdateEmployee,
  useDeleteEmployee,
} from '../hooks/useHR'
import type { Employee, CreateEmployeeInput, UpdateEmployeeInput } from '../types'

type StatusFilter = 'all' | 'active' | 'offboarding' | 'terminated'

function StatusBadge({ status }: { status: Employee['status'] }) {
  if (status === 'active') {
    return <Badge className="bg-green-500/20 text-green-400 border-green-500/30">Aktiv</Badge>
  }
  if (status === 'offboarding') {
    return <Badge className="bg-orange-500/20 text-orange-400 border-orange-500/30">Offboarding</Badge>
  }
  return <Badge variant="secondary">Ausgeschieden</Badge>
}

interface FormState {
  first_name: string
  last_name: string
  email: string
  department: string
  role: string
  start_date: string
  end_date: string
  status: 'active' | 'offboarding' | 'terminated'
  notes: string
}

function emptyForm(): FormState {
  return {
    first_name: '',
    last_name: '',
    email: '',
    department: '',
    role: '',
    start_date: '',
    end_date: '',
    status: 'active',
    notes: '',
  }
}

function formFromEmployee(e: Employee): FormState {
  return {
    first_name: e.first_name,
    last_name: e.last_name,
    email: e.email,
    department: e.department ?? '',
    role: e.role ?? '',
    start_date: e.start_date ?? '',
    end_date: e.end_date ?? '',
    status: e.status,
    notes: e.notes ?? '',
  }
}

export default function EmployeesPage() {
  const [page, setPage] = useState(1)
  const { data: employees = [], isLoading, pagination } = useEmployees(page)
  const createEmployee = useCreateEmployee()
  const updateEmployee = useUpdateEmployee()
  const deleteEmployee = useDeleteEmployee()

  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const [editTarget, setEditTarget] = useState<Employee | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm())

  const filtered = employees.filter(
    (e) => statusFilter === 'all' || e.status === statusFilter,
  )

  function openCreate() {
    setEditTarget(null)
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function openEdit(e: Employee) {
    setEditTarget(e)
    setForm(formFromEmployee(e))
    setDialogOpen(true)
  }

  function handleField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  async function handleSubmit() {
    if (editTarget) {
      const input: UpdateEmployeeInput = {
        first_name: form.first_name,
        last_name: form.last_name,
        department: form.department || undefined,
        role: form.role || undefined,
        end_date: form.end_date || undefined,
        status: form.status,
        notes: form.notes || undefined,
      }
      await updateEmployee.mutateAsync({ id: editTarget.id, input })
    } else {
      const input: CreateEmployeeInput = {
        first_name: form.first_name,
        last_name: form.last_name,
        email: form.email,
        department: form.department || undefined,
        role: form.role || undefined,
        start_date: form.start_date || undefined,
        notes: form.notes || undefined,
      }
      await createEmployee.mutateAsync(input)
    }
    setDialogOpen(false)
  }

  function handleDelete(id: string) {
    setDeleteTarget(id)
  }

  async function confirmDelete() {
    if (!deleteTarget) return
    await deleteEmployee.mutateAsync(deleteTarget)
    setDeleteTarget(null)
  }

  const isPending = createEmployee.isPending || updateEmployee.isPending

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="Mitarbeiter"
        description="Onboarding- und Offboarding-Verwaltung"
        actions={
          <Button onClick={openCreate} size="sm">
            <UserPlus className="w-4 h-4 mr-2" />
            Mitarbeiter hinzufügen
          </Button>
        }
      />

      {/* Status filter */}
      <div className="flex gap-2">
        {(['all', 'active', 'offboarding', 'terminated'] as StatusFilter[]).map((s) => (
          <Button
            key={s}
            variant={statusFilter === s ? 'default' : 'outline'}
            size="sm"
            onClick={() => setStatusFilter(s)}
          >
            {s === 'all' ? 'Alle' : s === 'active' ? 'Aktiv' : s === 'offboarding' ? 'Offboarding' : 'Ausgeschieden'}
          </Button>
        ))}
      </div>

      {isLoading && (
        <div className="flex justify-center py-12">
          <div className="w-6 h-6 border-2 border-brand border-t-transparent rounded-full animate-spin" />
        </div>
      )}

      {!isLoading && filtered.length === 0 && (
        <EmptyState
          icon={Users}
          title="Keine Mitarbeiter"
          description="Fügen Sie den ersten Mitarbeiter hinzu, um mit dem Onboarding zu beginnen."
        />
      )}

      {!isLoading && filtered.length > 0 && (
        <Card>
          <CardContent className="p-0">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-secondary text-xs uppercase tracking-wide">
                  <th className="text-left px-4 py-3 font-medium">Name</th>
                  <th className="text-left px-4 py-3 font-medium">E-Mail</th>
                  <th className="text-left px-4 py-3 font-medium">Abteilung</th>
                  <th className="text-left px-4 py-3 font-medium">Rolle</th>
                  <th className="text-left px-4 py-3 font-medium">Eintrittsdatum</th>
                  <th className="text-left px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody>
                {filtered.map((e) => (
                  <tr key={e.id} className="border-b border-border last:border-0 hover:bg-surface/50">
                    <td className="px-4 py-3 font-medium">
                      {e.first_name} {e.last_name}
                    </td>
                    <td className="px-4 py-3 text-secondary">{e.email}</td>
                    <td className="px-4 py-3 text-secondary">{e.department ?? '—'}</td>
                    <td className="px-4 py-3 text-secondary">{e.role ?? '—'}</td>
                    <td className="px-4 py-3 text-secondary">{e.start_date ?? '—'}</td>
                    <td className="px-4 py-3">
                      <StatusBadge status={e.status} />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 justify-end">
                        <Button variant="ghost" size="icon" onClick={() => openEdit(e)}>
                          <Pencil className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => void handleDelete(e.id)}
                          className="text-red-500 hover:text-red-600"
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      )}

      <Pagination
        page={page}
        totalPages={pagination?.total_pages ?? 1}
        onPageChange={setPage}
      />

      {/* Create / Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editTarget ? 'Mitarbeiter bearbeiten' : 'Mitarbeiter hinzufügen'}
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Vorname *</Label>
                <Input
                  value={form.first_name}
                  onChange={(e) => handleField('first_name', e.target.value)}
                  placeholder="Max"
                />
              </div>
              <div className="space-y-1">
                <Label>Nachname *</Label>
                <Input
                  value={form.last_name}
                  onChange={(e) => handleField('last_name', e.target.value)}
                  placeholder="Mustermann"
                />
              </div>
            </div>

            {!editTarget && (
              <div className="space-y-1">
                <Label>E-Mail *</Label>
                <Input
                  type="email"
                  value={form.email}
                  onChange={(e) => handleField('email', e.target.value)}
                  placeholder="max.mustermann@example.com"
                />
              </div>
            )}

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Abteilung</Label>
                <Input
                  value={form.department}
                  onChange={(e) => handleField('department', e.target.value)}
                  placeholder="IT"
                />
              </div>
              <div className="space-y-1">
                <Label>Rolle / Funktion</Label>
                <Input
                  value={form.role}
                  onChange={(e) => handleField('role', e.target.value)}
                  placeholder="DevOps Engineer"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              {!editTarget && (
                <div className="space-y-1">
                  <Label>Eintrittsdatum</Label>
                  <Input
                    type="date"
                    value={form.start_date}
                    onChange={(e) => handleField('start_date', e.target.value)}
                  />
                </div>
              )}
              {editTarget && (
                <>
                  <div className="space-y-1">
                    <Label>Austrittsdatum</Label>
                    <Input
                      type="date"
                      value={form.end_date}
                      onChange={(e) => handleField('end_date', e.target.value)}
                    />
                  </div>
                  <div className="space-y-1">
                    <Label>Status *</Label>
                    <Select
                      value={form.status}
                      onValueChange={(v) => handleField('status', v as FormState['status'])}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="active">Aktiv</SelectItem>
                        <SelectItem value="offboarding">Offboarding</SelectItem>
                        <SelectItem value="terminated">Ausgeschieden</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </>
              )}
            </div>

            <div className="space-y-1">
              <Label>Notizen</Label>
              <Textarea
                value={form.notes}
                onChange={(e) => handleField('notes', e.target.value)}
                placeholder="Interne Notizen..."
                rows={3}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Abbrechen
            </Button>
            <Button onClick={() => void handleSubmit()} disabled={isPending}>
              {isPending ? 'Speichern...' : editTarget ? 'Speichern' : 'Hinzufügen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteTarget !== null} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Mitarbeiter löschen?</AlertDialogTitle>
            <AlertDialogDescription>
              Der Mitarbeiter wird unwiderruflich gelöscht. Diese Aktion kann nicht rückgängig gemacht werden.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={() => void confirmDelete()} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              Löschen
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
