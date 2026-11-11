import 'i18next'
import type { Messages } from './i18n/id'

// Typed translation keys: t('register.titel') fails the typecheck.
declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: Messages }
  }
}
