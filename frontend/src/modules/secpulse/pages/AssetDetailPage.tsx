import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, ScanLine, Trash2 } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Breadcrumbs } from '../../../shared/components/Breadcrumbs'
import { trackPage } from '../../../shared/hooks/useRecentPages'
import { ConfirmDeleteDialog } from '../../../shared/components/ConfirmDeleteDialog'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../../components/ui/tabs'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useAsset, useTriggerScan, useDeleteAsset } from '../hooks/useAssets'
import { useFindings } from '../hooks/useFindings'
import type { Asset } from '../types'
import { cn } from '../../../lib/utils'
import { findingSeverityClass } from '../../../lib/statusMapping'
import { useState, useRef, useEffect } from 'react'

const criticalityClass: Record<Asset['criticality'], string> = {
  low:      'border-transparent bg-surface2 text-muted',
  medium:   'border-transparent bg-severity-medium-bg text-severity-medium',
  high:     'border-transparent bg-severity-high-bg text-severity-high',
  critical: 'border-transparent bg-severity-critical-bg text-severity-critical',
}

const severityClass = findingSeverityClass

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
  const [scanTriggered, setScanTriggered] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const { data: asset, isLoading, error } = useAsset(id ?? '')
  const triggerScan = useTriggerScan(id ?? '')
  const deleteAsset = useDeleteAsset()
  const { data: findingsResponse } = useFindings({ asset_id: id })
  const scanTimerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => clearTimeout(scanTimerRef.current), [])

  useEffect(() => {
    if (asset) trackPage(`/secpulse/assets/${id}`, asset.name, '🖥️')
  }, [asset?.id])

  async function handleScan() {
    try {
      await triggerScan.mutateAsync()
      setScanTriggered(true)
      scanTimerRef.current = setTimeout(() => setScanTriggered(false), 3000)
    } catch {
      // error handled by isPending/isError states
    }
  }

  function handleDeleteConfirm() {
    if (!id) return
    deleteAsset.mutate(id, {
      onSuccess: () => {
        navigate('/secpulse/assets')
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
        <p className="text-sm text-red-600">{error?.message ?? 'Asset not found'}</p>
        <Button variant="outline" className="mt-4" onClick={() => navigate('/secpulse/assets')}>
          <ArrowLeft className="w-4 h-4 mr-1" />
          Back to Assets
        </Button>
      </div>
    )
  }

  const findings = findingsResponse?.data ?? []

  return (
    <div className="flex flex-col h-full">
      <Breadcrumbs items={[
        { label: 'SecPulse', href: '/secpulse' },
        { label: 'Assets', href: '/secpulse/assets' },
        { label: asset.name },
      ]} />
      <PageHeader
        title={asset.name}
        description={asset.target}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={() => navigate('/secpulse/assets')}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              Back
            </Button>
            <Button onClick={() => { void handleScan() }} disabled={triggerScan.isPending}>
              {triggerScan.isPending ? (
                <Spinner size="sm" color="white" className="mr-2" />
              ) : (
                <ScanLine className="w-4 h-4 mr-1" />
              )}
              Trigger Scan
            </Button>
            <Button
              variant="outline"
              className="text-destructive hover:text-destructive hover:bg-destructive/10"
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2 className="w-4 h-4 mr-1" />
              Löschen
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

      {triggerScan.isError && (
        <div className="px-6 pt-4">
          <p className="text-sm text-red-600">Scan failed: {triggerScan.error.message}</p>
        </div>
      )}
      {scanTriggered && (
        <div className="px-6 pt-4">
          <p className="text-sm text-green-600">Scan triggered successfully.</p>
        </div>
      )}

      <div className="flex-1 p-6">
        <Tabs defaultValue="info">
          <TabsList>
            <TabsTrigger value="info">Info</TabsTrigger>
            <TabsTrigger value="findings">
              Findings {findings.length > 0 ? `(${findings.length.toString()})` : ''}
            </TabsTrigger>
          </TabsList>

          <TabsContent value="info" className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle>Asset Details</CardTitle>
              </CardHeader>
              <CardContent>
                <dl className="grid grid-cols-2 gap-x-8 gap-y-4 text-sm">
                  <div>
                    <dt className="text-secondary font-medium">Type</dt>
                    <dd className="mt-1 text-primary">{assetTypeLabels[asset.type]}</dd>
                  </div>
                  <div>
                    <dt className="text-secondary font-medium">Criticality</dt>
                    <dd className="mt-1">
                      <Badge className={cn('capitalize', criticalityClass[asset.criticality])}>
                        {asset.criticality}
                      </Badge>
                    </dd>
                  </div>
                  <div>
                    <dt className="text-secondary font-medium">Target</dt>
                    <dd className="mt-1 font-mono text-xs text-primary">{asset.target}</dd>
                  </div>
                  <div>
                    <dt className="text-secondary font-medium">Created</dt>
                    <dd className="mt-1 text-primary">
                      {new Date(asset.created_at).toLocaleDateString()}
                    </dd>
                  </div>
                  <div className="col-span-2">
                    <dt className="text-secondary font-medium">Tags</dt>
                    <dd className="mt-1 flex flex-wrap gap-1">
                      {asset.tags.length > 0
                        ? asset.tags.map((tag) => (
                            <Badge key={tag} variant="outline" className="text-xs">
                              {tag}
                            </Badge>
                          ))
                        : <span className="text-secondary">None</span>}
                    </dd>
                  </div>
                </dl>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="findings" className="mt-4">
            {findings.length === 0 ? (
              <p className="text-sm text-secondary py-8 text-center">No findings for this asset.</p>
            ) : (
              <div className="rounded-md border border-border bg-surface overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Title</TableHead>
                      <TableHead>Severity</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>CVE</TableHead>
                      <TableHead>CVSS</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {findings.map((finding) => (
                      <TableRow
                        key={finding.id}
                        className="cursor-pointer hover:bg-surface2"
                        onClick={() => navigate(`/secpulse/findings/${finding.id}`)}
                      >
                        <TableCell className="font-medium">{finding.title}</TableCell>
                        <TableCell>
                          <Badge className={cn('capitalize', severityClass[finding.severity])}>
                            {finding.severity}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <span className="text-sm text-secondary capitalize">
                            {finding.status.replace(/_/g, ' ')}
                          </span>
                        </TableCell>
                        <TableCell className="font-mono text-xs">{finding.cve_id ?? '—'}</TableCell>
                        <TableCell>{finding.cvss_score != null ? finding.cvss_score.toFixed(1) : '—'}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
