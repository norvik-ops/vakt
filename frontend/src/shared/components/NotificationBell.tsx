import { useState } from 'react'
import { Bell, BellDot, CheckCheck, Info, AlertTriangle, AlertCircle } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'
import { useNotifications, useMarkNotificationRead, useMarkAllRead } from '../../hooks/useDashboard'
import type { UserNotification } from '../../hooks/useDashboard'

/** Maps notification type to the corresponding Lucide icon component. */
const typeIcon: Record<string, React.ElementType> = {
  info: Info,
  warning: AlertTriangle,
  error: AlertCircle,
}

/** Tailwind colour class for each notification severity, matching brand conventions. */
const typeColor: Record<string, string> = {
  info: 'text-blue-400',
  warning: 'text-yellow-400',
  error: 'text-red-400',
}

/**
 * Header bell icon that opens a dropdown listing the current user's notifications.
 *
 * Shows a filled `BellDot` icon and a red badge (capped at "9+") when there are
 * unread items. Clicking anywhere on the transparent overlay behind the panel
 * dismisses it (click-outside behaviour). Marking an individual item or all items
 * as read is handled inline without navigating away.
 */
export function NotificationBell() {
  const [open, setOpen] = useState(false)
  const { data: notifications } = useNotifications()
  const markRead = useMarkNotificationRead()
  const markAll = useMarkAllRead()

  const unread = notifications?.filter((n) => !n.read).length ?? 0

  return (
    <div className="relative">
      <Button
        variant="ghost"
        size="icon"
        className="w-8 h-8 relative"
        aria-label="Benachrichtigungen"
        onClick={() => setOpen((v) => !v)}
      >
        {unread > 0 ? <BellDot className="w-4 h-4 text-brand" /> : <Bell className="w-4 h-4" />}
        {unread > 0 && (
          <span className="absolute -top-0.5 -right-0.5 w-4 h-4 bg-red-500 text-white text-[10px] font-bold rounded-full flex items-center justify-center">
            {unread > 9 ? '9+' : unread}
          </span>
        )}
      </Button>

      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className="absolute left-0 bottom-10 w-80 z-50 bg-surface border border-border rounded-xl shadow-xl overflow-hidden">
            <div className="flex items-center justify-between px-4 py-3 border-b border-border">
              <span className="text-sm font-semibold">Benachrichtigungen</span>
              {unread > 0 && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 text-xs gap-1"
                  onClick={() => markAll.mutate()}
                >
                  <CheckCheck className="w-3 h-3" />
                  Alle gelesen
                </Button>
              )}
            </div>
            <div className="max-h-96 overflow-y-auto divide-y divide-border">
              {!notifications || notifications.length === 0 ? (
                <p className="text-sm text-secondary text-center py-8">Keine Benachrichtigungen</p>
              ) : (
                notifications.map((n) => (
                  <NotificationItem
                    key={n.id}
                    notification={n}
                    onRead={() => markRead.mutate(n.id)}
                  />
                ))
              )}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

/**
 * Single row inside the notification dropdown.
 * Clicking the row immediately calls `onRead` so the unread dot and badge update
 * in the same interaction, before the panel is closed.
 */
function NotificationItem({ notification: n, onRead }: { notification: UserNotification; onRead: () => void }) {
  const Icon = typeIcon[n.type] ?? Info
  const date = new Date(n.created_at).toLocaleDateString('de-DE', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })

  return (
    <button
      onClick={onRead}
      className={cn(
        'w-full text-left flex items-start gap-3 px-4 py-3 hover:bg-surface2 transition-colors',
        !n.read && 'bg-brand/5',
      )}
    >
      <Icon className={cn('w-4 h-4 mt-0.5 shrink-0', typeColor[n.type] ?? 'text-secondary')} />
      <div className="flex-1 min-w-0">
        <p className={cn('text-xs font-medium', !n.read && 'text-primary')}>{n.title}</p>
        <p className="text-xs text-secondary line-clamp-2 mt-0.5">{n.body}</p>
        <p className="text-[10px] text-secondary mt-1">{date}</p>
      </div>
      {!n.read && <span className="w-2 h-2 bg-brand rounded-full shrink-0 mt-1" />}
    </button>
  )
}
