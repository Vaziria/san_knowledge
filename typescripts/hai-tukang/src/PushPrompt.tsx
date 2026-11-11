import { Box, Button, Stack, Text } from '@chakra-ui/react'
import { ConnectError } from '@connectrpc/connect'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { errorCode } from './api'
import { ErrorAlert } from './components'
import type { ErrorCode } from './i18n/id'
import { pushState, turnOnPush, type PushState } from './push'

// PushPrompt offers to turn on notifications for this device, or says why
// it cannot. Once on, it is one quiet line.
export function PushPrompt() {
  const { t, i18n } = useTranslation()
  const [state, setState] = useState<PushState | null>(null)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    void pushState().then(setState)
  }, [])

  const turnOn = async () => {
    setPending(true)
    setError(null)
    setFailed(false)
    try {
      setState(await turnOnPush(i18n.resolvedLanguage ?? 'id'))
    } catch (err) {
      // A browser's own refusal is not a Connect error; ours carries a reason.
      if (err instanceof ConnectError) setError(errorCode(err))
      else setFailed(true)
    } finally {
      setPending(false)
    }
  }

  if (state === null) return null
  if (state === 'on') {
    return (
      <Text fontSize="sm" color="ink.600" data-testid="push-state">
        {t('push.on')}
      </Text>
    )
  }
  if (state !== 'off') {
    return (
      <Text fontSize="sm" color="ink.600" data-testid="push-state">
        {state === 'blocked' ? t('push.blocked') : t('push.unsupported')}
      </Text>
    )
  }
  return (
    <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="xl" p="4" data-testid="push-state">
      <Stack gap="3">
        <Text fontSize="sm">{t('push.prompt')}</Text>
        <ErrorAlert code={error} />
        {failed && (
          <Text fontSize="sm" color="red.700" role="alert">
            {t('push.failed')}
          </Text>
        )}
        <Button size="lg" variant="outline" colorPalette="ink" loading={pending} onClick={() => void turnOn()}>
          {t('push.turnOn')}
        </Button>
      </Stack>
    </Box>
  )
}
