import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import de from './locales/de.json'
import en from './locales/en.json'
import fr from './locales/fr.json'
import nl from './locales/nl.json'

const LANG_STORAGE_KEY = 'vakt_lang'

const savedLang = localStorage.getItem(LANG_STORAGE_KEY)
const defaultLanguage =
  savedLang === 'en' || savedLang === 'de' || savedLang === 'fr' || savedLang === 'nl'
    ? savedLang
    : 'de'

i18n
  .use(initReactI18next)
  .init({
    resources: {
      de: { translation: de },
      en: { translation: en },
      fr: { translation: fr },
      nl: { translation: nl },
    },
    lng: defaultLanguage,
    fallbackLng: 'de',
    supportedLngs: ['de', 'en', 'fr', 'nl'],
    interpolation: {
      escapeValue: false, // React already escapes values
    },
  })

i18n.on('languageChanged', (lng) => {
  document.documentElement.lang = lng
})

// Set initial lang attribute
document.documentElement.lang = defaultLanguage

export default i18n
