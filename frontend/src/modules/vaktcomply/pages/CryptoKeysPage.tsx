import { useState } from 'react'
import { Key, Plus, RotateCcw, Trash2, AlertTriangle } from 'lucide-react'
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
import { useCryptoKeys, useCreateCryptoKey, useRotateCryptoKey, useDeleteCryptoKey } from '../hooks/useCryptoKeys'
import { toast } from '../../../shared/hooks/useToast'
import type { CryptoKey, CreateCryptoKeyInput, CryptoKeyType, RotationStatus } from '../types'

const ROTATION_STATUS_CLASS: Record<RotationStatus, string> = {
  ok: 'bg-green-500/20 text-green-400 border-green-500/30',
  due_soon: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  overdue: 'bg-red-500/20 text-red-400 border-red-500/30',
  none: 'bg-secondary text-secondary-foreground',
}

const ROTATION_STATUS_LABELS: Record<RotationStatus, string> = {
  ok: 'Aktuell',
  due_soon: 'Bald fällig',
  overdue: 'Überfällig',
  none: 'Kein Intervall',
}

const KEY_TYPE_LABELS: Record<CryptoKeyType, string> = {
  symmetric: 'Symmetrisch',
  asymmetric: 'Asymmetrisch',
  certificate: 'Zertifikat',
  hmac: 'HMAC',
  signing: 'Signatur',
  other: 'Sonstige',
}

function emptyForm(): CreateCryptoKeyInput {
  return {
    name: '',
    key_type: 'symmetric',
    algorithm: '',
    purpose: '',
    key_length: undefined,
    location: '',
    rotation_interval_days: undefined,
    last_rotation_date: '',
    expiry_date: '',
    notes: '',
  }
}

function RotateDialog({
  keyItem,
  onClose,
}: {
  keyItem: CryptoKey
  onClose: () => void
}) {
  const rotate = useRotateCryptoKey(keyItem.id)
  const [rotatedAt, setRotatedAt] = useState(new Date().toISOString().slice(0, 10))
  const [notes, setNotes] = useState('')

  function handleRotate() {
    rotate.mutate(
      { rotated_at: rotatedAt, rotation_interval_days: keyItem.rotation_interval_days ?? undefined, notes },
      {
        onSuccess: () => {
          toast(`${keyItem.name} wurde erfolgreich rotiert.`)
          onClose()
        },
        onError: (err) => {
          toast(err.message, 'error')
        },
      },
    )
  }

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Schlüssel rotieren: {keyItem.name}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-1">
            <Label>Rotations-Datum</Label>
            <Input type="date" value={rotatedAt} onChange={(e) => { setRotatedAt(e.target.value); }} />
          </div>
          <div className="space-y-1">
            <Label>Notizen (optional)</Label>
            <Textarea rows={2} value={notes} onChange={(e) => { setNotes(e.target.value); }} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Abbrechen</Button>
          <Button onClick={handleRotate} disabled={rotate.isPending}>
            {rotate.isPending ? 'Rotiere …' : 'Rotation bestätigen'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function CryptoKeysPage() {
  const { data: keys, isLoading } = useCryptoKeys()
  const createKey = useCreateCryptoKey()
  const deleteKey = useDeleteCryptoKey()

  const [createOpen, setCreateOpen] = useState(false)
  const [rotateTarget, setRotateTarget] = useState<CryptoKey | null>(null)
  const [form, setForm] = useState<CreateCryptoKeyInput>(emptyForm())

  function set<K extends keyof CreateCryptoKeyInput>(k: K, v: CreateCryptoKeyInput[K]) {
    setForm((f) => ({ ...f, [k]: v }))
  }

  function handleCreate() {
    createKey.mutate(form, {
      onSuccess: () => {
        toast('Schlüssel angelegt')
        setCreateOpen(false)
        setForm(emptyForm())
      },
      onError: (err) => {
        toast(err.message, 'error')
      },
    })
  }

  function handleDelete(k: CryptoKey) {
    if (!confirm(`Schlüssel "${k.name}" wirklich löschen?`)) return
    deleteKey.mutate(k.id, {
      onSuccess: () => { toast('Schlüssel gelöscht') },
      onError: (err) => { toast(err.message, 'error') },
    })
  }

  const weakCount = keys?.filter((k) => k.is_weak_algorithm).length ?? 0
  const overdueCount = keys?.filter((k) => k.rotation_status === 'overdue').length ?? 0

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Kryptographie-Schlüssel-Register"
        description="ISO 27001 A.8.24 — Dokumentation kryptographischer Schlüssel, Algorithmen und Rotations-Nachweise."
        actions={
          <Button onClick={() => { setCreateOpen(true); }} data-testid="add-key-btn">
            <Plus className="w-4 h-4 mr-1" />
            Schlüssel anlegen
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-4">
        {(weakCount > 0 || overdueCount > 0) && (
          <Card className="border-amber-500/30 bg-amber-500/5">
            <CardContent className="py-3 flex items-center gap-3 text-sm">
              <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0" />
              <span>
                {weakCount > 0 && (
                  <span className="text-amber-400 font-medium mr-2">
                    {weakCount} schwache{weakCount === 1 ? 'r' : ''} Algorithmus — Migration empfohlen.
                  </span>
                )}
                {overdueCount > 0 && (
                  <span className="text-red-400 font-medium">
                    {overdueCount} Rotation{overdueCount === 1 ? '' : 'en'} überfällig.
                  </span>
                )}
              </span>
            </CardContent>
          </Card>
        )}

        {isLoading && (
          <div className="flex items-center justify-center h-32">
            <Spinner size="md" color="primary" />
          </div>
        )}

        {!isLoading && (!keys || keys.length === 0) && (
          <EmptyState
            icon={Key}
            title="Keine Schlüssel eingetragen"
            description="Fügen Sie kryptographische Schlüssel, Zertifikate und Algorithmen hinzu, um ISO 27001 A.8.24 nachzuweisen."
            action={
              <Button size="sm" onClick={() => { setCreateOpen(true); }}>
                <Plus className="w-4 h-4 mr-1" />
                Ersten Schlüssel anlegen
              </Button>
            }
          />
        )}

        {keys && keys.length > 0 && (
          <div className="rounded-md border border-border overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Typ</TableHead>
                  <TableHead>Algorithmus</TableHead>
                  <TableHead>Verwendungszweck</TableHead>
                  <TableHead>Letzte Rotation</TableHead>
                  <TableHead>Nächste Rotation</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-20" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {keys.map((k) => (
                  <TableRow key={k.id} data-testid={`key-row-${k.id}`}>
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-1.5">
                        {k.is_weak_algorithm && (
                          <AlertTriangle className="w-3.5 h-3.5 text-amber-400 shrink-0" aria-label="Schwacher Algorithmus" />
                        )}
                        {k.name}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs">
                        {KEY_TYPE_LABELS[k.key_type] ?? k.key_type}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {k.algorithm}
                      {k.key_length && <span className="text-muted-foreground ml-1">({k.key_length} bit)</span>}
                    </TableCell>
                    <TableCell className="text-sm">{k.purpose}</TableCell>
                    <TableCell className="text-sm">{k.last_rotation_date ?? '—'}</TableCell>
                    <TableCell className="text-sm">{k.next_rotation_due ?? '—'}</TableCell>
                    <TableCell>
                      <Badge className={`text-xs ${ROTATION_STATUS_CLASS[k.rotation_status]}`}>
                        {ROTATION_STATUS_LABELS[k.rotation_status]}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-7 w-7"
                          onClick={() => { setRotateTarget(k); }}
                          title="Rotation dokumentieren"
                          data-testid={`rotate-btn-${k.id}`}
                        >
                          <RotateCcw className="w-3.5 h-3.5" />
                        </Button>
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-7 w-7 text-destructive hover:text-destructive"
                          onClick={() => { handleDelete(k); }}
                          data-testid={`delete-btn-${k.id}`}
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Kryptographischen Schlüssel anlegen</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 max-h-[60vh] overflow-y-auto pr-1">
            <div className="space-y-1">
              <Label>Name *</Label>
              <Input value={form.name} onChange={(e) => { set('name', e.target.value); }} placeholder="z.B. DB-Verschlüsselungs-Key" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Typ *</Label>
                <Select value={form.key_type} onValueChange={(v) => { set('key_type', v as CryptoKeyType); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {(Object.entries(KEY_TYPE_LABELS) as [CryptoKeyType, string][]).map(([v, label]) => (
                      <SelectItem key={v} value={v}>{label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <Label>Algorithmus *</Label>
                <Input value={form.algorithm} onChange={(e) => { set('algorithm', e.target.value); }} placeholder="z.B. AES-256-GCM" />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Schlüssellänge (Bit)</Label>
                <Input
                  type="number"
                  value={form.key_length ?? ''}
                  onChange={(e) => { set('key_length', e.target.value ? Number(e.target.value) : undefined); }}
                  placeholder="z.B. 256"
                />
              </div>
              <div className="space-y-1">
                <Label>Rotations-Intervall (Tage)</Label>
                <Input
                  type="number"
                  value={form.rotation_interval_days ?? ''}
                  onChange={(e) => { set('rotation_interval_days', e.target.value ? Number(e.target.value) : undefined); }}
                  placeholder="z.B. 365"
                />
              </div>
            </div>
            <div className="space-y-1">
              <Label>Verwendungszweck *</Label>
              <Input value={form.purpose} onChange={(e) => { set('purpose', e.target.value); }} placeholder="z.B. TLS-Termination" />
            </div>
            <div className="space-y-1">
              <Label>Speicherort</Label>
              <Input value={form.location ?? ''} onChange={(e) => { set('location', e.target.value); }} placeholder="z.B. AWS KMS, on-prem HSM" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Letzte Rotation</Label>
                <Input type="date" value={form.last_rotation_date ?? ''} onChange={(e) => { set('last_rotation_date', e.target.value); }} />
              </div>
              <div className="space-y-1">
                <Label>Ablaufdatum (Zertifikat)</Label>
                <Input type="date" value={form.expiry_date ?? ''} onChange={(e) => { set('expiry_date', e.target.value); }} />
              </div>
            </div>
            <div className="space-y-1">
              <Label>Notizen</Label>
              <Textarea rows={2} value={form.notes ?? ''} onChange={(e) => { set('notes', e.target.value); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCreateOpen(false); }}>Abbrechen</Button>
            <Button
              onClick={handleCreate}
              disabled={createKey.isPending || !form.name || !form.algorithm || !form.purpose}
              data-testid="create-key-submit"
            >
              {createKey.isPending ? 'Anlegen …' : 'Schlüssel anlegen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {rotateTarget && (
        <RotateDialog keyItem={rotateTarget} onClose={() => { setRotateTarget(null); }} />
      )}
    </div>
  )
}
