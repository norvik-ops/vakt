import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  Employee,
  CreateEmployeeInput,
  UpdateEmployeeInput,
  Checklist,
  CreateChecklistInput,
  ChecklistRun,
  StartChecklistRunInput,
  UpdateChecklistRunInput,
} from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

// --- Employees ---

export function useEmployees(page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Employee>>({
    queryKey: ['vakthr', 'employees', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Employee>>(`/vakthr/employees?page=${String(page)}&limit=${String(limit)}`),
    staleTime: 5 * 60 * 1000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function useCreateEmployee() {
  const queryClient = useQueryClient()
  return useMutation<Employee, Error, CreateEmployeeInput>({
    mutationFn: (input) =>
      apiFetch<Employee>('/vakthr/employees', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vakthr', 'employees'] })
    },
  })
}

export function useUpdateEmployee() {
  const queryClient = useQueryClient()
  return useMutation<Employee, Error, { id: string; input: UpdateEmployeeInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<Employee>(`/vakthr/employees/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vakthr', 'employees'] })
    },
  })
}

export function useDeleteEmployee() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/vakthr/employees/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vakthr', 'employees'] })
    },
  })
}

// --- Checklists ---

export function useChecklists() {
  return useQuery<Checklist[]>({
    queryKey: ['vakthr', 'checklists'],
    queryFn: () => apiFetch<Checklist[]>('/vakthr/checklists'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateChecklist() {
  const queryClient = useQueryClient()
  return useMutation<Checklist, Error, CreateChecklistInput>({
    mutationFn: (input) =>
      apiFetch<Checklist>('/vakthr/checklists', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vakthr', 'checklists'] })
    },
  })
}

export function useDeleteChecklist() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/vakthr/checklists/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vakthr', 'checklists'] })
    },
  })
}

// --- Checklist Runs ---

export function useChecklistRuns(employeeId?: string) {
  return useQuery<ChecklistRun[]>({
    queryKey: ['vakthr', 'checklist-runs', employeeId],
    queryFn: () => apiFetch<ChecklistRun[]>(`/vakthr/employees/${employeeId ?? ''}/checklist-runs`),
    enabled: !!employeeId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useStartChecklistRun() {
  const queryClient = useQueryClient()
  return useMutation<ChecklistRun, Error, StartChecklistRunInput>({
    mutationFn: (input) =>
      apiFetch<ChecklistRun>('/vakthr/checklist-runs', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ['vakthr', 'checklist-runs', variables.employee_id] })
    },
  })
}

export function useUpdateChecklistRun() {
  const queryClient = useQueryClient()
  return useMutation<ChecklistRun, Error, { id: string; input: UpdateChecklistRunInput; employeeId?: string }>({
    mutationFn: ({ id, input }) =>
      apiFetch<ChecklistRun>(`/vakthr/checklist-runs/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: (_data, variables) => {
      if (variables.employeeId) {
        void queryClient.invalidateQueries({ queryKey: ['vakthr', 'checklist-runs', variables.employeeId] })
      }
    },
  })
}
