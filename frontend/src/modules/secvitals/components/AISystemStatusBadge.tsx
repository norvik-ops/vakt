import { Badge } from '../../../components/ui/badge'

export const STATUS_LABELS: Record<string, string> = {
  under_review: 'In Prüfung',
  approved: 'Genehmigt',
  prohibited: 'Verboten',
  decommissioned: 'Stillgelegt',
  classified: 'Klassifiziert',
  compliant: 'Konform',
}

const STATUS_CLASS: Record<string, string> = {
  under_review: 'bg-gray-100 text-gray-800 border-gray-300',
  approved: 'bg-blue-100 text-blue-800 border-blue-300',
  classified: 'bg-blue-100 text-blue-800 border-blue-300',
  compliant: 'bg-green-100 text-green-800 border-green-300',
  decommissioned: 'bg-red-100 text-red-800 border-red-300',
  prohibited: 'bg-red-100 text-red-800 border-red-300',
}

interface Props {
  status: string
}

export function AISystemStatusBadge({ status }: Props) {
  const className = STATUS_CLASS[status] ?? 'bg-gray-100 text-gray-800 border-gray-300'
  return (
    <Badge className={className} data-testid="ai-status-badge" data-status={status}>
      {STATUS_LABELS[status] ?? status}
    </Badge>
  )
}
