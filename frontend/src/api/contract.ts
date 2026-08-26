import type { paths } from './generated'

/**
 * Helper types that name a request or response body by its OpenAPI path.
 *
 * Why these exist (Codeaudit v5b, SHAPE_MISMATCH class)
 * -----------------------------------------------------
 * Several hooks declared a response type by hand and got it wrong in a way
 * nothing caught: `useQuery<VVTEntry[]>` on an endpoint that returns
 * `{data, pagination}`, `data.count` read off a bare array. Neither is a
 * crash — the page renders a permanent empty state, which reads like an
 * empty system rather than a defect.
 *
 * A hand-written type cannot be wrong about the server, because nothing
 * compares the two. `generated.ts` can: it is produced by openapi-typescript
 * from `backend/internal/shared/apidocs/openapi.yaml`, and
 * `npm run api-types:check` fails if it drifts from that file. Naming a body
 * through these helpers therefore makes a mismatch a compile error instead of
 * an empty table.
 *
 * KNOWN LIMIT, do not overstate this: openapi.yaml is written by hand. It is
 * checked against the Go handlers by scripts/check_openapi_coverage.py for
 * *presence* of a route, not for the shape of its body. So these types pin the
 * frontend to the published contract, not to the Go struct behind it. When the
 * two disagree the spec is the bug, and it lives in backend/ — this file
 * cannot see it. (Verified for the endpoints fixed under this class: the spec
 * matched the handler in every case, and it was the frontend that deviated.)
 */

type JsonContent = { content: { 'application/json': unknown } }

/** The 200/201 JSON response body an endpoint is documented to return. */
export type ApiResponse<
  P extends keyof paths,
  M extends keyof paths[P],
> = paths[P][M] extends { responses: infer R }
  ? R extends { 200: JsonContent }
    ? R[200]['content']['application/json']
    : R extends { 201: JsonContent }
      ? R[201]['content']['application/json']
      : never
  : never

/** The JSON request body an endpoint is documented to accept. */
export type ApiRequest<
  P extends keyof paths,
  M extends keyof paths[P],
> = paths[P][M] extends { requestBody: JsonContent }
  ? paths[P][M]['requestBody']['content']['application/json']
  : paths[P][M] extends { requestBody?: JsonContent }
    ? NonNullable<paths[P][M]['requestBody']>['content']['application/json']
    : never
