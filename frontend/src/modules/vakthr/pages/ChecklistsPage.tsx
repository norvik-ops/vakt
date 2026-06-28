import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2, ClipboardList, ChevronDown, ChevronRight } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { Spinner } from '../../../components/Spinner'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
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
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { useDeferredDelete } from '../../../shared/hooks/useDeferredDelete'
import { useChecklists, useCreateChecklist, useDeleteChecklist } from '../hooks/useHR'
import type { Checklist, ChecklistItem, CreateChecklistInput } from '../types'

function TypeBadge({ type }: { type: Checklist['type'] }) {
  const { t } = useTranslation()
  return type === 'onboarding' ? (
    <Badge className="bg-blue-500/20 text-blue-400 border-blue-500/30">{t('vakthr.checklistsPage.typeOnboarding')}</Badge>
  ) : (
    <Badge className="bg-purple-500/20 text-purple-400 border-purple-500/30">{t('vakthr.checklistsPage.typeOffboarding')}</Badge>
  )
}

function generateId() {
  return Math.random().toString(36).slice(2, 10)
}

interface FormState {
  type: 'onboarding' | 'offboarding'
  name: string
  items: ChecklistItem[]
}

function emptyForm(): FormState {
  return { type: 'onboarding', name: '', items: [] }
}

function ChecklistCard({ checklist, onDelete }: { checklist: Checklist; onDelete: () => void }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <Card>
      <CardHeader className="py-3 px-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3 min-w-0">
            <button
              onClick={() => { setExpanded((x) => !x); }}
              className="text-secondary hover:text-primary transition-colors shrink-0"
            >
              {expanded ? (
                <ChevronDown className="w-4 h-4" />
              ) : (
                <ChevronRight className="w-4 h-4" />
              )}
            </button>
            <CardTitle className="text-sm font-medium truncate">{checklist.name}</CardTitle>
            <TypeBadge type={checklist.type} />
            <span className="text-xs text-secondary shrink-0">
              {checklist.items.length} Schritt{checklist.items.length !== 1 ? 'e' : ''}
            </span>
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={onDelete}
            className="text-red-500 hover:text-red-600 shrink-0"
          >
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </CardHeader>
      {expanded && checklist.items.length > 0 && (
        <CardContent className="pt-0 pb-3 px-4">
          <ul className="space-y-1 pl-7">
            {checklist.items.map((item) => (
              <li key={item.id} className="flex items-center gap-2 text-sm text-secondary">
                <span className="w-1.5 h-1.5 rounded-full bg-border shrink-0" />
                <span>{item.label}</span>
                {item.required && (
                  <Badge variant="outline" className="text-[10px] py-0 px-1 h-4">
                    Pflicht
                  </Badge>
                )}
              </li>
            ))}
          </ul>
        </CardContent>
      )}
      {expanded && checklist.items.length === 0 && (
        <CardContent className="pt-0 pb-3 px-4 pl-11">
          <p className="text-xs text-secondary italic">Keine Schritte definiert.</p>
        </CardContent>
      )}
    </Card>
  )
}

export default function ChecklistsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data: checklists = [], isLoading } = useChecklists()
  const createChecklist = useCreateChecklist()
  const deleteChecklist = useDeleteChecklist()

  const [dialogOpen, setDialogOpen] = useState(false)
  const [hiddenIds, setHiddenIds] = useState<Set<string>>(new Set())
  const [form, setForm] = useState<FormState>(emptyForm())
  const [newItemLabel, setNewItemLabel] = useState('')
  const [newItemRequired, setNewItemRequired] = useState(false)

  const { scheduleDelete } = useDeferredDelete<Checklist>({
    onDelete: (cl) => deleteChecklist.mutateAsync(cl.id),
    onUndo: () => void queryClient.invalidateQueries({ queryKey: ['vakthr', 'checklists'] }),
    getLabel: (cl) => cl.name,
  })

  function openCreate() {
    setForm(emptyForm())
    setNewItemLabel('')
    setNewItemRequired(false)
    setDialogOpen(true)
  }

  function addItem() {
    const label = newItemLabel.trim()
    if (!label) return
    setForm((f) => ({
      ...f,
      items: [...f.items, { id: generateId(), label, required: newItemRequired }],
    }))
    setNewItemLabel('')
    setNewItemRequired(false)
  }

  function removeItem(id: string) {
    setForm((f) => ({ ...f, items: f.items.filter((i) => i.id !== id) }))
  }

  async function handleSubmit() {
    const input: CreateChecklistInput = {
      type: form.type,
      name: form.name,
      items: form.items,
    }
    await createChecklist.mutateAsync(input)
    setDialogOpen(false)
  }

  function handleDelete(checklist: Checklist) {
    scheduleDelete(checklist, () => { setHiddenIds((prev) => new Set(prev).add(checklist.id)) })
  }

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="Checklisten"
        description="Onboarding- und Offboarding-Checklisten verwalten"
        actions={
          <Button onClick={openCreate} size="sm">
            <Plus className="w-4 h-4 mr-2" />
            Checkliste erstellen
          </Button>
        }
      />

      {isLoading && (
        <div className="flex justify-center py-12">
          <Spinner size="lg" />
        </div>
      )}

      {!isLoading && checklists.length === 0 && (
        <EmptyState
          icon={ClipboardList}
          title="Noch keine Checklisten"
          description="Erstellen Sie die erste Onboarding- oder Offboarding-Checkliste, um Mitarbeiter-Lifecycle strukturiert zu dokumentieren."
          action={
            <Button onClick={openCreate} size="sm">
              <Plus className="w-4 h-4 mr-2" />
              Checkliste erstellen
            </Button>
          }
        />
      )}

      {!isLoading && checklists.length > 0 && (
        <div className="space-y-3">
          {checklists.filter((c) => !hiddenIds.has(c.id)).map((c) => (
            <ChecklistCard
              key={c.id}
              checklist={c}
              onDelete={() => { handleDelete(c); }}
            />
          ))}
        </div>
      )}

      {/* Create Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Neue Checkliste</DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-1">
              <Label>Typ *</Label>
              <Select
                value={form.type}
                onValueChange={(v) => { setForm((f) => ({ ...f, type: v as FormState['type'] })); }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="onboarding">{t('vakthr.checklistsPage.typeOnboarding')}</SelectItem>
                  <SelectItem value="offboarding">{t('vakthr.checklistsPage.typeOffboarding')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Name *</Label>
              <Input
                value={form.name}
                onChange={(e) => { setForm((f) => ({ ...f, name: e.target.value })); }}
                placeholder="Standard-Onboarding IT"
              />
            </div>

            {/* Items */}
            <div className="space-y-2">
              <Label>{t('vakthr.checklistsPage.labelSteps')}</Label>
              {form.items.length > 0 && (
                <ul className="space-y-1 mb-2">
                  {form.items.map((item) => (
                    <li
                      key={item.id}
                      className="flex items-center justify-between gap-2 text-sm bg-surface rounded px-3 py-2"
                    >
                      <span className="flex-1 truncate">{item.label}</span>
                      {item.required && (
                        <Badge variant="outline" className="text-[10px] py-0 px-1 h-4 shrink-0">
                          Pflicht
                        </Badge>
                      )}
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-5 w-5 text-red-400 shrink-0"
                        onClick={() => { removeItem(item.id); }}
                      >
                        <Trash2 className="w-3 h-3" />
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
              <div className="flex gap-2">
                <Input
                  value={newItemLabel}
                  onChange={(e) => { setNewItemLabel(e.target.value); }}
                  onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addItem() } }}
                  placeholder="Schritt beschreiben..."
                  className="flex-1"
                />
                <label className="flex items-center gap-1.5 text-sm text-secondary cursor-pointer shrink-0">
                  <input
                    type="checkbox"
                    checked={newItemRequired}
                    onChange={(e) => { setNewItemRequired(e.target.checked); }}
                    className="rounded"
                  />
                  Pflicht
                </label>
                <Button type="button" variant="outline" size="sm" onClick={addItem}>
                  <Plus className="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>
              Abbrechen
            </Button>
            <Button
              onClick={() => void handleSubmit()}
              disabled={!form.name || createChecklist.isPending}
            >
              {createChecklist.isPending ? 'Speichern...' : 'Erstellen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

    </div>
  )
}
