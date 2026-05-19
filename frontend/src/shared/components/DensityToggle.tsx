import { AlignJustify, List } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { useTableDensity } from '../hooks/useTableDensity'

export function DensityToggle() {
  const [density, toggle] = useTableDensity()
  const isCompact = density === 'compact'

  return (
    <Button
      variant="ghost"
      size="icon"
      className="h-8 w-8 text-muted-foreground hover:text-primary"
      title={isCompact ? 'Komfortable Ansicht' : 'Kompakte Ansicht'}
      aria-label={isCompact ? 'Komfortable Ansicht' : 'Kompakte Ansicht'}
      onClick={toggle}
    >
      {isCompact
        ? <AlignJustify className="w-4 h-4" />
        : <List className="w-4 h-4" />
      }
    </Button>
  )
}
