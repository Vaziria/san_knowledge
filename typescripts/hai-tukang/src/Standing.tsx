import { Alert, Box, Button, Flex, Stack, Text } from '@chakra-ui/react'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { changedEvent, penaltyClient, pushEvent } from './api'
import { useFormatDate } from './components'
import { Role } from './gen/haitukang/v1/account_pb'
import { PenaltyPointState, PenaltyReason, type GetMyStandingResponse, type PenaltyPoint } from './gen/haitukang/v1/penalty_pb'

// Standing shows the reader's own penalty points and suspension
// (acceptance HT-008, HT-010): a banner when warned or suspended, and the
// points on request. Nothing shows for an account without points.
export function Standing({ role }: { role: Role }) {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const [standing, setStanding] = useState<GetMyStandingResponse | null>(null)
  const [open, setOpen] = useState(false)

  const load = useCallback(async () => {
    try {
      setStanding(await penaltyClient.getMyStanding({}))
    } catch {
      // The banner is extra: the actions it explains still say why they fail.
    }
  }, [])
  useEffect(() => {
    void load()
    const reload = () => void load()
    window.addEventListener(changedEvent, reload)
    window.addEventListener(pushEvent, reload)
    return () => {
      window.removeEventListener(changedEvent, reload)
      window.removeEventListener(pushEvent, reload)
    }
  }, [load])

  if (!standing || (standing.history.length === 0 && !standing.suspension)) return null
  const suspension = standing.suspension

  return (
    <Stack gap="3" data-testid="standing">
      {suspension ? (
        <Alert.Root status="error" role="alert">
          <Alert.Indicator />
          <Alert.Content>
            <Alert.Title>
              {suspension.closed ? t('penalties.closed') : t('penalties.suspended', { date: formatDate(suspension.endsAt) })}
            </Alert.Title>
            {!suspension.closed && (
              <Alert.Description>
                {role === Role.REPAIRMAN
                  ? t('penalties.suspendedRepairman')
                  : role === Role.STORE
                    ? t('penalties.suspendedStore')
                    : t('penalties.suspendedCustomer')}{' '}
                {role !== Role.STORE && t('penalties.confirmedGoAhead')}
              </Alert.Description>
            )}
          </Alert.Content>
        </Alert.Root>
      ) : standing.warned ? (
        <Alert.Root status="warning" role="status">
          <Alert.Indicator />
          <Alert.Title>{t('penalties.warned', { points: standing.points, suspendAt: standing.suspendAt })}</Alert.Title>
        </Alert.Root>
      ) : (
        <Text fontSize="sm" color="ink.600">
          {t('penalties.points', { points: standing.points })}
        </Text>
      )}
      {standing.history.length > 0 && (
        <Box>
          <Button size="sm" minH="11" variant="ghost" colorPalette="ink" px="0" aria-expanded={open} onClick={() => setOpen(!open)}>
            {open ? t('penalties.hide') : t('penalties.show')}
          </Button>
          {open && (
            <Stack gap="3" mt="2">
              {standing.history.map((p) => (
                <PointRow key={p.id.toString()} point={p} />
              ))}
              <Text fontSize="xs" color="ink.600">
                {t('penalties.rules')}
              </Text>
            </Stack>
          )}
        </Box>
      )}
    </Stack>
  )
}

const reasonKey = {
  [PenaltyReason.NO_SHOW]: 'no_show',
  [PenaltyReason.CUSTOMER_ABSENT]: 'customer_absent',
  [PenaltyReason.LATE_CANCEL]: 'late_cancel',
  [PenaltyReason.LATE_ARRIVAL]: 'late_arrival',
} as const

const stateKey = {
  [PenaltyPointState.PENDING]: 'pending',
  [PenaltyPointState.ACTIVE]: 'active',
  [PenaltyPointState.DISPUTED]: 'disputed',
  [PenaltyPointState.REMOVED]: 'removed',
  [PenaltyPointState.SPENT]: 'spent',
  [PenaltyPointState.EXPIRED]: 'expired',
} as const

export function PointRow({ point }: { point: PenaltyPoint }) {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const reason = point.reason in reasonKey ? reasonKey[point.reason as keyof typeof reasonKey] : null
  const state = point.state in stateKey ? stateKey[point.state as keyof typeof stateKey] : null
  const counted = point.state === PenaltyPointState.ACTIVE || point.state === PenaltyPointState.PENDING
  return (
    <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="lg" p="3">
      <Flex justify="space-between" gap="3" align="start">
        <Text fontSize="sm" fontWeight="semibold">
          {reason ? t(`penalties.reason.${reason}`) : ''}
        </Text>
        <Text fontSize="sm" fontWeight="semibold" whiteSpace="nowrap" textDecoration={counted ? undefined : 'line-through'}>
          {t('penalties.pointsValue', { points: point.points })}
        </Text>
      </Flex>
      <Text fontSize="xs" color="ink.600">
        {formatDate(point.createdAt)}
        {state &&
          ` · ${t(`penalties.state.${state}`, {
            date: formatDate(state === 'pending' ? point.countsFrom : point.expiresAt),
          })}`}
      </Text>
    </Box>
  )
}

export const isLateCancel = (start?: { seconds: bigint }) =>
  !!start && Date.now() > Number(start.seconds) * 1000 - 12 * 60 * 60 * 1000

export const timeOf = (ts?: Parameters<typeof timestampDate>[0]) => (ts ? timestampDate(ts).getTime() : 0)
