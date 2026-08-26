import { apiFetch } from './client'
import type { PaginatedResponse } from '../shared/types/pagination'

/**
 * Collect every page of a paginated endpoint.
 *
 * Why this exists (Codeaudit v5b)
 * ------------------------------
 * Two separate defect classes met in the same place.
 *
 * The first is silent truncation. Several call sites asked for one big page —
 * `?limit=200` — believing that got them everything. The offset paginator caps
 * at `pagination.MaxLimit = 100`, and it does not clamp an over-large limit, it
 * DISCARDS it and falls back to the default of 25 (shared/pagination/
 * pagination.go:39-41). So `limit=200` returns twenty-five rows and a
 * `pagination.total` that says how many there really were, which nobody read.
 * The list looked complete and was not.
 *
 * The second is the envelope itself: a page that types the response as a bare
 * array reads `undefined` for `.length` and renders an empty state forever.
 *
 * Both are quiet failures. Nothing throws, nothing logs, the page just shows
 * less than exists — which is worse than a crash, because a short list is
 * indistinguishable from a short dataset.
 *
 * `complete` is therefore part of the result, not an internal detail: a caller
 * that hits `maxItems` has to decide what to tell the user, and cannot do that
 * if the truncation is hidden from it. Callers that must not truncate (an
 * export offered as evidence) should check it and say so.
 */

export interface PagedResult<T> {
  items: T[]
  /** Rows the server says match the query, across all pages. */
  total: number
  /** False when `maxItems` cut the walk short — never truncate silently. */
  complete: boolean
}

export interface FetchAllOptions {
  /**
   * Rows per request. Must not exceed the endpoint's server-side cap or the
   * server quietly falls back to its default page size.
   */
  pageSize?: number
  /** Hard stop, so a runaway total cannot hang the tab. */
  maxItems?: number
}

/**
 * Generic page walker. `fetchPage` receives a 1-based page number and the page
 * size, and returns that page's rows plus the overall total.
 *
 * Stops on the first short or empty page as well as on the total, so a server
 * that under-reports `total` cannot spin the loop.
 */
export async function fetchAllPages<T>(
  fetchPage: (page: number, limit: number) => Promise<{ items: T[]; total: number }>,
  { pageSize = 100, maxItems = 10_000 }: FetchAllOptions = {},
): Promise<PagedResult<T>> {
  const items: T[] = []
  let total = 0
  let page = 1

  for (;;) {
    const res = await fetchPage(page, pageSize)
    total = res.total
    items.push(...res.items)

    if (items.length >= maxItems) {
      return { items: items.slice(0, maxItems), total, complete: items.length <= maxItems }
    }
    // A short page means the server has nothing more, whatever `total` claims.
    if (res.items.length < pageSize) break
    if (items.length >= total) break
    page++
  }

  return { items, total, complete: true }
}

/**
 * `fetchAllPages` for the standard `{data, pagination}` envelope that
 * `pagination.Wrap` produces. `path` must carry no page/limit of its own —
 * they are appended here.
 */
export async function fetchAllPaginated<T>(
  path: string,
  options: FetchAllOptions = {},
): Promise<PagedResult<T>> {
  const separator = path.includes('?') ? '&' : '?'
  return fetchAllPages<T>(async (page, limit) => {
    const res = await apiFetch<PaginatedResponse<T>>(
      `${path}${separator}page=${String(page)}&limit=${String(limit)}`,
    )
    return { items: res.data, total: res.pagination.total }
  }, options)
}
