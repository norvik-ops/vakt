import { useState, useRef, useEffect, useCallback } from 'react'
import { useDashboardScore, useDashboardAggregate } from '../hooks/useDashboard'
import { useFindings } from '../modules/secpulse/hooks/useFindings'
import { useFrameworks } from '../modules/secvitals/hooks/useFrameworks'
import { useProjects } from '../modules/secvault/hooks/useProjects'
import { useCampaigns } from '../modules/secreflex/hooks/useCampaigns'
import { useBreaches } from '../modules/secprivacy/hooks/useBreaches'
import { useScoreHistory } from '../modules/secvitals/hooks/useScoreHistory'
import { useNextMilestone } from '../modules/secvitals/hooks/useMilestones'
import { useRecentPages } from '../shared/hooks/useRecentPages'
import { useOnboardingStatus } from '../hooks/useOnboarding'
import { useDashboardOrder, DEFAULT_WIDGET_ORDER } from './useDashboardOrder'
import { loadWidgets, WIDGETS_KEY } from './WidgetConfigPanel'
import type { WidgetKey } from './WidgetConfigPanel'
import { DashboardLayout } from './DashboardLayout'
import { WidgetGrid } from './WidgetGrid'

export default function Dashboard() {
  const { data: onboarding } = useOnboardingStatus()
  const [wizardOpen, setWizardOpen] = useState(false)

  const { data: scoreData, isLoading: scoreLoading, isError: scoreError } = useDashboardScore()
  const { data: agg, isLoading: aggLoading, isError: aggError } = useDashboardAggregate()
  const { data: scoreHistory } = useScoreHistory(30)
  const { data: critFindings, isLoading: findingsLoading } = useFindings({ severity: 'critical' })
  const { data: frameworks, isLoading: fwLoading } = useFrameworks()
  const { data: projects, isLoading: projLoading } = useProjects()
  const { data: campaigns, isLoading: campLoading } = useCampaigns()
  const { data: breaches, isLoading: breachLoading } = useBreaches()
  const { data: nextMilestone } = useNextMilestone()
  const recentPages = useRecentPages()

  const [widgets, setWidgets] = useState<Record<WidgetKey, boolean>>(() => loadWidgets())
  const [widgetMenuOpen, setWidgetMenuOpen] = useState(false)
  const widgetMenuRef = useRef<HTMLDivElement>(null)
  const { order: widgetOrder, saveOrder } = useDashboardOrder(DEFAULT_WIDGET_ORDER)
  const [editMode, setEditMode] = useState(false)
  const dragItem = useRef<string | null>(null)
  const dragOverItem = useRef<string | null>(null)

  const handleDragStart = useCallback((widgetId: string) => {
    dragItem.current = widgetId
  }, [])

  const handleDragOver = useCallback((e: React.DragEvent, widgetId: string) => {
    e.preventDefault()
    dragOverItem.current = widgetId
  }, [])

  const handleDrop = useCallback(() => {
    if (!dragItem.current || !dragOverItem.current || dragItem.current === dragOverItem.current) return
    const newOrder = [...widgetOrder]
    const fromIdx = newOrder.indexOf(dragItem.current)
    const toIdx = newOrder.indexOf(dragOverItem.current)
    if (fromIdx === -1 || toIdx === -1) return
    newOrder.splice(fromIdx, 1)
    newOrder.splice(toIdx, 0, dragItem.current)
    saveOrder(newOrder)
    dragItem.current = null
    dragOverItem.current = null
  }, [widgetOrder, saveOrder])

  useEffect(() => {
    if (!widgetMenuOpen) return
    function handler(e: MouseEvent) {
      if (widgetMenuRef.current && !widgetMenuRef.current.contains(e.target as Node)) {
        setWidgetMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => { document.removeEventListener('mousedown', handler) }
  }, [widgetMenuOpen])

  function toggleWidget(key: WidgetKey) {
    setWidgets((prev) => {
      const next = { ...prev, [key]: !prev[key] }
      try { localStorage.setItem(WIDGETS_KEY, JSON.stringify(next)) } catch { /* storage unavailable */ }
      return next
    })
  }

  const scoreTrend: number | null = (() => {
    if (!scoreHistory || scoreHistory.length < 2) return null
    return scoreHistory[scoreHistory.length - 1].score - scoreHistory[0].score
  })()

  const critCount = critFindings?.pagination.total ?? null
  const fwCount = frameworks?.length ?? null
  const projCount = projects?.length ?? null
  const activeCampaignCount =
    campaigns?.filter((c) => c.status === 'running' || c.status === 'scheduled').length ?? null
  const openBreachCount = breaches?.filter((b) => b.status === 'open').length ?? null

  return (
    <div className="flex flex-col lg:flex-row h-full">
      <DashboardLayout
        scoreLoading={scoreLoading}
        scoreData={scoreData}
        scoreTrend={scoreTrend}
        critCount={critCount}
        findingsLoading={findingsLoading}
        fwCount={fwCount}
        fwLoading={fwLoading}
        projCount={projCount}
        projLoading={projLoading}
        activeCampaignCount={activeCampaignCount}
        campLoading={campLoading}
        openBreachCount={openBreachCount}
        breachLoading={breachLoading}
        editMode={editMode}
        setEditMode={setEditMode}
        widgets={widgets}
        toggleWidget={toggleWidget}
        widgetMenuOpen={widgetMenuOpen}
        setWidgetMenuOpen={setWidgetMenuOpen}
        widgetMenuRef={widgetMenuRef}
      />
      <WidgetGrid
        widgetOrder={widgetOrder}
        editMode={editMode}
        handleDragStart={handleDragStart}
        handleDragOver={handleDragOver}
        handleDrop={handleDrop}
        agg={agg}
        aggLoading={aggLoading}
        scoreHistory={scoreHistory}
        widgets={widgets}
        onboarding={onboarding}
        wizardOpen={wizardOpen}
        setWizardOpen={setWizardOpen}
        scoreError={scoreError}
        aggError={aggError}
        recentPages={recentPages}
        nextMilestone={nextMilestone}
        kpiLoading={aggLoading}
        critCount={critCount}
        fwCount={fwCount}
        projCount={projCount}
        activeCampaignCount={activeCampaignCount}
        openBreachCount={openBreachCount}
      />
    </div>
  )
}
