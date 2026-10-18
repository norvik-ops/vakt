// Asset-Inventar (ISO 27001 A.5.9), seit module-archivieren S1 in Vakt Comply.
// Spiegelt backend/internal/modules/vaktcomply/assets/models.go.

export type ClassificationLevel = 'public' | 'internal' | 'confidential' | 'restricted'

// K5-16: mirrors assets.Asset. There is no `target` field on the wire — the
// column is `external_url` (vb_assets has no dedicated ip/target column). The old name made the Target table column render "—" for every
// asset and made the create form's input a dead end.
export interface Asset {
  id: string
  org_id: string
  name: string
  type: 'web_app' | 'server' | 'database' | 'container' | 'repo'
  criticality: 'low' | 'medium' | 'high' | 'critical'
  environment?: string
  classification?: ClassificationLevel
  tags: string[]
  owner_id?: string | null
  external_url?: string | null
  protection_need_id?: string | null
  created_at: string
  updated_at: string
}

export interface ClassificationSummary {
  total_count: number
  classified_count: number
  unclassified_count: number
  by_level: Record<ClassificationLevel, number>
}
