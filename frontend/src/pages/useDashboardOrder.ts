import { useState } from 'react'

export const DEFAULT_WIDGET_ORDER = [
  'today', 'my_tasks', 'score_history', 'quick_wins',
  'compliance_progress', 'frameworks', 'risks', 'activity', 'modules',
]

export function useDashboardOrder(defaultOrder: string[]) {
  const [order, setOrder] = useState<string[]>(() => {
    try {
      const saved = JSON.parse(localStorage.getItem('vakt_dashboard_order') ?? '[]') as string[]
      const merged = saved.filter((id) => defaultOrder.includes(id))
      const added = defaultOrder.filter((id) => !merged.includes(id))
      const result = [...merged, ...added]
      return result.length > 0 ? result : defaultOrder
    } catch {
      return defaultOrder
    }
  })
  const saveOrder = (newOrder: string[]) => {
    setOrder(newOrder)
    localStorage.setItem('vakt_dashboard_order', JSON.stringify(newOrder))
  }
  return { order, saveOrder }
}
