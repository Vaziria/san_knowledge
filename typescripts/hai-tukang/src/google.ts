// Google's sign-in button (Google Identity Services), for sign-in and
// sign-up with Google (acceptance HT-013). The button gives an ID token,
// which the server checks (internal/google).

type CredentialResponse = { credential: string }

type GoogleId = {
  initialize(options: { client_id: string; callback: (response: CredentialResponse) => void; ux_mode?: 'popup' }): void
  renderButton(
    parent: HTMLElement,
    options: { type?: 'standard'; theme?: 'outline'; size?: 'large'; text?: 'continue_with'; shape?: 'pill'; width?: number; locale?: string },
  ): void
}

declare global {
  interface Window {
    google?: { accounts: { id: GoogleId } }
  }
}

const scriptURL = 'https://accounts.google.com/gsi/client'
let loading: Promise<GoogleId> | undefined

// loadGoogle loads Google's script once.
export function loadGoogle(): Promise<GoogleId> {
  loading ??= new Promise<GoogleId>((resolve, reject) => {
    if (window.google?.accounts?.id) return resolve(window.google.accounts.id)
    const script = document.createElement('script')
    script.src = scriptURL
    script.async = true
    script.onload = () => (window.google?.accounts?.id ? resolve(window.google.accounts.id) : reject(new Error('no google.accounts.id')))
    script.onerror = () => {
      loading = undefined
      reject(new Error('Google sign-in did not load'))
    }
    document.head.appendChild(script)
  })
  return loading
}
