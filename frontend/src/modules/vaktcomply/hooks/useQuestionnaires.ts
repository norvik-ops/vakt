import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  Questionnaire,
  Question,
  CreateQuestionnaireInput,
  CreateQuestionInput,
  ReorderQuestionsInput,
} from '../types'

export function useQuestionnaires(isTemplate?: boolean) {
  const qs = isTemplate !== undefined ? `?is_template=${isTemplate.toString()}` : ''
  return useQuery<Questionnaire[]>({
    queryKey: ['vaktcomply', 'questionnaires', isTemplate ?? 'all'],
    queryFn: () => apiFetch<Questionnaire[]>(`/vaktcomply/questionnaires${qs}`),
    staleTime: 5 * 60 * 1000,
  })
}

export function useQuestionnaire(id: string) {
  return useQuery<Questionnaire>({
    queryKey: ['vaktcomply', 'questionnaires', id],
    queryFn: () => apiFetch<Questionnaire>(`/vaktcomply/questionnaires/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useTemplates() {
  return useQuery<Questionnaire[]>({
    queryKey: ['vaktcomply', 'questionnaires', 'templates'],
    queryFn: () => apiFetch<Questionnaire[]>('/vaktcomply/questionnaires/templates'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateQuestionnaire() {
  const queryClient = useQueryClient()
  return useMutation<Questionnaire, Error, CreateQuestionnaireInput>({
    mutationFn: (input) =>
      apiFetch<Questionnaire>('/vaktcomply/questionnaires', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'questionnaires'] })
    },
  })
}

export function useUpdateQuestionnaire(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Questionnaire, Error, CreateQuestionnaireInput>({
    mutationFn: (input) =>
      apiFetch<Questionnaire>(`/vaktcomply/questionnaires/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'questionnaires'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'questionnaires', id] })
    },
  })
}

export function useDeleteQuestionnaire() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/questionnaires/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'questionnaires'] })
    },
  })
}

export function useAddQuestion(questionnaireId: string) {
  const queryClient = useQueryClient()
  return useMutation<Question, Error, CreateQuestionInput>({
    mutationFn: (input) =>
      apiFetch<Question>(`/vaktcomply/questionnaires/${questionnaireId}/questions`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'questionnaires', questionnaireId] })
    },
  })
}

export function useUpdateQuestion(questionnaireId: string, questionId: string) {
  const queryClient = useQueryClient()
  return useMutation<Question, Error, CreateQuestionInput>({
    mutationFn: (input) =>
      apiFetch<Question>(`/vaktcomply/questionnaires/${questionnaireId}/questions/${questionId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'questionnaires', questionnaireId] })
    },
  })
}

export function useDeleteQuestion(questionnaireId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (questionId) =>
      apiFetch<void>(`/vaktcomply/questionnaires/${questionnaireId}/questions/${questionId}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'questionnaires', questionnaireId] })
    },
  })
}

export function useReorderQuestions(questionnaireId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, ReorderQuestionsInput>({
    mutationFn: (input) =>
      apiFetch<void>(`/vaktcomply/questionnaires/${questionnaireId}/questions/reorder`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'questionnaires', questionnaireId] })
    },
  })
}
