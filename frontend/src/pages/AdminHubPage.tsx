import { Link, Navigate } from 'react-router-dom'
import {
  HeartPulse, Building2, ShieldAlert, ScrollText, ChevronRight,
} from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { useAuthStore } from '../shared/stores/auth'

interface AdminTile {
  to: string
  icon: React.ElementType
  title: string
  description: string
}

const TILES: AdminTile[] = [
  {
    to: '/admin/health',
    icon: HeartPulse,
    title: 'System-Status',
    description: 'Datenbank, Redis-Queue, Goroutines, API-Latenzen.',
  },
  {
    to: '/admin/tenants',
    icon: Building2,
    title: 'Mandanten',
    description: 'Organisationen verwalten, Nutzungsstatistiken einsehen.',
  },
  {
    to: '/admin/security',
    icon: ShieldAlert,
    title: 'Sicherheitsereignisse',
    description: 'Login-Fehlversuche, Lockouts, MFA-Resets, IP-Sperren.',
  },
  {
    to: '/settings/audit-log',
    icon: ScrollText,
    title: 'Audit-Log',
    description: 'Unveränderbares Protokoll aller administrativen Aktionen.',
  },
]

export default function AdminHubPage() {
  const { user } = useAuthStore()
  const isAdminOrOwner = user?.roles.includes('admin') || user?.roles.includes('owner')

  if (!isAdminOrOwner) {
    return <Navigate to="/" replace />
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Administration"
        description="System-Überblick, Mandanten, Sicherheit und Audit-Trail."
      />
      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-5xl">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {TILES.map(({ to, icon: Icon, title, description }) => (
              <Link
                key={to}
                to={to}
                className="group bg-surface border border-border rounded-xl p-5 hover:border-brand/40 hover:shadow-sm transition-all flex items-start gap-4"
              >
                <div className="p-2.5 rounded-lg bg-brand/10 text-brand shrink-0">
                  <Icon className="w-5 h-5" aria-hidden="true" />
                </div>
                <div className="flex-1 min-w-0">
                  <h2 className="text-sm font-semibold text-primary group-hover:text-brand transition-colors">
                    {title}
                  </h2>
                  <p className="text-[12px] text-secondary mt-1 leading-relaxed">
                    {description}
                  </p>
                </div>
                <ChevronRight
                  className="w-4 h-4 text-secondary group-hover:text-brand transition-colors mt-1 shrink-0"
                  aria-hidden="true"
                />
              </Link>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
