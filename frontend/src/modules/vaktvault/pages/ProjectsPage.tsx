import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Lock, Plus, Trash2 } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { useProjects, useCreateProject, useDeleteProject } from '../hooks/useProjects'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

export default function ProjectsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { formatDate } = useFormatDate()
  const { data: projects, isLoading } = useProjects()
  const createProject = useCreateProject()
  const deleteProject = useDeleteProject()

  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [deleteId, setDeleteId] = useState<string | null>(null)

  function handleCreate() {
    if (!name.trim()) return
    createProject.mutate(
      { name: name.trim(), description: description.trim() || undefined },
      {
        onSuccess: () => {
          setShowCreate(false)
          setName('')
          setDescription('')
        },
      },
    )
  }

  function handleDelete() {
    if (!deleteId) return
    deleteProject.mutate(deleteId, {
      onSuccess: () => { setDeleteId(null); },
    })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktvault.projectsPage.title')}
        description={t('vaktvault.projectsPage.description')}
        tier="pro"
        actions={
          <Button onClick={() => { setShowCreate(true); }}>
            <Plus className="w-4 h-4" />
            {t('vaktvault.projectsPage.newProject')}
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading ? (
          <div className="flex items-center justify-center h-40">
            <Spinner size="lg" />
          </div>
        ) : !projects || projects.length === 0 ? (
          <EmptyState
            icon={Lock}
            title={t('vaktvault.projectsPage.noProjects')}
            description={t('vaktvault.projectsPage.noProjectsDesc')}
            action={
              <Button onClick={() => { setShowCreate(true); }}>
                <Plus className="w-4 h-4 mr-1" />
                {t('vaktvault.projectsPage.newProject')}
              </Button>
            }
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {projects.map((project) => (
              <Card
                key={project.id}
                className="cursor-pointer hover:shadow-md transition-shadow"
                onClick={() => { navigate(`/vaktvault/projects/${project.id}`); }}
              >
                <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-semibold truncate max-w-[160px]">
                    {project.name}
                  </CardTitle>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 text-secondary hover:text-red-600 -mr-1 -mt-1 shrink-0"
                    onClick={(e) => {
                      e.stopPropagation()
                      setDeleteId(project.id)
                    }}
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </CardHeader>
                <CardContent>
                  {project.description && (
                    <p className="text-sm text-secondary mb-3 line-clamp-2">{project.description}</p>
                  )}
                  <p className="text-xs text-secondary">
                    {t('vaktvault.projectsPage.createdOn')}{' '}
                    {formatDate(project.created_at, { year: 'numeric', month: 'short', day: 'numeric' })}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>

      {/* Create dialog */}
      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktvault.projectsPage.createDialogTitle')}</DialogTitle>
            <DialogDescription>
              {t('vaktvault.projectsPage.createDialogDesc')}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="project-name">{t('vaktvault.projectsPage.labelName')}</Label>
              <Input
                id="project-name"
                placeholder="my-service"
                value={name}
                onChange={(e) => { setName(e.target.value); }}
                onKeyDown={(e) => { if (e.key === 'Enter') { handleCreate(); } }}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="project-desc">{t('vaktvault.projectsPage.labelDescription')}</Label>
              <Input
                id="project-desc"
                placeholder={t('vaktvault.projectsPage.placeholderDescription')}
                value={description}
                onChange={(e) => { setDescription(e.target.value); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setShowCreate(false); }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleCreate} disabled={!name.trim() || createProject.isPending}>
              {createProject.isPending ? t('vaktvault.projectsPage.creating') : t('vaktvault.projectsPage.create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation dialog */}
      <Dialog open={Boolean(deleteId)} onOpenChange={(open) => { if (!open) { setDeleteId(null); } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktvault.projectsPage.deleteDialogTitle')}</DialogTitle>
            <DialogDescription>
              {t('vaktvault.projectsPage.deleteDialogDesc')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteId(null); }}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteProject.isPending}
            >
              {deleteProject.isPending ? t('vaktvault.projectsPage.deleting') : t('vaktvault.projectsPage.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
