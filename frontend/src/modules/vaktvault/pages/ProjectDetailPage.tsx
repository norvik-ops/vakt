import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Plus, Eye, EyeOff, Trash2, Key, ClipboardList } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../../components/ui/tabs'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { useProject, useProjectHealth } from '../hooks/useProjects'
import { useEnvironments, useCreateEnvironment, useSecretKeys, useUpsertSecret, useDeleteSecret, useSecretValue, useProjectAccessLog } from '../hooks/useSecrets'
import type { Environment } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

function healthScoreColor(score: number) {
  if (score >= 80) return 'text-green-600'
  if (score >= 50) return 'text-yellow-600'
  return 'text-red-600'
}

function SecretRow({
  secretKey,
  projectId,
  envId,
}: {
  secretKey: string
  projectId: string
  envId: string
}) {
  const { t } = useTranslation()
  const [reveal, setReveal] = useState(false)
  const [editing, setEditing] = useState(false)
  const [newValue, setNewValue] = useState('')
  const deleteSecret = useDeleteSecret(projectId, envId)
  const upsertSecret = useUpsertSecret(projectId, envId)
  const { data: secretData } = useSecretValue(projectId, envId, secretKey, reveal)

  function handleSave() {
    upsertSecret.mutate({ key: secretKey, value: newValue }, {
      onSuccess: () => {
        setEditing(false)
        setNewValue('')
        setReveal(false)
      },
    })
  }

  return (
    <div className="flex items-center gap-3 p-3 bg-surface border border-border rounded-md">
      <Key className="w-4 h-4 text-secondary shrink-0" />
      <span className="font-mono text-sm font-medium flex-1 truncate">{secretKey}</span>

      {reveal && secretData ? (
        <span className="font-mono text-xs text-secondary max-w-48 truncate">{secretData.value}</span>
      ) : (
        <span className="font-mono text-xs text-secondary">••••••••</span>
      )}

      {editing ? (
        <div className="flex items-center gap-2">
          <Input
            className="h-7 text-xs w-40"
            placeholder="Neuer Wert…"
            value={newValue}
            onChange={(e) => { setNewValue(e.target.value); }}
            autoFocus
          />
          <Button size="sm" onClick={handleSave} disabled={upsertSecret.isPending}>{t('vault.projectDetail.secretSave')}</Button>
          <Button size="sm" variant="outline" onClick={() => { setEditing(false); }}>{t('vault.projectDetail.cancel')}</Button>
        </div>
      ) : (
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => { setReveal(!reveal); }}
            className="h-7 w-7 p-0"
          >
            {reveal ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => { setEditing(true); }}
            className="h-7 px-2 text-xs"
          >
            {t('vault.projectDetail.secretEdit')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 w-7 p-0 text-red-500 hover:text-red-700"
            onClick={() => { deleteSecret.mutate(secretKey); }}
            disabled={deleteSecret.isPending}
          >
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        </div>
      )}
    </div>
  )
}

function EnvTab({ projectId, env }: { projectId: string; env: Environment }) {
  const { t } = useTranslation()
  const { data: keys, isLoading } = useSecretKeys(projectId, env.id)
  const upsertSecret = useUpsertSecret(projectId, env.id)
  const [addOpen, setAddOpen] = useState(false)
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')
  const [keyTouched, setKeyTouched] = useState(false)

  function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    setKeyTouched(true)
    if (!newKey.trim()) return
    upsertSecret.mutate({ key: newKey.trim(), value: newValue }, {
      onSuccess: () => {
        setAddOpen(false)
        setNewKey('')
        setNewValue('')
        setKeyTouched(false)
      },
    })
  }

  function handleOpenChange(open: boolean) {
    if (!open) setKeyTouched(false)
    setAddOpen(open)
  }

  return (
    <div className="space-y-3">
      <div className="flex justify-end">
        <Button size="sm" onClick={() => { setAddOpen(true); }}>
          <Plus className="w-4 h-4 mr-1" />
          {t('vault.projectDetail.addSecret')}
        </Button>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-8">
          <Spinner size="md" />
        </div>
      ) : !keys || keys.length === 0 ? (
        <p className="text-sm text-secondary py-6 text-center">{t('vault.projectDetail.noSecrets')}</p>
      ) : (
        <div className="space-y-2">
          {keys.map((k) => (
            <SecretRow key={k} secretKey={k} projectId={projectId} envId={env.id} />
          ))}
        </div>
      )}

      <Dialog open={addOpen} onOpenChange={handleOpenChange}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('vault.projectDetail.addSecret')}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { handleAdd(e) }}>
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="secret-key">{t('vault.projectDetail.secretKey')}</Label>
                <Input
                  id="secret-key"
                  placeholder="DATABASE_URL"
                  value={newKey}
                  onChange={(e) => { setNewKey(e.target.value); }}
                  onBlur={() => { setKeyTouched(true); }}
                  aria-invalid={keyTouched && !newKey.trim()}
                />
                {keyTouched && !newKey.trim() && (
                  <p className="text-sm text-destructive mt-1">{t('vault.projectDetail.keyRequired')}</p>
                )}
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="secret-val">{t('vault.projectDetail.secretValue')}</Label>
                <Input id="secret-val" type="password" placeholder="••••••" value={newValue} onChange={(e) => { setNewValue(e.target.value); }} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { handleOpenChange(false); }}>{t('vault.projectDetail.cancel')}</Button>
              <Button type="submit" disabled={upsertSecret.isPending}>{upsertSecret.isPending ? t('vault.projectDetail.secretSaving') : t('vault.projectDetail.secretSave')}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function ProjectDetailPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDateTime } = useFormatDate()
  const projectId = id ?? ''

  const { data: project, isLoading, error } = useProject(projectId)
  const { data: health } = useProjectHealth(projectId)
  const { data: environments } = useEnvironments(projectId)
  const [logPage, setLogPage] = useState(1)
  const LOG_LIMIT = 25
  const { data: accessLogData } = useProjectAccessLog(projectId, logPage, LOG_LIMIT)
  const createEnv = useCreateEnvironment(projectId)

  const [envDialogOpen, setEnvDialogOpen] = useState(false)
  const [envName, setEnvName] = useState('')
  const [envNameTouched, setEnvNameTouched] = useState(false)
  const [activeEnv, setActiveEnv] = useState<string>('')

  const envList = environments ?? []
  const selectedEnv = activeEnv || envList[0]?.id || ''

  function handleCreateEnv(e: React.FormEvent) {
    e.preventDefault()
    setEnvNameTouched(true)
    if (!envName.trim()) return
    createEnv.mutate({ name: envName.trim() }, {
      onSuccess: (newEnv) => {
        setActiveEnv(newEnv.id)
        setEnvDialogOpen(false)
        setEnvName('')
        setEnvNameTouched(false)
      },
    })
  }

  function handleEnvDialogChange(open: boolean) {
    if (!open) setEnvNameTouched(false)
    setEnvDialogOpen(open)
  }

  if (isLoading) return (
    <div className="flex justify-center py-16">
      <Spinner size="md" />
    </div>
  )

  if (error || !project) return (
    <div className="p-6">
      <p className="text-sm text-red-600">{error?.message ?? 'Project not found'}</p>
      <Button variant="outline" className="mt-4" onClick={() => { navigate('/vaktvault'); }}>
        <ArrowLeft className="w-4 h-4 mr-1" />{t('common.back')}
      </Button>
    </div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={project.name}
        description={project.description}
        actions={
          <Button variant="outline" size="sm" onClick={() => { navigate('/vaktvault'); }}>
            <ArrowLeft className="w-4 h-4 mr-1" />{t('common.back')}
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {health && (
          <Card>
            <CardContent className="py-4 flex items-center gap-6">
              <div className="text-center">
                <div className={`text-3xl font-bold ${healthScoreColor(health.score)}`}>{health.score}</div>
                <div className="text-xs text-secondary mt-1">Health Score</div>
              </div>
              {health.issues.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {health.issues.map((issue, i) => (
                    <Badge key={i} variant="destructive" className="text-xs">{issue}</Badge>
                  ))}
                </div>
              )}
              {health.issues.length === 0 && (
                <span className="text-sm text-green-600 font-medium">{t('vaktvault.projectDetail.allSecretsHealthy')}</span>
              )}
            </CardContent>
          </Card>
        )}

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-4">
            <CardTitle>{t('vault.projectDetail.secretsTitle')}</CardTitle>
            <Button size="sm" variant="outline" onClick={() => { setEnvDialogOpen(true); }}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vault.projectDetail.addEnv')}
            </Button>
          </CardHeader>
          <CardContent>
            {envList.length === 0 ? (
              <p className="text-sm text-secondary text-center py-6">{t('vault.projectDetail.noEnvs')}</p>
            ) : (
              <Tabs value={selectedEnv} onValueChange={setActiveEnv}>
                <TabsList>
                  {envList.map((env) => (
                    <TabsTrigger key={env.id} value={env.id}>{env.name}</TabsTrigger>
                  ))}
                </TabsList>
                {envList.map((env) => (
                  <TabsContent key={env.id} value={env.id} className="mt-4">
                    <EnvTab projectId={projectId} env={env} />
                  </TabsContent>
                ))}
              </Tabs>
            )}
          </CardContent>
        </Card>

        {/* Access Log */}
        <Card>
          <CardHeader className="flex flex-row items-center gap-2 pb-4">
            <ClipboardList className="w-4 h-4 text-secondary" />
            <CardTitle>{t('vault.projectDetail.accessLog')}</CardTitle>
            {accessLogData && accessLogData.total > 0 && (
              <span className="ml-auto text-xs text-secondary">{t('vault.projectDetail.accessLogEntries', { count: accessLogData.total })}</span>
            )}
          </CardHeader>
          <CardContent>
            {!accessLogData || accessLogData.entries.length === 0 ? (
              <p className="text-sm text-secondary text-center py-6">{t('vault.projectDetail.noAccessLog')}</p>
            ) : (
              <>
                <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('vault.projectDetail.colSecret')}</TableHead>
                      <TableHead>{t('vault.projectDetail.colAction')}</TableHead>
                      <TableHead>{t('vault.projectDetail.colTime')}</TableHead>
                      <TableHead>{t('vault.projectDetail.colIp')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {accessLogData.entries.map((entry) => (
                      <TableRow key={entry.id}>
                        <TableCell className="font-mono text-sm">{entry.secret_key}</TableCell>
                        <TableCell>
                          <Badge variant="outline" className="text-xs">{entry.access_via}</Badge>
                        </TableCell>
                        <TableCell className="text-sm text-secondary">
                          {formatDateTime(entry.accessed_at, { dateStyle: 'short', timeStyle: 'short' })}
                        </TableCell>
                        <TableCell className="text-xs text-secondary font-mono">
                          {entry.ip_address ?? '—'}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
                </div>
                {accessLogData.total > LOG_LIMIT && (
                  <div className="flex items-center justify-between mt-3 pt-3 border-t border-border">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={logPage <= 1}
                      onClick={() => { setLogPage((p) => Math.max(1, p - 1)); }}
                    >
                      {t('common.previousPage')}
                    </Button>
                    <span className="text-xs text-secondary">
                      {t('common.pageOf', { page: logPage, total: Math.ceil(accessLogData.total / LOG_LIMIT) })}
                    </span>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={logPage >= Math.ceil(accessLogData.total / LOG_LIMIT)}
                      onClick={() => { setLogPage((p) => p + 1); }}
                    >
                      {t('common.nextPage')}
                    </Button>
                  </div>
                )}
              </>
            )}
          </CardContent>
        </Card>
      </div>

      <Dialog open={envDialogOpen} onOpenChange={handleEnvDialogChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vault.projectDetail.envDialogTitle')}</DialogTitle>
            <DialogDescription>{t('vault.projectDetail.envDialogDesc')}</DialogDescription>
          </DialogHeader>
          <form onSubmit={(e) => { handleCreateEnv(e) }}>
            <div className="py-4 space-y-1.5">
              <Label htmlFor="env-name">{t('vault.projectDetail.envNameLabel')}</Label>
              <Input
                id="env-name"
                className="mt-1.5"
                placeholder="production"
                value={envName}
                onChange={(e) => { setEnvName(e.target.value); }}
                onBlur={() => { setEnvNameTouched(true); }}
                aria-invalid={envNameTouched && !envName.trim()}
              />
              {envNameTouched && !envName.trim() && (
                <p className="text-sm text-destructive mt-1">{t('vault.projectDetail.envNameRequired')}</p>
              )}
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { handleEnvDialogChange(false); }}>{t('vault.projectDetail.cancel')}</Button>
              <Button type="submit" disabled={createEnv.isPending}>{createEnv.isPending ? t('vault.projectDetail.creating') : t('vault.projectDetail.addEnv')}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
