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
    queryKey: ['secvitals', 'questionnaires', isTemplate ?? 'all'],
    queryFn: () => apiFetch<Questionnaire[]>(`/secvitals/questionnaires${qs}`),
    staleTime: 5 * 60 * 1000,
  })
}

export function useQuestionnaire(id: string) {
  return useQuery<Questionnaire>({
    queryKey: ['secvitals', 'questionnaires', id],
    queryFn: () => apiFetch<Questionnaire>(`/secvitals/questionnaires/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useTemplates() {
  return useQuery<Questionnaire[]>({
    queryKey: ['secvitals', 'questionnaires', 'templates'],
    queryFn: () => apiFetch<Questionnaire[]>('/secvitals/questionnaires/templates'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateQuestionnaire() {
  const queryClient = useQueryClient()
  return useMutation<Questionnaire, Error, CreateQuestionnaireInput>({
    mutationFn: (input) =>
      apiFetch<Questionnaire>('/secvitals/questionnaires', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'questionnaires'] })
    },
  })
}

export function useUpdateQuestionnaire(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Questionnaire, Error, CreateQuestionnaireInput>({
    mutationFn: (input) =>
      apiFetch<Questionnaire>(`/secvitals/questionnaires/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'questionnaires'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'questionnaires', id] })
    },
  })
}

export function useDeleteQuestionnaire() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/secvitals/questionnaires/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'questionnaires'] })
    },
  })
}

export function useAddQuestion(questionnaireId: string) {
  const queryClient = useQueryClient()
  return useMutation<Question, Error, CreateQuestionInput>({
    mutationFn: (input) =>
      apiFetch<Question>(`/secvitals/questionnaires/${questionnaireId}/questions`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'questionnaires', questionnaireId] })
    },
  })
}

export function useUpdateQuestion(questionnaireId: string, questionId: string) {
  const queryClient = useQueryClient()
  return useMutation<Question, Error, CreateQuestionInput>({
    mutationFn: (input) =>
      apiFetch<Question>(`/secvitals/questionnaires/${questionnaireId}/questions/${questionId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'questionnaires', questionnaireId] })
    },
  })
}

export function useDeleteQuestion(questionnaireId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (questionId) =>
      apiFetch<void>(`/secvitals/questionnaires/${questionnaireId}/questions/${questionId}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'questionnaires', questionnaireId] })
    },
  })
}

export function useReorderQuestions(questionnaireId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, ReorderQuestionsInput>({
    mutationFn: (input) =>
      apiFetch<void>(`/secvitals/questionnaires/${questionnaireId}/questions/reorder`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'questionnaires', questionnaireId] })
    },
  })
}
