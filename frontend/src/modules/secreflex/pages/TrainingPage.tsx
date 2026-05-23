import { useState } from 'react'
import { GraduationCap, ChevronDown, ChevronUp, UserPlus, Download } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Label } from '../../../components/ui/label'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useTrainingModules, useAssignments, useAssignModule } from '../hooks/useTraining'
import type { TrainingModule } from '../types'
import { assignmentStatusVariant } from '../../../lib/statusMapping'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

async function downloadCertificate(assignmentId: string) {
  const response = await fetch(`/api/v1/secreflex/assignments/${assignmentId}/certificate`, {
    credentials: 'include',
  })
  if (!response.ok) return
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `certificate-${assignmentId}.pdf`
  a.click()
  URL.revokeObjectURL(url)
}


function ModuleRow({ module }: { module: TrainingModule }) {
  const [expanded, setExpanded] = useState(false)
  const [assignOpen, setAssignOpen] = useState(false)
  const [emails, setEmails] = useState('')
  const { data: assignments, isLoading } = useAssignments(expanded ? module.id : '')
  const assignModule = useAssignModule(module.id)
  const { formatDate } = useFormatDate()

  function handleAssign(e: React.FormEvent) {
    e.preventDefault()
    const emailList = emails.split(/[\s,;]+/).map((e) => e.trim()).filter(Boolean)
    if (emailList.length === 0) return
    assignModule.mutate({ user_emails: emailList }, {
      onSuccess: () => {
        setAssignOpen(false)
        setEmails('')
      },
    })
  }

  return (
    <div className="border border-border rounded-lg bg-surface overflow-x-auto">
      <div
        className="flex items-center gap-4 p-4 cursor-pointer hover:bg-surface2"
        onClick={() => { setExpanded(!expanded); }}
      >
        <GraduationCap className="w-5 h-5 text-blue-500 shrink-0" />
        <div className="flex-1 min-w-0">
          <p className="font-medium text-sm">{module.title}</p>
          <p className="text-xs text-secondary truncate">{module.description}</p>
        </div>
        <span className="text-xs text-secondary">Pass: {module.passing_score}%</span>
        <Button
          variant="outline"
          size="sm"
          className="gap-1"
          onClick={(e) => { e.stopPropagation(); setAssignOpen(true) }}
        >
          <UserPlus className="w-3.5 h-3.5" />
          Assign
        </Button>
        {expanded ? <ChevronUp className="w-4 h-4 text-secondary" /> : <ChevronDown className="w-4 h-4 text-secondary" />}
      </div>

      {expanded && (
        <div className="border-t border-border p-4">
          {isLoading ? (
            <div className="flex justify-center py-4">
              <Spinner size="sm" />
            </div>
          ) : !assignments || assignments.length === 0 ? (
            <p className="text-sm text-secondary text-center py-4">No assignments yet.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Benutzer</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Punktzahl</TableHead>
                  <TableHead>Zugewiesen</TableHead>
                  <TableHead>Abgeschlossen</TableHead>
                  <TableHead>Zertifikat</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {assignments.map((a) => (
                  <TableRow key={a.id}>
                    <TableCell className="font-mono text-xs">{a.user_email}</TableCell>
                    <TableCell>
                      <Badge variant={assignmentStatusVariant[a.status]} className="capitalize">{a.status}</Badge>
                    </TableCell>
                    <TableCell className="text-sm">
                      {a.score != null ? `${a.score}%` : '—'}
                    </TableCell>
                    <TableCell className="text-sm text-secondary">
                      {formatDate(a.assigned_at)}
                    </TableCell>
                    <TableCell className="text-sm text-secondary">
                      {a.completed_at ? formatDate(a.completed_at) : '—'}
                    </TableCell>
                    <TableCell>
                      {a.status === 'completed' && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="gap-1"
                          onClick={() => { void downloadCertificate(a.id) }}
                          title="Zertifikat herunterladen"
                        >
                          <Download className="w-3.5 h-3.5" />
                          PDF
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      )}

      <Dialog open={assignOpen} onOpenChange={setAssignOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Training zuweisen: {module.title}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { handleAssign(e) }}>
            <div className="py-4 space-y-1.5">
              <Label htmlFor="assign-emails">Benutzer-E-Mails</Label>
              <textarea
                id="assign-emails"
                rows={4}
                className="w-full rounded-md border border-border px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
                placeholder="user@example.com&#10;other@example.com"
                value={emails}
                onChange={(e) => { setEmails(e.target.value); }}
              />
              <p className="text-xs text-secondary">One email per line, or comma/semicolon separated.</p>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setAssignOpen(false); }}>Abbrechen</Button>
              <Button type="submit" disabled={assignModule.isPending}>
                {assignModule.isPending ? 'Assigning…' : 'Assign'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function TrainingPage() {
  const { data: modules, isLoading } = useTrainingModules()

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Sicherheitstraining"
        description="Sicherheitstrainings für Benutzer zuweisen und verwalten."
      />

      <div className="flex-1 p-6 space-y-3">
        {isLoading ? (
          <div className="flex justify-center py-16">
            <Spinner size="md" />
          </div>
        ) : !modules || modules.length === 0 ? (
          <EmptyState
            icon={GraduationCap}
            title="Noch keine Trainingsmodule vorhanden"
            description="Trainingsmodule werden von Ihrem Plattform-Administrator konfiguriert."
          />
        ) : (
          modules.map((m) => <ModuleRow key={m.id} module={m} />)
        )}
      </div>
    </div>
  )
}
