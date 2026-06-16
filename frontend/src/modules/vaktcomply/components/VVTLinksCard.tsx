import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { FileText, Plus, X } from 'lucide-react'
import { apiFetch } from '../../../api/client'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Button } from '../../../components/ui/button'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '../../../components/ui/select'
import { useControlVVTLinks, useLinkVVT, useUnlinkVVT } from '../hooks/useVVTLinks'

interface VVTEntry { id: string; name: string }
interface VVTListResponse { data?: VVTEntry[] }

// VVTLinksCard shows the VVT (processing activities) linked to a control and lets
// the user add/remove links. The VVT picker reads from the Vakt Privacy API
// (frontend-only cross-module data fetch — backend modules stay isolated). S88-9.
export function VVTLinksCard({ controlId }: { controlId: string }) {
  const { t } = useTranslation()
  const { data: links = [] } = useControlVVTLinks(controlId)
  const link = useLinkVVT(controlId)
  const unlink = useUnlinkVVT(controlId)
  const [selected, setSelected] = useState('')

  const { data: vvtResp } = useQuery<VVTListResponse>({
    queryKey: ['vaktprivacy', 'vvt', 'all-for-link'],
    queryFn: () => apiFetch<VVTListResponse>('/vaktprivacy/vvt?page=1&limit=200'),
    staleTime: 60_000,
  })
  const vvts = vvtResp?.data ?? []
  const linkedIds = new Set(links.map((l) => l.vvt_id))
  const available = vvts.filter((v) => !linkedIds.has(v.id))

  function addLink() {
    const v = vvts.find((x) => x.id === selected)
    if (!v) return
    link.mutate(
      { vvt_id: v.id, vvt_name: v.name, control_id: controlId },
      { onSuccess: () => { setSelected('') } },
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm flex items-center gap-2">
          <FileText className="w-4 h-4" />
          {t('vvtLinks.title')}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {links.length === 0 && (
          <p className="text-xs text-muted-foreground">{t('vvtLinks.empty')}</p>
        )}
        {links.map((l) => (
          <div key={l.id} className="flex items-center justify-between gap-2 text-sm border rounded-md px-2.5 py-1.5">
            <span className="truncate">{l.vvt_name || l.vvt_id}</span>
            <button
              onClick={() => { unlink.mutate(l.id) }}
              className="text-muted-foreground hover:text-red-400 shrink-0"
              aria-label={t('vvtLinks.remove')}
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        ))}
        <div className="flex items-center gap-2 pt-1">
          <Select value={selected} onValueChange={setSelected}>
            <SelectTrigger className="h-8 text-xs flex-1">
              <SelectValue placeholder={t('vvtLinks.selectPlaceholder')} />
            </SelectTrigger>
            <SelectContent>
              {available.map((v) => (
                <SelectItem key={v.id} value={v.id}>{v.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button size="sm" className="h-8 text-xs" onClick={addLink} disabled={!selected || link.isPending}>
            <Plus className="w-3.5 h-3.5 mr-1" />
            {t('vvtLinks.add')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
