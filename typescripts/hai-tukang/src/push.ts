import { notifications, pushEvent } from './api'

// Browser push notifications for this device (Web Push; the backend's
// internal/push). The service worker is public/sw.js.

export type PushState = 'unsupported' | 'blocked' | 'off' | 'on'

// pushSupported is false on an iPhone until Hai Tukang is opened from the
// Home Screen, and in browsers without Web Push.
export const pushSupported = () => 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window

const timeZone = () => Intl.DateTimeFormat().resolvedOptions().timeZone

async function currentSubscription() {
  const registration = await navigator.serviceWorker.getRegistration()
  return (await registration?.pushManager.getSubscription()) ?? null
}

async function save(subscription: PushSubscription, language: string) {
  const json = subscription.toJSON()
  await notifications.savePushSubscription({
    endpoint: subscription.endpoint,
    p256dh: json.keys?.p256dh ?? '',
    auth: json.keys?.auth ?? '',
    language,
    timeZone: timeZone(),
  })
}

export async function pushState(): Promise<PushState> {
  if (!pushSupported()) return 'unsupported'
  if (Notification.permission === 'denied') return 'blocked'
  return Notification.permission === 'granted' && (await currentSubscription()) ? 'on' : 'off'
}

// turnOnPush asks for permission, subscribes this device and saves it to
// the signed-in account.
export async function turnOnPush(language: string): Promise<PushState> {
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') return permission === 'denied' ? 'blocked' : 'off'
  const registration = await navigator.serviceWorker.register('/sw.js')
  await navigator.serviceWorker.ready
  const { publicKey } = await notifications.getPushKey({})
  const subscription =
    (await registration.pushManager.getSubscription()) ??
    (await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: fromBase64Url(publicKey) }))
  await save(subscription, language)
  return 'on'
}

// keepPush saves this device's subscription again, if it has one: after a
// sign-in (it moves to that account) and when the language changes.
export async function keepPush(language: string) {
  if (!pushSupported() || Notification.permission !== 'granted') return
  const subscription = await currentSubscription()
  if (subscription) await save(subscription, language).catch(() => undefined)
}

// forgetPush, before signing out, takes this device off the account and
// unsubscribes it, so the next person to sign in here gets none of it.
export async function forgetPush() {
  if (!pushSupported()) return
  const subscription = await currentSubscription()
  if (!subscription) return
  await notifications.deletePushSubscription({ endpoint: subscription.endpoint }).catch(() => undefined)
  await subscription.unsubscribe().catch(() => undefined)
}

// listenForPush passes the service worker's notice of a push on to the page.
export function listenForPush() {
  if (!('serviceWorker' in navigator)) return
  navigator.serviceWorker.addEventListener('message', (event) => {
    if (event.data?.type === 'hai-tukang:push') window.dispatchEvent(new Event(pushEvent))
  })
}

function fromBase64Url(value: string) {
  const base64 = (value + '='.repeat((4 - (value.length % 4)) % 4)).replace(/-/g, '+').replace(/_/g, '/')
  return Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
}
