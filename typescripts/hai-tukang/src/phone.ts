// Indonesian numbers only, for now: the form shows a fixed +62 and the
// reader types the rest. A +62 or leading 0 in it is dropped, so
// "0812-3456-7890" and "+62 812 3456 7890" both become "81234567890".
// (A bare "62" is kept: some local area codes start with 62.) Clean the
// whole value, not each keystroke: see App.tsx.
export const countryCode = '+62'

export function localPhone(raw: string): string {
  let value = raw.trim()
  if (value.startsWith(countryCode)) value = value.slice(countryCode.length)
  return value.replace(/\D/g, '').replace(/^0+/, '')
}

export const fullPhone = (local: string) => countryCode + local
