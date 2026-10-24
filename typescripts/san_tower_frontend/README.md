# San Tower frontend

The UI of the tower,
[golang/apps/san_tower](../../golang/apps/san_tower/README.md). React +
TypeScript + Chakra UI v3, with react-router for the pages. Vite builds it
into `golang/apps/san_tower/internal/web/dist`, which the binary embeds, so
rebuild the binary (`build.ps1` there) to see a change in `san_tower serve`.

The easiest way to work on it is `go run .\cmd\san_tower dev` in
`golang/apps/san_tower`, which runs the tower and this dev server together.

```powershell
npm install
npm run dev        # hot reload; TowerService is proxied to $env:SAN_TOWER_SERVER, default a `san_tower serve` on :7477
npm run build      # typecheck + build into the Go app
npm run gen        # regenerate src/gen from the protos (runs buf in golang/apps/san_tower)
```

`logo-candidates/` holds five logo ideas for the tower (open its `index.html`
to compare them at 16 to 128 px). None is used in the app yet.

## Files

- `src/routes.tsx`: the pages and the router. A new page is one entry in
  `pages`, which also puts it in the menu.
- `src/Layout.tsx`: the menu in a sidebar on the left, and the current page
  beside it.
- `src/pages/`:
  - `Home.tsx`: the tower's status.
  - `MessageBroker.tsx`: the San Message broker the tower runs. The Topics
    tab lists topics (last event, where the log ends, subscriber count),
    creates them, and deletes those without subscribers. The Subscribers tab
    lists subscribers (type, webhook URI or whether a pull subscriber is
    connected, where it is in the log, failed attempts and the next retry),
    filtered by topic, and creates and deletes them. The Settings tab sets
    the broker's data directory (BrokerService); saving a new one asks
    first, then switches the broker to that folder. The tab and the topic
    are in the URL (`?tab=subscribers&topic=orders`). It reloads after each
    change and on Refresh.
  - `NotFound.tsx`.
- `src/ui.tsx`: the toaster, and `FormDialog`, a form in a modal that shows
  the server's error and closes when it succeeds.
- `src/api.ts`: the TowerService, BrokerService and MessageService clients,
  same origin as the page, and `errorText`.
- `src/gen/`: generated from the protos; do not edit.
