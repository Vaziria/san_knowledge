import { Badge, Box, Button, Card, Field, Heading, Input, Link, Stack, Tabs, Text, Textarea } from '@chakra-ui/react'
import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { adminClient, errorCode } from './api'
import { serviceName } from './catalog'
import { ErrorAlert, PhoneField, SuccessAlert, useFormatDate } from './components'
import { Role, type Account } from './gen/haitukang/v1/account_pb'
import { AdminAction, type ClosedAccount, type Decision, type Dispute, type FindAccountResponse } from './gen/haitukang/v1/admin_pb'
import { VisitStatus } from './gen/haitukang/v1/visit_pb'
import type { ErrorCode } from './i18n/id'
import { fullPhone, localPhone } from './phone'
import { PointRow } from './Standing'
import { whatsappLink } from './whatsapp'

// AdminHome is the admins' review (acceptance HT-011, HT-012): disputed
// reports, closed accounts, suspending by hand, and the log of decisions.
// Every decision needs a written reason.
export function AdminHome() {
  const { t } = useTranslation()
  const [tab, setTab] = useState('disputes')
  return (
    <Stack gap="4">
      <Heading as="h2" size="md">
        {t('admin.title')}
      </Heading>
      <Tabs.Root value={tab} onValueChange={(e) => setTab(e.value)} fitted colorPalette="ink" lazyMount unmountOnExit>
        <Tabs.List mb="5">
          <Tabs.Trigger value="disputes" px="1">{t('admin.tabDisputes')}</Tabs.Trigger>
          <Tabs.Trigger value="closed" px="1">{t('admin.tabClosed')}</Tabs.Trigger>
          <Tabs.Trigger value="find" px="1">{t('admin.tabFind')}</Tabs.Trigger>
          <Tabs.Trigger value="log" px="1">{t('admin.tabLog')}</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="disputes">
          <Disputes />
        </Tabs.Content>
        <Tabs.Content value="closed">
          <ClosedAccounts />
        </Tabs.Content>
        <Tabs.Content value="find">
          <FindAccount />
        </Tabs.Content>
        <Tabs.Content value="log">
          <DecisionLog />
        </Tabs.Content>
      </Tabs.Root>
    </Stack>
  )
}

// useList loads a list, and reloads it after a decision.
function useList<T>(load: () => Promise<T[]>) {
  const [list, setList] = useState<T[] | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const reload = useCallback(async () => {
    try {
      setList(await load())
      setError(null)
    } catch (err) {
      setError(errorCode(err))
    }
  }, [load])
  useEffect(() => {
    void reload()
  }, [reload])
  return { list, error, reload }
}

function Empty({ text }: { text: string }) {
  return (
    <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="2xl" p="5">
      <Text color="ink.600">{text}</Text>
    </Box>
  )
}

// DecisionForm is a written reason and the buttons that decide with it.
function DecisionForm({
  choices,
  extra,
  onDecided,
}: {
  choices: { label: string; palette: string; variant?: 'solid' | 'outline'; decide: (reason: string) => Promise<unknown> }[]
  extra?: ReactNode
  onDecided: () => void
}) {
  const { t } = useTranslation()
  const [reason, setReason] = useState('')
  const [pending, setPending] = useState<number | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const decide = async (i: number) => {
    setError(null)
    if (reason.trim().length < 5) {
      setError('invalid_reason')
      return
    }
    setPending(i)
    try {
      await choices[i].decide(reason)
      setReason('')
      onDecided()
    } catch (err) {
      setError(errorCode(err))
    } finally {
      setPending(null)
    }
  }
  return (
    <Stack gap="3">
      {extra}
      <Field.Root required>
        <Field.Label>{t('admin.reason')}</Field.Label>
        <Textarea size="lg" rows={2} value={reason} onChange={(e) => setReason(e.currentTarget.value)} maxLength={500} />
        <Field.HelperText>{t('admin.reasonHelp')}</Field.HelperText>
      </Field.Root>
      <ErrorAlert code={error} />
      {choices.map((c, i) => (
        <Button
          key={c.label}
          size="lg"
          colorPalette={c.palette}
          variant={c.variant ?? 'solid'}
          loading={pending === i}
          disabled={pending !== null && pending !== i}
          onClick={() => void decide(i)}
        >
          {c.label}
        </Button>
      ))}
    </Stack>
  )
}

function Line({ children }: { children: ReactNode }) {
  return (
    <Text fontSize="sm" color="ink.700">
      {children}
    </Text>
  )
}

function Disputes() {
  const { t } = useTranslation()
  const load = useCallback(async () => (await adminClient.listDisputes({})).disputes, [])
  const { list, error, reload } = useList<Dispute>(load)
  const [done, setDone] = useState<string | null>(null)
  if (error) return <ErrorAlert code={error} />
  if (!list) return null
  return (
    <Stack gap="4">
      <SuccessAlert text={done} />
      {list.length === 0 ? (
        <Empty text={t('admin.noDisputes')} />
      ) : (
        list.map((d) => (
          <DisputeCard
            key={d.visit?.id.toString()}
            dispute={d}
            onDecided={() => {
              setDone(t('admin.decided'))
              void reload()
            }}
          />
        ))
      )}
    </Stack>
  )
}

function DisputeCard({ dispute, onDecided }: { dispute: Dispute; onDecided: () => void }) {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const v = dispute.visit
  if (!v) return null
  const absent = dispute.report === VisitStatus.CUSTOMER_ABSENT
  const reportedName = absent ? v.customerName : v.repairmanName
  const decide = (stand: boolean) => (reason: string) =>
    adminClient.decideDispute({ visitId: v.id, pointsStand: stand, reason })
  return (
    <Card.Root variant="outline" borderColor="ink.100" rounded="2xl" data-dispute={v.id.toString()}>
      <Card.Body gap="3" p="4">
        <Text fontWeight="semibold">
          {absent
            ? t('admin.reportAbsent', { repairman: v.repairmanName, customer: v.customerName })
            : t('admin.reportNoShow', { repairman: v.repairmanName, customer: v.customerName })}
        </Text>
        <Box>
          <Badge colorPalette="ink" variant="subtle" mb="1">
            {serviceName(t, v.service)}
          </Badge>
          <Text fontSize="sm">{v.description}</Text>
          <Text fontSize="sm" color="ink.600">
            {v.area}
          </Text>
        </Box>
        <Stack gap="1">
          <Line>{t('admin.schedule', { start: formatDate(v.window?.start), end: formatDate(v.window?.end) })}</Line>
          <Line>{t('admin.confirmedAt', { time: formatDate(dispute.confirmedAt) })}</Line>
          <Line>{t('admin.arrivalAttempts', { count: dispute.arrivalAttempts })}</Line>
          <Line>{t('admin.reportedAt', { time: formatDate(dispute.reportedAt) })}</Line>
          <Line>{t('admin.disputedAt', { name: reportedName, time: formatDate(v.disputedAt) })}</Line>
        </Stack>
        <Box bg="cream" rounded="lg" p="3">
          <Text fontSize="sm">{v.disputeNote ? `“${v.disputeNote}”` : t('admin.noNote')}</Text>
        </Box>
        <Text fontSize="sm" fontWeight="semibold">
          {t('admin.points', { name: reportedName, points: dispute.points })}
        </Text>
        <Stack gap="0">
          <Contact name={v.customerName} phone={v.customerPhone} />
          <Contact name={v.repairmanName} phone={v.repairmanPhone} />
        </Stack>
        <DecisionForm
          onDecided={onDecided}
          choices={[
            { label: t('admin.pointsStand'), palette: 'red', decide: decide(true) },
            { label: t('admin.removePoints'), palette: 'ink', variant: 'outline', decide: decide(false) },
          ]}
        />
      </Card.Body>
    </Card.Root>
  )
}

function Contact({ name, phone }: { name: string; phone: string }) {
  const { t } = useTranslation()
  if (!phone) return null
  return (
    <Link href={whatsappLink(phone, t('admin.waText'))} target="_blank" rel="noopener noreferrer" fontSize="sm" color="green.700" minH="11" display="inline-flex" alignItems="center">
      {t('admin.contact', { name })}
    </Link>
  )
}

function AccountLine({ account }: { account: Account }) {
  const { t } = useTranslation()
  return (
    <Box>
      <Text fontWeight="semibold">{account.fullName}</Text>
      <Text fontSize="sm" color="ink.600">
        {account.role === Role.REPAIRMAN ? t('roles.repairman') : t('roles.customer')} · {account.phone}
      </Text>
    </Box>
  )
}

function ClosedAccounts() {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const load = useCallback(async () => (await adminClient.listClosedAccounts({})).accounts, [])
  const { list, error, reload } = useList<ClosedAccount>(load)
  const [done, setDone] = useState<string | null>(null)
  if (error) return <ErrorAlert code={error} />
  if (!list) return null
  return (
    <Stack gap="4">
      <SuccessAlert text={done} />
      {list.length === 0 ? (
        <Empty text={t('admin.noClosed')} />
      ) : (
        list.map((c) =>
          c.account ? (
            <Card.Root key={c.account.id.toString()} variant="outline" borderColor="ink.100" rounded="2xl" data-closed={c.account.id.toString()}>
              <Card.Body gap="3" p="4">
                <AccountLine account={c.account} />
                <Line>{t('admin.closedSince', { time: formatDate(c.closedAt), number: c.number })}</Line>
                {c.reviewedAt && <Line>{t('admin.reviewedKeep', { time: formatDate(c.reviewedAt), reason: c.lastReason })}</Line>}
                <DecisionForm
                  onDecided={() => {
                    setDone(t('admin.decided'))
                    void reload()
                  }}
                  choices={[
                    {
                      label: t('admin.reopen'),
                      palette: 'ink',
                      decide: (reason) => adminClient.decideClosedAccount({ accountId: c.account!.id, reopen: true, reason }),
                    },
                    {
                      label: t('admin.keepClosed'),
                      palette: 'red',
                      variant: 'outline',
                      decide: (reason) => adminClient.decideClosedAccount({ accountId: c.account!.id, reopen: false, reason }),
                    },
                  ]}
                />
              </Card.Body>
            </Card.Root>
          ) : null,
        )
      )}
    </Stack>
  )
}

function FindAccount() {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const [phone, setPhone] = useState('')
  const [found, setFound] = useState<FindAccountResponse | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [pending, setPending] = useState(false)
  const [days, setDays] = useState('7')
  const [done, setDone] = useState<string | null>(null)

  const find = async (cleaned: string) => {
    setError(null)
    setNotFound(false)
    setPending(true)
    try {
      setFound(await adminClient.findAccount({ phone: fullPhone(cleaned) }))
    } catch (err) {
      setFound(null)
      const code = errorCode(err)
      if (code === 'not_found') setNotFound(true)
      else setError(code)
    } finally {
      setPending(false)
    }
  }
  const account = found?.account
  const standing = found?.standing
  const suspension = standing?.suspension

  return (
    <Stack gap="4">
      <form
        onSubmit={(event) => {
          event.preventDefault()
          setDone(null)
          const cleaned = localPhone(phone)
          setPhone(cleaned)
          void find(cleaned)
        }}
      >
        <Stack gap="3">
          <PhoneField value={phone} onChange={setPhone} />
          <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
            {t('admin.find')}
          </Button>
        </Stack>
      </form>
      <ErrorAlert code={error} />
      {notFound && <Empty text={t('admin.noAccount')} />}
      <SuccessAlert text={done} />
      {account && standing && (
        <Card.Root variant="outline" borderColor="ink.100" rounded="2xl" data-testid="found-account">
          <Card.Body gap="3" p="4">
            <AccountLine account={account} />
            <Text fontSize="sm" fontWeight="semibold" color={suspension ? 'red.700' : 'ink.700'}>
              {suspension
                ? suspension.closed
                  ? t('admin.isClosed')
                  : t('admin.suspendedUntil', { date: formatDate(suspension.endsAt) })
                : t('admin.notSuspended')}
            </Text>
            <Line>{t('penalties.points', { points: standing.points })}</Line>
            {standing.history.map((p) => (
              <PointRow key={p.id.toString()} point={p} />
            ))}
            {suspension?.closed ? (
              <Line>{t('admin.closedHere')}</Line>
            ) : suspension ? (
              <DecisionForm
                onDecided={() => {
                  setDone(t('admin.decided'))
                  void find(phone)
                }}
                choices={[{ label: t('admin.lift'), palette: 'ink', decide: (reason) => adminClient.liftSuspension({ accountId: account.id, reason }) }]}
              />
            ) : (
              <DecisionForm
                onDecided={() => {
                  setDone(t('admin.decided'))
                  void find(phone)
                }}
                extra={
                  <Field.Root required>
                    <Field.Label>{t('admin.days')}</Field.Label>
                    <Input size="lg" type="number" inputMode="numeric" min={1} max={365} value={days} onChange={(e) => setDays(e.currentTarget.value)} />
                    <Field.HelperText>{t('admin.daysHelp')}</Field.HelperText>
                  </Field.Root>
                }
                choices={[
                  {
                    label: t('admin.suspend'),
                    palette: 'red',
                    decide: (reason) => adminClient.suspendAccount({ accountId: account.id, days: Number(days) || 0, reason }),
                  },
                ]}
              />
            )}
          </Card.Body>
        </Card.Root>
      )}
    </Stack>
  )
}

const actionKey = {
  [AdminAction.POINTS_STAND]: 'points_stand',
  [AdminAction.POINTS_REMOVED]: 'points_removed',
  [AdminAction.REOPEN]: 'reopen',
  [AdminAction.KEEP_CLOSED]: 'keep_closed',
  [AdminAction.SUSPEND]: 'suspend',
  [AdminAction.LIFT]: 'lift',
} as const

function DecisionLog() {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const load = useCallback(async () => (await adminClient.listDecisions({})).decisions, [])
  const { list, error } = useList<Decision>(load)
  if (error) return <ErrorAlert code={error} />
  if (!list) return null
  if (list.length === 0) return <Empty text={t('admin.noLog')} />
  return (
    <Stack gap="3">
      {list.map((d) => {
        const key = d.action in actionKey ? actionKey[d.action as keyof typeof actionKey] : null
        return (
          <Box key={d.id.toString()} bg="white" borderWidth="1px" borderColor="ink.100" rounded="lg" p="3" data-decision={d.id.toString()}>
            <Text fontSize="sm" fontWeight="semibold">
              {key ? t(`admin.action.${key}`) : ''} · {d.accountName}
            </Text>
            <Text fontSize="sm">“{d.reason}”</Text>
            <Text fontSize="xs" color="ink.600">
              {t('admin.decisionBy', { admin: d.adminName, time: formatDate(d.createdAt) })}
            </Text>
          </Box>
        )
      })}
    </Stack>
  )
}
