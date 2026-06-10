import { useState } from 'react'
import { FileText, Plus, Trash2, Shield } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { useTemplates, useCreateTemplate, useDeleteTemplate } from '../hooks/useTemplates'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

export default function TemplatesPage() {
  const { formatDate } = useFormatDate()
  const { data: templates, isLoading } = useTemplates()
  const createTemplate = useCreateTemplate()
  const deleteTemplate = useDeleteTemplate()

  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [subject, setSubject] = useState('')
  const [fromName, setFromName] = useState('')
  const [fromEmail, setFromEmail] = useState('')
  const [htmlBody, setHtmlBody] = useState('')
  const [deleteId, setDeleteId] = useState<string | null>(null)

  function resetForm() {
    setName(''); setSubject(''); setFromName(''); setFromEmail(''); setHtmlBody('')
  }

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    createTemplate.mutate(
      { name, subject, from_name: fromName, from_email: fromEmail, html_body: htmlBody },
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
        title="E-Mail-Vorlagen"
        description="E-Mail-Vorlagen für Phishing-Simulationen verwalten."
        actions={
          <Button onClick={() => { setOpen(true); }}>
            <Plus className="w-4 h-4 mr-1" />
            Neue Vorlage
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading ? (
          <div className="flex justify-center py-16">
            <Spinner size="md" />
          </div>
        ) : !templates || templates.length === 0 ? (
          <EmptyState
            icon={FileText}
            title="Keine Templates"
            description="Erstelle dein erstes E-Mail-Template."
            action={
              <Button onClick={() => { setOpen(true); }}>
                <Plus className="w-4 h-4 mr-1" />Template erstellen
              </Button>
            }
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {templates.map((t) => (
              <Card key={t.id} className="relative">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <div className="min-w-0 flex-1 pr-2">
                      <CardTitle className="text-sm truncate">{t.name}</CardTitle>
                      <CardDescription className="mt-0.5 truncate">{t.subject}</CardDescription>
                    </div>
                    {t.is_preset && (
                      <Badge variant="secondary" className="shrink-0">
                        <Shield className="w-3 h-3 mr-1" />
                        Preset
                      </Badge>
                    )}
                  </div>
                </CardHeader>
                <CardContent>
                  <p className="text-xs text-secondary mb-3">
                    Von: {t.from_name} &lt;{t.from_email}&gt;
                  </p>
                  <p className="text-xs text-secondary">
                    Erstellt {formatDate(t.created_at)}
                  </p>
                  {!t.is_preset && (
                    <div className="mt-3 flex justify-end">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-red-500 hover:text-red-700 h-7 w-7 p-0"
                        onClick={() => { setDeleteId(t.id); }}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Neue E-Mail-Vorlage</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { handleCreate(e) }}>
            <div className="py-4 space-y-4 max-h-[60vh] overflow-y-auto pr-1">
              <div className="space-y-1.5">
                <Label htmlFor="tmpl-name">Vorlagenname</Label>
                <Input id="tmpl-name" value={name} onChange={(e) => { setName(e.target.value); }} required />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="tmpl-from-name">Absendername</Label>
                  <Input id="tmpl-from-name" value={fromName} onChange={(e) => { setFromName(e.target.value); }} required />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="tmpl-from-email">Absender-E-Mail</Label>
                  <Input id="tmpl-from-email" type="email" value={fromEmail} onChange={(e) => { setFromEmail(e.target.value); }} required />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="tmpl-subject">Betreff</Label>
                <Input id="tmpl-subject" value={subject} onChange={(e) => { setSubject(e.target.value); }} required />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="tmpl-body">HTML Body</Label>
                <textarea
                  id="tmpl-body"
                  rows={6}
                  className="w-full rounded-md border border-border px-3 py-2 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-brand"
                  value={htmlBody}
                  onChange={(e) => { setHtmlBody(e.target.value); }}
                  placeholder="<html>...</html>"
                  required
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setOpen(false); resetForm() }}>Abbrechen</Button>
              <Button type="submit" disabled={createTemplate.isPending}>
                {createTemplate.isPending ? 'Wird erstellt…' : 'Vorlage erstellen'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleteId} onOpenChange={(open) => { if (!open) { setDeleteId(null); } }}>
        <DialogContent>
          <DialogHeader><DialogTitle>Vorlage löschen</DialogTitle></DialogHeader>
          <p className="text-sm text-secondary py-2">Die Vorlage wird unwiderruflich gelöscht. Bestehende Kampagnen, die sie verwenden, sind nicht betroffen.</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteId(null); }}>Abbrechen</Button>
            <Button
              variant="destructive"
              onClick={() => { if (deleteId) deleteTemplate.mutate(deleteId, { onSuccess: () => { setDeleteId(null); } }); }}
              disabled={deleteTemplate.isPending}
            >
              {deleteTemplate.isPending ? 'Wird gelöscht…' : 'Löschen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
