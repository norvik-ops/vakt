import { NavLink } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { cn } from '../../../lib/utils'

/**
 * The two views behind the navigation entry „Nachweise": every evidence record
 * of the organisation, and automatically collected items still waiting for a
 * control.
 */
export function EvidenceTabs() {
  const { t } = useTranslation()
  const tabs = [
    { to: '/vaktcomply/evidence', label: t('vaktcomply.evidenceRegister.tabAll'), end: true },
    { to: '/vaktcomply/evidence/auto', label: t('vaktcomply.evidenceRegister.tabAuto'), end: false },
  ]
  return (
    <nav className="flex gap-1 border-b border-border" aria-label={t('vaktcomply.evidenceRegister.tabsLabel')}>
      {tabs.map((tab) => (
        <NavLink
          key={tab.to}
          to={tab.to}
          end={tab.end}
          className={({ isActive }) => cn(
            'px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors',
            isActive ? 'border-brand text-brand' : 'border-transparent text-secondary hover:text-primary',
          )}
        >
          {tab.label}
        </NavLink>
      ))}
    </nav>
  )
}
