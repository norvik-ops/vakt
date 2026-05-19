import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'

export default function NotFoundPage() {
  const { t } = useTranslation()

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center px-4">
      <p className="text-7xl font-bold text-brand mb-4">404</p>
      <h1 className="text-xl font-semibold text-primary mb-2">
        {t('errors.notFound.title', 'Seite nicht gefunden')}
      </h1>
      <p className="text-sm text-secondary mb-6 max-w-sm">
        {t('errors.notFound.description', 'Die aufgerufene Seite existiert nicht oder wurde verschoben.')}
      </p>
      <Link
        to="/"
        className="text-sm text-brand hover:underline"
      >
        {t('errors.notFound.back', 'Zurück zur Startseite')}
      </Link>
    </div>
  )
}
