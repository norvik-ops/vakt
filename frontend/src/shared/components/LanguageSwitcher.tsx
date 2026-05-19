import { useTranslation } from 'react-i18next'

const LANG_STORAGE_KEY = 'vakt_lang'

type SupportedLang = 'de' | 'en' | 'fr' | 'nl'

const LANGUAGES: { code: SupportedLang; label: string; flag: string }[] = [
  { code: 'de', label: 'Deutsch', flag: '🇩🇪' },
  { code: 'en', label: 'English', flag: '🇬🇧' },
  { code: 'fr', label: 'Français', flag: '🇫🇷' },
  { code: 'nl', label: 'Nederlands', flag: '🇳🇱' },
]

export function LanguageSwitcher() {
  const { i18n } = useTranslation()
  const currentLang = (LANGUAGES.some((l) => l.code === i18n.language)
    ? i18n.language
    : 'de') as SupportedLang

  function switchTo(lang: SupportedLang) {
    i18n.changeLanguage(lang)
    localStorage.setItem(LANG_STORAGE_KEY, lang)
  }

  return (
    <div className="flex items-center gap-1 px-3 py-[9px]">
      {LANGUAGES.map((lang, idx) => (
        <>
          {idx > 0 && (
            <span key={`sep-${lang.code}`} className="text-[12px] text-secondary/40 select-none">
              |
            </span>
          )}
          <button
            key={lang.code}
            onClick={() => switchTo(lang.code)}
            className={
              currentLang === lang.code
                ? 'text-[12px] font-semibold text-brand'
                : 'text-[12px] text-secondary hover:text-primary transition-colors'
            }
            aria-label={lang.label}
            title={lang.label}
          >
            {lang.code.toUpperCase()}
          </button>
        </>
      ))}
    </div>
  )
}
