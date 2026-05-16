import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { BookOpen, Plus, LayoutTemplate, Sparkles } from 'lucide-react'
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
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { ProGate } from '../../../shared/components/ProGate'
import { usePolicies, useCreatePolicy, useGeneratePolicyDraft } from '../hooks/usePolicies'
import { apiFetch } from '../../../api/client'
import type { Policy, CreatePolicyInput, Framework } from '../types'

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

const STATUS_LABELS: Record<Policy['status'], string> = {
  draft: 'Entwurf', active: 'Aktiv', archived: 'Archiviert',
}

function PolicyCard({ policy, onClick }: { policy: Policy; onClick: () => void }) {
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
            Review: {reviewDate}{isOverdue ? ' ⚠ Überfällig' : ''}
          </p>
        )}
        {policy.owner && <p className="text-xs text-muted-foreground">Verantwortlich: {policy.owner}</p>}
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
    },
  })

  function openDialog() {
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function handleSubmit() {
    createPolicy.mutate(form, { onSuccess: () => setDialogOpen(false) })
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
      },
    })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Richtlinienmanagement"
        description="Sicherheits- und Datenschutzrichtlinien erstellen, pflegen und prüfen."
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setTemplateOpen(true)}>
              <LayoutTemplate className="w-4 h-4 mr-1" />
              Aus Vorlage
            </Button>
            <Button variant="outline" onClick={openAiDialog}>
              <Sparkles className="w-4 h-4 mr-1" />
              KI-Entwurf generieren
            </Button>
            <Button onClick={openDialog}>
              <Plus className="w-4 h-4 mr-1" />
              Richtlinie anlegen
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">Fehler beim Laden der Richtlinien.</div>
        )}
        {!isLoading && !isError && policies?.length === 0 && (
          <EmptyState
            icon={BookOpen}
            title="Keine Richtlinien"
            description="Legen Sie Ihre Informationssicherheits- und Datenschutzrichtlinien an."
            action={<Button onClick={openDialog}><Plus className="w-4 h-4 mr-1" />Richtlinie anlegen</Button>}
          />
        )}
        {!isLoading && !isError && policies && policies.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {policies.map((p) => <PolicyCard key={p.id} policy={p} onClick={() => navigate(`/secvitals/policies/${p.id}`)} />)}
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
          <DialogHeader><DialogTitle>Richtlinie aus Vorlage erstellen</DialogTitle></DialogHeader>
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
                  <p className="text-xs text-muted-foreground mt-2">Erstelle Richtlinie …</p>
                )}
              </button>
            ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setTemplateOpen(false)}>Abbrechen</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* AI Policy Draft Dialog */}
      <Dialog open={aiDraftOpen} onOpenChange={setAiDraftOpen}>
        <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Sparkles className="w-4 h-4 text-brand" />
              KI-Richtlinienentwurf generieren
            </DialogTitle>
          </DialogHeader>

          {!aiDraft ? (
            <div className="space-y-4 py-2">
              <div className="space-y-1.5">
                <Label>Richtlinientyp *</Label>
                <Select value={aiPolicyType} onValueChange={setAiPolicyType}>
                  <SelectTrigger>
                    <SelectValue placeholder="Richtlinientyp auswählen …" />
                  </SelectTrigger>
                  <SelectContent>
                    {POLICY_TYPES.map((pt) => (
                      <SelectItem key={pt} value={pt}>{pt}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label>Framework als Kontext (optional)</Label>
                <Select value={aiFrameworkId} onValueChange={setAiFrameworkId}>
                  <SelectTrigger>
                    <SelectValue placeholder="Kein Framework ausgewählt" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">Kein Framework</SelectItem>
                    {frameworks?.map((fw) => (
                      <SelectItem key={fw.id} value={fw.id}>{fw.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label>Zusätzlicher Kontext (optional)</Label>
                <Textarea
                  rows={3}
                  placeholder="z.B. Wir nutzen Azure AD für Identity Management, 50 Mitarbeiter, Remote-First-Unternehmen …"
                  value={aiCustomContext}
                  onChange={(e) => setAiCustomContext(e.target.value)}
                />
              </div>

              <ProGate error={generateDraft.error}>{null}</ProGate>
              {generateDraft.isError && (
                <div className="text-sm text-red-400 p-3 bg-red-500/10 rounded-lg">
                  {generateDraft.error?.message?.includes('nicht konfiguriert')
                    ? 'KI-Features nicht konfiguriert. Bitte VAKT_AI_BASE_URL und VAKT_AI_PROVIDER in der Konfiguration setzen.'
                    : 'Fehler bei der KI-Generierung. Bitte versuchen Sie es erneut.'}
                </div>
              )}

              {generateDraft.isPending && (
                <div className="flex items-center gap-3 text-sm text-muted-foreground p-3 bg-accent rounded-lg">
                  <div className="w-4 h-4 border-2 border-brand border-t-transparent rounded-full animate-spin shrink-0" />
                  Richtlinie wird generiert … dies kann 10–30 Sekunden dauern.
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-4 py-2">
              <p className="text-sm text-muted-foreground">
                Der Entwurf wurde generiert. Sie können ihn bearbeiten, bevor Sie ihn speichern.
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
                <Button variant="outline" onClick={() => setAiDraftOpen(false)}>Abbrechen</Button>
                <Button
                  onClick={handleGenerateDraft}
                  disabled={!aiPolicyType || generateDraft.isPending}
                >
                  {generateDraft.isPending ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin mr-2" />
                      Generiert …
                    </>
                  ) : (
                    <>
                      <Sparkles className="w-4 h-4 mr-1" />
                      Entwurf generieren
                    </>
                  )}
                </Button>
              </>
            ) : (
              <>
                <Button variant="outline" onClick={() => { setAiDraft(''); generateDraft.reset() }}>
                  Neu generieren
                </Button>
                <Button variant="outline" onClick={() => setAiDraftOpen(false)}>Abbrechen</Button>
                <Button
                  onClick={handleSaveDraftAsPolicy}
                  disabled={createPolicy.isPending}
                >
                  {createPolicy.isPending ? 'Speichern …' : 'Als Richtlinie speichern'}
                </Button>
              </>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader><DialogTitle>Richtlinie anlegen</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="pol-title">Titel *</Label>
              <Input id="pol-title" placeholder="z.B. Informationssicherheits-Leitlinie" value={form.title}
                onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pol-category">Kategorie</Label>
              <Input id="pol-category" placeholder="z.B. IT-Sicherheit, Datenschutz, HR" value={form.category ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pol-desc">Beschreibung</Label>
              <Textarea id="pol-desc" rows={3} placeholder="Zweck und Geltungsbereich der Richtlinie …" value={form.description ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="pol-version">Version</Label>
                <Input id="pol-version" placeholder="1.0" value={form.version ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, version: e.target.value }))} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pol-owner">Verantwortlicher</Label>
                <Input id="pol-owner" placeholder="z.B. CISO" value={form.owner ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, owner: e.target.value }))} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="pol-effective">Gültig ab</Label>
                <Input id="pol-effective" type="date" value={form.effective_date ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, effective_date: e.target.value || undefined }))} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pol-review">Review-Datum</Label>
                <Input id="pol-review" type="date" value={form.review_date ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, review_date: e.target.value || undefined }))} />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>Abbrechen</Button>
            <Button onClick={handleSubmit} disabled={!form.title || createPolicy.isPending}>
              {createPolicy.isPending ? 'Speichern …' : 'Richtlinie anlegen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
