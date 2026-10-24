import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LayoutTemplate, BookOpen, ExternalLink } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../../components/ui/tabs'
import { EmptyState } from '../../../shared/components/EmptyState'
import { SkeletonCardGrid } from '../../../shared/components/SkeletonLoaders'
import { apiFetch } from '../../../api/client'
import { toast } from '../../../shared/hooks/useToast'
import { useCreatePolicy } from '../hooks/usePolicies'
import { handleApiError } from '../../../shared/utils/errorMessages'

interface DBTemplate {
  id: string
  category: string
  name: string
  description: string
  content: string
  tags: string[]
  framework?: string
  created_at: string
}

// DSFA- und AVV-Vorlagen führten zu Vakt Privacy, das archiviert ist.
type Category = 'policy'

const CATEGORY_LABELS: Record<Category, string> = {
  policy: 'Richtlinien',
}

const CATEGORY_ICONS: Record<Category, React.ElementType> = {
  policy: BookOpen,
}

const CATEGORY_DESTINATIONS: Record<Category, string> = {
  policy: '/vaktcomply/policies',
}

function TagBadge({ tag }: { tag: string }) {
  const colorMap: Record<string, string> = {
    iso27001: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
    nis2: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
    dsgvo: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
    art35: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
    art28: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
  }
  const cls = colorMap[tag] ?? 'bg-secondary text-secondary-foreground'
  return (
    <Badge className={`text-[10px] px-1.5 py-0 border ${cls}`}>{tag}</Badge>
  )
}

function TemplateCard({
  template,
  onPreview,
}: {
  template: DBTemplate
  onPreview: (t: DBTemplate) => void
}) {
  return (
    <Card
      className="cursor-pointer hover:border-brand/50 transition-colors"
      onClick={() => { onPreview(template); }}
    >
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm leading-snug">{template.name}</p>
          {template.framework && (
            <Badge variant="outline" className="shrink-0 text-[10px] px-1.5 py-0">
              {template.framework}
            </Badge>
          )}
        </div>
        <p className="text-xs text-muted-foreground line-clamp-3">{template.description}</p>
        {template.tags.length > 0 && (
          <div className="flex flex-wrap gap-1 pt-1">
            {template.tags.map((tag) => (
              <TagBadge key={tag} tag={tag} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function TemplateCategoryTab({ category }: { category: Category }) {
  const [preview, setPreview] = useState<DBTemplate | null>(null)
  const navigate = useNavigate()
  const createPolicy = useCreatePolicy()

  const { data: templates, isLoading, isError } = useQuery<DBTemplate[]>({
    queryKey: ['vaktcomply', 'templates', category],
    queryFn: () => apiFetch<DBTemplate[]>(`/vaktcomply/templates?category=${category}`),
    staleTime: 10 * 60 * 1000,
  })

  function handleUseTemplate(t: DBTemplate) {
    const dest = CATEGORY_DESTINATIONS[category]
    // R1-SA28-02: There is NO /vaktcomply/policies/new route — policies/:id would
    // treat "new" as a UUID path param and ValidateUUIDParams returns 400. The old
    // code navigated there and fired a success toast that lied. Instead create the
    // policy directly from the DB template (mirrors the backend
    // CreatePolicyFromTemplate flow: title=name, body=content) and land on its detail
    // page. The success toast now fires only after the policy actually exists.
    createPolicy.mutate(
      { title: t.name, description: t.content, category: t.framework ?? undefined },
      {
        onSuccess: (policy) => {
          toast(`Richtlinie "${t.name}" aus Vorlage erstellt`, 'success')
          navigate(`${dest}/${policy.id}`)
        },
        onError: (err) => toast(handleApiError(err), 'error'),
      },
    )
  }

  return (
    <>
      {isLoading && <SkeletonCardGrid count={4} />}
      {isError && (
        <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
          Vorlagen konnten nicht geladen werden.
        </div>
      )}
      {!isLoading && !isError && (!templates || templates.length === 0) && (
        <EmptyState
          icon={LayoutTemplate}
          title="Keine Vorlagen verfügbar"
          description="Für diese Kategorie sind noch keine Vorlagen vorhanden."
        />
      )}
      {!isLoading && !isError && templates && templates.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {templates.map((t) => (
            <TemplateCard key={t.id} template={t} onPreview={setPreview} />
          ))}
        </div>
      )}

      {/* Preview dialog */}
      <Dialog open={!!preview} onOpenChange={(open) => { if (!open) setPreview(null) }}>
        <DialogContent className="max-w-3xl max-h-[90vh] flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <LayoutTemplate className="w-4 h-4 text-brand shrink-0" />
              {preview?.name}
            </DialogTitle>
          </DialogHeader>

          {preview && (
            <div className="flex-1 overflow-y-auto space-y-4 py-2">
              <p className="text-sm text-muted-foreground">{preview.description}</p>

              {preview.tags.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  {preview.tags.map((tag) => <TagBadge key={tag} tag={tag} />)}
                  {preview.framework && (
                    <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                      {preview.framework}
                    </Badge>
                  )}
                </div>
              )}

              <div className="border border-border rounded-lg bg-surface p-4 text-xs font-mono whitespace-pre-wrap leading-relaxed max-h-[50vh] overflow-y-auto">
                {preview.content}
              </div>
            </div>
          )}

          <DialogFooter className="mt-2">
            <Button variant="outline" onClick={() => { setPreview(null); }}>
              Schließen
            </Button>
            <Button
              onClick={() => {
                if (preview) {
                  handleUseTemplate(preview)
                  setPreview(null)
                }
              }}
            >
              <ExternalLink className="w-4 h-4 mr-1" />
              Diese Vorlage verwenden
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

export default function PolicyTemplatesPage() {
  const [activeCategory, setActiveCategory] = useState<Category>('policy')

  const categories: Category[] = ['policy']

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Vorlagenbibliothek"
        description="Vorgefertigte Richtlinien-Vorlagen — kein leeres Blatt Papier mehr."
        actions={null}
      />

      <div className="flex-1 p-6">
        <Tabs
          value={activeCategory}
          onValueChange={(v) => { setActiveCategory(v as Category); }}
        >
          <TabsList className="mb-6">
            {categories.map((cat) => {
              const Icon = CATEGORY_ICONS[cat]
              return (
                <TabsTrigger key={cat} value={cat} className="gap-1.5">
                  <Icon className="w-3.5 h-3.5" />
                  {CATEGORY_LABELS[cat]}
                </TabsTrigger>
              )
            })}
          </TabsList>

          {categories.map((cat) => (
            <TabsContent key={cat} value={cat}>
              <TemplateCategoryTab category={cat} />
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </div>
  )
}
