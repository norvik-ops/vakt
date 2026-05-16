import { useQuery } from '@tanstack/react-query'
import { getAuthToken } from '../../../api/client'
import { AlertTriangle } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'

interface EvidenceItem {
  id: string
  title: string
  expires_at: string
  control_id: string
}

export function ExpiringEvidenceWidget() {
  const navigate = useNavigate()

  const { data: items = [], isLoading } = useQuery<EvidenceItem[]>({
    queryKey: ['evidence-expiring'],
    queryFn: async () => {
      const token = getAuthToken()
      const res = await fetch('/api/v1/secvitals/evidence/expiring?days=30', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) return []
      return res.json()
    },
    staleTime: 5 * 60 * 1000,
  })

  if (isLoading || items.length === 0) return null

  return (
    <Card className="border-amber-200 bg-amber-50 dark:bg-amber-950/20 dark:border-amber-800">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-amber-800 dark:text-amber-400 text-sm font-medium">
          <AlertTriangle className="h-4 w-4" />
          Ablaufende Nachweise
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-1 mb-3">
          {items.slice(0, 3).map((item) => (
            <div key={item.id} className="flex items-center justify-between text-sm">
              <span className="text-gray-700 dark:text-gray-300 truncate">{item.title}</span>
              <Badge variant="outline" className="ml-2 text-amber-700 border-amber-300 shrink-0">
                {new Date(item.expires_at).toLocaleDateString('de-DE')}
              </Badge>
            </div>
          ))}
          {items.length > 3 && (
            <p className="text-xs text-gray-500">+{items.length - 3} weitere</p>
          )}
        </div>
        <Button
          size="sm"
          variant="outline"
          className="w-full text-amber-700 border-amber-300 hover:bg-amber-100"
          onClick={() => navigate('/secvitals')}
        >
          Alle anzeigen
        </Button>
      </CardContent>
    </Card>
  )
}
