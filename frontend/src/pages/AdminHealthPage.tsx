import { RefreshCw, Database, Server, Cpu, Clock, Tag, Activity } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Badge } from '../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Skeleton } from '../components/ui/skeleton'

// ─── Types ────────────────────────────────────────────────────────────────────

interface ComponentHealth {
  ok: boolean
  latency_ms: number
}

interface QueueStats {
  pending: number
  active: number
  failed: number
}

interface HealthData {
  db: ComponentHealth
  redis: ComponentHealth
  queue: QueueStats
  version: string
  uptime_seconds: number
  goroutines: number
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `${h.toString()}h ${m.toString()}m`
}

function StatusBadge({ ok }: { ok: boolean }) {
  return ok ? (
    <Badge className="bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0">
      OK
    </Badge>
  ) : (
    <Badge className="bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0">
      Fehler
    </Badge>
  )
}

// ─── Skeleton ─────────────────────────────────────────────────────────────────

function HealthSkeleton() {
  return (
    <div className="p-6 space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {Array.from({ length: 6 }).map((_, i) => (
          <Card key={i}>
            <CardHeader>
              <Skeleton className="h-4 w-24" />
            </CardHeader>
            <CardContent>
              <Skeleton className="h-6 w-32" />
              <Skeleton className="mt-2 h-4 w-20" />
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}

// ─── Main page ────────────────────────────────────────────────────────────────

export default function AdminHealthPage() {
  const { data, isLoading, isError, error, refetch, isFetching } = useQuery<HealthData>({
    queryKey: ['admin', 'health'],
    queryFn: () => apiFetch<HealthData>('/admin/health'),
    refetchInterval: 30_000,
  })

  return (
    <div>
      <PageHeader
        title="System-Status"
        description="Echtzeit-Übersicht über Datenbankverbindung, Cache, Queue und Laufzeitmetriken."
        actions={
          <Button
            variant="outline"
            size="sm"
            onClick={() => void refetch()}
            disabled={isFetching}
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isFetching ? 'animate-spin' : ''}`} />
            Aktualisieren
          </Button>
        }
      />

      <div className="p-6 space-y-4">
        {isLoading && <HealthSkeleton />}

        {isError && (
          <div className="rounded-md bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 px-4 py-3 text-sm text-red-700 dark:text-red-300">
            Fehler beim Laden der Systemdaten:{' '}
            {error instanceof Error ? error.message : 'Unbekannter Fehler'}
          </div>
        )}

        {data && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {/* Database */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-medium text-secondary">
                  <Database className="w-4 h-4 shrink-0" />
                  Datenbank
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <StatusBadge ok={data.db.ok} />
                <p className="text-[13px] text-secondary">
                  Latenz:{' '}
                  <span className="font-semibold text-primary">{data.db.latency_ms.toString()} ms</span>
                </p>
              </CardContent>
            </Card>

            {/* Redis */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-medium text-secondary">
                  <Server className="w-4 h-4 shrink-0" />
                  Redis (Cache)
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <StatusBadge ok={data.redis.ok} />
                <p className="text-[13px] text-secondary">
                  Latenz:{' '}
                  <span className="font-semibold text-primary">{data.redis.latency_ms.toString()} ms</span>
                </p>
              </CardContent>
            </Card>

            {/* Queue */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-medium text-secondary">
                  <Activity className="w-4 h-4 shrink-0" />
                  Job-Queue
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-1 text-[13px]">
                  <div className="flex items-center justify-between">
                    <span className="text-secondary">Wartend</span>
                    <span className="font-semibold text-primary">{data.queue.pending.toString()}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-secondary">Aktiv</span>
                    <span className="font-semibold text-primary">{data.queue.active.toString()}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-secondary">Fehlgeschlagen</span>
                    <span className={`font-semibold ${data.queue.failed > 0 ? 'text-red-600 dark:text-red-400' : 'text-primary'}`}>
                      {data.queue.failed.toString()}
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Version */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-medium text-secondary">
                  <Tag className="w-4 h-4 shrink-0" />
                  Version
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-[20px] font-bold text-primary">{data.version}</p>
              </CardContent>
            </Card>

            {/* Uptime */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-medium text-secondary">
                  <Clock className="w-4 h-4 shrink-0" />
                  Laufzeit
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-[20px] font-bold text-primary">{formatUptime(data.uptime_seconds)}</p>
                <p className="text-[12px] text-secondary">{data.uptime_seconds.toLocaleString('de-DE')} Sekunden</p>
              </CardContent>
            </Card>

            {/* Goroutines */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-medium text-secondary">
                  <Cpu className="w-4 h-4 shrink-0" />
                  Goroutinen
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-[20px] font-bold text-primary">{data.goroutines.toLocaleString('de-DE')}</p>
                <p className="text-[12px] text-secondary">aktive Go-Routinen</p>
              </CardContent>
            </Card>
          </div>
        )}
      </div>
    </div>
  )
}
