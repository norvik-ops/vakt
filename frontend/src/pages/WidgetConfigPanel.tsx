export const WIDGETS_KEY = 'vakt_dashboard_widgets'

export type WidgetKey =
  | 'compliance_score'
  | 'open_findings'
  | 'incidents'
  | 'recent_pages'
  | 'onboarding'
  | 'evidence_expiry'

export const DEFAULT_WIDGETS: Record<WidgetKey, boolean> = {
  compliance_score: true,
  open_findings: true,
  incidents: true,
  recent_pages: true,
  onboarding: true,
  evidence_expiry: true,
}

export const WIDGET_LABELS: Record<WidgetKey, string> = {
  compliance_score: 'Compliance-Score',
  open_findings: 'Offene Findings',
  incidents: 'Incidents',
  recent_pages: 'Zuletzt besucht',
  onboarding: 'Onboarding-Checkliste',
  evidence_expiry: 'Erste Schritte',
}

export function loadWidgets(): Record<WidgetKey, boolean> {
  try {
    const saved = JSON.parse(
      localStorage.getItem(WIDGETS_KEY) ?? '{}',
    ) as Partial<Record<WidgetKey, boolean>>
    return { ...DEFAULT_WIDGETS, ...saved }
  } catch {
    return DEFAULT_WIDGETS
  }
}
