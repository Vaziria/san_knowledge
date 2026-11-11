import { Checkbox, Link } from '@chakra-ui/react'
import { Trans } from 'react-i18next'
import { termsHash } from '../route'

// TermsAgree is the "I agree to the Terms and Policies" box every sign-up
// has, unticked (acceptance HT-014). The link opens the page; App keeps the
// form, so going back finds it as it was.
export function TermsAgree({ checked, onChange }: { checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <Checkbox.Root
      checked={checked}
      onCheckedChange={(e) => onChange(e.checked === true)}
      colorPalette="ink"
      alignItems="flex-start"
      data-testid="terms-agree"
    >
      <Checkbox.HiddenInput />
      <Checkbox.Control mt="0.5" />
      <Checkbox.Label fontWeight="normal" lineHeight="1.6">
        <Trans
          i18nKey="terms.agree"
          // Not <link>: HTML's <link> is a void element, so Trans would
          // leave the text outside it.
          components={{
            page: <Link href={termsHash} color="ochre.700" textDecoration="underline" fontWeight="semibold" />,
          }}
        />
      </Checkbox.Label>
    </Checkbox.Root>
  )
}
