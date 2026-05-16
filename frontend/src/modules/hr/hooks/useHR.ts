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
    queryKey: ['hr', 'employees', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Employee>>(`/hr/employees?page=${page}&limit=${limit}`),
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
      apiFetch<Employee>('/hr/employees', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['hr', 'employees'] })
    },
  })
}

export function useUpdateEmployee() {
  const queryClient = useQueryClient()
  return useMutation<Employee, Error, { id: string; input: UpdateEmployeeInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<Employee>(`/hr/employees/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['hr', 'employees'] })
    },
  })
}

export function useDeleteEmployee() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/hr/employees/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['hr', 'employees'] })
    },
  })
}

// --- Checklists ---

export function useChecklists() {
  return useQuery<Checklist[]>({
    queryKey: ['hr', 'checklists'],
    queryFn: () => apiFetch<Checklist[]>('/hr/checklists'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateChecklist() {
  const queryClient = useQueryClient()
  return useMutation<Checklist, Error, CreateChecklistInput>({
    mutationFn: (input) =>
      apiFetch<Checklist>('/hr/checklists', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['hr', 'checklists'] })
    },
  })
}

export function useDeleteChecklist() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/hr/checklists/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['hr', 'checklists'] })
    },
  })
}

// --- Checklist Runs ---

export function useChecklistRuns(employeeId?: string) {
  return useQuery<ChecklistRun[]>({
    queryKey: ['hr', 'checklist-runs', employeeId],
    queryFn: () => apiFetch<ChecklistRun[]>(`/hr/employees/${employeeId}/checklist-runs`),
    enabled: !!employeeId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useStartChecklistRun() {
  const queryClient = useQueryClient()
  return useMutation<ChecklistRun, Error, StartChecklistRunInput>({
    mutationFn: (input) =>
      apiFetch<ChecklistRun>('/hr/checklist-runs', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ['hr', 'checklist-runs', variables.employee_id] })
    },
  })
}

export function useUpdateChecklistRun() {
  const queryClient = useQueryClient()
  return useMutation<ChecklistRun, Error, { id: string; input: UpdateChecklistRunInput; employeeId?: string }>({
    mutationFn: ({ id, input }) =>
      apiFetch<ChecklistRun>(`/hr/checklist-runs/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: (_data, variables) => {
      if (variables.employeeId) {
        void queryClient.invalidateQueries({ queryKey: ['hr', 'checklist-runs', variables.employeeId] })
      }
    },
  })
}
