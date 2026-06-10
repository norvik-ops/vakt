import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Spinner } from '../components/Spinner'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '../components/ui/card'
import { apiFetch } from '../api/client'

// ─── Types ────────────────────────────────────────────────────────────────────

interface RetentionConfig {
  audit_log_days: number
  findings_resolved_days: number
  notifications_days: number
  scan_history_days: number
  digest_enabled: boolean
  digest_day: number   // 0=So … 6=Sa
  digest_hour: number  // 0-23 UTC
}

const DEFAULT_CONFIG: RetentionConfig = {
  audit_log_days: 365,
  findings_resolved_days: 180,
  notifications_days: 90,
  scan_history_days: 365,
  digest_enabled: false,
  digest_day: 1,
  digest_hour: 8,
}

function useWeekdays() {
  const { t } = useTranslation()
  return [
    { value: 1, label: t('retention.weekdays.monday') },
    { value: 2, label: t('retention.weekdays.tuesday') },
    { value: 3, label: t('retention.weekdays.wednesday') },
    { value: 4, label: t('retention.weekdays.thursday') },
    { value: 5, label: t('retention.weekdays.friday') },
    { value: 6, label: t('retention.weekdays.saturday') },
    { value: 0, label: t('retention.weekdays.sunday') },
  ]
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

function useRetentionConfig() {
  return useQuery<RetentionConfig>({
    queryKey: ['retention', 'config'],
    queryFn: () => apiFetch<RetentionConfig>('/retention/config'),
  })
}

function useUpdateRetentionConfig() {
  const qc = useQueryClient()
  return useMutation<RetentionConfig, Error, Partial<RetentionConfig>>({
    mutationFn: (data) =>
      apiFetch<RetentionConfig>('/retention/config', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['retention'] })
    },
  })
}

// ─── Toast (minimal inline) ───────────────────────────────────────────────────

function useToast() {
  const [message, setMessage] = useState<string | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()
  useEffect(() => () => { clearTimeout(timerRef.current); }, [])
  function show(msg: string) {
    setMessage(msg)
    timerRef.current = setTimeout(() => { setMessage(null); }, 3000)
  }
  return { message, show }
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function RetentionConfigPage() {
  const { t } = useTranslation()
  const weekdays = useWeekdays()
  const { data, isLoading } = useRetentionConfig()
  const update = useUpdateRetentionConfig()
  const toast = useToast()

  const [retentionForm, setRetentionForm] = useState({
    audit_log_days: DEFAULT_CONFIG.audit_log_days,
    findings_resolved_days: DEFAULT_CONFIG.findings_resolved_days,
    notifications_days: DEFAULT_CONFIG.notifications_days,
    scan_history_days: DEFAULT_CONFIG.scan_history_days,
  })

  const [digestForm, setDigestForm] = useState({
    digest_enabled: DEFAULT_CONFIG.digest_enabled,
    digest_day: DEFAULT_CONFIG.digest_day,
    digest_hour: DEFAULT_CONFIG.digest_hour,
  })

  useEffect(() => {
    if (data) {
      setRetentionForm({
        audit_log_days: data.audit_log_days,
        findings_resolved_days: data.findings_resolved_days,
        notifications_days: data.notifications_days,
        scan_history_days: data.scan_history_days,
      })
      setDigestForm({
        digest_enabled: data.digest_enabled,
        digest_day: data.digest_day,
        digest_hour: data.digest_hour,
      })
    }
  }, [data])

  function handleRetentionSave() {
    update.mutate(retentionForm, {
      onSuccess: () => { toast.show(t('retention.retentionSaved')); },
    })
  }

  function handleDigestSave() {
    update.mutate(digestForm, {
      onSuccess: () => { toast.show(t('retention.digestSaved')); },
    })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('retention.title')}
        description={t('retention.description')}
      />

      {toast.message && (
        <div className="mx-6 mt-4 px-4 py-2 bg-green-50 border border-green-200 rounded-lg text-sm text-green-800">
          {toast.message}
        </div>
      )}

      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-2xl space-y-5">

          {/* Card 1: Aufbewahrungsfristen */}
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">{t('retention.retentionCard')}</CardTitle>
              <CardDescription>
                {t('retention.retentionCardHint')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <div className="flex items-center justify-center h-16">
                  <Spinner size="sm" />
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="space-y-1.5">
                    <Label className="text-xs text-secondary">{t('retention.auditLogDays')}</Label>
                    <Input
                      type="number"
                      min={0}
                      max={3650}
                      value={retentionForm.audit_log_days}
                      onChange={(e) =>
                        { setRetentionForm((f) => ({ ...f, audit_log_days: Number(e.target.value) })); }
                      }
                      className="h-8 text-sm"
                    />
                    <p className="text-[11px] text-secondary">{t('retention.permanent')}</p>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs text-secondary">{t('retention.findingsResolvedDays')}</Label>
                    <Input
                      type="number"
                      min={0}
                      max={3650}
                      value={retentionForm.findings_resolved_days}
                      onChange={(e) =>
                        { setRetentionForm((f) => ({ ...f, findings_resolved_days: Number(e.target.value) })); }
                      }
                      className="h-8 text-sm"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs text-secondary">{t('retention.notificationsDays')}</Label>
                    <Input
                      type="number"
                      min={0}
                      max={3650}
                      value={retentionForm.notifications_days}
                      onChange={(e) =>
                        { setRetentionForm((f) => ({ ...f, notifications_days: Number(e.target.value) })); }
                      }
                      className="h-8 text-sm"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs text-secondary">{t('retention.scanHistoryDays')}</Label>
                    <Input
                      type="number"
                      min={0}
                      max={3650}
                      value={retentionForm.scan_history_days}
                      onChange={(e) =>
                        { setRetentionForm((f) => ({ ...f, scan_history_days: Number(e.target.value) })); }
                      }
                      className="h-8 text-sm"
                    />
                  </div>
                </div>
              )}
            </CardContent>
            <CardFooter className="justify-end">
              <Button
                onClick={handleRetentionSave}
                disabled={isLoading || update.isPending}
                className="h-8 text-sm"
              >
                {update.isPending ? t('retention.saving') : t('retention.save')}
              </Button>
            </CardFooter>
          </Card>

          {/* Card 2: Wöchentlicher E-Mail-Digest */}
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">{t('retention.digestCard')}</CardTitle>
              <CardDescription>
                {t('retention.digestCardHint')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <div className="flex items-center justify-center h-16">
                  <Spinner size="sm" />
                </div>
              ) : (
                <div className="space-y-4">
                  <label className="flex items-center gap-3 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={digestForm.digest_enabled}
                      onChange={(e) =>
                        { setDigestForm((f) => ({ ...f, digest_enabled: e.target.checked })); }
                      }
                      className="w-4 h-4 rounded border-border accent-indigo-500"
                    />
                    <span className="text-sm text-primary">{t('retention.digestEnable')}</span>
                  </label>

                  {digestForm.digest_enabled && (
                    <div className="ml-7 space-y-3">
                      <div className="space-y-1.5">
                        <Label className="text-xs text-secondary">{t('retention.digestWeekday')}</Label>
                        <select
                          value={digestForm.digest_day}
                          onChange={(e) =>
                            { setDigestForm((f) => ({ ...f, digest_day: Number(e.target.value) })); }
                          }
                          className="h-8 text-sm rounded-md border border-input bg-background px-2 focus:outline-none focus:ring-1 focus:ring-brand"
                        >
                          {weekdays.map((d) => (
                            <option key={d.value} value={d.value}>{d.label}</option>
                          ))}
                        </select>
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs text-secondary">{t('retention.digestTime')}</Label>
                        <select
                          value={digestForm.digest_hour}
                          onChange={(e) =>
                            { setDigestForm((f) => ({ ...f, digest_hour: Number(e.target.value) })); }
                          }
                          className="h-8 text-sm rounded-md border border-input bg-background px-2 focus:outline-none focus:ring-1 focus:ring-brand"
                        >
                          {Array.from({ length: 24 }, (_, h) => (
                            <option key={h} value={h}>
                              {String(h).padStart(2, '0')}:00 UTC
                            </option>
                          ))}
                        </select>
                      </div>
                    </div>
                  )}

                  <p className="text-[11px] text-secondary leading-relaxed">
                    {t('retention.digestHint')}
                  </p>
                </div>
              )}
            </CardContent>
            <CardFooter className="justify-end">
              <Button
                onClick={handleDigestSave}
                disabled={isLoading || update.isPending}
                className="h-8 text-sm"
              >
                {update.isPending ? t('retention.saving') : t('retention.save')}
              </Button>
            </CardFooter>
          </Card>

        </div>
      </div>
    </div>
  )
}
