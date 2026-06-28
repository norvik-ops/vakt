import { useState } from 'react'
import { Zap, Plus, Trash2, Power } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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

export default function EnrollmentRulesPage() {
  const { t } = useTranslation()
  const triggerLabel = (s: string) => t('vaktaware.enrollmentRules.trigger_' + s, { defaultValue: s })
  const { data: rules, isLoading } = useEnrollmentRules()
  const { data: campaignsData } = useCampaigns()
  const campaigns = Array.isArray(campaignsData) ? campaignsData : (campaignsData as unknown as { data?: unknown[] })?.data ?? []
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
        title={t('vaktaware.enrollmentRules.title')}
        description={t('vaktaware.enrollmentRules.description')}
        actions={
          <Button onClick={() => { setOpen(true) }}>
            <Plus className="w-4 h-4 mr-1" />
            {t('vaktaware.enrollmentRules.newRule')}
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading ? (
          <div className="flex justify-center py-16"><Spinner size="md" /></div>
        ) : !rules || rules.length === 0 ? (
          <EmptyState
            icon={Zap}
            title={t('vaktaware.enrollmentRules.noRules')}
            description={t('vaktaware.enrollmentRules.noRulesDesc')}
            action={
              <Button onClick={() => { setOpen(true) }}>
                <Plus className="w-4 h-4 mr-1" />{t('vaktaware.enrollmentRules.createRule')}
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('common.name')}</TableHead>
                <TableHead>{t('vaktaware.enrollmentRules.colTrigger')}</TableHead>
                <TableHead>{t('vaktaware.enrollmentRules.colTargetCampaign')}</TableHead>
                <TableHead>{t('common.status')}</TableHead>
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
                      <Badge variant="outline">{triggerLabel(rule.trigger_type)}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-secondary">
                      {campaign?.name ?? rule.target_campaign_id ?? '—'}
                    </TableCell>
                    <TableCell>
                      <Badge variant={rule.is_active ? 'default' : 'secondary'}>
                        {rule.is_active ? t('vaktaware.enrollmentRules.active') : t('vaktaware.enrollmentRules.inactive')}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 w-7 p-0"
                          title={rule.is_active ? t('vaktaware.enrollmentRules.deactivate') : t('vaktaware.enrollmentRules.activate')}
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
          <DialogHeader><DialogTitle>{t('vaktaware.enrollmentRules.createDialogTitle')}</DialogTitle></DialogHeader>
          <form onSubmit={handleCreate}>
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="rule-name">{t('vaktaware.enrollmentRules.labelRuleName')}</Label>
                <Input
                  id="rule-name"
                  value={name}
                  onChange={(e) => { setName(e.target.value) }}
                  placeholder={t('vaktaware.enrollmentRules.placeholderRuleName')}
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('vaktaware.enrollmentRules.labelTrigger')}</Label>
                <Select value={triggerType} onValueChange={(v) => { setTriggerType(v as typeof triggerType) }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="new_employee">{t('vaktaware.enrollmentRules.triggerNewEmployee')}</SelectItem>
                    <SelectItem value="phishing_click">{t('vaktaware.enrollmentRules.triggerPhishingClick')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('vaktaware.enrollmentRules.labelTargetCampaignOptional')}</Label>
                <Select value={campaignId} onValueChange={setCampaignId}>
                  <SelectTrigger><SelectValue placeholder={t('vaktaware.enrollmentRules.placeholderCampaign')} /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">{t('vaktaware.enrollmentRules.none')}</SelectItem>
                    {(campaigns as { id: string; name: string }[]).map((c) => (
                      <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setOpen(false); resetForm() }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={createRule.isPending}>
                {createRule.isPending ? t('vaktaware.enrollmentRules.creating') : t('vaktaware.enrollmentRules.createRule')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete confirm dialog */}
      <Dialog open={!!deleteId} onOpenChange={(o) => { if (!o) setDeleteId(null) }}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('vaktaware.enrollmentRules.deleteDialogTitle')}</DialogTitle></DialogHeader>
          <p className="text-sm text-secondary py-2">{t('vaktaware.enrollmentRules.deleteDialogDesc')}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteId(null) }}>{t('common.cancel')}</Button>
            <Button
              variant="destructive"
              onClick={() => { if (deleteId) deleteRule.mutate(deleteId, { onSuccess: () => { setDeleteId(null) } }) }}
              disabled={deleteRule.isPending}
            >
              {deleteRule.isPending ? t('vaktaware.enrollmentRules.deleting') : t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
