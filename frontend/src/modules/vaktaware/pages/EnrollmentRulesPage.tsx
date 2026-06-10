import { useState } from 'react'
import { Zap, Plus, Trash2, Power } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Spinner } from '../../../components/Spinner'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import {
  useEnrollmentRules,
  useCreateEnrollmentRule,
  useUpdateEnrollmentRule,
  useDeleteEnrollmentRule,
} from '../hooks/useEnrollmentRules'
import { useCampaigns } from '../hooks/useCampaigns'

const TRIGGER_LABELS: Record<string, string> = {
  new_employee: 'Neuer Mitarbeiter',
  phishing_click: 'Phishing-Klick',
}

export default function EnrollmentRulesPage() {
  const { data: rules, isLoading } = useEnrollmentRules()
  const { data: campaignsData } = useCampaigns()
  const campaigns = Array.isArray(campaignsData) ? campaignsData : (campaignsData as { data?: unknown[] })?.data ?? []
  const createRule = useCreateEnrollmentRule()
  const updateRule = useUpdateEnrollmentRule()
  const deleteRule = useDeleteEnrollmentRule()

  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [triggerType, setTriggerType] = useState<'new_employee' | 'phishing_click'>('new_employee')
  const [campaignId, setCampaignId] = useState('')
  const [deleteId, setDeleteId] = useState<string | null>(null)

  function resetForm() {
    setName('')
    setTriggerType('new_employee')
    setCampaignId('')
  }

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    createRule.mutate(
      {
        name,
        trigger_type: triggerType,
        target_campaign_id: campaignId || undefined,
      },
      {
        onSuccess: () => {
          setOpen(false)
          resetForm()
        },
      },
    )
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Auto-Enrollment-Regeln"
        description="Automatisch Mitarbeiter für Kampagnen enrollen — bei Neuanstellung oder Phishing-Klick."
        actions={
          <Button onClick={() => { setOpen(true) }}>
            <Plus className="w-4 h-4 mr-1" />
            Neue Regel
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading ? (
          <div className="flex justify-center py-16"><Spinner size="md" /></div>
        ) : !rules || rules.length === 0 ? (
          <EmptyState
            icon={Zap}
            title="Keine Enrollment-Regeln"
            description="Erstelle Regeln, um Mitarbeiter automatisch für Phishing-Simulationen zu enrollen."
            action={
              <Button onClick={() => { setOpen(true) }}>
                <Plus className="w-4 h-4 mr-1" />Regel erstellen
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Trigger</TableHead>
                <TableHead>Ziel-Kampagne</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-24" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rules.map((rule) => {
                const campaign = (campaigns as { id: string; name: string }[]).find((c) => c.id === rule.target_campaign_id)
                return (
                  <TableRow key={rule.id}>
                    <TableCell className="font-medium">{rule.name}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{TRIGGER_LABELS[rule.trigger_type] ?? rule.trigger_type}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-secondary">
                      {campaign?.name ?? rule.target_campaign_id ?? '—'}
                    </TableCell>
                    <TableCell>
                      <Badge variant={rule.is_active ? 'default' : 'secondary'}>
                        {rule.is_active ? 'Aktiv' : 'Inaktiv'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 w-7 p-0"
                          title={rule.is_active ? 'Deaktivieren' : 'Aktivieren'}
                          onClick={() => { updateRule.mutate({ id: rule.id, isActive: !rule.is_active }) }}
                        >
                          <Power className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-red-500 hover:text-red-700 h-7 w-7 p-0"
                          onClick={() => { setDeleteId(rule.id) }}
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </div>

      {/* Create dialog */}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Neue Enrollment-Regel</DialogTitle></DialogHeader>
          <form onSubmit={handleCreate}>
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="rule-name">Regelname</Label>
                <Input
                  id="rule-name"
                  value={name}
                  onChange={(e) => { setName(e.target.value) }}
                  placeholder="z.B. Onboarding Phishing"
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label>Trigger</Label>
                <Select value={triggerType} onValueChange={(v) => { setTriggerType(v as typeof triggerType) }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="new_employee">Neuer Mitarbeiter</SelectItem>
                    <SelectItem value="phishing_click">Phishing-Klick</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>Ziel-Kampagne (optional)</Label>
                <Select value={campaignId} onValueChange={setCampaignId}>
                  <SelectTrigger><SelectValue placeholder="Kampagne auswählen…" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">Keine</SelectItem>
                    {(campaigns as { id: string; name: string }[]).map((c) => (
                      <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setOpen(false); resetForm() }}>Abbrechen</Button>
              <Button type="submit" disabled={createRule.isPending}>
                {createRule.isPending ? 'Wird erstellt…' : 'Regel erstellen'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete confirm dialog */}
      <Dialog open={!!deleteId} onOpenChange={(o) => { if (!o) setDeleteId(null) }}>
        <DialogContent>
          <DialogHeader><DialogTitle>Regel löschen</DialogTitle></DialogHeader>
          <p className="text-sm text-secondary py-2">Die Enrollment-Regel wird unwiderruflich gelöscht. Bereits enrollte Teilnehmer sind nicht betroffen.</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteId(null) }}>Abbrechen</Button>
            <Button
              variant="destructive"
              onClick={() => { if (deleteId) deleteRule.mutate(deleteId, { onSuccess: () => { setDeleteId(null) } }) }}
              disabled={deleteRule.isPending}
            >
              {deleteRule.isPending ? 'Wird gelöscht…' : 'Löschen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
