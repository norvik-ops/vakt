export interface PaginationMeta {
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface PaginatedResponse<T> {
  data: T[]
  pagination: PaginationMeta
}

/** Mirrors backend pagination.CursorMeta (cursor branch of dual-branch list endpoints). */
export interface CursorMeta {
  limit: number
  next_cursor?: string // absent on the last page
  has_more: boolean
}

export interface CursorPaginatedResponse<T> {
  data: T[]
  pagination: CursorMeta
}
