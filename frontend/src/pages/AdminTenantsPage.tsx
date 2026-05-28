import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Building2, Plus, ExternalLink, Trash2, Search, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../api/client'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Badge } from '../components/ui/badge'
import { Card, CardContent } from '../components/ui/card'
import { Input } from '../components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../components/ui/alert-dialog'
import { Skeleton } from '../components/ui/skeleton'
import { useToast } from '../shared/hooks/useToast'
import { useAuthStore } from '../shared/stores/auth'
import { useFormatDate } from '../shared/hooks/useFormatDate'

// ─── Types ────────────────────────────────────────────────────────────────────

interface ManagedOrg {
  id: string
  name: string
  plan: string
  created_at: string
  scheduled_deletion_at?: string
}

interface CreateOrgInput {
  name: string
  plan: string
}

interface ImpersonateResponse {
  access_token: string
  expires_in: number
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

const PLAN_LABELS: Record<string, string> = {
  msp_managed: 'MSP Managed',
  standard: 'Standard',
  enterprise: 'Enterprise',
}

const PLAN_COLORS: Record<string, string> = {
  msp_managed: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  standard: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  enterprise: 'bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300',
}

function PlanBadge({ plan }: { plan: string }) {
  return (
    <Badge className={`border-0 text-[11px] ${PLAN_COLORS[plan] ?? PLAN_COLORS.standard}`}>
      {PLAN_LABELS[plan] ?? plan}
    </Badge>
  )
}

// ─── Create Dialog ────────────────────────────────────────────────────────────

interface CreateDialogProps {
  open: boolean
  onClose: () => void
  onCreated: () => void
}

function CreateOrgDialog({ open, onClose, onCreated }: CreateDialogProps) {
  const { t } = useTranslation()
  const [form, setForm] = useState<CreateOrgInput>({ name: '', plan: 'msp_managed' })
  const [error, setError] = useState<string | null>(null)
  const { toast } = useToast()

  const mutation = useMutation({
    mutationFn: (input: CreateOrgInput) =>
      apiFetch<{ data: ManagedOrg }>('/admin/organizations', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      toast({ title: t('admin.tenants.createSuccess'), variant: 'default' })
      setForm({ name: '', plan: 'msp_managed' })
      setError(null)
      onCreated()
      onClose()
    },
    onError: (e: Error) => {
      setError(e.message)
    },
  })

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) {
      setError(t('admin.tenants.nameRequired'))
      return
    }
    mutation.mutate(form)
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('admin.tenants.createDialogTitle')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4 pt-2">
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t('admin.tenants.labelName')}</label>
            <Input
              value={form.name}
              onChange={(e) => { setForm({ ...form, name: e.target.value }); }}
              placeholder={t('admin.tenants.namePlaceholder')}
              autoFocus
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t('admin.tenants.labelPlan')}</label>
            <select
              value={form.plan}
              onChange={(e) => { setForm({ ...form, plan: e.target.value }); }}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand/40"
            >
              <option value="msp_managed">MSP Managed</option>
              <option value="standard">Standard</option>
              <option value="enterprise">Enterprise</option>
            </select>
          </div>

          {error && (
            <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t('admin.tenants.cancel')}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t('admin.tenants.creating') : t('admin.tenants.create')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function AdminTenantsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const { formatDate } = useFormatDate()
  const clearAuth = useAuthStore((s) => s.clearAuth)
  const [search, setSearch] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [deactivateTarget, setDeactivateTarget] = useState<ManagedOrg | null>(null)

  const { data, isLoading, isError, error, refetch, isFetching } = useQuery<{ data: ManagedOrg[] }>({
    queryKey: ['admin', 'tenants'],
    queryFn: () => apiFetch<{ data: ManagedOrg[] }>('/admin/organizations'),
  })

  const deleteMutation = useMutation({
    mutationFn: (orgId: string) =>
      apiFetch<void>(`/admin/organizations/${orgId}`, { method: 'DELETE' }),
    onSuccess: () => {
      toast({ title: t('admin.tenants.deactivateSuccess'), variant: 'default' })
      void queryClient.invalidateQueries({ queryKey: ['admin', 'tenants'] })
    },
    onError: (e: Error) => {
      toast({ title: t('admin.tenants.genericError'), description: e.message, variant: 'destructive' })
    },
  })

  const impersonateMutation = useMutation({
    mutationFn: (orgId: string) =>
      apiFetch<ImpersonateResponse>(`/admin/organizations/${orgId}/impersonate`, {
        method: 'POST',
      }),
    onSuccess: () => {
      // Cookie-based auth: the backend sets the impersonation token as an httpOnly
      // cookie in the response. Clear in-memory auth state and reload — the
      // store will re-hydrate from /auth/me with the new tenant context.
      clearAuth()
      window.location.href = '/'
    },
    onError: (e: Error) => {
      toast({ title: t('admin.tenants.impersonateError'), description: e.message, variant: 'destructive' })
    },
  })

  const orgs = data?.data ?? []
  const filtered = orgs.filter((o) =>
    o.name.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div>
      <PageHeader
        title={t('admin.tenants.title')}
        description={t('admin.tenants.description')}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => void refetch()} disabled={isFetching}>
              <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isFetching ? 'animate-spin' : ''}`} />
              {t('admin.tenants.refresh')}
            </Button>
            <Button size="sm" onClick={() => { setCreateOpen(true); }}>
              <Plus className="w-3.5 h-3.5 mr-1.5" />
              {t('admin.tenants.newOrg')}
            </Button>
          </div>
        }
      />

      <div className="p-6 space-y-4">
        {/* Search */}
        <div className="relative max-w-sm">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-secondary" />
          <Input
            value={search}
            onChange={(e) => { setSearch(e.target.value); }}
            placeholder={t('admin.tenants.searchPlaceholder')}
            className="pl-8"
          />
        </div>

        {/* Error */}
        {isError && (
          <div className="rounded-md bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 px-4 py-3 text-sm text-red-700 dark:text-red-300">
            {t('admin.tenants.loadError', { message: error instanceof Error ? error.message : t('admin.tenants.unknownError') })}
          </div>
        )}

        {/* Skeleton */}
        {isLoading && (
          <div className="space-y-2">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-14 w-full rounded-md" />
            ))}
          </div>
        )}

        {/* Table */}
        {!isLoading && (
          <Card>
            <CardContent className="p-0">
              {filtered.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-16 text-secondary gap-3">
                  <Building2 className="w-8 h-8 opacity-30" />
                  <p className="text-sm">
                    {search ? t('admin.tenants.noSearchResults') : t('admin.tenants.noOrgs')}
                  </p>
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-border text-secondary text-[12px] uppercase tracking-wider">
                        <th className="px-4 py-3 text-left font-medium">{t('admin.tenants.colOrg')}</th>
                        <th className="px-4 py-3 text-left font-medium">{t('admin.tenants.colPlan')}</th>
                        <th className="px-4 py-3 text-left font-medium">{t('admin.tenants.colCreated')}</th>
                        <th className="px-4 py-3 text-left font-medium">{t('admin.tenants.colStatus')}</th>
                        <th className="px-4 py-3 text-right font-medium">{t('admin.tenants.colActions')}</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                      {filtered.map((org) => (
                        <tr key={org.id} className="hover:bg-surface/50 transition-colors">
                          <td className="px-4 py-3 font-medium text-primary">{org.name}</td>
                          <td className="px-4 py-3">
                            <PlanBadge plan={org.plan} />
                          </td>
                          <td className="px-4 py-3 text-secondary">{formatDate(org.created_at)}</td>
                          <td className="px-4 py-3">
                            {org.scheduled_deletion_at ? (
                              <Badge className="border-0 bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300 text-[11px]">
                                {t('admin.tenants.statusDeletion', { date: formatDate(org.scheduled_deletion_at) })}
                              </Badge>
                            ) : (
                              <Badge className="border-0 bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300 text-[11px]">
                                {t('admin.tenants.statusActive')}
                              </Badge>
                            )}
                          </td>
                          <td className="px-4 py-3">
                            <div className="flex items-center justify-end gap-1.5">
                              <Button
                                variant="outline"
                                size="sm"
                                disabled={impersonateMutation.isPending || !!org.scheduled_deletion_at}
                                onClick={() => { impersonateMutation.mutate(org.id); }}
                              >
                                <ExternalLink className="w-3 h-3 mr-1" />
                                {t('admin.tenants.open')}
                              </Button>
                              <Button
                                variant="outline"
                                size="sm"
                                className="text-red-600 hover:text-red-700 border-red-200 hover:border-red-300 dark:border-red-800 dark:hover:border-red-700"
                                disabled={!!org.scheduled_deletion_at}
                                onClick={() => { setDeactivateTarget(org); }}
                              >
                                <Trash2 className="w-3 h-3 mr-1" />
                                {t('admin.tenants.deactivate')}
                              </Button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </div>

      {/* Create Dialog */}
      <CreateOrgDialog
        open={createOpen}
        onClose={() => { setCreateOpen(false); }}
        onCreated={() => void queryClient.invalidateQueries({ queryKey: ['admin', 'tenants'] })}
      />

      {/* Deactivate Confirmation */}
      <AlertDialog open={!!deactivateTarget} onOpenChange={(v) => { if (!v) setDeactivateTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('admin.tenants.deactivateDialogTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              <strong>{deactivateTarget?.name}</strong> {t('admin.tenants.deactivateDialogDesc', { name: '' }).replace('<strong>{{name}}</strong> ', '')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('admin.tenants.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-red-600 hover:bg-red-700"
              onClick={() => {
                if (deactivateTarget) {
                  deleteMutation.mutate(deactivateTarget.id)
                  setDeactivateTarget(null)
                }
              }}
            >
              {t('admin.tenants.deactivateConfirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
