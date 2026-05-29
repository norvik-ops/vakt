import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Download, CheckCircle, XCircle } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '../../../components/ui/table'
import { apiFetch } from '../../../api/client'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'

interface SoAEntry {
  control_id: string
  framework_name: string
  domain: string
  title: string
  applicable: boolean
  status: string
  justification_applicable: string
  justification_not_applicable: string
}

function useSoA() {
  return useQuery<SoAEntry[]>({
    queryKey: ['vaktcomply', 'soa'],
    queryFn: () => apiFetch<SoAEntry[]>('/vaktcomply/soa'),
    staleTime: 2 * 60 * 1000,
  })
}

function useUpdateApplicability() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ controlId, applicable, justYes, justNo }: {
      controlId: string
      applicable: boolean
      justYes: string
      justNo: string
    }) =>
      apiFetch<void>(`/vaktcomply/soa/${controlId}`, {
        method: 'PATCH',
        body: JSON.stringify({
          applicable,
          justification_applicable: justYes,
          justification_not_applicable: justNo,
        }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['vaktcomply', 'soa'] }),
  })
}

const STATUS_LABELS: Record<string, string> = {
  implemented: 'Umgesetzt',
  in_progress: 'In Bearbeitung',
  not_started: 'Nicht begonnen',
  not_applicable: 'Nicht anwendbar',
  partial: 'Teilweise',
}

const STATUS_COLORS: Record<string, string> = {
  implemented: 'bg-green-100 text-green-800',
  in_progress: 'bg-blue-100 text-blue-800',
  not_started: 'bg-gray-100 text-gray-700',
  not_applicable: 'bg-gray-100 text-gray-500',
  partial: 'bg-yellow-100 text-yellow-800',
}

export default function SoAPage() {
  const { data = [], isLoading } = useSoA()
  const updateMut = useUpdateApplicability()
  const [filter, setFilter] = useState<'all' | 'applicable' | 'not_applicable'>('all')
  const [activeFramework, setActiveFramework] = useState<string>('all')

  const frameworks = [...new Set(data.map(e => e.framework_name))]

  const filtered = data.filter(e => {
    if (activeFramework !== 'all' && e.framework_name !== activeFramework) return false
    if (filter === 'applicable') return e.applicable
    if (filter === 'not_applicable') return !e.applicable
    return true
  })

  const applicable = data.filter(e => e.applicable).length
  const notApplicable = data.filter(e => !e.applicable).length

  function toggleApplicable(entry: SoAEntry) {
    updateMut.mutate({
      controlId: entry.control_id,
      applicable: !entry.applicable,
      justYes: entry.justification_applicable,
      justNo: entry.justification_not_applicable,
    })
  }

  function handleCsvDownload() {
    void apiFetch<Blob>('/vaktcomply/soa.csv').then(blob => {
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `vakt-soa-${new Date().toISOString().slice(0, 10)}.csv`
      a.click()
      URL.revokeObjectURL(url)
    })
  }

  if (isLoading) return <div className="p-8"><SkeletonTable rows={8} cols={6} /></div>

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Statement of Applicability</h1>
          <p className="text-gray-500 text-sm mt-1">ISO 27001 Anhang A — Anwendbarkeit aller Maßnahmen</p>
        </div>
        <Button variant="outline" onClick={handleCsvDownload}>
          <Download className="h-4 w-4 mr-2" />
          CSV exportieren
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4">
        <div className="bg-white border rounded-lg p-4">
          <div className="text-2xl font-bold">{data.length}</div>
          <div className="text-sm text-gray-500">Kontrollen gesamt</div>
        </div>
        <div className="bg-green-50 border border-green-200 rounded-lg p-4">
          <div className="text-2xl font-bold text-green-700">{applicable}</div>
          <div className="text-sm text-green-600">Anwendbar</div>
        </div>
        <div className="bg-gray-50 border rounded-lg p-4">
          <div className="text-2xl font-bold text-gray-500">{notApplicable}</div>
          <div className="text-sm text-gray-500">Nicht anwendbar</div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap-2 flex-wrap">
        <select
          className="border rounded px-3 py-1.5 text-sm bg-white"
          value={activeFramework}
          onChange={e => { setActiveFramework(e.target.value); }}
        >
          <option value="all">Alle Frameworks</option>
          {frameworks.map(f => <option key={f} value={f}>{f}</option>)}
        </select>
        {(['all', 'applicable', 'not_applicable'] as const).map(f => (
          <button
            key={f}
            onClick={() => { setFilter(f); }}
            className={`px-3 py-1.5 rounded text-sm border ${filter === f ? 'bg-blue-600 text-white border-blue-600' : 'bg-white text-gray-700'}`}
          >
            {f === 'all' ? 'Alle' : f === 'applicable' ? 'Anwendbar' : 'Nicht anwendbar'}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="bg-white rounded-lg border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Framework</TableHead>
              <TableHead>Bereich</TableHead>
              <TableHead>Kontrolle</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-center">Anwendbar</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-gray-500 py-8">
                  Keine Einträge
                </TableCell>
              </TableRow>
            )}
            {filtered.map(entry => (
              <TableRow key={entry.control_id} className="hover:bg-gray-50">
                <TableCell className="text-xs text-gray-500">{entry.framework_name}</TableCell>
                <TableCell className="text-xs text-gray-500">{entry.domain}</TableCell>
                <TableCell className="font-medium text-sm">{entry.title}</TableCell>
                <TableCell>
                  <span className={`text-xs px-2 py-0.5 rounded-full ${STATUS_COLORS[entry.status] ?? 'bg-gray-100 text-gray-700'}`}>
                    {STATUS_LABELS[entry.status] ?? entry.status}
                  </span>
                </TableCell>
                <TableCell className="text-center">
                  <button
                    onClick={() => { toggleApplicable(entry); }}
                    disabled={updateMut.isPending}
                    className="hover:opacity-70 transition-opacity"
                    title={entry.applicable ? 'Als nicht anwendbar markieren' : 'Als anwendbar markieren'}
                  >
                    {entry.applicable
                      ? <CheckCircle className="h-5 w-5 text-green-600 mx-auto" />
                      : <XCircle className="h-5 w-5 text-gray-300 mx-auto" />
                    }
                  </button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
