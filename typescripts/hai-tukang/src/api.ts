import { Code, ConnectError, createClient } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { AccountService } from './gen/haitukang/v1/account_pb'
import { ErrorDetailSchema } from './gen/haitukang/v1/error_pb'
import { NotificationService } from './gen/haitukang/v1/notification_pb'
import { PenaltyService } from './gen/haitukang/v1/penalty_pb'
import { AdminService } from './gen/haitukang/v1/admin_pb'
import { RepairRequestService } from './gen/haitukang/v1/repair_request_pb'
import { VisitService } from './gen/haitukang/v1/visit_pb'
import { isErrorCode } from './i18n'
import type { ErrorCode } from './i18n/id'

// The services are on the page's origin: in `npm run dev`, Vite proxies
// /haitukang.* to the backend. The sign-in is an HttpOnly cookie the browser
// sends with every call.
const transport = createConnectTransport({ baseUrl: window.location.origin })

export const accounts = createClient(AccountService, transport)
export const repairRequests = createClient(RepairRequestService, transport)
export const visitClient = createClient(VisitService, transport)
export const notifications = createClient(NotificationService, transport)
export const penaltyClient = createClient(PenaltyService, transport)
export const adminClient = createClient(AdminService, transport)

// changedEvent fires after the reader changed a visit; the standing reloads.
export const changedEvent = 'hai-tukang:changed'

// pushEvent fires when a push notification arrives while the app is open
// (public/sw.js tells the page); lists reload on it.
export const pushEvent = 'hai-tukang:push'

// signedOutEvent fires when a call finds the sign-in gone (expired, or
// signed out elsewhere); App then shows the sign-in screen.
export const signedOutEvent = 'hai-tukang:signed-out'

// termsEvent fires when a call is refused because the terms changed since
// the account agreed; App then reloads it and asks (TermsGate).
export const termsEvent = 'hai-tukang:terms'

// errorCode is the i18n key for an error from the services: their stable
// reason (ErrorDetail), else network when they could not be reached.
export function errorCode(err: unknown): ErrorCode {
  const connectErr = ConnectError.from(err)
  const reason = connectErr.findDetails(ErrorDetailSchema)[0]?.reason
  if (isErrorCode(reason)) {
    if (reason === 'unauthorized') window.dispatchEvent(new Event(signedOutEvent))
    if (reason === 'terms_not_accepted') window.dispatchEvent(new Event(termsEvent))
    return reason
  }
  switch (connectErr.code) {
    case Code.Unavailable:
    case Code.Unknown:
    case Code.DeadlineExceeded:
      return 'network'
    case Code.InvalidArgument:
      return 'invalid_request'
    default:
      return 'internal'
  }
}
