import { Box, Button, Heading, Stack, Text } from '@chakra-ui/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { accounts, errorCode } from '../api'
import { ErrorAlert } from '../components'
import type { ErrorCode } from '../i18n/id'
import { TermsAgree } from './TermsAgree'
import { TERMS_VERSION } from './version'

// TermsGate stands before the app for an account that has not agreed to
// the current terms, because they changed (acceptance HT-014): nothing
// else works until it agrees, or signs out.
export function TermsGate({ onAgreed }: { onAgreed: () => void }) {
  const { t } = useTranslation()
  const [agreed, setAgreed] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)
  const accept = async () => {
    if (!agreed) {
      setError('terms_not_accepted')
      return
    }
    setPending(true)
    setError(null)
    try {
      await accounts.acceptTerms({ termsVersion: TERMS_VERSION })
      onAgreed()
    } catch (err) {
      setError(errorCode(err))
      setPending(false)
    }
  }
  return (
    <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="2xl" p={{ base: '5', sm: '8' }} data-testid="terms-gate">
      <Stack gap="4">
        <Heading as="h1" size="xl">
          {t('terms.gateTitle')}
        </Heading>
        <Text lineHeight="1.65">{t('terms.gateText')}</Text>
        <TermsAgree checked={agreed} onChange={setAgreed} />
        <ErrorAlert code={error} />
        <Button size="lg" colorPalette="ink" loading={pending} onClick={() => void accept()}>
          {t('terms.gateAccept')}
        </Button>
      </Stack>
    </Box>
  )
}
