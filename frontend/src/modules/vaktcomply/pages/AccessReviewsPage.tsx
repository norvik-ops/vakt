import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ClipboardCheck, Plus, ChevronDown, ChevronRight, Pencil, Trash2, Check, X } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../../../components/ui/table'
import { useAuthStore } from '../../../shared/stores/auth'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import {
  useAccessReviewCampaigns,
  useCreateAccessReviewCampaign,
  useUpdateAccessReviewCampaign,
  useDeleteAccessReviewCampaign,
  useAccessReviewItems,
  useCreateAccessReviewItem,
  useUpdateAccessReviewItem,
} from '../hooks/useAccessReviews'
import type {
  AccessReviewCampaign,
  AccessReviewItem,
  CreateAccessReviewCampaignInput,
  CreateAccessReviewItemInput,
} from '../types'

// ─── Badge helpers ────────────────────────────────────────────────────────────

const STATUS_CLASS: Record<AccessReviewCampaign['status'], string> = {
  draft: 'bg-slate-500/20 text-slate-400 border-slate-500/30',
  active: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
  cancelled: 'bg-red-500/20 text-red-400 border-red-500/30',
}

// STATUS_I18N_KEY / DECISION_I18N_KEY map domain enums to the corresponding
// i18n key path. They are constants so they live at module level, but they
// must NOT be resolved here — t() is only available inside components.
const STATUS_I18N_KEY: Record<AccessReviewCampaign['status'], string> = {
  draft: 'vaktcomply.accessReviews.status.draft',
  active: 'vaktcomply.accessReviews.status.active',
  completed: 'vaktcomply.accessReviews.status.completed',
  cancelled: 'vaktcomply.accessReviews.status.cancelled',
}

const DECISION_CLASS: Record<AccessReviewItem['decision'], string> = {
  pending: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  approved: 'bg-green-500/20 text-green-400 border-green-500/30',
  revoked: 'bg-red-500/20 text-red-400 border-red-500/30',
}

const DECISION_I18N_KEY: Record<AccessReviewItem['decision'], string> = {
  pending: 'vaktcomply.accessReviews.decision.pending',
  approved: 'vaktcomply.accessReviews.decision.approved',
  revoked: 'vaktcomply.accessReviews.decision.revoked',
}

// ─── Empty campaign form ──────────────────────────────────────────────────────

function emptyCampaignForm(): CreateAccessReviewCampaignInput {
  return {
    title: '',
    description: '',
    reviewer_email: '',
    scope: '',
    due_date: '',
  }
}

function campaignToForm(c: AccessReviewCampaign): CreateAccessReviewCampaignInput {
  return {
    title: c.title,
    description: c.description ?? '',
    reviewer_email: c.reviewer_email,
    scope: c.scope ?? '',
    due_date: c.due_date ? c.due_date.slice(0, 10) : '',
  }
}

// ─── Review Items panel ───────────────────────────────────────────────────────

function ReviewItemsPanel({ campaign }: { campaign: AccessReviewCampaign }) {
  const { t } = useTranslation()
  const { data: items = [], isLoading } = useAccessReviewItems(campaign.id)
  const createItem = useCreateAccessReviewItem()
  const updateItem = useUpdateAccessReviewItem()

  const [newItemForm, setNewItemForm] = useState<Omit<CreateAccessReviewItemInput, 'campaign_id'>>({
    user_email: '',
    access_level: '',
  })
  const [addingItem, setAddingItem] = useState(false)

  function handleAddItem() {
    if (!newItemForm.user_email || !newItemForm.access_level) return
    createItem.mutate(
      { campaign_id: campaign.id, ...newItemForm },
      {
        onSuccess: () => {
          setNewItemForm({ user_email: '', access_level: '' })
          setAddingItem(false)
        },
      },
    )
  }

  function handleDecision(item: AccessReviewItem, decision: 'approved' | 'revoked') {
    updateItem.mutate({
      campaignId: campaign.id,
      itemId: item.id,
      input: { decision },
    })
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-20">
        <Spinner size="md" color="primary" />
      </div>
    )
  }

  return (
    <div className="mt-4 space-y-3">
      {items.length > 0 ? (
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('vaktcomply.accessReviews.fields.user')}</TableHead>
                <TableHead>{t('vaktcomply.accessReviews.fields.role')}</TableHead>
                <TableHead>{t('vaktcomply.accessReviews.fields.decision')}</TableHead>
                <TableHead>{t('vaktcomply.accessReviews.fields.comment')}</TableHead>
                <TableHead className="w-32">{t('vaktcomply.accessReviews.fields.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-mono text-sm">{item.user_email}</TableCell>
                  <TableCell>{item.access_level}</TableCell>
                  <TableCell>
                    <Badge className={DECISION_CLASS[item.decision]}>
                      {t(DECISION_I18N_KEY[item.decision])}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-xs truncate">
                    {item.reviewer_comment ?? '—'}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-green-400 hover:text-green-300"
                        title="Bestätigen"
                        disabled={item.decision === 'approved'}
                        onClick={() => { handleDecision(item, 'approved'); }}
                      >
                        <Check className="w-3.5 h-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-red-400 hover:text-red-300"
                        title="Widerrufen"
                        disabled={item.decision === 'revoked'}
                        onClick={() => { handleDecision(item, 'revoked'); }}
                      >
                        <X className="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground py-4 text-center">
          Keine Einträge. Fügen Sie Benutzer zur Überprüfung hinzu.
        </p>
      )}

      {addingItem ? (
        <div className="flex items-end gap-2 pt-2">
          <div className="flex-1 space-y-1">
            <Label className="text-xs">{t('vaktcomply.accessReviews.fields.userEmail')}</Label>
            <Input
              placeholder={t('vaktcomply.accessReviews.placeholders.userEmail')}
              value={newItemForm.user_email}
              onChange={(e) => { setNewItemForm((f) => ({ ...f, user_email: e.target.value })); }}
            />
          </div>
          <div className="flex-1 space-y-1">
            <Label className="text-xs">{t('vaktcomply.accessReviews.fields.role')}</Label>
            <Input
              placeholder={t('vaktcomply.accessReviews.placeholders.role')}
              value={newItemForm.access_level}
              onChange={(e) => { setNewItemForm((f) => ({ ...f, access_level: e.target.value })); }}
            />
          </div>
          <Button
            size="sm"
            onClick={handleAddItem}
            disabled={!newItemForm.user_email || !newItemForm.access_level || createItem.isPending}
          >
            Hinzufügen
          </Button>
          <Button size="sm" variant="outline" onClick={() => { setAddingItem(false); }}>
            Abbrechen
          </Button>
        </div>
      ) : (
        <Button size="sm" variant="outline" onClick={() => { setAddingItem(true); }}>
          <Plus className="w-3.5 h-3.5 mr-1" />
          Benutzer hinzufügen
        </Button>
      )}
    </div>
  )
}

// ─── Campaign row ─────────────────────────────────────────────────────────────

function CampaignRow({
  campaign,
  isAdmin,
  onEdit,
  onDelete,
  onActivate,
}: {
  campaign: AccessReviewCampaign
  isAdmin: boolean
  onEdit: () => void
  onDelete: () => void
  onActivate: () => void
}) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const { formatDate } = useFormatDate()

  return (
    <Card>
      <CardContent className="pt-4 pb-4 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <button
            className="flex items-center gap-2 flex-1 text-left"
            onClick={() => { setExpanded((v) => !v); }}
          >
            {expanded ? (
              <ChevronDown className="w-4 h-4 text-muted-foreground shrink-0" />
            ) : (
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            )}
            <div className="space-y-1">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-sm font-medium">{campaign.title}</span>
                <Badge className={STATUS_CLASS[campaign.status]}>
                  {t(STATUS_I18N_KEY[campaign.status])}
                </Badge>
              </div>
              <div className="flex items-center gap-3 text-xs text-muted-foreground flex-wrap">
                <span>Prüfer: {campaign.reviewer_email}</span>
                {campaign.scope && <span>Scope: {campaign.scope}</span>}
                {campaign.due_date && (
                  <span>
                    Fällig: {formatDate(campaign.due_date)}
                  </span>
                )}
              </div>
            </div>
          </button>

          <div className="flex items-center gap-1 shrink-0">
            {isAdmin && campaign.status === 'draft' && (
              <Button size="sm" variant="outline" onClick={onActivate}>
                Aktivieren
              </Button>
            )}
            {isAdmin && (
              <>
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
              </>
            )}
          </div>
        </div>

        {expanded && <ReviewItemsPanel campaign={campaign} />}
      </CardContent>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function AccessReviewsPage() {
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const isAdmin = user?.roles.includes('Admin') ?? false

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateAccessReviewCampaignInput>(emptyCampaignForm())

  const { data: campaigns = [], isLoading, isError } = useAccessReviewCampaigns()
  const createCampaign = useCreateAccessReviewCampaign()
  const updateCampaign = useUpdateAccessReviewCampaign()
  const deleteCampaign = useDeleteAccessReviewCampaign()

  function openCreate() {
    setEditId(null)
    setForm(emptyCampaignForm())
    setDialogOpen(true)
  }

  function openEdit(c: AccessReviewCampaign) {
    setEditId(c.id)
    setForm(campaignToForm(c))
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm('Kampagne wirklich löschen? Alle Prüfeinträge werden ebenfalls gelöscht.')) {
      deleteCampaign.mutate(id)
    }
  }

  function handleActivate(c: AccessReviewCampaign) {
    updateCampaign.mutate({ id: c.id, input: { status: 'active' } })
  }

  function handleSubmit() {
    const payload: CreateAccessReviewCampaignInput = {
      ...form,
      due_date: form.due_date
        ? new Date(form.due_date).toISOString()
        : undefined,
    }
    if (editId) {
      updateCampaign.mutate(
        { id: editId, input: payload },
        { onSuccess: () => { setDialogOpen(false); } },
      )
    } else {
      createCampaign.mutate(payload, { onSuccess: () => { setDialogOpen(false); } })
    }
  }

  const isPending = createCampaign.isPending || updateCampaign.isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktcomply.accessReviews.title')}
        description={t('vaktcomply.accessReviews.description')}
        actions={
          isAdmin ? (
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vaktcomply.accessReviews.addCampaign')}
            </Button>
          ) : undefined
        }
      />

      <div className="flex-1 p-6 space-y-3">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}

        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Fehler beim Laden der Zugriffsüberprüfungen.
          </div>
        )}

        {!isLoading && !isError && campaigns.length === 0 && (
          <EmptyState
            icon={ClipboardCheck}
            title={t('vaktcomply.accessReviews.emptyTitle')}
            description={t('vaktcomply.accessReviews.emptyDescription')}
            action={
              isAdmin ? (
                <Button onClick={openCreate}>
                  <Plus className="w-4 h-4 mr-1" />
                  {t('vaktcomply.accessReviews.addCampaign')}
                </Button>
              ) : undefined
            }
          />
        )}

        {!isLoading && !isError && campaigns.length > 0 && (
          <div className="space-y-3">
            {campaigns.map((c) => (
              <CampaignRow
                key={c.id}
                campaign={c}
                isAdmin={isAdmin}
                onEdit={() => { openEdit(c); }}
                onDelete={() => { handleDelete(c.id); }}
                onActivate={() => { handleActivate(c); }}
              />
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editId ? t('vaktcomply.accessReviews.editCampaign') : t('vaktcomply.accessReviews.addCampaign')}
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.accessReviews.fields.title')} *</Label>
              <Input
                placeholder={t('vaktcomply.accessReviews.placeholders.title')}
                value={form.title}
                onChange={(e) => { setForm((f) => ({ ...f, title: e.target.value })); }}
              />
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.accessReviews.fields.description')}</Label>
              <Textarea
                rows={3}
                placeholder={t('vaktcomply.accessReviews.placeholders.description')}
                value={form.description ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, description: e.target.value })); }}
              />
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.accessReviews.fields.reviewerEmail')} *</Label>
              <Input
                type="email"
                placeholder={t('vaktcomply.accessReviews.placeholders.reviewerEmail')}
                value={form.reviewer_email}
                onChange={(e) => { setForm((f) => ({ ...f, reviewer_email: e.target.value })); }}
              />
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.accessReviews.fields.scope')}</Label>
              <Input
                placeholder={t('vaktcomply.accessReviews.placeholders.scope')}
                value={form.scope ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, scope: e.target.value })); }}
              />
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.accessReviews.fields.dueDate')}</Label>
              <Input
                type="date"
                value={form.due_date ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, due_date: e.target.value })); }}
              />
            </div>

            {editId && (
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.accessReviews.fields.status')}</Label>
                <Select
                  value={(form as { status?: string }).status ?? 'draft'}
                  onValueChange={(v) => { setForm((f) => ({ ...f, status: v })); }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="draft">Entwurf</SelectItem>
                    <SelectItem value="active">Aktiv</SelectItem>
                    <SelectItem value="completed">Abgeschlossen</SelectItem>
                    <SelectItem value="cancelled">Abgebrochen</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>
              Abbrechen
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={!form.title || !form.reviewer_email || isPending}
            >
              {isPending ? 'Speichern …' : editId ? 'Speichern' : 'Erstellen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
