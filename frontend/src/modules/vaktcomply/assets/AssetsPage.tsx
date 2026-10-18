import { useState } from 'react'
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
import { useAssets, useCreateAsset } from './hooks'
import { useFirstAction } from '../../../shared/hooks/useFirstAction'
import { useFeature } from '../../../shared/hooks/useFeature'
import { ClassificationBadge } from './ClassificationBadge'
import type { Asset, ClassificationLevel } from './types'
import type { CreateAssetInput } from './hooks'
import { toast } from '../../../shared/hooks/useToast'
import { Skeleton } from '../../../components/ui/skeleton'
import { ErrorState } from '../../../shared/components/ErrorState'
import { CSVImportDialog } from '../../../shared/components/CSVImportDialog'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

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
  external_url: '',
  criticality: 'medium',
  tags: [],
  classification: 'internal',
}

function ASSET_COLUMNS(t: (key: string) => string, formatDate: (v: string) => string): Column<SortableAsset>[] {
  return [
    {
      key: 'name',
      label: t('vaktcomply.assets.page.colName'),
      mobileTitle: true,
      render: (row) => <span className="font-medium">{row.name}</span>,
    },
    {
      key: 'type',
      label: t('vaktcomply.assets.page.colType'),
      render: (row) => <span>{assetTypeLabels[row.type]}</span>,
    },
    {
      key: 'external_url',
      label: t('vaktcomply.assets.page.colTarget'),
      mobileHide: true,
      render: (row) => (
        <span className="font-mono text-xs text-secondary">
          {row.external_url || <span className="text-secondary/40">—</span>}
        </span>
      ),
    },
    {
      key: 'criticality',
      label: t('vaktcomply.assets.page.colCriticality'),
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
      label: t('vaktcomply.assets.page.colTags'),
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
      key: 'classification',
      label: t('vaktcomply.assets.page.colClassification'),
      mobileHide: true,
      render: (row) => row.classification
        ? <ClassificationBadge level={row.classification} />
        : <span className="text-xs text-muted-foreground">—</span>,
    },
    {
      key: 'created_at',
      label: t('common.date'),
      render: (row) => (
        <span className="text-sm text-secondary">
          {formatDate(row.created_at)}
        </span>
      ),
    },
  ]
}

export default function AssetsPage() {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const { data: rawAssets, isLoading, isError, error, pagination, refetch } = useAssets(page)
  useFirstAction('asset:first-created', (rawAssets?.length ?? 0) > 0)
  const assetsWithOrder: SortableAsset[] = (rawAssets ?? []).map((a) => ({
    ...a,
    criticality_order: CRITICALITY_ORDER[a.criticality],
  }))
  const { sorted: sortedAssets } = useSortableTable<SortableAsset>(
    assetsWithOrder, { key: 'name', dir: 'asc' },
  )
  const assets = rawAssets // keep for length check
  const sortedAssetsForRender = sortedAssets
  const createAsset = useCreateAsset()
  const [open, setOpen] = useState(false)
  const [csvImportOpen, setCsvImportOpen] = useState(false)
  const [form, setForm] = useState<CreateAssetInput>(emptyForm)

  // R23-02: the page carried a SECOND, hand-rolled import dialog for
  // POST /vaktcomply/assets/import, and its `importOpen` state was only ever set
  // to false — no call site anywhere set it to true, so the dialog could not be
  // opened and useImportAssets had exactly one caller, a dead one.
  //
  // Deleting it would have hidden a worse problem than it solved. The two
  // endpoints are not duplicates: /vaktcomply/assets/import is Community, while
  // /vaktcomply/assets/import/csv is gated behind FeatureSecPulse
  // (backend vaktcomply/assets/routes.go). The only REACHABLE button pointed at the
  // Pro route, so on a Community licence asset CSV import answered 402 and the
  // route that would have worked had no way in. Picking the endpoint by licence
  // keeps one button and makes the feature work on both tiers.
  const { enabled: hasSecPulse } = useFeature('vaktscan_advanced')
  const importEndpoint = hasSecPulse ? '/vaktcomply/assets/import/csv' : '/vaktcomply/assets/import'
  const [tagsInput, setTagsInput] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  function handleOpen() {
    setForm(emptyForm)
    setTagsInput('')
    setFormError(null)
    setOpen(true)
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
      toast(t('vaktcomply.assets.page.created'), 'success')
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to create asset'
      setFormError(msg)
      toast(`${t('common.error')}: ${msg}`, 'error')
    }
  }

  return (
    <div className="flex flex-col h-full">
      <CSVImportDialog
        open={csvImportOpen}
        onClose={() => { setCsvImportOpen(false); }}
        endpoint={importEndpoint}
        entityLabel="Assets"
        columns={['name', 'type', 'external_url', 'criticality', 'tags']}
        onSuccess={() => void refetch()}
      />
      <PageHeader
        title={t('vaktcomply.assets.page.title')}
        description={t('vaktcomply.assets.page.description')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { setCsvImportOpen(true); }}>
              <Upload className="w-4 h-4 mr-1" />
              {t('vaktcomply.assets.page.csvImport')}
            </Button>
            <Button onClick={handleOpen}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vaktcomply.assets.page.newAsset')}
            </Button>
          </div>
        }
      />

      <InfoBanner icon={ScanSearch} title={t('vaktcomply.assets.page.scannerInfo')}>
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
            title={t('vaktcomply.assets.page.emptyTitle')}
            description={t('vaktcomply.assets.page.emptyDesc')}
            action={
              <Button onClick={handleOpen}>
                <Plus className="w-4 h-4 mr-1" />
                {t('vaktcomply.assets.page.addAsset')}
              </Button>
            }
          />
        )}

        {!isLoading && !isError && assets && assets.length > 0 && (
          <ResponsiveTable<SortableAsset>
            keyField="id"
            data={sortedAssetsForRender}
            onRowClick={(asset) => { navigate(`/vaktcomply/assets/${asset.id}`); }}
            columns={ASSET_COLUMNS(t, formatDate)}
          />
        )}
        <Pagination
          page={page}
          totalPages={pagination?.total_pages ?? 1}
          onPageChange={setPage}
        />
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.assets.page.newAssetDialogTitle')}</DialogTitle>
            <DialogDescription>{t('vaktcomply.assets.page.newAssetDialogDesc')}</DialogDescription>
          </DialogHeader>
          <form onSubmit={(e) => { void handleSubmit(e) }}>
            <div className="space-y-4 py-2">
              <div className="space-y-1">
                <Label htmlFor="asset-name">{t('vaktcomply.assets.page.labelName')}</Label>
                <Input
                  id="asset-name"
                  placeholder="My Web App"
                  value={form.name}
                  onChange={(e) => { setForm({ ...form, name: e.target.value }); }}
                  required
                  onInvalid={(e) => { (e.target as HTMLInputElement).setCustomValidity(t('validation.required')); }}
                  onInput={(e) => { (e.target as HTMLInputElement).setCustomValidity(''); }}
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-type">{t('vaktcomply.assets.page.labelType')}</Label>
                <Select
                  value={form.type}
                  onValueChange={(val) => { setForm({ ...form, type: val as Asset['type'] }); }}
                >
                  <SelectTrigger id="asset-type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="web_app">Web App</SelectItem>
                    <SelectItem value="server">{t('vaktcomply.assets.page.typeServer')}</SelectItem>
                    <SelectItem value="database">{t('vaktcomply.assets.page.typeDatabase')}</SelectItem>
                    <SelectItem value="container">{t('vaktcomply.assets.page.typeContainer')}</SelectItem>
                    <SelectItem value="repo">{t('vaktcomply.assets.page.typeRepo')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-target">{t('vaktcomply.assets.page.labelTarget')}</Label>
                <Input
                  id="asset-target"
                  placeholder="https://example.com or 192.168.1.1"
                  value={form.external_url}
                  onChange={(e) => { setForm({ ...form, external_url: e.target.value }); }}
                  required
                  onInvalid={(e) => { (e.target as HTMLInputElement).setCustomValidity(t('validation.required')); }}
                  onInput={(e) => { (e.target as HTMLInputElement).setCustomValidity(''); }}
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-criticality">{t('vaktcomply.assets.page.labelCriticality')}</Label>
                <Select
                  value={form.criticality}
                  onValueChange={(val) => { setForm({ ...form, criticality: val as Asset['criticality'] }); }}
                >
                  <SelectTrigger id="asset-criticality">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="low">{t('vaktcomply.assets.criticality.low')}</SelectItem>
                    <SelectItem value="medium">{t('vaktcomply.assets.criticality.medium')}</SelectItem>
                    <SelectItem value="high">{t('vaktcomply.assets.criticality.high')}</SelectItem>
                    <SelectItem value="critical">{t('vaktcomply.assets.criticality.critical')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-classification">{t('vaktcomply.assets.page.labelClassification')}</Label>
                <Select
                  value={form.classification ?? 'internal'}
                  onValueChange={(val) => { setForm({ ...form, classification: val as ClassificationLevel }); }}
                >
                  <SelectTrigger id="asset-classification">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="public">{t('vaktcomply.assets.classification.public')}</SelectItem>
                    <SelectItem value="internal">{t('vaktcomply.assets.classification.internal')}</SelectItem>
                    <SelectItem value="confidential">{t('vaktcomply.assets.classification.confidential')}</SelectItem>
                    <SelectItem value="restricted">{t('vaktcomply.assets.classification.restricted')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-tags">{t('vaktcomply.assets.page.labelTags')}</Label>
                <Input
                  id="asset-tags"
                  placeholder={t('vaktcomply.assets.page.placeholderTags')}
                  value={tagsInput}
                  onChange={(e) => { setTagsInput(e.target.value); }}
                />
              </div>

              {formError && (
                <p className="text-sm text-red-600">{formError}</p>
              )}
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setOpen(false); }}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" disabled={createAsset.isPending}>
                {createAsset.isPending ? (
                  <Spinner size="sm" color="white" className="mr-2" />
                ) : null}
                {createAsset.isPending ? t('vaktcomply.assets.page.creating') : t('vaktcomply.assets.page.createAsset')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
