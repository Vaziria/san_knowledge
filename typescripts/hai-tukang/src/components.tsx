import { Alert, Button, Field, Input, InputGroup, NativeSelect } from '@chakra-ui/react'
import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt'
import { useTranslation } from 'react-i18next'
import { services } from './catalog'
import type { ErrorCode } from './i18n/id'
import { countryCode, localPhone } from './phone'
import { whatsappLink } from './whatsapp'

// Chakra's Alert has no ARIA role of its own: errors are announced as
// alerts, successes as status.
export function ErrorAlert({ code }: { code: ErrorCode | null }) {
  const { t } = useTranslation()
  if (!code) return null
  return (
    <Alert.Root status="error" role="alert">
      <Alert.Indicator />
      <Alert.Title>{t(`errors.${code}`)}</Alert.Title>
    </Alert.Root>
  )
}

export function SuccessAlert({ text }: { text: string | null }) {
  if (!text) return null
  return (
    <Alert.Root status="success" role="status">
      <Alert.Indicator />
      <Alert.Title>{text}</Alert.Title>
    </Alert.Root>
  )
}

// PhoneField is a fixed +62 and the rest of the number. The value is what
// was typed; it is cleaned (localPhone) when the field is left, and the form
// cleans it again on submit, since a submit can come before the field is left.
export function PhoneField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { t } = useTranslation()
  return (
    <Field.Root required>
      <Field.Label>{t('auth.phone')}</Field.Label>
      {/* No maxLength: the browser would cut a pasted "+62 812-…" before it is cleaned. */}
      <InputGroup startAddon={countryCode} startAddonProps={{ fontWeight: 'semibold' }}>
        <Input
          size="lg"
          type="tel"
          inputMode="tel"
          autoComplete="tel-national"
          value={value}
          onChange={(event) => onChange(event.currentTarget.value)}
          onBlur={() => onChange(localPhone(value))}
          placeholder={t('auth.phonePlaceholder')}
          minLength={8}
          required
        />
      </InputGroup>
      <Field.HelperText>{t('auth.phoneHelp')}</Field.HelperText>
    </Field.Root>
  )
}

// ServiceSelect picks a kind of work; with allowAll its empty choice means
// any, else it is a prompt and the field is required.
export function ServiceSelect({
  value,
  onChange,
  allowAll,
}: {
  value: string
  onChange: (value: string) => void
  allowAll?: boolean
}) {
  const { t } = useTranslation()
  return (
    <Field.Root required={!allowAll}>
      <Field.Label>{t('common.service')}</Field.Label>
      <NativeSelect.Root size="lg">
        {/* NativeSelect.Field drops `required` from its props and Field.Root's does
            not reach it, so it is set on the <select> itself. */}
        <NativeSelect.Field
          ref={(el) => {
            if (el) el.required = !allowAll
          }}
          value={value}
          onChange={(event) => onChange(event.currentTarget.value)}
        >
          <option value="">{allowAll ? t('common.allServices') : t('common.chooseService')}</option>
          {services.map((code) => (
            <option key={code} value={code}>
              {t(`services.${code}`)}
            </option>
          ))}
        </NativeSelect.Field>
        <NativeSelect.Indicator />
      </NativeSelect.Root>
    </Field.Root>
  )
}

export function WhatsAppButton({ phone, text, label }: { phone: string; text: string; label?: string }) {
  const { t } = useTranslation()
  return (
    <Button asChild size="lg" colorPalette="green" w="full">
      <a href={whatsappLink(phone, text)} target="_blank" rel="noopener noreferrer">
        {label ?? t('common.whatsapp')}
      </a>
    </Button>
  )
}

export function useFormatDate() {
  const { i18n } = useTranslation()
  const format = new Intl.DateTimeFormat(i18n.resolvedLanguage, { dateStyle: 'medium', timeStyle: 'short' })
  return (ts?: Timestamp) => (ts ? format.format(timestampDate(ts)) : '')
}
