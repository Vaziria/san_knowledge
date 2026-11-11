import { useTranslation } from 'react-i18next'

// useFormatRupiah writes whole rupiah as in Indonesia, "Rp 15.000", in
// either language (en-ID groups digits with dots too).
export function useFormatRupiah() {
  const { i18n } = useTranslation()
  const format = new Intl.NumberFormat(`${i18n.resolvedLanguage ?? 'id'}-ID`, {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0,
  })
  return (amount: bigint | number) => format.format(Number(amount))
}

// rupiahInput keeps the digits of what was typed ("15.000" is 15000).
export const rupiahInput = (typed: string) => typed.replace(/\D/g, '').replace(/^0+(?=\d)/, '').slice(0, 13)
