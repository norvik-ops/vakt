import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2, ChevronDown, ChevronUp, CheckCircle2 } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import {
  useISMSScope,
  useISMSScopeVersions,
  useSaveISMSScope,
  useApproveISMSScope,
} from '../hooks/useISMSScope'
import type { ISMSScopeExclusion, CreateISMSScopeInput } from '../types'

const STATUS_CLASS: Record<'draft' | 'approved', string> = {
  draft: 'bg-secondary text-secondary-foreground',
  approved: 'bg-green-500/20 text-green-400 border-green-500/30',
}

const STATUS_LABEL: Record<'draft' | 'approved', string> = {
  draft: 'Entwurf',
  approved: 'Genehmigt',
}

export default function ISMSScopePage() {
  const { t } = useTranslation()
  const { data: current, isLoading } = useISMSScope()
  const { data: versions = [] } = useISMSScopeVersions()
  const saveMutation = useSaveISMSScope()
  const approveMutation = useApproveISMSScope()

  const [scopeDefinition, setScopeDefinition] = useState('')
  const [outsourcing, setOutsourcing] = useState('')
  const [changeNote, setChangeNote] = useState('')
  const [exclusions, setExclusions] = useState<ISMSScopeExclusion[]>([])
  const [historyOpen, setHistoryOpen] = useState(false)

  useEffect(() => {
    if (current) {
      setScopeDefinition(current.scope_definition)
      setOutsourcing(current.outsourcing_dependencies)
      setChangeNote(current.change_note)
      setExclusions(Array.isArray(current.exclusions) ? current.exclusions : [])
    }
  }, [current])

  function addExclusion() {
    setExclusions((prev) => [...prev, { item: '', justification: '' }])
  }

  function removeExclusion(idx: number) {
    setExclusions((prev) => prev.filter((_, i) => i !== idx))
  }

  function updateExclusion(idx: number, field: keyof ISMSScopeExclusion, value: string) {
    setExclusions((prev) => prev.map((e, i) => (i === idx ? { ...e, [field]: value } : e)))
  }

  function handleSave() {
    const input: CreateISMSScopeInput = {
      scope_definition: scopeDefinition,
      exclusions,
      outsourcing_dependencies: outsourcing,
      change_note: changeNote,
    }
    saveMutation.mutate(input)
  }

  function handleApprove() {
    if (!current) return
    approveMutation.mutate({ id: current.id })
  }

  if (isLoading) return <Spinner />

  return (
    <div className="space-y-6">
      <PageHeader
        title="ISMS-Scope-Dokument"
        description="Definieren Sie den Geltungsbereich Ihres Informationssicherheitsmanagementsystems (ISO 27001 Kap. 4.3)."
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>
              Geltungsbereich
              {current && (
                <Badge className={`ml-3 text-xs ${STATUS_CLASS[current.status]}`}>
                  {STATUS_LABEL[current.status]} · v{current.version}
                </Badge>
              )}
            </span>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="scope-definition">Scope-Definition</Label>
            <Textarea
              id="scope-definition"
              rows={6}
              placeholder="Beschreiben Sie den Geltungsbereich des ISMS (Organisationseinheiten, Standorte, Systeme, Prozesse) …"
              value={scopeDefinition}
              onChange={(e) => { setScopeDefinition(e.target.value); }}
            />
          </div>

          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <Label>{t('vaktcomply.ismsScopePage.exclusionsLabel')}</Label>
              <Button type="button" variant="outline" size="sm" onClick={addExclusion}>
                <Plus className="mr-1 h-4 w-4" />
                {t('vaktcomply.ismsScopePage.addExclusion')}
              </Button>
            </div>
            {exclusions.length === 0 && (
              <p className="text-sm text-muted-foreground">{t('vaktcomply.ismsScopePage.noExclusions')}</p>
            )}
            {exclusions.map((ex, idx) => (
              <div key={idx} className="flex gap-2 items-start">
                <Input
                  placeholder="Ausgeschlossener Bereich"
                  value={ex.item}
                  onChange={(e) => { updateExclusion(idx, 'item', e.target.value); }}
                  className="flex-1"
                />
                <Input
                  placeholder="Begründung"
                  value={ex.justification}
                  onChange={(e) => { updateExclusion(idx, 'justification', e.target.value); }}
                  className="flex-1"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() => { removeExclusion(idx); }}
                  className="text-red-400 hover:text-red-300"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}
          </div>

          <div className="space-y-2">
            <Label htmlFor="outsourcing">Outsourcing-Abhängigkeiten</Label>
            <Textarea
              id="outsourcing"
              rows={3}
              placeholder="Beschreiben Sie wesentliche ausgelagerte Dienste oder Prozesse …"
              value={outsourcing}
              onChange={(e) => { setOutsourcing(e.target.value); }}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="change-note">Änderungshinweis (für neue Version)</Label>
            <Textarea
              id="change-note"
              rows={2}
              placeholder="Was hat sich gegenüber der letzten Version geändert?"
              value={changeNote}
              onChange={(e) => { setChangeNote(e.target.value); }}
            />
          </div>

          <div className="flex gap-3 pt-2">
            <Button onClick={handleSave} disabled={saveMutation.isPending}>
              {saveMutation.isPending ? 'Wird gespeichert …' : 'Speichern (neue Version)'}
            </Button>
            {current && current.status === 'draft' && (
              <Button
                variant="outline"
                onClick={handleApprove}
                disabled={approveMutation.isPending}
                className="gap-2"
              >
                <CheckCircle2 className="h-4 w-4 text-green-400" />
                {approveMutation.isPending ? 'Wird genehmigt …' : 'Genehmigen'}
              </Button>
            )}
          </div>

          {saveMutation.isError && (
            <p className="text-sm text-red-400">{saveMutation.error?.message ?? 'Fehler beim Speichern.'}</p>
          )}
          {approveMutation.isError && (
            <p className="text-sm text-red-400">{approveMutation.error?.message ?? 'Fehler beim Genehmigen.'}</p>
          )}
        </CardContent>
      </Card>

      {versions.length > 0 && (
        <Card>
          <CardHeader
            className="cursor-pointer select-none"
            onClick={() => { setHistoryOpen((o) => !o); }}
          >
            <CardTitle className="flex items-center justify-between text-base">
              <span>Versionsverlauf ({versions.length})</span>
              {historyOpen ? (
                <ChevronUp className="h-4 w-4 text-muted-foreground" />
              ) : (
                <ChevronDown className="h-4 w-4 text-muted-foreground" />
              )}
            </CardTitle>
          </CardHeader>
          {historyOpen && (
            <CardContent>
              <ul className="divide-y divide-border">
                {versions.map((v) => (
                  <li key={v.id} className="py-3 flex items-start justify-between gap-4">
                    <div>
                      <span className="font-medium text-sm">Version {v.version}</span>
                      {v.change_note && (
                        <p className="text-xs text-muted-foreground mt-0.5">{v.change_note}</p>
                      )}
                      <p className="text-xs text-muted-foreground">
                        {new Date(v.created_at).toLocaleDateString('de-DE')}
                      </p>
                    </div>
                    <Badge className={`text-xs ${STATUS_CLASS[v.status]}`}>
                      {STATUS_LABEL[v.status]}
                    </Badge>
                  </li>
                ))}
              </ul>
            </CardContent>
          )}
        </Card>
      )}

      {!current && !isLoading && (
        <p className="text-sm text-muted-foreground">{t('vaktcomply.ismsScopePage.noScope')}</p>
      )}
    </div>
  )
}
