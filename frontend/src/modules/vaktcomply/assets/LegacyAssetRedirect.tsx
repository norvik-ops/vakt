import { Navigate, useParams } from 'react-router-dom'

// Das Asset-Inventar lag bis module-archivieren S1 unter /vaktscan/assets. Alte Lesezeichen und
// Links landen im Comply-Inventar — unabhängig davon, ob Scan eingeschaltet ist. router.tsx hängt
// die Route außerhalb der Scan-ModuleShell ein, sonst würde ein abgeschaltetes Scan sie blocken.
export function LegacyAssetRedirect() {
  const { id } = useParams<{ id?: string }>()
  return <Navigate to={id ? `/vaktcomply/assets/${id}` : '/vaktcomply/assets'} replace />
}
