// Hai Tukang's service worker. It shows the push notifications the backend
// sends (golang/apps/hai-tukang/internal/push), and tells open pages, so
// they reload what changed. A message is {title, body, url, tag}.

// Take over pages already open, so the first push after turning
// notifications on reaches them too.
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()))

self.addEventListener('push', (event) => {
  let message = { title: 'Hai Tukang', body: '', url: '/' }
  try {
    message = { ...message, ...event.data.json() }
  } catch {
    // Not JSON: show the default title.
  }
  event.waitUntil(
    (async () => {
      await self.registration.showNotification(message.title, {
        body: message.body,
        tag: message.tag,
        data: { url: message.url },
      })
      const pages = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      for (const page of pages) page.postMessage({ type: 'hai-tukang:push', message })
    })(),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = (event.notification.data && event.notification.data.url) || '/'
  event.waitUntil(
    (async () => {
      const pages = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      const open = pages.find((page) => new URL(page.url).origin === self.location.origin)
      if (open) return open.focus()
      return self.clients.openWindow(url)
    })(),
  )
})
