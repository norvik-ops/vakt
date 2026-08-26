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
import { useAssets, useCreateAsset } from '../hooks/useAssets'
import { useFirstAction } from '../../../shared/hooks/useFirstAction'
import { useFeature } from '../../../shared/hooks/useFeature'
import { ClassificationBadge } from '../components/ClassificationBadge'
import type { Asset, ClassificationLevel } from '../types'
import type { CreateAssetInput } from '../hooks/useAssets'
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
      label: t('vaktscan.assetsPage.colName'),
      mobileTitle: true,
      render: (row) => <span className="font-medium">{row.name}</span>,
    },
    {
      key: 'type',
      label: t('vaktscan.assetsPage.colType'),
      render: (row) => <span>{assetTypeLabels[row.type]}</span>,
    },
    {
      key: 'external_url',
      label: t('vaktscan.assetsPage.colTarget'),
      mobileHide: true,
      render: (row) => (
        <span className="font-mono text-xs text-secondary">
          {row.external_url || <span className="text-secondary/40">—</span>}
        </span>
      ),
    },
    {
      key: 'criticality',
      label: t('vaktscan.assetsPage.colCriticality'),
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
      label: t('vaktscan.assetsPage.colTags'),
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
      label: t('vaktscan.assetsPage.colClassification'),
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
  // POST /vaktscan/assets/import, and its `importOpen` state was only ever set
  // to false — no call site anywhere set it to true, so the dialog could not be
  // opened and useImportAssets had exactly one caller, a dead one.
  //
  // Deleting it would have hidden a worse problem than it solved. The two
  // endpoints are not duplicates: /vaktscan/assets/import is Community, while
  // /vaktscan/assets/import/csv is gated behind FeatureSecPulse
  // (vaktscan/routes.go:31 vs :54). The only REACHABLE button pointed at the
  // Pro route, so on a Community licence asset CSV import answered 402 and the
  // route that would have worked had no way in. Picking the endpoint by licence
  // keeps one button and makes the feature work on both tiers.
  const { enabled: hasSecPulse } = useFeature('vaktscan_advanced')
  const importEndpoint = hasSecPulse ? '/vaktscan/assets/import/csv' : '/vaktscan/assets/import'
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
      toast(t('vaktscan.assetsPage.created'), 'success')
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
        title={t('vaktscan.assetsPage.title')}
        description={t('vaktscan.assetsPage.description')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { setCsvImportOpen(true); }}>
              <Upload className="w-4 h-4 mr-1" />
              {t('vaktscan.assetsPage.csvImport')}
            </Button>
            <Button onClick={handleOpen}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vaktscan.assetsPage.newAsset')}
            </Button>
          </div>
        }
      />

      <InfoBanner icon={ScanSearch} title={t('vaktscan.assetsPage.scannerInfo')}>
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
            title={t('vaktscan.assetsPage.emptyTitle')}
            description={t('vaktscan.assetsPage.emptyDesc')}
            action={
              <Button onClick={handleOpen}>
                <Plus className="w-4 h-4 mr-1" />
                {t('vaktscan.assetsPage.addAsset')}
              </Button>
            }
          />
        )}

        {!isLoading && !isError && assets && assets.length > 0 && (
          <ResponsiveTable<SortableAsset>
            keyField="id"
            data={sortedAssetsForRender}
            onRowClick={(asset) => { navigate(`/vaktscan/assets/${asset.id}`); }}
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
            <DialogTitle>{t('vaktscan.assetsPage.newAssetDialogTitle')}</DialogTitle>
            <DialogDescription>{t('vaktscan.assetsPage.newAssetDialogDesc')}</DialogDescription>
          </DialogHeader>
          <form onSubmit={(e) => { void handleSubmit(e) }}>
            <div className="space-y-4 py-2">
              <div className="space-y-1">
                <Label htmlFor="asset-name">{t('vaktscan.assetsPage.labelName')}</Label>
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
                <Label htmlFor="asset-type">{t('vaktscan.assetsPage.labelType')}</Label>
                <Select
                  value={form.type}
                  onValueChange={(val) => { setForm({ ...form, type: val as Asset['type'] }); }}
                >
                  <SelectTrigger id="asset-type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="web_app">Web App</SelectItem>
                    <SelectItem value="server">{t('vaktscan.assetsPage.typeServer')}</SelectItem>
                    <SelectItem value="database">{t('vaktscan.assetsPage.typeDatabase')}</SelectItem>
                    <SelectItem value="container">{t('vaktscan.assetsPage.typeContainer')}</SelectItem>
                    <SelectItem value="repo">{t('vaktscan.assetsPage.typeRepo')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-target">{t('vaktscan.assetsPage.labelTarget')}</Label>
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
                <Label htmlFor="asset-criticality">{t('vaktscan.assetsPage.labelCriticality')}</Label>
                <Select
                  value={form.criticality}
                  onValueChange={(val) => { setForm({ ...form, criticality: val as Asset['criticality'] }); }}
                >
                  <SelectTrigger id="asset-criticality">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="low">{t('vaktscan.severity.low')}</SelectItem>
                    <SelectItem value="medium">{t('vaktscan.severity.medium')}</SelectItem>
                    <SelectItem value="high">{t('vaktscan.severity.high')}</SelectItem>
                    <SelectItem value="critical">{t('vaktscan.severity.critical')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-classification">{t('vaktscan.assetsPage.labelClassification')}</Label>
                <Select
                  value={form.classification ?? 'internal'}
                  onValueChange={(val) => { setForm({ ...form, classification: val as ClassificationLevel }); }}
                >
                  <SelectTrigger id="asset-classification">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="public">{t('vaktscan.classification.public')}</SelectItem>
                    <SelectItem value="internal">{t('vaktscan.classification.internal')}</SelectItem>
                    <SelectItem value="confidential">{t('vaktscan.classification.confidential')}</SelectItem>
                    <SelectItem value="restricted">{t('vaktscan.classification.restricted')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-tags">{t('vaktscan.assetsPage.labelTags')}</Label>
                <Input
                  id="asset-tags"
                  placeholder={t('vaktscan.assetsPage.placeholderTags')}
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
                {createAsset.isPending ? t('vaktscan.assetsPage.creating') : t('vaktscan.assetsPage.createAsset')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
