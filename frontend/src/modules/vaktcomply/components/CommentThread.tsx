import { useState } from 'react'
import { Trash2, MessageSquare } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Button } from '../../../components/ui/button'
import { useComments, useCreateComment, useDeleteComment } from '../hooks/useTasks'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

// ── Relative time helper ──────────────────────────────────────────────────────

function relativeTime(dateStr: string, fallbackFormat: (v: string) => string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const minutes = Math.floor(diff / 60_000)
  if (minutes < 1) return 'Gerade eben'
  if (minutes < 60) return `vor ${minutes.toString()} Min.`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `vor ${hours.toString()} Std.`
  const days = Math.floor(hours / 24)
  if (days < 7) return `vor ${days.toString()} Tag${days === 1 ? '' : 'en'}`
  return fallbackFormat(dateStr)
}

// ── Main component ────────────────────────────────────────────────────────────

export function CommentThread({
  entityType,
  entityId,
}: {
  entityType: string
  entityId: string
}) {
  const { data: comments, isLoading } = useComments(entityType, entityId)
  const createComment = useCreateComment(entityType, entityId)
  const deleteComment = useDeleteComment(entityType, entityId)
  const { formatDate, formatDateTime } = useFormatDate()

  const [body, setBody] = useState('')
  const [authorEmail, setAuthorEmail] = useState('')

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!body.trim()) return
    createComment.mutate(
      { body: body.trim(), author_email: authorEmail.trim() || undefined },
      { onSuccess: () => { setBody(''); } },
    )
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm flex items-center gap-2">
          <MessageSquare className="w-4 h-4" />
          Kommentare
          {comments && comments.length > 0 && (
            <span className="text-xs font-normal text-secondary">({comments.length.toString()})</span>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Comment list */}
        {isLoading ? (
          <div className="flex justify-center py-4">
            <Spinner size="sm" />
          </div>
        ) : comments && comments.length > 0 ? (
          <ul className="space-y-3">
            {comments.map((comment) => (
              <li
                key={comment.id}
                className="flex gap-3 group"
              >
                {/* Avatar placeholder */}
                <div className="w-7 h-7 rounded-full bg-surface2 border border-border flex items-center justify-center shrink-0 text-xs font-medium text-secondary">
                  {comment.author_email ? comment.author_email.charAt(0).toUpperCase() : '?'}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs font-medium text-primary">
                      {comment.author_email || 'Anonym'}
                    </span>
                    <span className="text-xs text-secondary" title={formatDateTime(comment.created_at)}>
                      {relativeTime(comment.created_at, formatDate)}
                    </span>
                  </div>
                  <p className="text-sm text-primary leading-relaxed whitespace-pre-wrap break-words">
                    {comment.body}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => { deleteComment.mutate(comment.id); }}
                  disabled={deleteComment.isPending}
                  className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity shrink-0 mt-0.5"
                  title="Kommentar löschen"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-xs text-muted-foreground">
            Noch keine Kommentare. Schreibe den ersten Kommentar.
          </p>
        )}

        {/* New comment form */}
        <form onSubmit={handleSubmit} className="space-y-2 border-t border-border pt-4">
          {!authorEmail && (
            <input
              type="email"
              placeholder="Deine E-Mail (optional)"
              value={authorEmail}
              onChange={(e) => { setAuthorEmail(e.target.value); }}
              className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-1.5 text-xs focus:outline-none focus:ring-1 focus:ring-brand"
            />
          )}
          <textarea
            rows={3}
            placeholder="Kommentar schreiben …"
            value={body}
            onChange={(e) => { setBody(e.target.value); }}
            maxLength={5000}
            className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand resize-none"
          />
          <div className="flex justify-end">
            <Button
              type="submit"
              size="sm"
              disabled={!body.trim() || createComment.isPending}
            >
              {createComment.isPending ? 'Wird gesendet…' : 'Kommentar senden'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
