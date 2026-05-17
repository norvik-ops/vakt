import { useState } from 'react'
import { Trash2, MessageSquare, ChevronDown, ChevronUp } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { Button } from '../../components/ui/button'
import { apiFetch } from '../../api/client'

// ── Types ──────────────────────────────────────────────────────────────────────

interface Comment {
  id: string
  author_name: string
  author_id: string
  content: string
  created_at: string
  can_delete: boolean
}

// ── Relative time helper ──────────────────────────────────────────────────────

function relativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const minutes = Math.floor(diff / 60_000)
  if (minutes < 1) return 'Gerade eben'
  if (minutes < 60) return `vor ${minutes.toString()} Min.`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `vor ${hours.toString()} Std.`
  const days = Math.floor(hours / 24)
  if (days < 7) return `vor ${days.toString()} Tag${days === 1 ? '' : 'en'}`
  return new Date(dateStr).toLocaleDateString('de-DE')
}

// ── Avatar initials helper ────────────────────────────────────────────────────

function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  if (parts.length >= 2) return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
  return name.slice(0, 2).toUpperCase()
}

// ── Hooks ──────────────────────────────────────────────────────────────────────

function useComments(entityType: string, entityId: string) {
  return useQuery<Comment[]>({
    queryKey: ['comments', entityType, entityId],
    queryFn: () =>
      apiFetch<Comment[]>(`/comments?entity_type=${entityType}&entity_id=${entityId}`),
    enabled: !!entityType && !!entityId,
    staleTime: 2 * 60 * 1000,
  })
}

function useCreateComment(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<Comment, Error, { content: string }>({
    mutationFn: (input) =>
      apiFetch<Comment>('/comments', {
        method: 'POST',
        body: JSON.stringify({ entity_type: entityType, entity_id: entityId, content: input.content }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['comments', entityType, entityId] })
    },
  })
}

function useDeleteComment(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (commentId) =>
      apiFetch<void>(`/comments/${commentId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['comments', entityType, entityId] })
    },
  })
}

// ── Skeleton ──────────────────────────────────────────────────────────────────

function CommentSkeleton() {
  return (
    <div className="flex gap-3 animate-pulse">
      <div className="w-7 h-7 rounded-full bg-surface2 shrink-0" />
      <div className="flex-1 space-y-2">
        <div className="h-3 bg-surface2 rounded w-1/4" />
        <div className="h-3 bg-surface2 rounded w-3/4" />
      </div>
    </div>
  )
}

// ── Props ──────────────────────────────────────────────────────────────────────

export interface CommentsProps {
  entityType: 'finding' | 'control'
  entityId: string
}

// ── Main component ─────────────────────────────────────────────────────────────

export function Comments({ entityType, entityId }: CommentsProps) {
  const { data: comments, isLoading } = useComments(entityType, entityId)
  const createComment = useCreateComment(entityType, entityId)
  const deleteComment = useDeleteComment(entityType, entityId)

  const [content, setContent] = useState('')
  const [expanded, setExpanded] = useState(true)

  const count = comments?.length ?? 0

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!content.trim()) return
    createComment.mutate(
      { content: content.trim() },
      { onSuccess: () => setContent('') },
    )
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm flex items-center gap-2">
            <MessageSquare className="w-4 h-4" />
            Kommentare
            {count > 0 && (
              <span className="text-xs font-normal text-secondary">({count.toString()})</span>
            )}
          </CardTitle>
          {count > 0 && (
            <button
              type="button"
              onClick={() => setExpanded((v) => !v)}
              className="flex items-center gap-1 text-xs text-secondary hover:text-primary transition-colors"
            >
              {expanded ? (
                <>
                  <ChevronUp className="w-3.5 h-3.5" />
                  Ausblenden
                </>
              ) : (
                <>
                  <ChevronDown className="w-3.5 h-3.5" />
                  {count.toString()} {count === 1 ? 'Kommentar' : 'Kommentare'} anzeigen
                </>
              )}
            </button>
          )}
        </div>
      </CardHeader>

      <CardContent className="space-y-4">
        {/* Comment list */}
        {isLoading ? (
          <div className="space-y-3">
            <CommentSkeleton />
            <CommentSkeleton />
          </div>
        ) : count === 0 ? (
          <p className="text-xs text-muted-foreground">Noch keine Kommentare.</p>
        ) : expanded ? (
          <ul className="space-y-3">
            {comments?.map((comment) => (
              <li key={comment.id} className="flex gap-3 group">
                {/* Avatar circle with initials */}
                <div className="w-7 h-7 rounded-full bg-brand/10 border border-border flex items-center justify-center shrink-0 text-xs font-medium text-brand">
                  {initials(comment.author_name)}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs font-medium text-primary">{comment.author_name}</span>
                    <span
                      className="text-xs text-secondary"
                      title={new Date(comment.created_at).toLocaleString('de-DE')}
                    >
                      {relativeTime(comment.created_at)}
                    </span>
                  </div>
                  <p className="text-sm text-primary leading-relaxed whitespace-pre-wrap break-words">
                    {comment.content}
                  </p>
                </div>
                {comment.can_delete && (
                  <button
                    type="button"
                    onClick={() => deleteComment.mutate(comment.id)}
                    disabled={deleteComment.isPending}
                    className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity shrink-0 mt-0.5"
                    aria-label="Kommentar löschen"
                    title="Kommentar löschen"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                )}
              </li>
            ))}
          </ul>
        ) : null}

        {/* New comment form */}
        <form onSubmit={handleSubmit} className="space-y-2 border-t border-border pt-4">
          <textarea
            rows={3}
            placeholder="Kommentar schreiben …"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            maxLength={4000}
            className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand resize-none"
          />
          <div className="flex justify-end">
            <Button
              type="submit"
              size="sm"
              disabled={!content.trim() || createComment.isPending}
            >
              {createComment.isPending ? 'Wird gesendet…' : 'Kommentieren'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
