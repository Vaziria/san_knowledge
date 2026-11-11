import { useEffect, useState } from 'react'

// The app is one page; the Terms and Policies have their own address, so a
// link to them can be shared and opened by anyone.
export const termsHash = '#/syarat'

// useHash is the page's location hash, kept current.
export function useHash() {
  const [hash, setHash] = useState(window.location.hash)
  useEffect(() => {
    const changed = () => setHash(window.location.hash)
    window.addEventListener('hashchange', changed)
    return () => window.removeEventListener('hashchange', changed)
  }, [])
  return hash
}
