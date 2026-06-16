import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload, FileUp, CheckCircle2, AlertTriangle } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Spinner } from '../../../components/Spinner'
import {
  useVeriniceImportPreview,
  useVeriniceImportCommit,
  type VeriniceImportPreview,
  type VeriniceImportResult,
} from '../hooks/useVeriniceImport'

type Step = 'upload' | 'preview' | 'done'

export default function VeriniceImportPage() {
  const { t } = useTranslation()
  const [step, setStep] = useState<Step>('upload')
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<VeriniceImportPreview | null>(null)
  const [result, setResult] = useState<VeriniceImportResult | null>(null)

  const previewMut = useVeriniceImportPreview()
  const commitMut = useVeriniceImportCommit()

  function onFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0] ?? null
    setFile(f)
  }

  function runPreview() {
    if (!file) return
    previewMut.mutate(file, {
      onSuccess: (p) => { setPreview(p); setStep('preview') },
    })
  }

  function runCommit() {
    if (!file) return
    commitMut.mutate(file, {
      onSuccess: (r) => { setResult(r); setStep('done') },
    })
  }

  function reset() {
    setStep('upload'); setFile(null); setPreview(null); setResult(null)
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader title={t('verinice.title')} description={t('verinice.description')} />
      <div className="flex-1 p-6 max-w-2xl space-y-6">
        {/* Step 1: Upload */}
        {step === 'upload' && (
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <FileUp className="w-4 h-4" />{t('verinice.uploadTitle')}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-sm text-muted-foreground">{t('verinice.uploadHint')}</p>
              <input
                type="file"
                accept=".vna,.zip"
                onChange={onFileChange}
                className="block w-full text-sm file:mr-3 file:rounded-md file:border-0 file:bg-primary/10 file:px-3 file:py-1.5 file:text-primary"
              />
              {previewMut.isError && (
                <p className="text-sm text-red-400">{previewMut.error.message}</p>
              )}
              <Button onClick={runPreview} disabled={!file || previewMut.isPending}>
                {previewMut.isPending ? <Spinner size="sm" /> : <Upload className="w-4 h-4 mr-1" />}
                {t('verinice.analyze')}
              </Button>
            </CardContent>
          </Card>
        )}

        {/* Step 2: Dry-run preview */}
        {step === 'preview' && preview && (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t('verinice.previewTitle')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-4 gap-3 text-center">
                <div className="p-2 rounded bg-muted/40"><p className="text-xl font-bold">{preview.assets}</p><p className="text-xs text-muted-foreground">{t('verinice.assets')}</p></div>
                <div className="p-2 rounded bg-muted/40"><p className="text-xl font-bold">{preview.controls}</p><p className="text-xs text-muted-foreground">{t('verinice.controls')}</p></div>
                <div className="p-2 rounded bg-muted/40"><p className="text-xl font-bold">{preview.risks}</p><p className="text-xs text-muted-foreground">{t('verinice.risks')}</p></div>
                <div className="p-2 rounded bg-muted/40"><p className="text-xl font-bold">{preview.unmapped}</p><p className="text-xs text-muted-foreground">{t('verinice.unmapped')}</p></div>
              </div>
              {preview.unmapped_types.length > 0 && (
                <div className="text-xs">
                  <p className="flex items-center gap-1 text-amber-400 mb-1">
                    <AlertTriangle className="w-3.5 h-3.5" />{t('verinice.unmappedTypes')}
                  </p>
                  <div className="flex flex-wrap gap-1">
                    {preview.unmapped_types.map((ut) => (
                      <Badge key={ut} variant="outline" className="text-[10px]">{ut}</Badge>
                    ))}
                  </div>
                </div>
              )}
              {commitMut.isError && (
                <p className="text-sm text-red-400">{commitMut.error.message}</p>
              )}
              <div className="flex gap-2">
                <Button variant="outline" onClick={reset}>{t('common.cancel')}</Button>
                <Button onClick={runCommit} disabled={commitMut.isPending}>
                  {commitMut.isPending ? <Spinner size="sm" /> : null}
                  {t('verinice.confirmImport')}
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Step 3: Result report */}
        {step === 'done' && result && (
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2 text-green-400">
                <CheckCircle2 className="w-4 h-4" />{t('verinice.doneTitle')}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <ul className="text-sm space-y-1">
                <li>{t('verinice.assets')}: <strong>{result.assets_created}</strong></li>
                <li>{t('verinice.controls')}: <strong>{result.controls_created}</strong></li>
                <li>{t('verinice.risks')}: <strong>{result.risks_created}</strong></li>
                <li>{t('verinice.skipped')}: <strong>{result.skipped}</strong></li>
              </ul>
              <Button onClick={reset}>{t('verinice.importAnother')}</Button>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
