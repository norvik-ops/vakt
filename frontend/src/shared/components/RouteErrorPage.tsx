import { isRouteErrorResponse, useRouteError } from 'react-router-dom'
import { isChunkLoadError } from '../chunkReload'
import { ChunkLoadFallback } from './ChunkLoadFallback'
import { ErrorFallback } from './ErrorBoundary'

// errorElement der Wurzel-Route. Ersetzt den englischen react-router-Standard
// „Unexpected Application Error!“: Chunk-Ladefehler nach einem Deploy laden
// einmal neu, alle anderen Fehler zeigen die gewohnte Fehlerseite mit Meldung.
export function RouteErrorPage() {
  const error = useRouteError()
  if (isChunkLoadError(error)) return <ChunkLoadFallback />
  if (isRouteErrorResponse(error)) {
    return <ErrorFallback error={new Error(`${error.status} ${error.statusText}`)} />
  }
  console.error('[RouteErrorPage]', error)
  return <ErrorFallback error={error instanceof Error ? error : new Error(String(error))} />
}
