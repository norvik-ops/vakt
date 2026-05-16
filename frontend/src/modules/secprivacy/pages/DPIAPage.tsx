import { useState } from 'react'
import { ClipboardCheck, Plus, Pencil, Trash2, ShieldCheck, Download } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../../../components/ui/alert-dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { ProGate } from '../../../shared/components/ProGate'
import { useDPIAs, useCreateDPIA, useUpdateDPIA, useApproveDPIA, useDeleteDPIA, useExportDPIA } from '../hooks/useDPIAs'
import { useVVT } from '../hooks/useVVT'
import type { DPIA, CreateDPIAInput, UpdateDPIAInput } from '../types'

const STATUS_LABELS: Record<DPIA['status'], string> = {
  draft: 'Entwurf',
  in_review: 'In Prüfung',
  approved: 'Freigegeben',
}

const STATUS_CLASS: Record<DPIA['status'], string> = {
  draft: 'bg-secondary text-secondary-foreground',
  in_review: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  approved: 'bg-green-500/20 text-green-400 border-green-500/30',
}

interface DPIAFormState {
  title: string
  description: string
  necessity_assessment: string
  risk_assessment: string
  mitigation_measures: string
  residual_risk: string
  dpo_consultation: boolean
  vvt_entry_id?: string
}

function emptyForm(): DPIAFormState {
  return {
    title: '',
    description: '',
    necessity_assessment: '',
    risk_assessment: '',
    mitigation_measures: '',
    residual_risk: '',
    dpo_consultation: false,
    vvt_entry_id: undefined,
  }
}

function formFromEntry(d: DPIA): DPIAFormState {
  return {
    title: d.title,
    description: d.description ?? '',
    necessity_assessment: d.necessity_assessment ?? '',
    risk_assessment: d.risk_assessment ?? '',
    mitigation_measures: d.mitigation_measures ?? '',
    residual_risk: d.residual_risk ?? '',
    dpo_consultation: d.dpo_consultation,
    vvt_entry_id: d.vvt_entry_id,
  }
}

function DPIACard({
  dpia,
  onEdit,
  onDelete,
  onApprove,
}: {
  dpia: DPIA
  onEdit: (d: DPIA) => void
  onDelete: (id: string) => void
  onApprove: (id: string) => void
}) {
  const date = new Date(dpia.created_at).toLocaleDateString('de-DE', {
    year: 'numeric', month: 'short', day: 'numeric',
  })
  return (
    <Card>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm">{dpia.title}</p>
          <Badge className={STATUS_CLASS[dpia.status]}>{STATUS_LABELS[dpia.status]}</Badge>
        </div>
        {dpia.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{dpia.description}</p>
        )}
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          {dpia.dpo_consultation && (
            <span className="text-cyan-400">DSB konsultiert</span>
          )}
          <span>Erstellt {date}</span>
        </div>
        <div className="flex items-center justify-between pt-1">
          {dpia.status !== 'approved' ? (
            <Button
              size="sm"
              variant="outline"
              className="text-xs border-green-500/40 text-green-400 hover:bg-green-500/10 h-7"
              onClick={() => onApprove(dpia.id)}
            >
              <ShieldCheck className="w-3.5 h-3.5 mr-1" />
              Freigeben
            </Button>
          ) : (
            <span />
          )}
          <div className="flex gap-1">
            <Button size="icon" variant="ghost" className="h-7 w-7" aria-label="Bearbeiten" onClick={() => onEdit(dpia)}>
              <Pencil className="w-3.5 h-3.5" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              className="h-7 w-7 text-destructive hover:text-destructive"
              aria-label="Löschen"
              onClick={() => onDelete(dpia.id)}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function DPIAForm({
  form,
  onChange,
  showVvtSelector,
  vvtEntries,
}: {
  form: DPIAFormState
  onChange: (f: DPIAFormState) => void
  showVvtSelector: boolean
  vvtEntries?: { id: string; name: string }[]
}) {
  const set = (patch: Partial<DPIAFormState>) => onChange({ ...form, ...patch })

  return (
    <div className="space-y-4 py-2">
      <div className="space-y-1.5">
        <Label>Titel *</Label>
        <Input
          placeholder="z.B. DSFA für KI-gestützte Videoüberwachung"
          value={form.title}
          onChange={(e) => set({ title: e.target.value })}
        />
      </div>
      {showVvtSelector && vvtEntries && vvtEntries.length > 0 && (
        <div className="space-y-1.5">
          <Label>Verknüpfter VVT-Eintrag (optional)</Label>
          <select
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={form.vvt_entry_id ?? ''}
            onChange={(e) => set({ vvt_entry_id: e.target.value || undefined })}
          >
            <option value="">— Keiner —</option>
            {vvtEntries.map((v) => (
              <option key={v.id} value={v.id}>{v.name}</option>
            ))}
          </select>
        </div>
      )}
      <div className="space-y-1.5">
        <Label>Beschreibung</Label>
        <Textarea
          placeholder="Allgemeine Beschreibung der Verarbeitung …"
          rows={2}
          value={form.description}
          onChange={(e) => set({ description: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Erforderlichkeit & Verhältnismäßigkeit</Label>
        <Textarea
          placeholder="Warum ist diese Verarbeitung erforderlich und verhältnismäßig?"
          rows={2}
          value={form.necessity_assessment}
          onChange={(e) => set({ necessity_assessment: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Risikobewertung</Label>
        <Textarea
          placeholder="Identifizierte Risiken für die Rechte und Freiheiten der Betroffenen …"
          rows={3}
          value={form.risk_assessment}
          onChange={(e) => set({ risk_assessment: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Abhilfemaßnahmen</Label>
        <Textarea
          placeholder="Technische und organisatorische Maßnahmen zur Risikominderung …"
          rows={2}
          value={form.mitigation_measures}
          onChange={(e) => set({ mitigation_measures: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Restrisiko</Label>
        <Textarea
          placeholder="Verbleibendes Restrisiko nach Maßnahmen …"
          rows={2}
          value={form.residual_risk}
          onChange={(e) => set({ residual_risk: e.target.value })}
        />
      </div>
      <div className="flex items-center gap-2">
        <input
          type="checkbox"
          id="dpia-dpo"
          checked={form.dpo_consultation}
          onChange={(e) => set({ dpo_consultation: e.target.checked })}
          className="w-4 h-4"
        />
        <Label htmlFor="dpia-dpo">Datenschutzbeauftragter wurde konsultiert</Label>
      </div>
    </div>
  )
}

export default function DPIAPage() {
  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<DPIAFormState>(emptyForm())
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [exportError, setExportError] = useState<unknown>(null)

  const { data: dpias, isLoading, isError } = useDPIAs()
  const { data: vvtEntries } = useVVT()
  const createDPIA = useCreateDPIA()
  const updateDPIA = useUpdateDPIA()
  const approveDPIA = useApproveDPIA()
  const deleteDPIA = useDeleteDPIA()
  const exportDPIA = useExportDPIA()

  async function handleExport() {
    try {
      setExportError(null)
      await exportDPIA()
    } catch (err) {
      setExportError(err)
    }
  }

  function openCreate() {
    setForm(emptyForm())
    setEditId(null)
    setDialogMode('create')
  }

  function openEdit(dpia: DPIA) {
    setForm(formFromEntry(dpia))
    setEditId(dpia.id)
    setDialogMode('edit')
  }

  function handleDelete(id: string) {
    setDeleteId(id)
  }

  function confirmDelete() {
    if (deleteId) deleteDPIA.mutate(deleteId)
    setDeleteId(null)
  }

  function handleApprove(id: string) {
    approveDPIA.mutate(id)
  }

  function handleSubmit() {
    if (dialogMode === 'create') {
      const payload: CreateDPIAInput = {
        title: form.title,
        description: form.description || undefined,
        necessity_assessment: form.necessity_assessment || undefined,
        risk_assessment: form.risk_assessment || undefined,
        mitigation_measures: form.mitigation_measures || undefined,
        residual_risk: form.residual_risk || undefined,
        dpo_consultation: form.dpo_consultation,
        vvt_entry_id: form.vvt_entry_id,
      }
      createDPIA.mutate(payload, { onSuccess: () => setDialogMode(null) })
    } else if (dialogMode === 'edit' && editId) {
      const payload: UpdateDPIAInput = {
        title: form.title,
        description: form.description || undefined,
        necessity_assessment: form.necessity_assessment || undefined,
        risk_assessment: form.risk_assessment || undefined,
        mitigation_measures: form.mitigation_measures || undefined,
        residual_risk: form.residual_risk || undefined,
        dpo_consultation: form.dpo_consultation,
      }
      updateDPIA.mutate({ id: editId, input: payload }, { onSuccess: () => setDialogMode(null) })
    }
  }

  const isPending = createDPIA.isPending || updateDPIA.isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Datenschutz-Folgenabschätzung (DSFA)"
        description="Art. 35 DSGVO — Risikobeurteilung für riskante Verarbeitungstätigkeiten."
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { void handleExport() }} disabled={!dpias || dpias.length === 0}>
              <Download className="w-4 h-4 mr-1" />
              Als PDF exportieren
            </Button>
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              DSFA erstellen
            </Button>
          </div>
        }
      />
      <ProGate error={exportError}>{null}</ProGate>

      <InfoBanner icon={ClipboardCheck} title="Datenschutz-Folgenabschätzung (Art. 35 DSGVO)">
        <p>Eine DSFA ist <strong>verpflichtend</strong> bei Verarbeitungen mit voraussichtlich hohem Risiko — z.B. Profiling, Biometrie, Videoüberwachung oder großangelegte Verarbeitung besonderer Datenkategorien (Art. 9 DSGVO).</p>
        <p className="mt-1">Die DSFA sollte vor dem Start der Verarbeitung durchgeführt und bei wesentlichen Änderungen wiederholt werden. Der Datenschutzbeauftragte ist einzubeziehen.</p>
      </InfoBanner>

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Fehler beim Laden der Datenschutz-Folgenabschätzungen.
          </div>
        )}

        {!isLoading && !isError && dpias?.length === 0 && (
          <EmptyState
            icon={ClipboardCheck}
            title="Noch keine DSFAs"
            description="Erstellen Sie eine Datenschutz-Folgenabschätzung für risikoreiche Verarbeitungen."
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                DSFA erstellen
              </Button>
            }
          />
        )}

        {!isLoading && !isError && dpias && dpias.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {dpias.map((d) => (
              <DPIACard
                key={d.id}
                dpia={d}
                onEdit={openEdit}
                onDelete={handleDelete}
                onApprove={handleApprove}
              />
            ))}
          </div>
        )}
      </div>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => !open && setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>DSFA löschen?</AlertDialogTitle>
            <AlertDialogDescription>
              Diese Aktion kann nicht rückgängig gemacht werden.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setDeleteId(null)}>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">Löschen</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={dialogMode !== null} onOpenChange={(open) => !open && setDialogMode(null)}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {dialogMode === 'create' ? 'DSFA erstellen' : 'DSFA bearbeiten'}
            </DialogTitle>
          </DialogHeader>
          <DPIAForm
            form={form}
            onChange={setForm}
            showVvtSelector={dialogMode === 'create'}
            vvtEntries={vvtEntries}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogMode(null)}>
              Abbrechen
            </Button>
            <Button onClick={handleSubmit} disabled={!form.title || isPending}>
              {isPending ? 'Speichern …' : dialogMode === 'create' ? 'DSFA erstellen' : 'Speichern'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
