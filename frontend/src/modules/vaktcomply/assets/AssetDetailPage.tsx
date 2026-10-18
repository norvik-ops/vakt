import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Trash2, Link2 } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Breadcrumbs } from '../../../shared/components/Breadcrumbs'
import { trackPage } from '../../../shared/hooks/useRecentPages'
import { ConfirmDeleteDialog } from '../../../shared/components/ConfirmDeleteDialog'
import { ErrorState } from '../../../shared/components/ErrorState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { useAsset, useDeleteAsset } from './hooks'
import type { Asset } from './types'
import { useState, useEffect } from 'react'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const criticalityClass: Record<Asset['criticality'], string> = {
  low:      'border-transparent bg-surface2 text-muted',
  medium:   'border-transparent bg-severity-medium-bg text-severity-medium',
  high:     'border-transparent bg-severity-high-bg text-severity-high',
  critical: 'border-transparent bg-severity-critical-bg text-severity-critical',
}

const assetTypeLabels: Record<Asset['type'], string> = {
  web_app: 'Web App',
  server: 'Server',
  database: 'Database',
  container: 'Container',
  repo: 'Repository',
}

export default function AssetDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const [deleteOpen, setDeleteOpen] = useState(false)
  const { data: asset, isLoading, error } = useAsset(id ?? '')
  const deleteAsset = useDeleteAsset()

  useEffect(() => {
    if (asset) trackPage(`/vaktcomply/assets/${id ?? ''}`, asset.name, '🖥️')
  }, [asset?.id])

  function handleDeleteConfirm() {
    if (!id) return
    deleteAsset.mutate(id, {
      onSuccess: () => {
        navigate('/vaktcomply/assets')
      },
    })
  }

  if (isLoading) {
    return (
      <div className="flex justify-center py-16">
        <Spinner size="md" />
      </div>
    )
  }

  if (error || !asset) {
    return (
      <div className="p-6">
        <ErrorState
          title={t('vaktcomply.assets.detail.notFound')}
          message={error?.message}
          onRetry={() => { navigate('/vaktcomply/assets') }}
        />
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      <Breadcrumbs items={[
        { label: 'Vakt Comply', href: '/vaktcomply' },
        { label: 'Assets', href: '/vaktcomply/assets' },
        { label: asset.name },
      ]} />
      <PageHeader
        title={asset.name}
        description={asset.external_url ?? undefined}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={() => { navigate('/vaktcomply/assets'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('common.back')}
            </Button>
            <Button
              variant="outline"
              className="text-destructive hover:text-destructive hover:bg-destructive/10"
              onClick={() => { setDeleteOpen(true); }}
            >
              <Trash2 className="w-4 h-4 mr-1" />
              {t('common.delete')}
            </Button>
          </div>
        }
      />

      {asset && (
        <ConfirmDeleteDialog
          open={deleteOpen}
          onOpenChange={setDeleteOpen}
          resourceName={asset.name}
          resourceType="Asset"
          onConfirm={handleDeleteConfirm}
          isPending={deleteAsset.isPending}
        />
      )}

      <div className="flex-1 p-6">
        <Card>
          <CardHeader>
            <CardTitle>{t('vaktcomply.assets.detail.assetDetails')}</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-x-8 gap-y-4 text-sm">
              <div>
                <dt className="text-secondary font-medium">{t('common.type')}</dt>
                <dd className="mt-1 text-primary">{assetTypeLabels[asset.type]}</dd>
              </div>
              <div>
                <dt className="text-secondary font-medium">{t('vaktcomply.assets.detail.criticality')}</dt>
                <dd className="mt-1">
                  <Badge className={criticalityClass[asset.criticality]}>
                    {t('vaktcomply.assets.criticality.' + asset.criticality)}
                  </Badge>
                </dd>
              </div>
              <div>
                <dt className="text-secondary font-medium">{t('vaktcomply.assets.detail.target')}</dt>
                <dd className="mt-1 font-mono text-xs text-primary">{asset.external_url ?? '—'}</dd>
              </div>
              <div>
                <dt className="text-secondary font-medium">{t('vaktcomply.assets.detail.created')}</dt>
                <dd className="mt-1 text-primary">
                  {formatDate(asset.created_at)}
                </dd>
              </div>
              <div className="col-span-2">
                <dt className="text-secondary font-medium">{t('vaktcomply.assets.detail.tags')}</dt>
                <dd className="mt-1 flex flex-wrap gap-1">
                  {asset.tags.length > 0
                    ? asset.tags.map((tag) => (
                        <Badge key={tag} variant="outline" className="text-xs">
                          {tag}
                        </Badge>
                      ))
                    : <span className="text-secondary">{t('common.none')}</span>}
                </dd>
              </div>
              {asset.protection_need_id && (
                <div className="col-span-2">
                  <dt className="text-secondary font-medium">{t('vaktcomply.assets.detail.protectionNeed')}</dt>
                  <dd className="mt-1">
                    <button
                      type="button"
                      className="inline-flex items-center gap-1.5 text-xs text-blue-400 hover:text-blue-300"
                      onClick={() => { navigate('/vaktcomply/protection-needs'); }}
                    >
                      <Link2 className="w-3.5 h-3.5" />
                      {t('vaktcomply.assets.detail.viewProtectionNeed')}
                    </button>
                  </dd>
                </div>
              )}
            </dl>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
