import { useState, useEffect } from 'react'
import { ReadOnlyHint } from '../../../shared/components/ReadOnlyHint'
import { WriteFieldset } from '../../../shared/components/WriteFieldset'
import { useCanWrite } from '../../../shared/hooks/useCanWrite'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, CheckCircle2, Eye, Pencil, Save, Users } from 'lucide-react'
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
import { policyVersionLine } from '../utils/policyVersion'
import { MarkdownView } from '../../../shared/components/MarkdownView'

function toDateInput(ts?: string): string {
  if (!ts) return ''
  return ts.slice(0, 10)
}

export default function PolicyDetailPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const canWrite = useCanWrite()
  const { formatDate } = useFormatDate()
  const { data: policy, isLoading, isError } = usePolicy(id ?? '')
  const update = useUpdatePolicy(id ?? '')

  const [form, setForm] = useState<UpdatePolicyInput | null>(null)
  const [dirty, setDirty] = useState(false)
  // UX-23: Richtlinientext gerendert lesen, zum Ändern auf den Editor umschalten.
  // null = noch nicht gewählt → Vorschau, sobald Text da ist, sonst Editor.
  const [textMode, setTextMode] = useState<'preview' | 'edit' | null>(null)

  const statusLabels: Record<Policy['status'], string> = {
    draft: t('vaktcomply.policyDetail.statusDraft'),
    active: t('vaktcomply.policyDetail.statusActive'),
    archived: t('vaktcomply.policyDetail.statusArchived'),
  }

  // UX-21: ck_policies has two date columns for the same thing — review_date
  // (create dialog, list card, overdue badge, CSV export "Überprüfung fällig")
  // and next_review_due (only ever edited here). The page shows one field,
  // „Nächste Überprüfung", backed by review_date, and writes the same value into
  // next_review_due so the two stop drifting apart. An older row that only has
  // next_review_due set is read from there.
  useEffect(() => {
    if (policy && !form) {
      setForm({
        title: policy.title,
        description: policy.description ?? '',
        category: policy.category ?? '',
        status: policy.status,
        version: policy.version,
        effective_date: toDateInput(policy.effective_date),
        review_date: toDateInput(policy.review_date ?? policy.next_review_due),
        owner: policy.owner ?? '',
        version_note: '',
        updated_by: policy.last_updated_by ?? '',
      })
    }
  }, [policy, form])

  function set<K extends keyof UpdatePolicyInput>(key: K, value: UpdatePolicyInput[K]) {
    setForm((f) => f ? { ...f, [key]: value } : f)
    setDirty(true)
  }

  function save(input: UpdatePolicyInput) {
    const payload: UpdatePolicyInput = {
      ...input,
      effective_date: input.effective_date || undefined,
      review_date: input.review_date || undefined,
      next_review_due: input.review_date || undefined,
      version_note: input.version_note || undefined,
      updated_by: input.updated_by || undefined,
    }
    update.mutate(payload, {
      onSuccess: () => {
        setDirty(false)
        // Reset version_note after save so it's blank for the next change
        setForm({ ...input, version_note: '' })
      },
    })
  }

  function handleSave() {
    if (!form) return
    save(form)
  }

  // UX-20: „Freigeben" is the same PATCH as „Speichern" with status active.
  //
  // It deliberately records NO approver. The only "who" the policy row has is
  // last_updated_by — free text the client sends — and PATCH /policies/:id writes
  // no audit-trail entry with the authenticated identity. Showing either as
  // „freigegeben von" would be a forgeable audit statement (security review
  // 2026-11-18, integrity/forgeable-approval-attribution). An approver display
  // needs a server-side source and therefore a migration (R1-KW-60).
  function handleApprove() {
    if (!form) return
    save({
      ...form,
      status: 'active',
      version_note: form.version_note || t('vaktcomply.policyDetail.approveNote'),
    })
  }

  const mode: 'preview' | 'edit' = !canWrite
    ? 'preview'
    : textMode ?? (form?.description?.trim() ? 'preview' : 'edit')

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
        description={policyVersionLine(policy, t)}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { navigate('/vaktcomply/policies'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('common.back')}
            </Button>
            {/* R1-KW-11: einziger UI-Weg zu den Akzeptanz-Kampagnen dieser Richtlinie. */}
            <Button variant="outline" onClick={() => { navigate(`/vaktcomply/policies/${policy.id}/acceptance`); }}>
              <Users className="w-4 h-4 mr-1" />
              {t('vaktcomply.policyDetail.acceptanceLink')}
            </Button>
            {canWrite && policy.status !== 'active' && (
              <Button
                variant="outline"
                onClick={handleApprove}
                disabled={update.isPending}
                title={t('vaktcomply.policyDetail.approveHint')}
              >
                <CheckCircle2 className="w-4 h-4 mr-1" />
                {update.isPending ? t('vaktcomply.policyDetail.approvePending') : t('vaktcomply.policyDetail.approveBtn')}
              </Button>
            )}
            {canWrite && (
              <Button onClick={handleSave} disabled={!dirty || update.isPending}>
                <Save className="w-4 h-4 mr-1" />
                {update.isPending ? t('common.savePending') : t('common.save')}
              </Button>
            )}
          </div>
        }
      />

      {form && (
        <div className="flex-1 p-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-4">
            <ReadOnlyHint />
            <WriteFieldset>
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
                  <div className="flex items-center justify-between gap-2">
                    <Label htmlFor="pol-text">{t('vaktcomply.policyDetail.labelDescriptionPurpose')}</Label>
                    {canWrite && (
                      <div className="inline-flex rounded-md border border-border p-0.5" role="group" aria-label={t('vaktcomply.policyDetail.textModeLabel')}>
                        <Button
                          type="button"
                          size="sm"
                          variant={mode === 'preview' ? 'secondary' : 'ghost'}
                          className="h-6 px-2 text-xs"
                          aria-pressed={mode === 'preview'}
                          onClick={() => { setTextMode('preview'); }}
                        >
                          <Eye className="w-3.5 h-3.5 mr-1" />
                          {t('vaktcomply.policyDetail.textPreview')}
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          variant={mode === 'edit' ? 'secondary' : 'ghost'}
                          className="h-6 px-2 text-xs"
                          aria-pressed={mode === 'edit'}
                          onClick={() => { setTextMode('edit'); }}
                        >
                          <Pencil className="w-3.5 h-3.5 mr-1" />
                          {t('vaktcomply.policyDetail.textEdit')}
                        </Button>
                      </div>
                    )}
                  </div>
                  {mode === 'edit' ? (
                    <>
                      <Textarea
                        id="pol-text"
                        rows={18}
                        className="font-mono text-xs"
                        value={form.description ?? ''}
                        onChange={(e) => { set('description', e.target.value); }}
                      />
                      <p className="text-xs text-muted-foreground">{t('vaktcomply.policyDetail.textMarkdownHint')}</p>
                    </>
                  ) : form.description?.trim() ? (
                    <MarkdownView
                      source={form.description}
                      className="rounded-md border border-border bg-surface p-4 max-h-[36rem] overflow-y-auto"
                    />
                  ) : (
                    <p className="text-sm text-muted-foreground">{t('vaktcomply.policyDetail.textEmpty')}</p>
                  )}
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
                <div className="space-y-1.5">
                  <Label htmlFor="pol-updated-by">{t('vaktcomply.policyDetail.labelUpdatedBy')}</Label>
                  <Input
                    id="pol-updated-by"
                    placeholder={t('vaktcomply.policyDetail.placeholderUpdatedBy')}
                    value={form.updated_by ?? ''}
                    onChange={(e) => { set('updated_by', e.target.value); }}
                  />
                </div>
              </CardContent>
            </Card>

            </WriteFieldset>
            {/* Version history — always visible */}
            <PolicyVersionHistory policyId={policy.id} currentVersion={policy.version_num} />
          </div>

          <div className="space-y-4">
            <WriteFieldset>
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
                  <p className="text-xs text-muted-foreground">{t('vaktcomply.policyDetail.revisionHint')}</p>
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
                  <Label htmlFor="pol-next-review">{t('vaktcomply.policyDetail.labelReviewDate')}</Label>
                  <Input id="pol-next-review" type="date" value={form.review_date ?? ''} onChange={(e) => { set('review_date', e.target.value); }} />
                </div>
              </CardContent>
            </Card>
            </WriteFieldset>

            <Card>
              <CardContent className="pt-4 space-y-1 text-xs text-muted-foreground">
                <p>{t('vaktcomply.policyDetail.metaCreated')} {formatDate(policy.created_at)}</p>
                <p>{t('vaktcomply.policyDetail.metaUpdated')} {formatDate(policy.updated_at)}</p>
                {policy.reviewed_at && (
                  <p>{t('vaktcomply.policyDetail.metaReviewed')} {formatDate(policy.reviewed_at)}</p>
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
