import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Library, Plus } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../../../components/ui/dialog'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Spinner } from '../../../components/Spinner'
import { useThreatCatalog, useCreateRiskFromCatalog, type ThreatCatalogFilter } from '../hooks/useThreatCatalog'

const FRAMEWORKS = ['ISO27001', 'BSI', 'NIS2', 'DSGVO-TOM', 'C5', 'DORA']
const ASSET_TYPES = ['server', 'endpoint', 'data', 'identity', 'network', 'application', 'cloud', 'facility', 'supplier', 'process', 'service']
const CIA = ['confidentiality', 'integrity', 'availability']

export function ThreatCatalogDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<ThreatCatalogFilter>({})
  const { data: items = [], isLoading } = useThreatCatalog(filter)
  const createFromCatalog = useCreateRiskFromCatalog()

  function toggle(key: keyof ThreatCatalogFilter, value: string) {
    setFilter((f) => ({ ...f, [key]: f[key] === value ? undefined : value }))
  }

  function createRisk(catalogId: string) {
    createFromCatalog.mutate({ catalog_id: catalogId }, { onSuccess: onClose })
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-4xl max-h-[85vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Library className="w-4 h-4" />
            {t('threatCatalog.title')}
          </DialogTitle>
        </DialogHeader>
        <div className="flex gap-4 flex-1 min-h-0">
          {/* Filter sidebar */}
          <aside className="w-48 shrink-0 space-y-3 overflow-y-auto text-xs">
            <div>
              <p className="font-medium mb-1">{t('threatCatalog.framework')}</p>
              <div className="flex flex-wrap gap-1">
                {FRAMEWORKS.map((fw) => (
                  <button
                    key={fw}
                    onClick={() => { toggle('framework', fw) }}
                    className={`px-2 py-0.5 rounded border ${filter.framework === fw ? 'bg-primary/20 border-primary text-primary' : 'border-border'}`}
                  >{fw}</button>
                ))}
              </div>
            </div>
            <div>
              <p className="font-medium mb-1">{t('threatCatalog.assetType')}</p>
              <div className="flex flex-wrap gap-1">
                {ASSET_TYPES.map((a) => (
                  <button
                    key={a}
                    onClick={() => { toggle('asset_type', a) }}
                    className={`px-2 py-0.5 rounded border ${filter.asset_type === a ? 'bg-primary/20 border-primary text-primary' : 'border-border'}`}
                  >{t(`threatCatalog.asset.${a}`)}</button>
                ))}
              </div>
            </div>
            <div>
              <p className="font-medium mb-1">{t('threatCatalog.cia')}</p>
              <div className="flex flex-wrap gap-1">
                {CIA.map((c) => (
                  <button
                    key={c}
                    onClick={() => { toggle('cia', c) }}
                    className={`px-2 py-0.5 rounded border ${filter.cia === c ? 'bg-primary/20 border-primary text-primary' : 'border-border'}`}
                  >{t(`threatCatalog.ciaShort.${c}`)}</button>
                ))}
              </div>
            </div>
          </aside>

          {/* Item list */}
          <div className="flex-1 overflow-y-auto space-y-2 pr-1">
            {isLoading && <div className="flex justify-center py-8"><Spinner size="md" /></div>}
            {!isLoading && items.length === 0 && (
              <p className="text-sm text-muted-foreground py-8 text-center">{t('threatCatalog.empty')}</p>
            )}
            {items.map((it) => (
              <div key={it.id} className="border rounded-lg p-3 space-y-1.5">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <p className="text-sm font-medium">{it.title}</p>
                    <p className="text-xs text-muted-foreground line-clamp-2">{it.scenario}</p>
                  </div>
                  <Button
                    size="sm"
                    className="h-7 text-xs shrink-0"
                    onClick={() => { createRisk(it.id) }}
                    disabled={createFromCatalog.isPending}
                  >
                    <Plus className="w-3 h-3 mr-1" />
                    {t('threatCatalog.createRisk')}
                  </Button>
                </div>
                <div className="flex flex-wrap items-center gap-1">
                  <Badge variant="outline" className="text-[10px]">{it.category}</Badge>
                  {it.frameworks.map((fw) => (
                    <Badge key={fw} variant="outline" className="text-[10px]">{fw}</Badge>
                  ))}
                  <span className="text-[10px] text-muted-foreground ml-auto">
                    {t('threatCatalog.defaultScore')}: {it.default_likelihood}×{it.default_impact}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
