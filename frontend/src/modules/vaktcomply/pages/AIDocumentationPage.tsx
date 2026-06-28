import { useState, useEffect, useRef } from 'react'
import { useParams, Link } from 'react-router-dom'
import { FileText, Download, History, ChevronLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Textarea } from '../../../components/ui/textarea'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Badge } from '../../../components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { useAISystem } from '../hooks/useAISystems'
import { useAIDocumentation, useAIDocumentationVersions, useSaveAIDocumentation } from '../hooks/useAISystems'
import type { UpsertAIDocumentationInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

function emptyForm(): UpsertAIDocumentationInput {
  return {
    system_description: '',
    intended_purpose: '',
    training_data: '',
    data_quality: '',
    performance_metrics: '',
    system_limits: '',
    risk_management: '',
    human_oversight: '',
    logging_audit_trail: '',
    authored_by: '',
    status: 'draft',
  }
}

const SECTIONS: Array<{ key: keyof UpsertAIDocumentationInput; label: string; article: string; rows: number }> = [
  { key: 'system_description', label: 'Systembeschreibung', article: 'Annex IV Nr. 1a', rows: 3 },
  { key: 'intended_purpose', label: 'Bestimmungsgemäßer Verwendungszweck', article: 'Annex IV Nr. 1b', rows: 3 },
  { key: 'training_data', label: 'Trainingsdaten', article: 'Annex IV Nr. 2a', rows: 3 },
  { key: 'data_quality', label: 'Datenqualität und -governance', article: 'Annex IV Nr. 2b', rows: 3 },
  { key: 'performance_metrics', label: 'Leistungsmetriken', article: 'Annex IV Nr. 3a', rows: 3 },
  { key: 'system_limits', label: 'Systemgrenzen und bekannte Einschränkungen', article: 'Annex IV Nr. 3b', rows: 3 },
  { key: 'risk_management', label: 'Risikomanagementsystem (Verweis auf Risiko-Register)', article: 'Art. 9 EU AI Act', rows: 4 },
  { key: 'human_oversight', label: 'Maßnahmen für menschliche Aufsicht', article: 'Art. 14 EU AI Act', rows: 3 },
  { key: 'logging_audit_trail', label: 'Protokollierung und Audit-Trail', article: 'Art. 12 EU AI Act', rows: 3 },
]

export default function AIDocumentationPage() {
  const { id } = useParams<{ id: string }>()
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const { data: system } = useAISystem(id ?? '')
  const { data: latestDoc, isLoading: docLoading } = useAIDocumentation(id ?? '')
  const { data: versions } = useAIDocumentationVersions(id ?? '')
  const saveDoc = useSaveAIDocumentation(id ?? '')
  const [form, setForm] = useState<UpsertAIDocumentationInput>(emptyForm())
  const [showVersions, setShowVersions] = useState(false)
  const [saved, setSaved] = useState(false)
  const [pdfError, setPdfError] = useState<string | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    if (latestDoc) {
      setForm({
        system_description: latestDoc.system_description ?? '',
        intended_purpose: latestDoc.intended_purpose ?? '',
        training_data: latestDoc.training_data ?? '',
        data_quality: latestDoc.data_quality ?? '',
        performance_metrics: latestDoc.performance_metrics ?? '',
        system_limits: latestDoc.system_limits ?? '',
        risk_management: latestDoc.risk_management ?? '',
        human_oversight: latestDoc.human_oversight ?? '',
        logging_audit_trail: latestDoc.logging_audit_trail ?? '',
        authored_by: latestDoc.authored_by ?? '',
        status: latestDoc.status ?? 'draft',
      })
    }
  }, [latestDoc])

  useEffect(() => () => { clearTimeout(timerRef.current); }, [])

  function handleSave(status?: 'draft' | 'final') {
    saveDoc.mutate(
      { ...form, status: status ?? form.status },
      {
        onSuccess: () => {
          setSaved(true)
          timerRef.current = setTimeout(() => { setSaved(false); }, 2500)
        },
      },
    )
  }

  async function handleExportPDF() {
    try {
      setPdfError(null)
      const res = await fetch(`/api/v1/vaktcomply/ai-systems/${id}/documentation/export-pdf`, {
        credentials: 'include',
      })
      if (!res.ok) throw new Error('Export failed')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `ai-documentation-${id ?? 'export'}.pdf`
      a.click()
      URL.revokeObjectURL(url)
    } catch {
      setPdfError(t('vaktcomply.aiDocumentation.pdfExportFailed'))
    }
  }

  function setField(key: keyof UpsertAIDocumentationInput, value: string) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={`Technisches Dossier${system ? ` — ${system.name}` : ''}`}
        description="EU AI Act Art. 11 / Annex IV — Technische Dokumentation für Hochrisiko-KI-Systeme"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => { setShowVersions((v) => !v); }}>
              <History className="w-4 h-4 mr-1" />
              {versions?.length ?? 0} Version{(versions?.length ?? 0) !== 1 ? 'en' : ''}
            </Button>
            <div className="flex flex-col items-end gap-1">
              <Button variant="outline" size="sm" onClick={() => void handleExportPDF()} data-testid="export-pdf-btn">
                <Download className="w-4 h-4 mr-1" />
                PDF-Export
              </Button>
              {pdfError && <p className="text-xs text-red-500">{pdfError}</p>}
            </div>
            <Button onClick={() => { handleSave('final'); }} disabled={saveDoc.isPending} data-testid="finalize-doc-btn">
              {t('vaktcomply.aiDocumentation.saveAsFinal')}
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-2">
        {/* Back link */}
        <Link to="../ai-systems" className="text-sm text-muted-foreground hover:text-primary flex items-center gap-1 mb-4">
          <ChevronLeft className="w-3.5 h-3.5" />
          {t('vaktcomply.aiDocumentation.backToInventory')}
        </Link>

        {/* Version history panel */}
        {showVersions && versions && versions.length > 0 && (
          <div className="mb-4 p-4 bg-muted/30 rounded-lg border border-border">
            <p className="text-sm font-medium mb-2">{t('vaktcomply.aiDocumentation.versionHistory')}</p>
            <div className="space-y-1">
              {versions.map((v) => (
                <div key={v.id} className="flex items-center gap-3 text-xs text-muted-foreground">
                  <span className="font-mono">v{v.version}</span>
                  <Badge variant="outline" className="text-xs">{v.status}</Badge>
                  <span>{formatDate(v.created_at)}</span>
                  {v.authored_by && <span>{t('vaktcomply.aiDocumentation.versionBy', { author: v.authored_by })}</span>}
                </div>
              ))}
            </div>
          </div>
        )}

        {docLoading ? (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        ) : (
          <div className="space-y-6 max-w-3xl">
            {SECTIONS.map((sec) => (
              <div key={sec.key} className="space-y-1.5">
                <div className="flex items-center gap-2">
                  <Label className="font-medium">{sec.label}</Label>
                  <span className="text-xs text-primary/60">{sec.article}</span>
                </div>
                <Textarea
                  rows={sec.rows}
                  placeholder={t('vaktcomply.aiDocumentation.fieldPlaceholder', { label: sec.label })}
                  value={(form[sec.key] as string) ?? ''}
                  onChange={(e) => { setField(sec.key, e.target.value); }}
                  data-testid={`doc-field-${sec.key}`}
                />
              </div>
            ))}

            {/* Metadata row */}
            <div className="grid grid-cols-2 gap-4 pt-2">
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.aiDocumentation.authoredBy')}</Label>
                <Input
                  placeholder={t('vaktcomply.aiDocumentation.authorPlaceholder')}
                  value={form.authored_by ?? ''}
                  onChange={(e) => { setField('authored_by', e.target.value); }}
                  data-testid="doc-field-authored_by"
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('common.status')}</Label>
                <Select
                  value={form.status ?? 'draft'}
                  onValueChange={(v) => { setForm((f) => ({ ...f, status: v as 'draft' | 'final' })); }}
                >
                  <SelectTrigger data-testid="doc-status-select">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="draft">{t('common.statusLabels.draft')}</SelectItem>
                    <SelectItem value="final">{t('vaktcomply.aiDocumentation.statusFinal')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="flex gap-3 pt-2">
              <Button
                variant="outline"
                onClick={() => { handleSave('draft'); }}
                disabled={saveDoc.isPending}
                data-testid="save-draft-btn"
              >
                {t('vaktcomply.aiDocumentation.saveAsDraft')}
              </Button>
              <Button
                onClick={() => { handleSave('final'); }}
                disabled={saveDoc.isPending}
                data-testid="save-final-btn"
              >
                {saveDoc.isPending ? t('vaktcomply.aiDocumentation.saving') : t('vaktcomply.aiDocumentation.saveAsFinal')}
              </Button>
              {saved && (
                <span className="text-sm text-green-400 flex items-center gap-1">
                  <FileText className="w-4 h-4" />
                  {t('vaktcomply.aiDocumentation.saved')}
                </span>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
