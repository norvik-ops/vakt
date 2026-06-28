import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { UserPlus, Pencil, Trash2, Users, Play } from 'lucide-react'
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
import { useDeferredDelete } from '../../../shared/hooks/useDeferredDelete'
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
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { FieldError } from '../../../shared/components/FieldError'
import { useFormValidation } from '../../../shared/hooks/useFormValidation'
import { toast } from '../../../shared/hooks/useToast'
import {
  useEmployees,
  useCreateEmployee,
  useUpdateEmployee,
  useDeleteEmployee,
  useChecklistRuns,
  useChecklists,
  useStartChecklistRun,
} from '../hooks/useHR'
import type { Employee, CreateEmployeeInput, UpdateEmployeeInput } from '../types'

function ChecklistRunCell({ employee }: { employee: Employee }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: runs } = useChecklistRuns(employee.id)
  const { data: checklists } = useChecklists()
  const startRun = useStartChecklistRun()
  const [pickOpen, setPickOpen] = useState(false)
  const [selectedChecklistId, setSelectedChecklistId] = useState('')

  const activeRun = runs?.find((r) => r.status === 'in_progress')

  if (activeRun) {
    return (
      <Link to={`/vakthr/checklist-runs/${activeRun.id}`}>
        <Badge className="bg-blue-500/20 text-blue-400 border-blue-500/30 cursor-pointer hover:bg-blue-500/30 transition-colors">
          {t('vakthr.checklistRun.active')}
        </Badge>
      </Link>
    )
  }

  async function handleStart() {
    if (!selectedChecklistId) return
    const run = await startRun.mutateAsync({ employee_id: employee.id, checklist_id: selectedChecklistId })
    setPickOpen(false)
    navigate(`/vakthr/checklist-runs/${run.id}`)
  }

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        className="h-7 px-2 text-xs text-secondary hover:text-primary"
        onClick={() => { setPickOpen(true) }}
      >
        <Play className="w-3 h-3 mr-1" />
        {t('vakthr.checklistRun.start')}
      </Button>

      <Dialog open={pickOpen} onOpenChange={setPickOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('vakthr.checklistRun.selectTitle')}</DialogTitle>
          </DialogHeader>
          <div className="py-3 space-y-3">
            <p className="text-sm text-secondary">
              {t('vakthr.checklistRun.selectDesc', { name: `${employee.first_name} ${employee.last_name}` })}
            </p>
            <Select value={selectedChecklistId} onValueChange={setSelectedChecklistId}>
              <SelectTrigger>
                <SelectValue placeholder={t('vakthr.checklistRun.selectPlaceholder')} />
              </SelectTrigger>
              <SelectContent>
                {(checklists ?? []).map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.name} ({c.type === 'onboarding' ? 'Onboarding' : 'Offboarding'})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setPickOpen(false) }}>{t('common.cancel')}</Button>
            <Button
              onClick={() => { void handleStart() }}
              disabled={!selectedChecklistId || startRun.isPending}
            >
              {startRun.isPending ? t('vakthr.checklistRun.starting') : t('vakthr.checklistRun.startButton')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

type StatusFilter = 'all' | 'active' | 'offboarding' | 'terminated'

function StatusBadge({ status }: { status: Employee['status'] }) {
  const { t } = useTranslation()
  if (status === 'active') {
    return <Badge className="bg-green-500/20 text-green-400 border-green-500/30">{t('vakthr.status.active')}</Badge>
  }
  if (status === 'offboarding') {
    return <Badge className="bg-orange-500/20 text-orange-400 border-orange-500/30">{t('vakthr.status.offboarding')}</Badge>
  }
  return <Badge variant="secondary">{t('vakthr.status.terminated')}</Badge>
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
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const { data: employees = [], isLoading, pagination } = useEmployees(page)
  const createEmployee = useCreateEmployee()
  const updateEmployee = useUpdateEmployee()
  const deleteEmployee = useDeleteEmployee()
  const { errors: empErrors, validate: validateEmp, clearError: clearEmpError, clearAll: clearEmpErrors } = useFormValidation<Record<string, unknown>>({
    first_name: { required: true },
    last_name: { required: true },
    email: { required: true, pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, patternMessage: t('validation.email') },
  })

  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [hiddenIds, setHiddenIds] = useState<Set<string>>(new Set())
  const [editTarget, setEditTarget] = useState<Employee | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm())

  const { scheduleDelete } = useDeferredDelete<Employee>({
    onDelete: (emp) => deleteEmployee.mutateAsync(emp.id),
    onUndo: () => void queryClient.invalidateQueries({ queryKey: ['vakthr', 'employees'] }),
    getLabel: (emp) => `${emp.first_name} ${emp.last_name}`,
  })

  const filtered = employees.filter(
    (e) => (statusFilter === 'all' || e.status === statusFilter) && !hiddenIds.has(e.id),
  )

  function openCreate() {
    setEditTarget(null)
    setForm(emptyForm())
    clearEmpErrors()
    setDialogOpen(true)
  }

  function openEdit(e: Employee) {
    setEditTarget(e)
    setForm(formFromEmployee(e))
    clearEmpErrors()
    setDialogOpen(true)
  }

  function handleField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((f) => ({ ...f, [key]: value }))
    clearEmpError(key)
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
      setDialogOpen(false)
    } else {
      if (!validateEmp({ first_name: form.first_name, last_name: form.last_name, email: form.email })) return
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
      toast(t('vakthr.employees.addedToast'), 'success')
      setDialogOpen(false)
    }
  }

  function handleDelete(employee: Employee) {
    scheduleDelete(employee, () => { setHiddenIds((prev) => new Set(prev).add(employee.id)) })
  }

  const isPending = createEmployee.isPending || updateEmployee.isPending

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title={t('vakthr.employees.title')}
        description={t('vakthr.employees.description')}
        actions={
          <Button onClick={openCreate} size="sm">
            <UserPlus className="w-4 h-4 mr-2" />
            {t('vakthr.employees.addEmployee')}
          </Button>
        }
      />

      <InfoBanner icon={Users} title={t('vakthr.employees.bannerTitle')}>
        <p>
          {t('vakthr.employees.bannerDesc1')}
        </p>
        <p className="mt-1">
          {t('vakthr.employees.bannerDesc2')}
        </p>
      </InfoBanner>

      {/* Status filter */}
      <div className="flex gap-2">
        {(['all', 'active', 'offboarding', 'terminated'] as StatusFilter[]).map((s) => (
          <Button
            key={s}
            variant={statusFilter === s ? 'default' : 'outline'}
            size="sm"
            onClick={() => { setStatusFilter(s); }}
          >
            {s === 'all' ? t('common.all') : s === 'active' ? t('vakthr.status.active') : s === 'offboarding' ? t('vakthr.status.offboarding') : t('vakthr.status.terminated')}
          </Button>
        ))}
      </div>

      {isLoading && <SkeletonTable rows={5} cols={8} />}

      {!isLoading && filtered.length === 0 && (
        <EmptyState
          icon={Users}
          title={t('vakthr.employees.emptyTitle')}
          description={t('vakthr.employees.emptyDesc')}
          action={<Button size="sm" onClick={openCreate}><UserPlus className="w-4 h-4 mr-2" />{t('vakthr.employees.addEmployee')}</Button>}
        />
      )}

      {!isLoading && filtered.length > 0 && (
        <Card>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-secondary text-xs uppercase tracking-wide">
                  <th className="text-left px-4 py-3 font-medium">{t('common.name')}</th>
                  <th className="text-left px-4 py-3 font-medium">{t('common.email')}</th>
                  <th className="text-left px-4 py-3 font-medium">{t('vakthr.employees.colDept')}</th>
                  <th className="text-left px-4 py-3 font-medium">{t('vakthr.employees.colRole')}</th>
                  <th className="text-left px-4 py-3 font-medium">{t('vakthr.employees.colStartDate')}</th>
                  <th className="text-left px-4 py-3 font-medium">{t('common.status')}</th>
                  <th className="text-left px-4 py-3 font-medium">{t('vakthr.employees.colChecklist')}</th>
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
                      <ChecklistRunCell employee={e} />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 justify-end">
                        <Button variant="ghost" size="icon" onClick={() => { openEdit(e); }}>
                          <Pencil className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => { handleDelete(e); }}
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
            </div>
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
              {editTarget ? t('vakthr.employees.editTitle') : t('vakthr.employees.addEmployee')}
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>{t('vakthr.employees.labelFirstName')} <span className="text-red-400 text-xs">*</span></Label>
                <Input
                  value={form.first_name}
                  onChange={(e) => { handleField('first_name', e.target.value); }}
                  placeholder="Max"
                />
                <FieldError error={empErrors.first_name ?? null} />
              </div>
              <div className="space-y-1">
                <Label>{t('vakthr.employees.labelLastName')} <span className="text-red-400 text-xs">*</span></Label>
                <Input
                  value={form.last_name}
                  onChange={(e) => { handleField('last_name', e.target.value); }}
                  placeholder="Mustermann"
                />
                <FieldError error={empErrors.last_name ?? null} />
              </div>
            </div>

            {!editTarget && (
              <div className="space-y-1">
                <Label>{t('common.email')} <span className="text-red-400 text-xs">*</span></Label>
                <Input
                  type="email"
                  value={form.email}
                  onChange={(e) => { handleField('email', e.target.value); }}
                  placeholder="max.mustermann@example.com"
                />
                <FieldError error={empErrors.email ?? null} />
              </div>
            )}

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>{t('vakthr.employees.labelDept')}</Label>
                <Input
                  value={form.department}
                  onChange={(e) => { handleField('department', e.target.value); }}
                  placeholder="IT"
                />
              </div>
              <div className="space-y-1">
                <Label>{t('vakthr.employees.labelRole')}</Label>
                <Input
                  value={form.role}
                  onChange={(e) => { handleField('role', e.target.value); }}
                  placeholder="DevOps Engineer"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              {!editTarget && (
                <div className="space-y-1">
                  <Label>{t('vakthr.employees.labelStartDate')}</Label>
                  <Input
                    type="date"
                    value={form.start_date}
                    onChange={(e) => { handleField('start_date', e.target.value); }}
                  />
                </div>
              )}
              {editTarget && (
                <>
                  <div className="space-y-1">
                    <Label>{t('vakthr.employees.labelEndDate')}</Label>
                    <Input
                      type="date"
                      value={form.end_date}
                      onChange={(e) => { handleField('end_date', e.target.value); }}
                    />
                  </div>
                  <div className="space-y-1">
                    <Label>{t('vakthr.employees.labelStatus')} *</Label>
                    <Select
                      value={form.status}
                      onValueChange={(v) => { handleField('status', v as FormState['status']); }}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="active">{t('vakthr.status.active')}</SelectItem>
                        <SelectItem value="offboarding">{t('vakthr.status.offboarding')}</SelectItem>
                        <SelectItem value="terminated">{t('vakthr.status.terminated')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </>
              )}
            </div>

            <div className="space-y-1">
              <Label>{t('vakthr.employees.labelNotes')}</Label>
              <Textarea
                value={form.notes}
                onChange={(e) => { handleField('notes', e.target.value); }}
                placeholder={t('vakthr.employees.notesPlaceholder')}
                rows={3}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={() => void handleSubmit()} disabled={isPending}>
              {isPending ? t('common.saving') : editTarget ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

    </div>
  )
}
