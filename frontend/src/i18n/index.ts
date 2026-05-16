import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import de from './locales/de.json'
import en from './locales/en.json'

const LANG_STORAGE_KEY = 'vakt_lang'

const savedLang = localStorage.getItem(LANG_STORAGE_KEY)
const defaultLanguage = savedLang === 'en' || savedLang === 'de' ? savedLang : 'de'

i18n
  .use(initReactI18next)
  .init({
    resources: {
      de: { translation: de },
      en: { translation: en },
    },
    lng: defaultLanguage,
    fallbackLng: 'de',
    supportedLngs: ['de', 'en'],
    interpolation: {
      escapeValue: false, // React already escapes values
    },
  })

export default i18n
