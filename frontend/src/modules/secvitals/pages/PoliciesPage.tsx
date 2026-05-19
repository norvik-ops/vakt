import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { BookOpen, Plus, LayoutTemplate, Sparkles, ChevronsUpDown, ChevronUp, ChevronDown, Printer } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ExportButton } from '../../../shared/components/ExportButton'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { ProGate } from '../../../shared/components/ProGate'
import { FieldError } from '../../../shared/components/FieldError'
import { useSavedFilters } from '../../../shared/hooks/useSavedFilters'
import { useFormValidation } from '../../../shared/hooks/useFormValidation'
import { usePolicies, useCreatePolicy, useGeneratePolicyDraft } from '../hooks/usePolicies'
import { apiFetch } from '../../../api/client'
import type { Policy, CreatePolicyInput, Framework } from '../types'
import type { SortDir } from '../../../shared/hooks/useSortableTable'

import { toast } from '../../../shared/hooks/useToast'
import { handleApiError } from '../../../shared/utils/errorMessages'
import { SkeletonCardGrid } from '../../../shared/components/SkeletonLoaders'

const POLICY_TYPES = [
  'Informationssicherheitsrichtlinie (ISO 27001 A.5.1)',
  'Passwort- und Authentifizierungsrichtlinie (ISO 27001 A.9.4)',
  'BYOD-Richtlinie (ISO 27001 A.6.2)',
  'Incident-Response-Richtlinie (ISO 27001 A.16)',
  'Datenschutzrichtlinie (DSGVO Art. 24)',
  'Clean-Desk & Clear-Screen-Richtlinie (ISO 27001 A.11.2)',
  'Remote-Work-Richtlinie (ISO 27001 A.6.2)',
  'Acceptable-Use-Policy (ISO 27001 A.8.1)',
  'Backup-Richtlinie (ISO 27001 A.12.3)',
  'Change-Management-Richtlinie (ISO 27001 A.14.2)',
]

interface PolicyTemplate {
  id: string
  title: string
  category: string
  description: string
  content: string
}

const STATUS_CLASS: Record<Policy['status'], string> = {
  draft: 'bg-secondary text-secondary-foreground',
  active: 'bg-green-500/20 text-green-400 border-green-500/30',
  archived: 'bg-secondary text-muted-foreground',
}

function PolicyCard({ policy, onClick }: { policy: Policy; onClick: () => void }) {
  const { t } = useTranslation()
  const STATUS_LABELS: Record<Policy['status'], string> = {
    draft: t('secvitals.policiesPage.statusDraft'),
    active: t('secvitals.policiesPage.statusActive'),
    archived: t('secvitals.policiesPage.statusArchived'),
  }
  const reviewDate = policy.review_date
    ? new Date(policy.review_date).toLocaleDateString('de-DE', { year: 'numeric', month: 'short', day: 'numeric' })
    : null
  const isOverdue = policy.review_date && new Date(policy.review_date) < new Date()

  return (
    <Card className="cursor-pointer hover:border-brand/50 transition-colors" onClick={onClick}>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <div>
            <div className="flex items-center gap-1.5">
              <p className="font-medium text-sm">{policy.title}</p>
              <Badge className="bg-blue-500/20 text-blue-400 border-blue-500/30 text-xs px-1.5 py-0 shrink-0">
                v{policy.version_num}
              </Badge>
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">{policy.version}{policy.category ? ` · ${policy.category}` : ''}</p>
          </div>
          <Badge className={STATUS_CLASS[policy.status]}>{STATUS_LABELS[policy.status]}</Badge>
        </div>
        {policy.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{policy.description}</p>
        )}
        {reviewDate && (
          <p className={`text-xs ${isOverdue ? 'text-amber-400' : 'text-muted-foreground'}`}>
            {t('secvitals.policiesPage.reviewDate')}: {reviewDate}{isOverdue ? ` ⚠ ${t('secvitals.policiesPage.overdue')}` : ''}
          </p>
        )}
        {policy.owner && <p className="text-xs text-muted-foreground">{t('secvitals.policiesPage.owner')}: {policy.owner}</p>}
      </CardContent>
    </Card>
  )
}

function emptyForm(): CreatePolicyInput {
  return {
    title: '',
    description: '',
    category: '',
    version: '1.0',
    owner: '',
  }
}

export default function PoliciesPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [templateOpen, setTemplateOpen] = useState(false)
  const [aiDraftOpen, setAiDraftOpen] = useState(false)
  const [aiPolicyType, setAiPolicyType] = useState('')
  const [aiFrameworkId, setAiFrameworkId] = useState('')
  const [aiCustomContext, setAiCustomContext] = useState('')
  const [aiDraft, setAiDraft] = useState('')
  const [form, setForm] = useState<CreatePolicyInput>(emptyForm())
  const [page, setPage] = useState(1)

  const { data: policies, isLoading, isError, pagination } = usePolicies(page)
  const createPolicy = useCreatePolicy()
  const generateDraft = useGeneratePolicyDraft()
  const { errors: polErrors, validate: validatePol, clearError: clearPolError, clearAll: clearPolErrors } = useFormValidation<Record<string, unknown>>({
    title: { required: true },
  })

  const POLICY_SORT_OPTIONS: { key: keyof Policy; label: string }[] = [
    { key: 'title', label: t('common.name') },
    { key: 'status', label: t('common.status') },
    { key: 'review_date', label: t('secvitals.policiesPage.labelReviewDate') },
    { key: 'version_num', label: t('secvitals.policiesPage.labelVersion') },
  ]

  const [policySort, setPolicySort] = useSavedFilters<{ sortKey: keyof Policy; sortDir: SortDir }>(
    'policies_sort',
    { sortKey: 'title', sortDir: 'asc' },
  )
  const { sortKey, sortDir } = policySort

  function toggleSort(key: keyof Policy) {
    setPolicySort((prev) =>
      prev.sortKey === key
        ? { ...prev, sortDir: prev.sortDir === 'asc' ? 'desc' : 'asc' }
        : { sortKey: key, sortDir: 'asc' },
    )
  }

  const sortedPolicies = useMemo(() => {
    const data = policies ?? []
    return [...data].sort((a, b) => {
      const av = a[sortKey]
      const bv = b[sortKey]
      if (av == null && bv == null) return 0
      if (av == null) return 1
      if (bv == null) return -1
      let cmp: number
      if (typeof av === 'number' && typeof bv === 'number') {
        cmp = av - bv
      } else {
        cmp = String(av).localeCompare(String(bv), 'de', { sensitivity: 'base' })
      }
      return sortDir === 'asc' ? cmp : -cmp
    })
  }, [policies, sortKey, sortDir])

  const { data: frameworks } = useQuery<Framework[]>({
    queryKey: ['secvitals', 'frameworks'],
    queryFn: () => apiFetch<Framework[]>('/secvitals/frameworks'),
    staleTime: 5 * 60 * 1000,
  })

  const { data: templates, isLoading: templatesLoading } = useQuery<PolicyTemplate[]>({
    queryKey: ['policy-templates'],
    queryFn: () => apiFetch<PolicyTemplate[]>('/secvitals/policy-templates'),
    enabled: templateOpen,
    staleTime: 5 * 60 * 1000,
  })

  const applyTemplate = useMutation<Policy, Error, string>({
    mutationFn: (templateId) =>
      apiFetch<Policy>(`/secvitals/policy-templates/${templateId}/apply`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'policies'] })
      setTemplateOpen(false)
      toast(t('secvitals.policiesPage.toastTemplateCreated'), 'success')
    },
    onError: (err) => toast(handleApiError(err), 'error'),
  })

  function openDialog() {
    setForm(emptyForm())
    clearPolErrors()
    setDialogOpen(true)
  }

  function handleSubmit() {
    if (!validatePol({ title: form.title })) return
    createPolicy.mutate(form, {
      onSuccess: () => {
        setDialogOpen(false)
        toast('Richtlinie erstellt', 'success')
      },
      onError: (err) => toast(handleApiError(err), 'error'),
    })
  }

  function openAiDialog() {
    setAiPolicyType('')
    setAiFrameworkId('')
    setAiCustomContext('')
    setAiDraft('')
    setAiDraftOpen(true)
  }

  function handleGenerateDraft() {
    if (!aiPolicyType) return
    generateDraft.mutate(
      { policy_type: aiPolicyType, framework_id: aiFrameworkId || undefined, custom_context: aiCustomContext || undefined },
      { onSuccess: (data) => setAiDraft(data.draft) },
    )
  }

  function handleSaveDraftAsPolicy() {
    const titleMatch = aiPolicyType.match(/^([^(]+)/)
    const title = titleMatch ? titleMatch[1].trim() : aiPolicyType
    const newForm: CreatePolicyInput = {
      title,
      description: aiDraft,
      category: 'IT-Sicherheit',
      version: '1.0',
      owner: '',
    }
    createPolicy.mutate(newForm, {
      onSuccess: () => {
        setAiDraftOpen(false)
        setAiDraft('')
        toast(t('secvitals.policiesPage.toastAiCreated'), 'success')
      },
      onError: (err) => toast(handleApiError(err), 'error'),
    })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('secvitals.policiesPage.title')}
        description={t('secvitals.policiesPage.description')}
        actions={
          <div className="flex gap-2">
            <ExportButton
              endpoint="/api/v1/secvitals/policies/export/xlsx"
              filename={`richtlinien-${new Date().toISOString().slice(0, 10)}`}
              label="Exportieren"
              format="xlsx"
            />
            <Button variant="outline" size="sm" onClick={() => window.print()} className="no-print">
              <Printer className="w-4 h-4 mr-1" />
              Drucken
            </Button>
            <Button variant="outline" onClick={() => setTemplateOpen(true)} aria-haspopup="dialog">
              <LayoutTemplate className="w-4 h-4 mr-1" />
              {t('secvitals.policiesPage.fromTemplate')}
            </Button>
            <Button variant="outline" onClick={openAiDialog} aria-haspopup="dialog">
              <Sparkles className="w-4 h-4 mr-1" />
              {t('secvitals.policiesPage.aiDraft')}
            </Button>
            <Button onClick={openDialog} aria-haspopup="dialog">
              <Plus className="w-4 h-4 mr-1" />
              {t('secvitals.policiesPage.createPolicy')}
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6">
        {/* Sort toolbar */}
        {!isLoading && !isError && policies && policies.length > 0 && (
          <div className="flex flex-wrap items-center gap-2 text-xs text-secondary mb-4">
            <span className="font-medium">{t('common.filter')}:</span>
            {POLICY_SORT_OPTIONS.map((opt) => {
              const isActive = sortKey === opt.key
              return (
                <button
                  key={String(opt.key)}
                  onClick={() => toggleSort(opt.key)}
                  className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-md border transition-colors ${
                    isActive
                      ? 'border-brand/50 bg-brand/10 text-brand'
                      : 'border-border hover:border-brand/30 hover:bg-surface2'
                  }`}
                >
                  {opt.label}
                  {isActive ? (
                    sortDir === 'asc'
                      ? <ChevronUp className="w-3 h-3" />
                      : <ChevronDown className="w-3 h-3" />
                  ) : (
                    <ChevronsUpDown className="w-3 h-3 opacity-50" />
                  )}
                </button>
              )
            })}
          </div>
        )}
        {isLoading && <SkeletonCardGrid count={6} />}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">{t('secvitals.policiesPage.loadError')}</div>
        )}
        {!isLoading && !isError && policies?.length === 0 && (
          <EmptyState
            icon={BookOpen}
            title={t('secvitals.policiesPage.noPolicies')}
            description={t('secvitals.policiesPage.noPoliciesDesc')}
            action={<Button onClick={openDialog} aria-haspopup="dialog"><Plus className="w-4 h-4 mr-1" />{t('secvitals.policiesPage.createPolicy')}</Button>}
          />
        )}
        {!isLoading && !isError && policies && policies.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {sortedPolicies.map((p) => <PolicyCard key={p.id} policy={p} onClick={() => navigate(`/secvitals/policies/${p.id}`)} />)}
          </div>
        )}
        <Pagination
          page={page}
          totalPages={pagination?.total_pages ?? 1}
          onPageChange={setPage}
        />
      </div>

      {/* Template picker dialog */}
      <Dialog open={templateOpen} onOpenChange={setTemplateOpen}>
        <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
          <DialogHeader><DialogTitle>{t('secvitals.policiesPage.templateDialogTitle')}</DialogTitle></DialogHeader>
          <div className="space-y-3 py-2">
            {templatesLoading && (
              <div className="flex items-center justify-center h-32">
                <div className="w-5 h-5 border-2 border-primary border-t-transparent rounded-full animate-spin" />
              </div>
            )}
            {templates && templates.map((tpl) => (
              <button
                key={tpl.id}
                className="w-full text-left p-4 rounded-lg border border-border hover:border-brand/50 hover:bg-accent transition-colors disabled:opacity-50"
                disabled={applyTemplate.isPending}
                onClick={() => applyTemplate.mutate(tpl.id)}
              >
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <p className="font-medium text-sm">{tpl.title}</p>
                    <p className="text-xs text-muted-foreground mt-1 line-clamp-2">{tpl.description}</p>
                  </div>
                  <Badge variant="outline" className="shrink-0 text-xs">{tpl.category}</Badge>
                </div>
                {applyTemplate.isPending && applyTemplate.variables === tpl.id && (
                  <p className="text-xs text-muted-foreground mt-2">{t('secvitals.policiesPage.creatingFromTemplate')}</p>
                )}
              </button>
            ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setTemplateOpen(false)}>{t('common.cancel')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* AI Policy Draft Dialog */}
      <Dialog open={aiDraftOpen} onOpenChange={setAiDraftOpen}>
        <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Sparkles className="w-4 h-4 text-brand" />
              {t('secvitals.policiesPage.aiDialogTitle')}
            </DialogTitle>
          </DialogHeader>

          {!aiDraft ? (
            <div className="space-y-4 py-2">
              <div className="space-y-1.5">
                <Label>{t('secvitals.policiesPage.aiPolicyType')} *</Label>
                <Select value={aiPolicyType} onValueChange={setAiPolicyType}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('secvitals.policiesPage.aiPolicyTypePlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    {POLICY_TYPES.map((pt) => (
                      <SelectItem key={pt} value={pt}>{pt}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label>{t('secvitals.policiesPage.aiFramework')}</Label>
                <Select value={aiFrameworkId} onValueChange={setAiFrameworkId}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('secvitals.policiesPage.aiFrameworkPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">{t('secvitals.policiesPage.noFramework')}</SelectItem>
                    {frameworks?.map((fw) => (
                      <SelectItem key={fw.id} value={fw.id}>{fw.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label>{t('secvitals.policiesPage.aiContext')}</Label>
                <Textarea
                  rows={3}
                  placeholder={t('secvitals.policiesPage.aiContextPlaceholder')}
                  value={aiCustomContext}
                  onChange={(e) => setAiCustomContext(e.target.value)}
                />
              </div>

              <ProGate error={generateDraft.error}>{null}</ProGate>
              {generateDraft.isError && (
                <div className="text-sm text-red-400 p-3 bg-red-500/10 rounded-lg">
                  {generateDraft.error?.message?.includes('nicht konfiguriert')
                    ? t('secvitals.policiesPage.aiNotConfigured')
                    : t('secvitals.policiesPage.aiError')}
                </div>
              )}

              {generateDraft.isPending && (
                <div className="flex items-center gap-3 text-sm text-muted-foreground p-3 bg-accent rounded-lg">
                  <div className="w-4 h-4 border-2 border-brand border-t-transparent rounded-full animate-spin shrink-0" />
                  {t('secvitals.policiesPage.aiGenerating')}
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-4 py-2">
              <p className="text-sm text-muted-foreground">
                {t('secvitals.policiesPage.aiDraftGenerated')}
              </p>
              <Textarea
                rows={18}
                value={aiDraft}
                onChange={(e) => setAiDraft(e.target.value)}
                className="font-mono text-xs"
              />
            </div>
          )}

          <DialogFooter>
            {!aiDraft ? (
              <>
                <Button variant="outline" onClick={() => setAiDraftOpen(false)}>{t('common.cancel')}</Button>
                <Button
                  onClick={handleGenerateDraft}
                  disabled={!aiPolicyType || generateDraft.isPending}
                >
                  {generateDraft.isPending ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin mr-2" />
                      {t('secvitals.policiesPage.generating')}
                    </>
                  ) : (
                    <>
                      <Sparkles className="w-4 h-4 mr-1" />
                      {t('secvitals.policiesPage.generateDraft')}
                    </>
                  )}
                </Button>
              </>
            ) : (
              <>
                <Button variant="outline" onClick={() => { setAiDraft(''); generateDraft.reset() }}>
                  {t('secvitals.policiesPage.regenerate')}
                </Button>
                <Button variant="outline" onClick={() => setAiDraftOpen(false)}>{t('common.cancel')}</Button>
                <Button
                  onClick={handleSaveDraftAsPolicy}
                  disabled={createPolicy.isPending}
                >
                  {createPolicy.isPending ? t('secvitals.policiesPage.saving') : t('secvitals.policiesPage.saveAsPolicy')}
                </Button>
              </>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader><DialogTitle>{t('secvitals.policiesPage.createDialogTitle')}</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="pol-title">{t('secvitals.policiesPage.labelTitle')} <span className="text-red-400 text-xs">*</span></Label>
              <Input id="pol-title" placeholder={t('secvitals.policiesPage.placeholderTitle')} value={form.title}
                onChange={(e) => { setForm((f) => ({ ...f, title: e.target.value })); clearPolError('title') }} />
              <FieldError error={polErrors.title ?? null} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pol-category">{t('secvitals.policiesPage.labelCategory')}</Label>
              <Input id="pol-category" placeholder={t('secvitals.policiesPage.placeholderCategory')} value={form.category ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pol-desc">{t('secvitals.policiesPage.labelDescription')}</Label>
              <Textarea id="pol-desc" rows={3} placeholder={t('secvitals.policiesPage.placeholderDescription')} value={form.description ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="pol-version">{t('secvitals.policiesPage.labelVersion')}</Label>
                <Input id="pol-version" placeholder="1.0" value={form.version ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, version: e.target.value }))} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pol-owner">{t('secvitals.policiesPage.labelOwner')}</Label>
                <Input id="pol-owner" placeholder={t('secvitals.policiesPage.placeholderOwner')} value={form.owner ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, owner: e.target.value }))} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="pol-effective">{t('secvitals.policiesPage.labelEffectiveDate')}</Label>
                <Input id="pol-effective" type="date" value={form.effective_date ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, effective_date: e.target.value || undefined }))} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pol-review">{t('secvitals.policiesPage.labelReviewDate')}</Label>
                <Input id="pol-review" type="date" value={form.review_date ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, review_date: e.target.value || undefined }))} />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleSubmit} disabled={createPolicy.isPending}>
              {createPolicy.isPending ? t('secvitals.policiesPage.saving') : t('secvitals.policiesPage.createPolicy')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
