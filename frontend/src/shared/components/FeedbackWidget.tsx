import { useState } from 'react'
import { MessageSquarePlus, X, Star, Send, CheckCircle } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { useLocation } from 'react-router-dom'

type State = 'closed' | 'open' | 'success'

export function FeedbackWidget() {
  const [state, setState] = useState<State>('closed')
  const [rating, setRating] = useState(0)
  const [hover, setHover] = useState(0)
  const [message, setMessage] = useState('')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const location = useLocation()

  function reset() {
    setRating(0)
    setHover(0)
    setMessage('')
    setName('')
    setEmail('')
    setError(null)
  }

  function close() {
    setState('closed')
    reset()
  }

  async function submit() {
    if (rating === 0) { setError('Bitte eine Bewertung vergeben.'); return }
    if (!message.trim()) { setError('Bitte kurzes Feedback eingeben.'); return }
    setSubmitting(true)
    setError(null)
    try {
      const res = await fetch('/api/v1/feedback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ rating, message: message.trim(), name: name.trim(), email: email.trim(), page: location.pathname }),
      })
      if (!res.ok) throw new Error('Fehler beim Senden')
      setState('success')
      reset()
    } catch {
      setError('Feedback konnte nicht gesendet werden. Bitte erneut versuchen.')
    } finally {
      setSubmitting(false)
    }
  }

  const labels: Record<number, string> = {
    1: 'Nicht hilfreich',
    2: 'Ausbaufähig',
    3: 'In Ordnung',
    4: 'Gut',
    5: 'Ausgezeichnet',
  }

  return (
    <>
      {/* Floating trigger button */}
      {state === 'closed' && (
        <button
          onClick={() => setState('open')}
          aria-label="Feedback geben"
          className="fixed bottom-6 right-6 z-50 flex items-center gap-2 bg-brand text-white text-sm font-medium px-4 py-2.5 rounded-full shadow-lg hover:bg-brand/90 transition-all duration-200 hover:scale-105"
        >
          <MessageSquarePlus className="w-4 h-4" />
          Feedback geben
        </button>
      )}

      {/* Success state */}
      {state === 'success' && (
        <div className="fixed bottom-6 right-6 z-50 bg-surface border border-border rounded-xl shadow-xl p-5 w-80 flex flex-col items-center gap-3 text-center">
          <CheckCircle className="w-10 h-10 text-green-400" />
          <p className="font-semibold text-primary">Danke für Ihr Feedback!</p>
          <p className="text-sm text-secondary">Ihr Feedback hilft uns, Vakt besser zu machen.</p>
          <Button size="sm" variant="outline" onClick={() => setState('closed')}>Schließen</Button>
        </div>
      )}

      {/* Feedback modal */}
      {state === 'open' && (
        <div className="fixed bottom-6 right-6 z-50 bg-surface border border-border rounded-xl shadow-xl w-80 overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-border">
            <div>
              <p className="text-sm font-semibold text-primary">Wie gefällt Ihnen Vakt?</p>
              <p className="text-xs text-secondary mt-0.5">Demo-Feedback · anonym möglich</p>
            </div>
            <button onClick={close} aria-label="Schließen" className="text-secondary hover:text-primary transition-colors p-1 rounded">
              <X className="w-4 h-4" />
            </button>
          </div>

          <div className="p-4 space-y-4">
            {/* Star rating */}
            <div>
              <div className="flex gap-1 justify-center mb-1">
                {[1, 2, 3, 4, 5].map((star) => (
                  <button
                    key={star}
                    onClick={() => setRating(star)}
                    onMouseEnter={() => setHover(star)}
                    onMouseLeave={() => setHover(0)}
                    aria-label={`${star} Stern${star !== 1 ? 'e' : ''}`}
                    className="transition-transform hover:scale-110"
                  >
                    <Star
                      className={`w-8 h-8 transition-colors ${
                        star <= (hover || rating)
                          ? 'text-amber-400 fill-amber-400'
                          : 'text-secondary'
                      }`}
                    />
                  </button>
                ))}
              </div>
              {(hover || rating) > 0 && (
                <p className="text-xs text-center text-secondary">{labels[hover || rating]}</p>
              )}
            </div>

            {/* Message */}
            <div>
              <textarea
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                placeholder="Was hat Ihnen gefallen? Was fehlt noch?"
                rows={3}
                className="w-full text-sm rounded-lg border border-border bg-bg px-3 py-2 text-primary placeholder:text-secondary resize-none focus:outline-none focus:ring-1 focus:ring-brand/50"
              />
            </div>

            {/* Optional fields */}
            <div className="grid grid-cols-2 gap-2">
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Name (optional)"
                className="text-xs rounded-lg border border-border bg-bg px-3 py-2 text-primary placeholder:text-secondary focus:outline-none focus:ring-1 focus:ring-brand/50"
              />
              <input
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="E-Mail (optional)"
                type="email"
                className="text-xs rounded-lg border border-border bg-bg px-3 py-2 text-primary placeholder:text-secondary focus:outline-none focus:ring-1 focus:ring-brand/50"
              />
            </div>

            {error && (
              <p className="text-xs text-red-400">{error}</p>
            )}

            <Button
              onClick={() => void submit()}
              disabled={submitting}
              className="w-full gap-2"
              size="sm"
            >
              <Send className="w-3.5 h-3.5" />
              {submitting ? 'Wird gesendet…' : 'Feedback senden'}
            </Button>
          </div>
        </div>
      )}
    </>
  )
}
