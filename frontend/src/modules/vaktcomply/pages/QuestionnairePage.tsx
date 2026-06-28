import { useState, useRef } from 'react'
import { useParams } from 'react-router-dom'
import { GripVertical, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  useQuestionnaire,
  useAddQuestion,
  useReorderQuestions,
} from '../hooks/useQuestionnaires'
import type { Question, QuestionType, CreateQuestionInput } from '../types'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const TYPE_LABELS: Record<QuestionType, string> = {
  yes_no: 'Ja/Nein',
  multiple_choice: 'Mehrfachauswahl',
  free_text: 'Freitext',
  file_upload: 'Dateiupload',
}

const TYPE_CLASS: Record<QuestionType, string> = {
  yes_no: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  multiple_choice: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
  free_text: 'bg-secondary text-secondary-foreground',
  file_upload: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
}

/**
 * buildReorderPayload moves draggedId to just before targetId in the currentOrder array.
 * Exported for testing.
 */
export function buildReorderPayload(
  draggedId: string,
  targetId: string,
  currentOrder: string[],
): string[] {
  if (draggedId === targetId) return [...currentOrder]
  const filtered = currentOrder.filter((id) => id !== draggedId)
  const targetIndex = filtered.indexOf(targetId)
  if (targetIndex === -1) {
    // targetId not found — append at end
    return [...filtered, draggedId]
  }
  const result = [...filtered]
  result.splice(targetIndex, 0, draggedId)
  return result
}

// ─── QuestionRow ─────────────────────────────────────────────────────────────

interface QuestionRowProps {
  question: Question
  onDragStart: (id: string) => void
  onDragOver: (e: React.DragEvent, id: string) => void
  onDrop: (targetId: string) => void
}

function QuestionRow({ question, onDragStart, onDragOver, onDrop }: QuestionRowProps) {
  const { t } = useTranslation()
  return (
    <Card
      className="mb-2 cursor-grab active:cursor-grabbing"
      draggable
      onDragStart={() => { onDragStart(question.id) }}
      onDragOver={(e) => { onDragOver(e, question.id) }}
      onDrop={() => { onDrop(question.id) }}
    >
      <CardContent className="flex items-center gap-3 p-3">
        <span
          data-testid="drag-handle"
          className="text-muted-foreground cursor-grab"
          aria-label="Drag handle"
        >
          <GripVertical className="h-4 w-4" />
        </span>
        <span className="flex-1 text-sm">{question.question_text}</span>
        <Badge className={TYPE_CLASS[question.question_type]}>
          {TYPE_LABELS[question.question_type]}
        </Badge>
        {question.required && (
          <span className="text-xs text-muted-foreground">{t('vaktcomply.questionnaire.required')}</span>
        )}
        {question.control_id && (
          <span className="text-xs text-muted-foreground" title={`Control: ${question.control_id}`}>
            🔗
          </span>
        )}
      </CardContent>
    </Card>
  )
}

// ─── AddQuestionDialog ───────────────────────────────────────────────────────

interface AddQuestionDialogProps {
  questionnaireId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

function AddQuestionDialog({ questionnaireId, open, onOpenChange }: AddQuestionDialogProps) {
  const { t } = useTranslation()
  const addQuestion = useAddQuestion(questionnaireId)
  const [questionText, setQuestionText] = useState('')
  const [questionType, setQuestionType] = useState<QuestionType>('yes_no')
  const [optionsRaw, setOptionsRaw] = useState('')

  function handleSubmit() {
    const input: CreateQuestionInput = {
      question_text: questionText,
      question_type: questionType,
      required: true,
    }
    if (questionType === 'multiple_choice') {
      input.options = optionsRaw
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean)
    }
    addQuestion.mutate(input, {
      onSuccess: () => {
        setQuestionText('')
        setQuestionType('yes_no')
        setOptionsRaw('')
        onOpenChange(false)
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('vaktcomply.questionnaire.addQuestion')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div>
            <Label htmlFor="question-text">{t('vaktcomply.questionnaire.labelQuestionText')}</Label>
            <Input
              id="question-text"
              value={questionText}
              onChange={(e) => { setQuestionText(e.target.value) }}
              placeholder={t('vaktcomply.questionnaire.placeholderQuestionText')}
            />
          </div>
          <div>
            <Label htmlFor="question-type">{t('vaktcomply.questionnaire.labelQuestionType')}</Label>
            <Select
              value={questionType}
              onValueChange={(v) => { setQuestionType(v as QuestionType) }}
            >
              <SelectTrigger id="question-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.entries(TYPE_LABELS) as [QuestionType, string][]).map(([value, label]) => (
                  <SelectItem key={value} value={value}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {questionType === 'multiple_choice' && (
            <div>
              <Label htmlFor="options">{t('vaktcomply.questionnaire.labelOptions')}</Label>
              <textarea
                id="options"
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                rows={4}
                value={optionsRaw}
                onChange={(e) => { setOptionsRaw(e.target.value) }}
                placeholder="Option A&#10;Option B&#10;Option C"
              />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => { onOpenChange(false) }}>
            {t('common.cancel')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!questionText.trim() || addQuestion.isPending}
          >
            {t('vaktcomply.questionnaire.add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── QuestionnairePage ───────────────────────────────────────────────────────

export default function QuestionnairePage() {
  const { t } = useTranslation()
  const { id = '' } = useParams<{ id: string }>()
  const { data: questionnaire, isLoading } = useQuestionnaire(id)
  const reorderQuestions = useReorderQuestions(id)
  const [dialogOpen, setDialogOpen] = useState(false)
  const draggedIdRef = useRef<string | null>(null)

  const questions = (questionnaire?.questions ?? []).slice().sort((a, b) => a.order_idx - b.order_idx)

  function handleDragStart(questionId: string) {
    draggedIdRef.current = questionId
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  function handleDragOver(e: React.DragEvent, _targetId: string) {
    e.preventDefault()
  }

  function handleDrop(targetId: string) {
    const draggedId = draggedIdRef.current
    if (!draggedId || draggedId === targetId) return
    draggedIdRef.current = null
    const currentOrder = questions.map((q) => q.id)
    const newOrder = buildReorderPayload(draggedId, targetId, currentOrder)
    reorderQuestions.mutate({ order: newOrder })
  }

  if (isLoading) {
    return <div className="p-6 text-muted-foreground">{t('vaktcomply.questionnaire.loading')}</div>
  }

  if (!questionnaire) {
    return <div className="p-6 text-muted-foreground">{t('vaktcomply.questionnaire.notFound')}</div>
  }

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title={questionnaire.name}
        description={questionnaire.description}
      />

      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {t('vaktcomply.questionnaire.questionCount', { count: questions.length })}
        </p>
        <Button onClick={() => { setDialogOpen(true) }} size="sm">
          <Plus className="mr-1 h-4 w-4" />
          {t('vaktcomply.questionnaire.addQuestion')}
        </Button>
      </div>

      <div>
        {questions.length === 0 ? (
          <div className="text-center py-12 text-muted-foreground text-sm">
            {t('vaktcomply.questionnaire.emptyState')}
          </div>
        ) : (
          questions.map((q) => (
            <QuestionRow
              key={q.id}
              question={q}
              onDragStart={handleDragStart}
              onDragOver={handleDragOver}
              onDrop={handleDrop}
            />
          ))
        )}
      </div>

      <AddQuestionDialog
        questionnaireId={id}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
      />
    </div>
  )
}
