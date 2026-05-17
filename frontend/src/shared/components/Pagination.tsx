import { useTranslation } from 'react-i18next'
import { Button } from '../../components/ui/button'

interface Props {
  page: number
  totalPages: number
  onPageChange: (p: number) => void
}

export function Pagination({ page, totalPages, onPageChange }: Props) {
  const { t } = useTranslation()

  if (totalPages <= 1) return null
  return (
    <div className="flex items-center justify-between mt-4 text-sm">
      <span className="text-muted-foreground">
        {t('pagination.pageOf', { page, total: totalPages })}
      </span>
      <div className="flex gap-1">
        <Button
          variant="outline"
          size="sm"
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
          aria-label={t('pagination.previous')}
        >
          &larr;
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => onPageChange(page + 1)}
          disabled={page >= totalPages}
          aria-label={t('pagination.next')}
        >
          &rarr;
        </Button>
      </div>
    </div>
  )
}
