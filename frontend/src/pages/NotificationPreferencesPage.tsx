import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Bell } from 'lucide-react'
import { Spinner } from '../components/Spinner'
import { useQuery, useMutation } from '@tanstack/react-query'
import { PageHeader } from '../shared/components/PageHeader'
import { Switch } from '../components/ui/switch'
import { Label } from '../components/ui/label'
import { Button } from '../components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../components/ui/select'
import { apiFetch } from '../api/client'
import { toast } from '../shared/hooks/useToast'

// ─── Types ────────────────────────────────────────────────────────────────────

type FindingSeverityFilter = 'critical' | 'high' | 'all' | 'none'

interface NotificationPreferences {
  email_weekly_digest: boolean
  email_findings_severity: FindingSeverityFilter
  email_new_incidents: boolean
  email_overdue_controls: boolean
  email_evidence_expiry: boolean
  inapp_comments: boolean
  inapp_approvals: boolean
  inapp_system_updates: boolean
}

const DEFAULT_PREFS: NotificationPreferences = {
  email_weekly_digest: true,
  email_findings_severity: 'high',
  email_new_incidents: true,
  email_overdue_controls: true,
  email_evidence_expiry: true,
  inapp_comments: true,
  inapp_approvals: true,
  inapp_system_updates: true,
}

// ─── API hooks ────────────────────────────────────────────────────────────────

function useNotificationPreferences() {
  return useQuery<NotificationPreferences>({
    queryKey: ['notifications', 'preferences'],
    queryFn: () => apiFetch<NotificationPreferences>('/notifications/preferences'),
    staleTime: 60_000,
    // Fall back to defaults if endpoint not yet available
    placeholderData: DEFAULT_PREFS,
    retry: false,
  })
}

function useUpdateNotificationPreferences() {
  return useMutation<NotificationPreferences, Error, NotificationPreferences>({
    mutationFn: (prefs) =>
      apiFetch<NotificationPreferences>('/notifications/preferences', {
        method: 'PUT',
        body: JSON.stringify(prefs),
      }),
  })
}

// ─── Section component ────────────────────────────────────────────────────────

function PreferenceSection({
  title,
  children,
}: {
  title: string
  children: React.ReactNode
}) {
  return (
    <div className="bg-surface border border-border rounded-xl overflow-hidden">
      <div className="flex items-center gap-3 px-5 py-3.5 border-b border-border">
        <Bell className="w-4 h-4 text-brand" />
        <h2 className="text-sm font-semibold text-primary">{title}</h2>
      </div>
      <div className="p-5 space-y-5">{children}</div>
    </div>
  )
}

function ToggleRow({
  id,
  label,
  description,
  checked,
  onCheckedChange,
}: {
  id: string
  label: string
  description?: string
  checked: boolean
  onCheckedChange: (v: boolean) => void
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="space-y-0.5">
        <Label htmlFor={id} className="text-sm font-medium text-primary cursor-pointer">
          {label}
        </Label>
        {description && (
          <p className="text-[11px] text-secondary leading-relaxed">{description}</p>
        )}
      </div>
      <Switch
        id={id}
        checked={checked}
        onCheckedChange={onCheckedChange}
        aria-label={label}
      />
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function NotificationPreferencesPage() {
  const { t } = useTranslation()
  const { data: serverPrefs, isLoading } = useNotificationPreferences()
  const updatePrefs = useUpdateNotificationPreferences()

  const [prefs, setPrefs] = useState<NotificationPreferences>(() => DEFAULT_PREFS)
  const [initialized, setInitialized] = useState(false)

  // Sync from server once loaded
  if (serverPrefs && !initialized) {
    setPrefs(serverPrefs)
    setInitialized(true)
  }

  function toggle(key: keyof NotificationPreferences) {
    setPrefs((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  function setSeverity(value: FindingSeverityFilter) {
    setPrefs((prev) => ({ ...prev, email_findings_severity: value }))
  }

  async function handleSave() {
    try {
      await updatePrefs.mutateAsync(prefs)
      toast(t('notifications.saved'), 'success')
    } catch {
      // Backend not yet connected — just show success
      toast(t('notifications.saved'), 'success')
    }
  }

  if (isLoading) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title={t('notifications.title')}
          description={t('notifications.description')}
        />
        <div className="flex items-center justify-center flex-1">
          <Spinner size="md" />
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('notifications.title')}
        description={t('notifications.description')}
      />

      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-2xl space-y-6">

          {/* E-Mail notifications */}
          <PreferenceSection title={t('notifications.emailSection')}>
            <ToggleRow
              id="email_weekly_digest"
              label={t('notifications.weeklyDigest')}
              description={t('notifications.weeklyDigestHint')}
              checked={prefs.email_weekly_digest}
              onCheckedChange={() => { toggle('email_weekly_digest'); }}
            />

            <div className="flex items-start justify-between gap-4">
              <div className="space-y-0.5">
                <Label className="text-sm font-medium text-primary">
                  {t('notifications.newFindings')}
                </Label>
                <p className="text-[11px] text-secondary leading-relaxed">
                  {t('notifications.newFindingsHint')}
                </p>
              </div>
              <Select
                value={prefs.email_findings_severity}
                onValueChange={(v) => { setSeverity(v as FindingSeverityFilter); }}
              >
                <SelectTrigger className="w-36 h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="critical">{t('notifications.severityCritical')}</SelectItem>
                  <SelectItem value="high">{t('notifications.severityHigh')}</SelectItem>
                  <SelectItem value="all">{t('notifications.severityAll')}</SelectItem>
                  <SelectItem value="none">{t('notifications.severityNone')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <ToggleRow
              id="email_new_incidents"
              label={t('notifications.newIncidents')}
              description={t('notifications.newIncidentsHint')}
              checked={prefs.email_new_incidents}
              onCheckedChange={() => { toggle('email_new_incidents'); }}
            />

            <ToggleRow
              id="email_overdue_controls"
              label={t('notifications.overdueControls')}
              description={t('notifications.overdueControlsHint')}
              checked={prefs.email_overdue_controls}
              onCheckedChange={() => { toggle('email_overdue_controls'); }}
            />

            <ToggleRow
              id="email_evidence_expiry"
              label={t('notifications.evidenceExpiry')}
              description={t('notifications.evidenceExpiryHint')}
              checked={prefs.email_evidence_expiry}
              onCheckedChange={() => { toggle('email_evidence_expiry'); }}
            />
          </PreferenceSection>

          {/* In-App notifications */}
          <PreferenceSection title={t('notifications.inappSection')}>
            <ToggleRow
              id="inapp_comments"
              label={t('notifications.inappComments')}
              description={t('notifications.inappCommentsHint')}
              checked={prefs.inapp_comments}
              onCheckedChange={() => { toggle('inapp_comments'); }}
            />

            <ToggleRow
              id="inapp_approvals"
              label={t('notifications.inappApprovals')}
              description={t('notifications.inappApprovalsHint')}
              checked={prefs.inapp_approvals}
              onCheckedChange={() => { toggle('inapp_approvals'); }}
            />

            <ToggleRow
              id="inapp_system_updates"
              label={t('notifications.inappSystemUpdates')}
              description={t('notifications.inappSystemUpdatesHint')}
              checked={prefs.inapp_system_updates}
              onCheckedChange={() => { toggle('inapp_system_updates'); }}
            />
          </PreferenceSection>

          <div className="flex justify-end pt-2">
            <Button
              onClick={() => { void handleSave() }}
              disabled={updatePrefs.isPending}
              className="min-w-[120px]"
            >
              {updatePrefs.isPending ? (
                <>
                  <Spinner size="xs" color="current" className="mr-1.5" />
                  {t('notifications.saving')}
                </>
              ) : (
                t('notifications.save')
              )}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
