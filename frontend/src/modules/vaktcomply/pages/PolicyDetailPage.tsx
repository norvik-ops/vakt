import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Spinner } from '../../../components/Spinner'
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
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

function toDateInput(ts?: string): string {
  if (!ts) return ''
  return ts.slice(0, 10)
}

export default function PolicyDetailPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDate } = useFormatDate()
  const { data: policy, isLoading, isError } = usePolicy(id ?? '')
  const update = useUpdatePolicy(id ?? '')

  const [form, setForm] = useState<UpdatePolicyInput | null>(null)
  const [dirty, setDirty] = useState(false)

  const statusLabels: Record<Policy['status'], string> = {
    draft: t('vaktcomply.policyDetail.statusDraft'),
    active: t('vaktcomply.policyDetail.statusActive'),
    archived: t('vaktcomply.policyDetail.statusArchived'),
  }

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
      <Spinner size="lg" color="primary" />
    </div>
  )
  if (isError || !policy) return (
    <div className="p-6 text-sm text-red-400">{t('vaktcomply.policyDetail.notFound')}</div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={policy.title}
        description={`v${policy.version} · Revision ${policy.version_num}${policy.category ? ` · ${policy.category}` : ''}`}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { navigate('/vaktcomply/policies'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('common.back')}
            </Button>
            <Button onClick={handleSave} disabled={!dirty || update.isPending}>
              <Save className="w-4 h-4 mr-1" />
              {update.isPending ? t('common.savePending') : t('common.save')}
            </Button>
          </div>
        }
      />

      {form && (
        <div className="flex-1 p-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.policyDetail.cardContent')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.policyDetail.labelTitle')}</Label>
                  <Input value={form.title} onChange={(e) => { set('title', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.policyDetail.labelCategory')}</Label>
                  <Input value={form.category ?? ''} placeholder={t('vaktcomply.policyDetail.placeholderCategory')} onChange={(e) => { set('category', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.policyDetail.labelDescriptionPurpose')}</Label>
                  <Textarea rows={5} value={form.description ?? ''} onChange={(e) => { set('description', e.target.value); }} />
                </div>
              </CardContent>
            </Card>

            {/* Versioning fields — shown when editing */}
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.policyDetail.cardChangeDoc')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="pol-version-note">
                    {t('vaktcomply.policyDetail.labelVersionNote')}
                    <span className="text-muted-foreground font-normal ml-1">{t('vaktcomply.policyDetail.versionNoteHint')}</span>
                  </Label>
                  <Textarea
                    id="pol-version-note"
                    rows={2}
                    placeholder={t('vaktcomply.policyDetail.placeholderVersionNote')}
                    value={form.version_note ?? ''}
                    onChange={(e) => { set('version_note', e.target.value); }}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <Label htmlFor="pol-updated-by">{t('vaktcomply.policyDetail.labelUpdatedBy')}</Label>
                    <Input
                      id="pol-updated-by"
                      placeholder={t('vaktcomply.policyDetail.placeholderUpdatedBy')}
                      value={form.updated_by ?? ''}
                      onChange={(e) => { set('updated_by', e.target.value); }}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="pol-next-review">{t('vaktcomply.policyDetail.labelNextReview')}</Label>
                    <Input
                      id="pol-next-review"
                      type="date"
                      value={form.next_review_due ?? ''}
                      onChange={(e) => { set('next_review_due', e.target.value); }}
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
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.policyDetail.cardMetadata')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>{t('common.status')}</Label>
                  <Select value={form.status} onValueChange={(v) => { set('status', v as Policy['status']); }}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {(Object.keys(statusLabels) as Policy['status'][]).map((k) => (
                        <SelectItem key={k} value={k}>{statusLabels[k]}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.policyDetail.labelVersion')}</Label>
                  <Input value={form.version ?? ''} onChange={(e) => { set('version', e.target.value); }} placeholder={t('vaktcomply.policyDetail.placeholderVersion')} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.policyDetail.labelOwner')}</Label>
                  <Input value={form.owner ?? ''} onChange={(e) => { set('owner', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.policyDetail.labelEffectiveDate')}</Label>
                  <Input type="date" value={form.effective_date ?? ''} onChange={(e) => { set('effective_date', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.policyDetail.labelReviewDate')}</Label>
                  <Input type="date" value={form.review_date ?? ''} onChange={(e) => { set('review_date', e.target.value); }} />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="pt-4 space-y-1 text-xs text-muted-foreground">
                <p>{t('vaktcomply.policyDetail.metaCreated')} {formatDate(policy.created_at)}</p>
                <p>{t('vaktcomply.policyDetail.metaUpdated')} {formatDate(policy.updated_at)}</p>
                {policy.reviewed_at && (
                  <p>{t('vaktcomply.policyDetail.metaReviewed')} {formatDate(policy.reviewed_at)}</p>
                )}
                {policy.next_review_due && (
                  <p>{t('vaktcomply.policyDetail.metaNextReview')} {formatDate(policy.next_review_due)}</p>
                )}
                {policy.last_updated_by && (
                  <p>{t('vaktcomply.policyDetail.metaUpdatedBy')} {policy.last_updated_by}</p>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      )}
    </div>
  )
}
