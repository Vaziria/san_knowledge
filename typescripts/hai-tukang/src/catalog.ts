import type { TFunction } from 'i18next'

// The kinds of work, by code, as the backend's catalog has them
// (golang/apps/hai-tukang/internal/catalog). Names are in src/i18n.
export const services = ['listrik', 'pipa', 'atap', 'cat', 'ac', 'bangunan', 'kayu', 'lainnya'] as const
export type ServiceCode = (typeof services)[number]

export const isService = (value: string): value is ServiceCode => (services as readonly string[]).includes(value)

export const serviceName = (t: TFunction, code: string) => (isService(code) ? t(`services.${code}`) : code)
