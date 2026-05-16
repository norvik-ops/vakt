import { useState, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Server, ScanSearch, Upload } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { Pagination } from '../../../shared/components/Pagination'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useAssets, useCreateAsset, useImportAssets } from '../hooks/useAssets'
import type { Asset } from '../types'
import type { CreateAssetInput, ImportAssetsResult } from '../hooks/useAssets'

const criticalityVariant: Record<Asset['criticality'], React.ComponentProps<typeof Badge>['variant']> = {
  low: 'secondary',
  medium: 'warning',
  high: 'outline',
  critical: 'destructive',
}

const criticalityClass: Record<Asset['criticality'], string> = {
  low:      '',
  medium:   '',
  high:     'border-transparent bg-[#7c2d12] text-[#f97316]',
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

export default function AssetsPage() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const { data: assets, isLoading, error, pagination } = useAssets(page)
  const createAsset = useCreateAsset()
  const importAssets = useImportAssets()
  const [open, setOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
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

  function handleImportOpen() {
    setImportResult(null)
    setImportOpen(true)
  }

  function handleImportFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    const fd = new FormData()
    fd.append('file', file)
    importAssets.mutate(fd, {
      onSuccess: (result) => setImportResult(result),
      onError: (err) => setImportResult({ inserted: 0, errored: 0, errors: [err.message] }),
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
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'Failed to create asset')
    }
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Assets"
        description="Überwachte Assets und Infrastruktur verwalten"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={handleImportOpen}>
              <Upload className="w-4 h-4 mr-1" />
              CSV importieren
            </Button>
            <Button onClick={handleOpen}>
              <Plus className="w-4 h-4 mr-1" />
              Neues Asset
            </Button>
          </div>
        }
      />

      <InfoBanner icon={ScanSearch} title="So funktioniert Vakt Scan">
        <p>Vakt Scan orchestriert lokale Scanner wie <strong>Trivy</strong>, <strong>Nuclei</strong> und <strong>OpenVAS</strong>. Lege zuerst ein Asset (Server, Web-App, Repo …) an — dann startest du einen Scan direkt aus der Asset-Detailseite.</p>
        <p className="mt-1">Die Scanner müssen von deiner Vakt-Instanz aus erreichbar sein. URLs und Credentials trägst du in <strong>Settings → Scanner</strong> ein.</p>
      </InfoBanner>

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex justify-center py-16">
            <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {error && (
          <p className="text-sm text-red-600 p-4">
            Error: {error.message}
          </p>
        )}

        {!isLoading && !error && assets && assets.length === 0 && (
          <EmptyState
            icon={Server}
            title="Noch keine Assets vorhanden"
            description="Fügen Sie Ihr erstes Asset hinzu, um Schwachstellenscans zu starten."
            action={
              <Button onClick={handleOpen}>
                <Plus className="w-4 h-4 mr-1" />
                Neues Asset
              </Button>
            }
          />
        )}

        {!isLoading && !error && assets && assets.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Criticality</TableHead>
                  <TableHead>Tags</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {assets.map((asset) => (
                  <TableRow
                    key={asset.id}
                    className="cursor-pointer hover:bg-surface2"
                    onClick={() => navigate(`/secpulse/assets/${asset.id}`)}
                  >
                    <TableCell className="font-medium">{asset.name}</TableCell>
                    <TableCell>{assetTypeLabels[asset.type]}</TableCell>
                    <TableCell className="font-mono text-xs text-secondary">{asset.target}</TableCell>
                    <TableCell>
                      <Badge
                        variant={criticalityVariant[asset.criticality]}
                        className={criticalityClass[asset.criticality]}
                      >
                        {asset.criticality}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {asset.tags.map((tag) => (
                          <Badge key={tag} variant="outline" className="text-xs">
                            {tag}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
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
            <DialogTitle>Assets per CSV importieren</DialogTitle>
            <DialogDescription>
              CSV mit Spalten: <code className="text-xs bg-surface2 px-1 rounded">name, type, target, criticality, tags</code>. Erlaubte Typen: web_app, server, database, container, repo. Criticality: low, medium, high, critical.
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
                <span className="w-3.5 h-3.5 border-2 border-brand border-t-transparent rounded-full animate-spin inline-block" />
                Importiere …
              </p>
            )}
            {importResult && (
              <div className={`p-3 rounded-lg text-sm space-y-1 ${importResult.errors.length > 0 ? 'bg-yellow-500/10' : 'bg-green-500/10'}`}>
                <p className="font-medium">{importResult.inserted} Assets importiert, {importResult.errored} Fehler</p>
                {importResult.errors.map((e, i) => (
                  <p key={i} className="text-xs text-red-400">{e}</p>
                ))}
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportOpen(false)}>Schließen</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Neues Asset</DialogTitle>
            <DialogDescription>Fügen Sie ein neues Asset hinzu, um es auf Schwachstellen zu überwachen.</DialogDescription>
          </DialogHeader>
          <form onSubmit={(e) => { void handleSubmit(e) }}>
            <div className="space-y-4 py-2">
              <div className="space-y-1">
                <Label htmlFor="asset-name">Name</Label>
                <Input
                  id="asset-name"
                  placeholder="My Web App"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  required
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-type">Type</Label>
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
                <Label htmlFor="asset-target">Target</Label>
                <Input
                  id="asset-target"
                  placeholder="https://example.com or 192.168.1.1"
                  value={form.target}
                  onChange={(e) => setForm({ ...form, target: e.target.value })}
                  required
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="asset-criticality">Criticality</Label>
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
                <Label htmlFor="asset-tags">Tags (comma-separated)</Label>
                <Input
                  id="asset-tags"
                  placeholder="production, web, external"
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
                Cancel
              </Button>
              <Button type="submit" disabled={createAsset.isPending}>
                {createAsset.isPending ? (
                  <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin mr-2" />
                ) : null}
                Create Asset
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
