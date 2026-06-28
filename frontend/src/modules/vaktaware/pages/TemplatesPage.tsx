import { useState } from 'react'
import { FileText, Plus, Trash2, Shield } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
        title={t('vaktaware.templates.title')}
        description={t('vaktaware.templates.description')}
        actions={
          <Button onClick={() => { setOpen(true); }}>
            <Plus className="w-4 h-4 mr-1" />
            {t('vaktaware.templates.newTemplate')}
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
            title={t('vaktaware.templates.noTemplates')}
            description={t('vaktaware.templates.noTemplatesDesc')}
            action={
              <Button onClick={() => { setOpen(true); }}>
                <Plus className="w-4 h-4 mr-1" />{t('vaktaware.templates.createTemplate')}
              </Button>
            }
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {templates.map((tmpl) => (
              <Card key={tmpl.id} className="relative">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <div className="min-w-0 flex-1 pr-2">
                      <CardTitle className="text-sm truncate">{tmpl.name}</CardTitle>
                      <CardDescription className="mt-0.5 truncate">{tmpl.subject}</CardDescription>
                    </div>
                    {tmpl.is_preset && (
                      <Badge variant="secondary" className="shrink-0">
                        <Shield className="w-3 h-3 mr-1" />
                        Preset
                      </Badge>
                    )}
                  </div>
                </CardHeader>
                <CardContent>
                  <p className="text-xs text-secondary mb-3">
                    {t('vaktaware.templates.from')} {tmpl.from_name} &lt;{tmpl.from_email}&gt;
                  </p>
                  <p className="text-xs text-secondary">
                    {t('vaktaware.templates.createdOn')} {formatDate(tmpl.created_at)}
                  </p>
                  {!tmpl.is_preset && (
                    <div className="mt-3 flex justify-end">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-red-500 hover:text-red-700 h-7 w-7 p-0"
                        onClick={() => { setDeleteId(tmpl.id); }}
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
          <DialogHeader><DialogTitle>{t('vaktaware.templates.createDialogTitle')}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { handleCreate(e) }}>
            <div className="py-4 space-y-4 max-h-[60vh] overflow-y-auto pr-1">
              <div className="space-y-1.5">
                <Label htmlFor="tmpl-name">{t('vaktaware.templates.labelTemplateName')}</Label>
                <Input id="tmpl-name" value={name} onChange={(e) => { setName(e.target.value); }} required />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="tmpl-from-name">{t('vaktaware.templates.labelFromName')}</Label>
                  <Input id="tmpl-from-name" value={fromName} onChange={(e) => { setFromName(e.target.value); }} required />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="tmpl-from-email">{t('vaktaware.templates.labelFromEmail')}</Label>
                  <Input id="tmpl-from-email" type="email" value={fromEmail} onChange={(e) => { setFromEmail(e.target.value); }} required />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="tmpl-subject">{t('vaktaware.templates.labelSubject')}</Label>
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
              <Button type="button" variant="outline" onClick={() => { setOpen(false); resetForm() }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={createTemplate.isPending}>
                {createTemplate.isPending ? t('vaktaware.templates.creating') : t('vaktaware.templates.createEntry')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleteId} onOpenChange={(open) => { if (!open) { setDeleteId(null); } }}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('vaktaware.templates.deleteDialogTitle')}</DialogTitle></DialogHeader>
          <p className="text-sm text-secondary py-2">{t('vaktaware.templates.deleteDialogDesc')}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteId(null); }}>{t('common.cancel')}</Button>
            <Button
              variant="destructive"
              onClick={() => { if (deleteId) deleteTemplate.mutate(deleteId, { onSuccess: () => { setDeleteId(null); } }); }}
              disabled={deleteTemplate.isPending}
            >
              {deleteTemplate.isPending ? t('vaktaware.templates.deleting') : t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
