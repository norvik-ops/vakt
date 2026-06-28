import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Shield, Plus, Pencil, Trash2, Lock, Link2 } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  useProtectionNeeds,
  useCreateProtectionNeed,
  useUpdateProtectionNeed,
  useFinalizeProtectionNeed,
  useDeleteProtectionNeed,
  useLinkAssetToPNA,
} from '../hooks/useProtectionNeeds'
import { apiFetch } from '../../../api/client'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import type {
  ProtectionNeedAssessment,
  ProtectionLevel,
  ProtectionObjectType,
  CreateProtectionNeedInput,
} from '../types'

// Minimal type for the asset picker — only what we display.
interface AssetOption {
  id: string
  name: string
  criticality: 'low' | 'medium' | 'high' | 'critical'
  type: string
}

const criticality_de: Record<string, string> = {
  low: 'Niedrig', medium: 'Mittel', high: 'Hoch', critical: 'Kritisch',
}

// ─── Constants ────────────────────────────────────────────────────────────────

const LEVEL_CLASS: Record<ProtectionLevel, string> = {
  normal: 'bg-green-500/20 text-green-400 border-green-500/30',
  hoch: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  sehr_hoch: 'bg-red-500/20 text-red-400 border-red-500/30',
}

const OBJECT_TYPES: ProtectionObjectType[] = ['process', 'system', 'information', 'location']
const LEVELS: ProtectionLevel[] = ['normal', 'hoch', 'sehr_hoch']

function emptyForm(): CreateProtectionNeedInput {
  return {
    name: '',
    object_type: 'system',
    object_name: '',
    confidentiality: 'normal',
    integrity: 'normal',
    availability: 'normal',
  }
}

function assessmentToForm(a: ProtectionNeedAssessment): CreateProtectionNeedInput {
  return {
    name: a.name,
    object_type: a.object_type,
    object_name: a.object_name,
    confidentiality: a.confidentiality,
    integrity: a.integrity,
    availability: a.availability,
  }
}

// ─── Card ─────────────────────────────────────────────────────────────────────

function ProtectionNeedCard({
  item,
  onEdit,
  onFinalize,
  onDelete,
  onLinkAsset,
}: {
  item: ProtectionNeedAssessment
  onEdit: () => void
  onFinalize: () => void
  onDelete: () => void
  onLinkAsset?: () => void
}) {
  const { t } = useTranslation()
  const finalized = item.status === 'finalized'

  return (
    <Card>
      <CardContent className="pt-5 space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div className="flex-1 space-y-1">
            <p className="font-medium text-sm">{item.name}</p>
            <p className="text-xs text-muted-foreground">
              {t(`protectionNeeds.objectType.${item.object_type}`)} · {item.object_name}
            </p>
          </div>
          <div className="flex items-center gap-1 shrink-0">
            {!finalized && (
              <>
                <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit}>
                  <Pencil className="w-3.5 h-3.5" />
                </Button>
                {item.object_type === 'system' && onLinkAsset && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    title={t('protectionNeeds.linkAsset')}
                    onClick={onLinkAsset}
                  >
                    <Link2 className="w-3.5 h-3.5" />
                  </Button>
                )}
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  title={t('protectionNeeds.finalize')}
                  onClick={onFinalize}
                >
                  <Lock className="w-3.5 h-3.5" />
                </Button>
              </>
            )}
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

        <div className="grid grid-cols-4 gap-2 text-xs">
          <div className="space-y-1 text-center">
            <p className="text-muted-foreground text-[10px] uppercase tracking-wide">C</p>
            <Badge className={LEVEL_CLASS[item.confidentiality]} variant="outline">
              {item.confidentiality}
            </Badge>
          </div>
          <div className="space-y-1 text-center">
            <p className="text-muted-foreground text-[10px] uppercase tracking-wide">I</p>
            <Badge className={LEVEL_CLASS[item.integrity]} variant="outline">
              {item.integrity}
            </Badge>
          </div>
          <div className="space-y-1 text-center">
            <p className="text-muted-foreground text-[10px] uppercase tracking-wide">A</p>
            <Badge className={LEVEL_CLASS[item.availability]} variant="outline">
              {item.availability}
            </Badge>
          </div>
          <div className="space-y-1 text-center">
            <p className="text-muted-foreground text-[10px] uppercase tracking-wide">{t('protectionNeeds.overall')}</p>
            <Badge className={LEVEL_CLASS[item.overall]} variant="outline">
              {item.overall}
            </Badge>
          </div>
        </div>

        {item.vb_asset_id && (
          <p className="text-xs text-muted-foreground flex items-center gap-1">
            <Link2 className="w-3 h-3 text-blue-400" />
            <span className="text-blue-400">{t('protectionNeeds.assetLinked')}</span>
          </p>
        )}
        {finalized && (
          <p className="text-xs text-muted-foreground flex items-center gap-1">
            <Lock className="w-3 h-3" />
            {t('protectionNeeds.finalizedAt', { date: item.finalized_at?.slice(0, 10) ?? '' })}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ProtectionNeedsPage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateProtectionNeedInput>(emptyForm())

  const [linkDialogPnaId, setLinkDialogPnaId] = useState<string | null>(null)

  const { data: items = [], isLoading, isError } = useProtectionNeeds()
  const createItem = useCreateProtectionNeed()
  const updateItem = useUpdateProtectionNeed(editId ?? '')
  const finalizeItem = useFinalizeProtectionNeed()
  const deleteItem = useDeleteProtectionNeed()
  const linkAsset = useLinkAssetToPNA()

  const { data: assetsResp } = useQuery<{ data: AssetOption[] } | AssetOption[]>({
    queryKey: ['vaktscan', 'assets', 'picker'],
    queryFn: () => apiFetch<{ data: AssetOption[] } | AssetOption[]>('/vaktscan/assets'),
    staleTime: 60_000,
    enabled: linkDialogPnaId != null,
  })
  const assetList: AssetOption[] = Array.isArray(assetsResp)
    ? assetsResp
    : ((assetsResp as { data?: AssetOption[] })?.data ?? [])

  function openCreate() {
    setEditId(null)
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function openEdit(item: ProtectionNeedAssessment) {
    setEditId(item.id)
    setForm(assessmentToForm(item))
    setDialogOpen(true)
  }

  function handleFinalize(id: string) {
    if (confirm(t('protectionNeeds.finalizeConfirm'))) {
      finalizeItem.mutate(id)
    }
  }

  function handleDelete(id: string) {
    if (confirm(t('protectionNeeds.deleteConfirm'))) {
      deleteItem.mutate(id)
    }
  }

  function handleSubmit() {
    if (editId) {
      updateItem.mutate(form, { onSuccess: () => { setDialogOpen(false); } })
    } else {
      createItem.mutate(form, { onSuccess: () => { setDialogOpen(false); } })
    }
  }

  const isPending = createItem.isPending || updateItem.isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('protectionNeeds.title')}
        description={t('protectionNeeds.description')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            {t('protectionNeeds.new')}
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
            {t('protectionNeeds.loadError')}
          </div>
        )}
        {!isLoading && !isError && items.length === 0 && (
          <EmptyState
            icon={Shield}
            title={t('protectionNeeds.emptyTitle')}
            description={t('protectionNeeds.emptyDescription')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('protectionNeeds.new')}
              </Button>
            }
          />
        )}
        {!isLoading && !isError && items.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {items.map((item) => (
              <ProtectionNeedCard
                key={item.id}
                item={item}
                onEdit={() => { openEdit(item); }}
                onFinalize={() => { handleFinalize(item.id); }}
                onDelete={() => { handleDelete(item.id); }}
                onLinkAsset={item.object_type === 'system' ? () => { setLinkDialogPnaId(item.id); } : undefined}
              />
            ))}
          </div>
        )}
      </div>

      {/* Asset-Link Dialog */}
      <Dialog open={linkDialogPnaId != null} onOpenChange={(o) => { if (!o) setLinkDialogPnaId(null); }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('protectionNeeds.linkAsset')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2 max-h-64 overflow-y-auto">
            {assetList.length === 0 && (
              <p className="text-sm text-muted-foreground">{t('protectionNeeds.noAssetsFound')}</p>
            )}
            {assetList.map((a) => (
              <button
                key={a.id}
                type="button"
                className="w-full flex items-center justify-between px-3 py-2 rounded-lg border border-border hover:bg-surface2 text-left"
                onClick={() => {
                  if (linkDialogPnaId) {
                    linkAsset.mutate(
                      { pnaId: linkDialogPnaId, assetId: a.id },
                      { onSuccess: () => { setLinkDialogPnaId(null); } },
                    )
                  }
                }}
              >
                <span className="text-sm font-medium">{a.name}</span>
                <span className="text-xs text-muted-foreground">{criticality_de[a.criticality] ?? a.criticality}</span>
              </button>
            ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setLinkDialogPnaId(null); }}>{t('common.cancel')}</Button>
            {linkDialogPnaId && items.find((i) => i.id === linkDialogPnaId)?.vb_asset_id && (
              <Button
                variant="destructive"
                onClick={() => {
                  linkAsset.mutate(
                    { pnaId: linkDialogPnaId, assetId: null },
                    { onSuccess: () => { setLinkDialogPnaId(null); } },
                  )
                }}
              >
                {t('protectionNeeds.unlinkAsset')}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editId ? t('protectionNeeds.edit') : t('protectionNeeds.new')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('protectionNeeds.name')} *</Label>
              <Input
                placeholder={t('protectionNeeds.namePlaceholder')}
                value={form.name}
                onChange={(e) => { setForm((f) => ({ ...f, name: e.target.value })); }}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('protectionNeeds.objectTypeLabel')} *</Label>
                <Select
                  value={form.object_type}
                  onValueChange={(v) => { setForm((f) => ({ ...f, object_type: v as ProtectionObjectType })); }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {OBJECT_TYPES.map((ot) => (
                      <SelectItem key={ot} value={ot}>
                        {t(`protectionNeeds.objectType.${ot}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('protectionNeeds.objectName')} *</Label>
                <Input
                  placeholder={t('protectionNeeds.objectNamePlaceholder')}
                  value={form.object_name}
                  onChange={(e) => { setForm((f) => ({ ...f, object_name: e.target.value })); }}
                />
              </div>
            </div>
            <div className="space-y-2">
              <p className="text-sm font-medium">{t('protectionNeeds.levels')}</p>
              <div className="grid grid-cols-3 gap-3">
                {(['confidentiality', 'integrity', 'availability'] as const).map((dim) => (
                  <div key={dim} className="space-y-1.5">
                    <Label className="text-xs"><TermTooltip term="Schutzbedarf" glossaryKey="Schutzbedarf">{t(`protectionNeeds.dim.${dim}`)}</TermTooltip></Label>
                    <Select
                      value={form[dim]}
                      onValueChange={(v) => { setForm((f) => ({ ...f, [dim]: v as ProtectionLevel })); }}
                    >
                      <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        {LEVELS.map((l) => (
                          <SelectItem key={l} value={l}>
                            {t(`protectionNeeds.level.${l}`)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">{t('protectionNeeds.overallHint')}</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>
              {t('common.cancel')}
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={!form.name || !form.object_name || isPending}
            >
              {isPending ? t('common.saving') : editId ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
