import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Phone, Plus, Pencil, Trash2, CheckCircle2 } from 'lucide-react'
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
import { Switch } from '../../../components/ui/switch'
import {
  useEmergencyContacts,
  useCreateEmergencyContact,
  useUpdateEmergencyContact,
  useDeleteEmergencyContact,
} from '../hooks/useEmergencyContacts'
import type { EmergencyContact, CreateEmergencyContactInput } from '../types'

const LEVEL_CLASS: Record<1 | 2 | 3, string> = {
  1: 'bg-red-500/20 text-red-400 border-red-500/30',
  2: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  3: 'bg-secondary text-secondary-foreground',
}

function emptyForm(): CreateEmergencyContactInput {
  return {
    name: '',
    role: '',
    phone: '',
    email: '',
    escalation_level: 2,
    available_247: false,
    notes: '',
  }
}

function contactToForm(c: EmergencyContact): CreateEmergencyContactInput {
  return {
    name: c.name,
    role: c.role,
    phone: c.phone,
    email: c.email,
    escalation_level: c.escalation_level,
    available_247: c.available_247,
    notes: c.notes,
  }
}

function ContactCard({
  contact,
  onEdit,
  onDelete,
}: {
  contact: EmergencyContact
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1 flex-1 min-w-0">
            <CardTitle className="text-sm leading-tight">{contact.name}</CardTitle>
            <div className="flex items-center gap-1.5 flex-wrap">
              <Badge className={LEVEL_CLASS[contact.escalation_level]} variant="outline">
                {t('bcm.emergencyContacts.level')} {contact.escalation_level}
              </Badge>
              {contact.role && (
                <span className="text-xs text-muted-foreground">{contact.role}</span>
              )}
              {contact.available_247 && (
                <span className="flex items-center gap-0.5 text-xs text-green-400">
                  <CheckCircle2 className="w-3 h-3" />
                  24/7
                </span>
              )}
            </div>
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
      <CardContent className="pt-0 space-y-1">
        {contact.phone && (
          <a
            href={`tel:${contact.phone}`}
            className="text-xs text-primary hover:underline flex items-center gap-1"
          >
            <Phone className="w-3 h-3" />
            {contact.phone}
          </a>
        )}
        {contact.email && (
          <p className="text-xs text-muted-foreground">{contact.email}</p>
        )}
        {contact.notes && (
          <p className="text-xs text-muted-foreground line-clamp-2">{contact.notes}</p>
        )}
      </CardContent>
    </Card>
  )
}

export default function EmergencyContactsPage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateEmergencyContactInput>(emptyForm())

  const { data: contacts = [], isLoading, isError } = useEmergencyContacts()
  const create = useCreateEmergencyContact()
  const update = useUpdateEmergencyContact(editId ?? '')
  const del = useDeleteEmergencyContact()

  function openCreate() {
    setEditId(null)
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function openEdit(c: EmergencyContact) {
    setEditId(c.id)
    setForm(contactToForm(c))
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm(t('bcm.emergencyContacts.deleteConfirm'))) {
      del.mutate(id)
    }
  }

  function handleSubmit() {
    if (editId) {
      update.mutate(form, { onSuccess: () => { setDialogOpen(false) } })
    } else {
      create.mutate(form, { onSuccess: () => { setDialogOpen(false) } })
    }
  }

  const isPending = create.isPending || update.isPending

  // Group by escalation level
  const byLevel = [1, 2, 3] as const
  const grouped = byLevel.map((level) => ({
    level,
    contacts: contacts.filter((c) => c.escalation_level === level),
  }))

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('bcm.emergencyContacts.title')}
        description={t('bcm.emergencyContacts.description')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            {t('bcm.emergencyContacts.new')}
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
            {t('bcm.emergencyContacts.loadError')}
          </div>
        )}
        {!isLoading && !isError && contacts.length === 0 && (
          <EmptyState
            icon={Phone}
            title={t('bcm.emergencyContacts.emptyTitle')}
            description={t('bcm.emergencyContacts.emptyDescription')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('bcm.emergencyContacts.new')}
              </Button>
            }
          />
        )}
        {!isLoading && !isError && contacts.length > 0 && (
          <div className="space-y-6">
            {grouped.map(({ level, contacts: lvlContacts }) => lvlContacts.length > 0 && (
              <div key={level}>
                <h3 className="text-sm font-semibold mb-3 flex items-center gap-2">
                  <Badge className={LEVEL_CLASS[level]} variant="outline">
                    {t('bcm.emergencyContacts.level')} {level}
                  </Badge>
                  <span className="text-muted-foreground font-normal">
                    {t(`bcm.emergencyContacts.levelDesc.${level}`)}
                  </span>
                </h3>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {lvlContacts.map((c) => (
                    <ContactCard
                      key={c.id}
                      contact={c}
                      onEdit={() => { openEdit(c) }}
                      onDelete={() => { handleDelete(c.id) }}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {editId ? t('bcm.emergencyContacts.edit') : t('bcm.emergencyContacts.new')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>{t('bcm.emergencyContacts.name')} *</Label>
              <Input
                value={form.name}
                onChange={(e) => { setForm((f) => ({ ...f, name: e.target.value })) }}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('bcm.emergencyContacts.role')}</Label>
                <Input
                  value={form.role ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, role: e.target.value })) }}
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('bcm.emergencyContacts.escalationLevel')}</Label>
                <Select
                  value={String(form.escalation_level ?? 2)}
                  onValueChange={(v) => {
                    setForm((f) => ({ ...f, escalation_level: Number(v) as 1 | 2 | 3 }))
                  }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1">{t('bcm.emergencyContacts.levelDesc.1')}</SelectItem>
                    <SelectItem value="2">{t('bcm.emergencyContacts.levelDesc.2')}</SelectItem>
                    <SelectItem value="3">{t('bcm.emergencyContacts.levelDesc.3')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('bcm.emergencyContacts.phone')}</Label>
                <Input
                  type="tel"
                  value={form.phone ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, phone: e.target.value })) }}
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('bcm.emergencyContacts.email')}</Label>
                <Input
                  type="email"
                  value={form.email ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, email: e.target.value })) }}
                />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Switch
                id="available247"
                checked={form.available_247 ?? false}
                onCheckedChange={(v) => { setForm((f) => ({ ...f, available_247: v })) }}
              />
              <Label htmlFor="available247">{t('bcm.emergencyContacts.available247')}</Label>
            </div>
            <div className="space-y-1.5">
              <Label>{t('bcm.emergencyContacts.notes')}</Label>
              <Textarea
                rows={2}
                value={form.notes ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, notes: e.target.value })) }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false) }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={!form.name || isPending}>
              {isPending ? t('common.saving') : editId ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
