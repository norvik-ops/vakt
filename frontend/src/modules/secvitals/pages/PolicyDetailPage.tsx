import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Save } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { usePolicy, useUpdatePolicy } from '../hooks/usePolicies'
import PolicyVersionHistory from '../components/PolicyVersionHistory'
import type { Policy, UpdatePolicyInput } from '../types'

const STATUS_LABELS: Record<Policy['status'], string> = {
  draft: 'Entwurf', active: 'Aktiv', archived: 'Archiviert',
}

function toDateInput(ts?: string): string {
  if (!ts) return ''
  return ts.slice(0, 10)
}

export default function PolicyDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data: policy, isLoading, isError } = usePolicy(id ?? '')
  const update = useUpdatePolicy(id ?? '')

  const [form, setForm] = useState<UpdatePolicyInput | null>(null)
  const [dirty, setDirty] = useState(false)

  useEffect(() => {
    if (policy && !form) {
      setForm({
        title: policy.title,
        description: policy.description ?? '',
        category: policy.category ?? '',
        status: policy.status,
        version: policy.version,
        effective_date: toDateInput(policy.effective_date),
        review_date: toDateInput(policy.review_date),
        owner: policy.owner ?? '',
        version_note: '',
        updated_by: policy.last_updated_by ?? '',
        next_review_due: toDateInput(policy.next_review_due),
      })
    }
  }, [policy, form])

  function set<K extends keyof UpdatePolicyInput>(key: K, value: UpdatePolicyInput[K]) {
    setForm((f) => f ? { ...f, [key]: value } : f)
    setDirty(true)
  }

  function handleSave() {
    if (!form) return
    const payload: UpdatePolicyInput = {
      ...form,
      effective_date: form.effective_date || undefined,
      review_date: form.review_date || undefined,
      next_review_due: form.next_review_due || undefined,
      version_note: form.version_note || undefined,
      updated_by: form.updated_by || undefined,
    }
    update.mutate(payload, {
      onSuccess: () => {
        setDirty(false)
        // Reset version_note after save so it's blank for the next change
        setForm((f) => f ? { ...f, version_note: '' } : f)
      },
    })
  }

  if (isLoading) return (
    <div className="flex items-center justify-center h-48">
      <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
    </div>
  )
  if (isError || !policy) return (
    <div className="p-6 text-sm text-red-400">Richtlinie nicht gefunden.</div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={policy.title}
        description={`v${policy.version} · Revision ${policy.version_num}${policy.category ? ` · ${policy.category}` : ''}`}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => navigate('/secvitals/policies')}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              Zurück
            </Button>
            <Button onClick={handleSave} disabled={!dirty || update.isPending}>
              <Save className="w-4 h-4 mr-1" />
              {update.isPending ? 'Speichern …' : 'Speichern'}
            </Button>
          </div>
        }
      />

      {form && (
        <div className="flex-1 p-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">Richtlinieninhalt</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>Titel</Label>
                  <Input value={form.title} onChange={(e) => set('title', e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <Label>Kategorie</Label>
                  <Input value={form.category ?? ''} placeholder="z.B. IT-Sicherheit, Datenschutz" onChange={(e) => set('category', e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <Label>Beschreibung / Zweck</Label>
                  <Textarea rows={5} value={form.description ?? ''} onChange={(e) => set('description', e.target.value)} />
                </div>
              </CardContent>
            </Card>

            {/* Versioning fields — shown when editing */}
            <Card>
              <CardHeader><CardTitle className="text-sm">Änderungsdokumentation</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="pol-version-note">
                    Änderungsnotiz
                    <span className="text-muted-foreground font-normal ml-1">(wird beim Speichern in der Historie gespeichert)</span>
                  </Label>
                  <Textarea
                    id="pol-version-note"
                    rows={2}
                    placeholder="Kurze Beschreibung der Änderungen, z.B. 'Abschnitt 3 überarbeitet – neue BSI-Anforderungen'"
                    value={form.version_note ?? ''}
                    onChange={(e) => set('version_note', e.target.value)}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <Label htmlFor="pol-updated-by">Geändert von</Label>
                    <Input
                      id="pol-updated-by"
                      placeholder="z.B. CISO oder E-Mail"
                      value={form.updated_by ?? ''}
                      onChange={(e) => set('updated_by', e.target.value)}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="pol-next-review">Nächste Überprüfung</Label>
                    <Input
                      id="pol-next-review"
                      type="date"
                      value={form.next_review_due ?? ''}
                      onChange={(e) => set('next_review_due', e.target.value)}
                    />
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Version history — always visible */}
            <PolicyVersionHistory policyId={policy.id} currentVersion={policy.version_num} />
          </div>

          <div className="space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">Metadaten</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>Status</Label>
                  <Select value={form.status} onValueChange={(v) => set('status', v as Policy['status'])}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {(Object.keys(STATUS_LABELS) as Policy['status'][]).map((k) => (
                        <SelectItem key={k} value={k}>{STATUS_LABELS[k]}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>Versionsbezeichnung</Label>
                  <Input value={form.version ?? ''} onChange={(e) => set('version', e.target.value)} placeholder="z.B. 1.0, 2.1" />
                </div>
                <div className="space-y-1.5">
                  <Label>Verantwortlicher</Label>
                  <Input value={form.owner ?? ''} onChange={(e) => set('owner', e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <Label>Gültig ab</Label>
                  <Input type="date" value={form.effective_date ?? ''} onChange={(e) => set('effective_date', e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <Label>Review-Datum</Label>
                  <Input type="date" value={form.review_date ?? ''} onChange={(e) => set('review_date', e.target.value)} />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="pt-4 space-y-1 text-xs text-muted-foreground">
                <p>Erstellt: {new Date(policy.created_at).toLocaleDateString('de-DE')}</p>
                <p>Geändert: {new Date(policy.updated_at).toLocaleDateString('de-DE')}</p>
                {policy.reviewed_at && (
                  <p>Zuletzt geprüft: {new Date(policy.reviewed_at).toLocaleDateString('de-DE')}</p>
                )}
                {policy.next_review_due && (
                  <p>Nächste Prüfung: {new Date(policy.next_review_due).toLocaleDateString('de-DE')}</p>
                )}
                {policy.last_updated_by && (
                  <p>Bearbeitet von: {policy.last_updated_by}</p>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      )}
    </div>
  )
}
