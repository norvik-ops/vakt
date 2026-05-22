import { useState, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Server, ScanSearch, Upload } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { Pagination } from '../../../shared/components/Pagination'
import { ResponsiveTable } from '../../../shared/components/ResponsiveTable'
import type { Column } from '../../../shared/components/ResponsiveTable'
import { useSortableTable } from '../../../shared/hooks/useSortableTable'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import { useAssets, useCreateAsset, useImportAssets } from '../hooks/useAssets'
import { useFirstAction } from '../../../shared/hooks/useFirstAction'
import type { Asset } from '../types'
import type { CreateAssetInput, ImportAssetsResult } from '../hooks/useAssets'
import { toast } from '../../../shared/hooks/useToast'
import { Skeleton } from '../../../components/ui/skeleton'
import { ErrorState } from '../../../shared/components/ErrorState'
import { CSVImportDialog } from '../../../shared/components/CSVImportDialog'
import { formatLocale } from '../../../shared/utils/locale'

const CRITICALITY_ORDER: Record<Asset['criticality'], number> = {
  critical: 4, high: 3, medium: 2, low: 1,
}

type SortableAsset = Asset & { criticality_order: number }

const criticalityVariant: Record<Asset['criticality'], React.ComponentProps<typeof Badge>['variant']> = {
  low: 'secondary',
  medium: 'warning',
  high: 'outline',
  critical: 'destructive',
}

const criticalityClass: Record<Asset['criticality'], string> = {
  low:      '',
  medium:   '',
  high:     'border-transparent bg-severity-high-bg text-severity-high',
  critical: '',
}

const assetTypeLabels: Record<Asset['type'], string> = {
  web_app: 'Web App',
  server: 'Server',
  database: 'Database',
  container: 'Container',
  repo: 'Repository',
}

const emptyForm: CreateAssetInput = {
  name: '',
  type: 'server',
  target: '',
  criticality: 'medium',
  tags: [],
}

function ASSET_COLUMNS(t: (key: string) => string): Column<SortableAsset>[] {
  return [
    {
      key: 'name',
      label: t('secpulse.assetsPage.colName'),
      mobileTitle: true,
      render: (row) => <span className="font-medium">{row.name}</span>,
    },
    {
      key: 'type',
      label: t('secpulse.assetsPage.colType'),
      render: (row) => <span>{assetTypeLabels[row.type]}</span>,
    },
    {
      key: 'target',
      label: t('secpulse.assetsPage.colTarget'),
      mobileHide: true,
      render: (row) => <span className="font-mono text-xs text-secondary">{row.target}</span>,
    },
    {
      key: 'criticality',
      label: t('secpulse.assetsPage.colCriticality'),
      render: (row) => (
        <Badge
          variant={criticalityVariant[row.criticality]}
          className={criticalityClass[row.criticality]}
        >
          {row.criticality}
        </Badge>
      ),
    },
    {
      key: 'tags',
      label: t('secpulse.assetsPage.colTags'),
      mobileHide: true,
      render: (row) => (
        <div className="flex flex-wrap gap-1">
          {row.tags.map((tag) => (
            <Badge key={tag} variant="outline" className="text-xs">
              {tag}
            </Badge>
          ))}
        </div>
      ),
    },
    {
      key: 'created_at',
      label: t('common.date'),
      render: (row) => (
        <span className="text-sm text-secondary">
          {new Date(row.created_at).toLocaleDateString(formatLocale())}
        </span>
      ),
    },
  ]
}

export default function AssetsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const { data: rawAssets, isLoading, isError, error, pagination, refetch } = useAssets(page)
  useFirstAction('asset:first-created', (rawAssets?.length ?? 0) > 0)
  const assetsWithOrder: SortableAsset[] = (rawAssets ?? []).map((a) => ({
    ...a,
    criticality_order: CRITICALITY_ORDER[a.criticality] ?? 0,
  }))
  const { sorted: sortedAssets } = useSortableTable<SortableAsset>(
    assetsWithOrder, { key: 'name', dir: 'asc' },
  )
  const assets = rawAssets // keep for length check
  const sortedAssetsForRender = sortedAssets
  const createAsset = useCreateAsset()
  const importAssets = useImportAssets()
  const [open, setOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [csvImportOpen, setCsvImportOpen] = useState(false)
  const [importResult, setImportResult] = useState<ImportAssetsResult | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [form, setForm] = useState<CreateAssetInput>(emptyForm)
  const [tagsInput, setTagsInput] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  function handleOpen() {
    setForm(emptyForm)
    setTagsInput('')
    setFormError(null)
    setOpen(true)
  }

  function handleImportFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    const fd = new FormData()
    fd.append('file', file)
    importAssets.mutate(fd, {
      onSuccess: (result) => {
        setImportResult(result)
        if (result.errors.length === 0) {
          toast(`${result.inserted} Assets importiert`, 'success')
        } else {
          toast(`${result.inserted} importiert, ${result.errored} Fehler`, 'info')
        }
      },
      onError: (err) => {
        setImportResult({ inserted: 0, errored: 0, errors: [err.message] })
        toast(`Fehler: ${err.message}`, 'error')
      },
    })
    e.target.value = ''
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)
    const tags = tagsInput
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean)
    try {
      await createAsset.mutateAsync({ ...form, tags })
      setOpen(false)
      toast('Erfolgreich erstellt', 'success')
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to create asset'
      setFormError(msg)
      toast(`Fehler: ${msg}`, 'error')
    }
  }

  return (
    <div className="flex flex-col h-full">
      <CSVImportDialog
        open={csvImportOpen}
        onClose={() => setCsvImportOpen(false)}
        endpoint="/api/v1/secpulse/assets/import/csv"
        entityLabel="Assets"
        columns={['name', 'type', 'target', 'criticality', 'tags']}
        onSuccess={() => void refetch()}
      />
      <PageHeader
        title={t('secpulse.assetsPage.title')}
        description={t('secpulse.assetsPage.description')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setCsvImportOpen(true)}>
              <Upload className="w-4 h-4 mr-1" />
              {t('secpulse.assetsPage.csvImport')}
            </Button>
            <Button onClick={handleOpen}>
              <Plus className="w-4 h-4 mr-1" />
              {t('secpulse.assetsPage.newAsset')}
            </Button>
          </div>
        }
      />

      <InfoBanner icon={ScanSearch} title={t('secpulse.assetsPage.scannerInfo')}>
        <p>Vakt Scan orchestriert lokale Scanner wie <strong>Trivy</strong>, <strong>Nuclei</strong> und <strong>OpenVAS</strong>. Lege zuerst ein Asset (Server, Web-App, Repo …) an — dann startest du einen Scan direkt aus der Asset-Detailseite.</p>
        <p className="mt-1">Die Scanner müssen von deiner Vakt-Instanz aus erreichbar sein. URLs und Credentials trägst du in <strong>Settings → Scanner</strong> ein.</p>
      </InfoBanner>

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full rounded-lg" />
            ))}
          </div>
        )}

        {isError && (
          <ErrorState
            message={error?.message}
            onRetry={() => void refetch()}
          />
        )}

        {!isLoading && !isError && assets && assets.length === 0 && (
          <EmptyState
            icon={Server}
            title="Noch kein Asset angelegt"
            description="Leg deinen ersten Server, Web-App oder Container an — dann startest du deinen ersten Scan"
            action={
              <Button onClick={handleOpen}>
                <Plus className="w-4 h-4 mr-1" />
                Asset anlegen
              </Button>
            }
          />
        )}

        {!isLoading && !isError && assets && assets.length > 0 && (
          <ResponsiveTable<SortableAsset>
            keyField="id"
            data={sortedAssetsForRender}
            onRowClick={(asset) => navigate(`/secpulse/assets/${asset.id}`)}
            columns={ASSET_COLUMNS(t)}
          />
        )}
        <Pagination
          page={page}
          totalPages={pagination?.total_pages ?? 1}
          onPageChange={setPage}
        />
      </div>

      {/* CSV Import Dialog */}
      <Dialog open={importOpen} onOpenChange={(v) => { if (!v) setImportOpen(false) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('secpulse.assetsPage.importDialogTitle')}</DialogTitle>
            <DialogDescription>
              {t('secpulse.assetsPage.importDialogDesc')}
            </DialogDescription>
          </DialogHeader>
          <div className="py-4 space-y-4">
            <input
              type="file"
              accept=".csv"
              ref={fileInputRef}
              className="w-full text-sm text-primary file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border file:border-border file:bg-surface2 file:text-xs file:font-medium file:text-primary hover:file:bg-surface cursor-pointer"
              onChange={handleImportFile}
              disabled={importAssets.isPending}
            />
            {importAssets.isPending && (
              <p className="text-sm text-secondary flex items-center gap-2">
                <Spinner size="sm" className="w-3.5 h-3.5" />
                {t('secpulse.assetsPage.importing')}
              </p>
            )}
            {importResult && (
              <div className={`p-3 rounded-lg text-sm space-y-1 ${importResult.errors.length > 0 ? 'bg-yellow-500/10' : 'bg-green-500/10'}`}>
                <p className="font-medium">{t('secpulse.assetsPage.importResult', { inserted: importResult.inserted, errored: importResult.errored })}</p>
                {importResult.errors.map((e, i) => (
                  <p key={i} className="text-xs text-red-400">{e}</p>
                ))}
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportOpen(false)}>{t('common.close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('secpulse.assetsPage.newAssetDialogTitle')}</DialogTitle>
            <DialogDescription>{t('secpulse.assetsPage.newAssetDialogDesc')}</DialogDescription>
          </DialogHeader>
          <form onSubmit={(e) => { void handleSubmit(e) }}>
            <div className="space-y-4 py-2">
              <div className="space-y-1">
                <Label htmlFor="asset-name">{t('secpulse.assetsPage.labelName')}</Label>
                <Input
                  id="asset-name"
                  placeholder="My Web App"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  required
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-type">{t('secpulse.assetsPage.labelType')}</Label>
                <Select
                  value={form.type}
                  onValueChange={(val) => setForm({ ...form, type: val as Asset['type'] })}
                >
                  <SelectTrigger id="asset-type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="web_app">Web App</SelectItem>
                    <SelectItem value="server">Server</SelectItem>
                    <SelectItem value="database">Database</SelectItem>
                    <SelectItem value="container">Container</SelectItem>
                    <SelectItem value="repo">Repository</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-target">{t('secpulse.assetsPage.labelTarget')}</Label>
                <Input
                  id="asset-target"
                  placeholder="https://example.com or 192.168.1.1"
                  value={form.target}
                  onChange={(e) => setForm({ ...form, target: e.target.value })}
                  required
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-criticality">{t('secpulse.assetsPage.labelCriticality')}</Label>
                <Select
                  value={form.criticality}
                  onValueChange={(val) => setForm({ ...form, criticality: val as Asset['criticality'] })}
                >
                  <SelectTrigger id="asset-criticality">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="low">Low</SelectItem>
                    <SelectItem value="medium">Medium</SelectItem>
                    <SelectItem value="high">High</SelectItem>
                    <SelectItem value="critical">Critical</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-tags">{t('secpulse.assetsPage.labelTags')}</Label>
                <Input
                  id="asset-tags"
                  placeholder={t('secpulse.assetsPage.placeholderTags')}
                  value={tagsInput}
                  onChange={(e) => setTagsInput(e.target.value)}
                />
              </div>

              {formError && (
                <p className="text-sm text-red-600">{formError}</p>
              )}
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" disabled={createAsset.isPending}>
                {createAsset.isPending ? (
                  <Spinner size="sm" color="white" className="mr-2" />
                ) : null}
                {createAsset.isPending ? t('secpulse.assetsPage.creating') : t('secpulse.assetsPage.createAsset')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
