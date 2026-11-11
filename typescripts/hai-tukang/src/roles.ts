import type { TFunction } from 'i18next'
import { Role } from './gen/haitukang/v1/account_pb'

// roleName is an account's role, as its badge says it.
export const roleName = (t: TFunction, role: Role) =>
  role === Role.ADMIN
    ? t('roles.admin')
    : role === Role.REPAIRMAN
      ? t('roles.repairman')
      : role === Role.STORE
        ? t('roles.store')
        : t('roles.customer')
