import { useTranslation } from 'react-i18next'

const LANG_STORAGE_KEY = 'vakt_lang'

export function LanguageSwitcher() {
  const { i18n } = useTranslation()
  const currentLang = i18n.language === 'en' ? 'en' : 'de'

  function switchTo(lang: 'de' | 'en') {
    i18n.changeLanguage(lang)
    localStorage.setItem(LANG_STORAGE_KEY, lang)
  }

  return (
    <div className="flex items-center gap-1 px-3 py-[9px]">
      <button
        onClick={() => switchTo('de')}
        className={
          currentLang === 'de'
            ? 'text-[12px] font-semibold text-brand'
            : 'text-[12px] text-secondary hover:text-primary transition-colors'
        }
        aria-label="Deutsch"
      >
        DE
      </button>
      <span className="text-[12px] text-secondary/40 select-none">|</span>
      <button
        onClick={() => switchTo('en')}
        className={
          currentLang === 'en'
            ? 'text-[12px] font-semibold text-brand'
            : 'text-[12px] text-secondary hover:text-primary transition-colors'
        }
        aria-label="English"
      >
        EN
      </button>
    </div>
  )
}
