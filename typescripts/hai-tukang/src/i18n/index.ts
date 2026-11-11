import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { en } from './en'
import { id, type ErrorCode } from './id'

// The languages the app offers, by i18n code, each named in itself.
// Bahasa Indonesia is the default; the reader's choice is remembered.
export const languages = { id: 'ID', en: 'EN' } as const
export type Language = keyof typeof languages
export const defaultLanguage: Language = 'id'

const storageKey = 'hai-tukang-language'

const isLanguage = (value: string | null): value is Language => value !== null && value in languages

export const isErrorCode = (value: unknown): value is ErrorCode => typeof value === 'string' && value in id.errors

const saved = localStorage.getItem(storageKey)
const initial: Language = isLanguage(saved) ? saved : defaultLanguage

void i18n.use(initReactI18next).init({
  resources: { id: { translation: id }, en: { translation: en } },
  lng: initial,
  fallbackLng: defaultLanguage,
  supportedLngs: Object.keys(languages),
  interpolation: { escapeValue: false }, // React escapes text itself
  initAsync: false,
})

document.documentElement.lang = initial
i18n.on('languageChanged', (language) => {
  localStorage.setItem(storageKey, language)
  document.documentElement.lang = language
})

export default i18n
