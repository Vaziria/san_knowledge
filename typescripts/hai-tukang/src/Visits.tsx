import { Badge, Box, Button, Card, Field, Flex, HStack, Input, Stack, Text, Textarea } from '@chakra-ui/react'
import { timestampDate, timestampFromDate } from '@bufbuild/protobuf/wkt'
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { changedEvent, errorCode, pushEvent, visitClient } from './api'
import { serviceName } from './catalog'
import { ErrorAlert, useFormatDate, WhatsAppButton } from './components'
import { Role } from './gen/haitukang/v1/account_pb'
import { DisputeOutcome, VisitStatus, type Visit, type VisitWindow } from './gen/haitukang/v1/visit_pb'
import type { ErrorCode } from './i18n/id'
import { isLateCancel, timeOf } from './Standing'

// Visits (acceptance HT-005 to HT-007): the customer schedules one with a
// repairman who responded; the other side accepts or proposes another
// time; the repairman enters the customer's arrival code when he arrives;
// the customer reports a no-show 30 minutes after the window.

export const isActive = (v: Visit) => v.status === VisitStatus.PROPOSED || v.status === VisitStatus.CONFIRMED

// latestFirst orders visits by when they were made, newest first.
export const latestFirst = (a: Visit, b: Visit) =>
  Number((b.createdAt?.seconds ?? 0n) - (a.createdAt?.seconds ?? 0n)) || Number(b.id - a.id)

// useVisits loads the signed-in account's visits.
export function useVisits() {
  const [list, setList] = useState<Visit[] | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const load = useCallback(async () => {
    try {
      setList((await visitClient.listMyVisits({})).visits)
      setError(null)
    } catch (err) {
      setError(errorCode(err))
    }
  }, [])
  useEffect(() => void load(), [load])
  // A push while the app is open: something about a visit changed.
  useEffect(() => {
    const reload = () => void load()
    window.addEventListener(pushEvent, reload)
    return () => window.removeEventListener(pushEvent, reload)
  }, [load])
  return { list, error, load }
}

function useWindowText() {
  const { i18n } = useTranslation()
  const day = new Intl.DateTimeFormat(i18n.resolvedLanguage, { weekday: 'long', day: 'numeric', month: 'long' })
  const time = new Intl.DateTimeFormat(i18n.resolvedLanguage, { hour: '2-digit', minute: '2-digit' })
  return (w?: VisitWindow) => {
    if (!w?.start || !w.end) return ''
    const start = timestampDate(w.start)
    return `${day.format(start)}, ${time.format(start)}–${time.format(timestampDate(w.end))}`
  }
}

// useAction runs one visit call at a time, then onDone; it keeps the error.
function useAction(onDone: () => void) {
  const [pending, setPending] = useState<string | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const run = async (name: string, call: () => Promise<unknown>) => {
    setPending(name)
    setError(null)
    try {
      await call()
      onDone()
      window.dispatchEvent(new Event(changedEvent))
    } catch (err) {
      setError(errorCode(err))
    } finally {
      setPending(null)
    }
  }
  return { pending, error, run }
}

const pad = (n: number) => String(n).padStart(2, '0')
const localDay = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

// WindowForm picks a day and a time range, in the reader's time zone.
function WindowForm({
  submitLabel,
  onSubmit,
  onClose,
}: {
  submitLabel: string
  onSubmit: (w: VisitWindow) => Promise<void>
  onClose: () => void
}) {
  const { t } = useTranslation()
  const [day, setDay] = useState(() => localDay(new Date(Date.now() + 24 * 60 * 60 * 1000)))
  const [from, setFrom] = useState('09:00')
  const [until, setUntil] = useState('11:00')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPending(true)
    setError(null)
    try {
      await onSubmit({
        $typeName: 'haitukang.v1.VisitWindow',
        start: timestampFromDate(new Date(`${day}T${from}`)),
        end: timestampFromDate(new Date(`${day}T${until}`)),
      })
    } catch (err) {
      setError(errorCode(err))
      setPending(false)
    }
  }

  return (
    <form onSubmit={submit}>
      <Stack gap="3">
        <Field.Root required>
          <Field.Label>{t('visits.date')}</Field.Label>
          <Input size="lg" type="date" min={localDay(new Date())} value={day} onChange={(e) => setDay(e.currentTarget.value)} required />
        </Field.Root>
        <HStack gap="3" align="start">
          <Field.Root required>
            <Field.Label>{t('visits.from')}</Field.Label>
            <Input size="lg" type="time" value={from} onChange={(e) => setFrom(e.currentTarget.value)} required />
          </Field.Root>
          <Field.Root required>
            <Field.Label>{t('visits.until')}</Field.Label>
            <Input size="lg" type="time" value={until} onChange={(e) => setUntil(e.currentTarget.value)} required />
          </Field.Root>
        </HStack>
        <ErrorAlert code={error} />
        <HStack gap="2">
          <Button type="submit" size="lg" colorPalette="ink" flex="1" loading={pending}>
            {submitLabel}
          </Button>
          <Button type="button" size="lg" variant="ghost" colorPalette="ink" onClick={onClose}>
            {t('visits.close')}
          </Button>
        </HStack>
      </Stack>
    </form>
  )
}

// ScheduleVisit is the customer's button, on a repairman's response, to
// propose a visit to him.
export function ScheduleVisit({ requestId, repairmanId, onDone }: { requestId: bigint; repairmanId: bigint; onDone: () => void }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  if (!open) {
    return (
      <Button size="lg" variant="outline" colorPalette="ink" w="full" onClick={() => setOpen(true)}>
        {t('visits.schedule')}
      </Button>
    )
  }
  return (
    <Box borderWidth="1px" borderColor="ink.100" rounded="xl" p="4">
      <WindowForm
        submitLabel={t('visits.propose')}
        onSubmit={async (window) => {
          await visitClient.proposeVisit({ requestId, repairmanId, window })
          setOpen(false)
          onDone()
        }}
        onClose={() => setOpen(false)}
      />
    </Box>
  )
}

// Answer is accept / propose another time / cancel, for the side whose
// turn it is.
function Answer({ visit, cancelLabel, onDone }: { visit: Visit; cancelLabel: string; onDone: () => void }) {
  const { t } = useTranslation()
  const { pending, error, run } = useAction(onDone)
  const [countering, setCountering] = useState(false)
  if (countering) {
    return (
      <WindowForm
        submitLabel={t('visits.propose')}
        onSubmit={async (window) => {
          await visitClient.counterVisit({ visitId: visit.id, window })
          setCountering(false)
          onDone()
        }}
        onClose={() => setCountering(false)}
      />
    )
  }
  return (
    <Stack gap="2">
      <Button size="lg" colorPalette="ink" loading={pending === 'accept'} onClick={() => void run('accept', () => visitClient.acceptVisit({ visitId: visit.id }))}>
        {t('visits.accept')}
      </Button>
      <Button size="lg" variant="outline" colorPalette="ink" onClick={() => setCountering(true)}>
        {t('visits.proposeOther')}
      </Button>
      <Button size="lg" variant="ghost" colorPalette="ink" loading={pending === 'cancel'} onClick={() => void run('cancel', () => visitClient.cancelVisit({ visitId: visit.id }))}>
        {cancelLabel}
      </Button>
      <ErrorAlert code={error} />
    </Stack>
  )
}

// CancelButton cancels; a confirmed visit less than 12 hours away costs a
// point, so it asks first.
function CancelButton({ visit, onDone }: { visit: Visit; onDone: () => void }) {
  const { t } = useTranslation()
  const { pending, error, run } = useAction(onDone)
  const [confirming, setConfirming] = useState(false)
  const cancel = () => void run('cancel', () => visitClient.cancelVisit({ visitId: visit.id }))
  const costly = visit.status === VisitStatus.CONFIRMED && isLateCancel(visit.window?.start)
  if (confirming) {
    return (
      <Stack gap="2">
        <Text fontSize="sm">{t('visits.lateCancelWarning')}</Text>
        <Button size="lg" colorPalette="red" loading={pending !== null} onClick={cancel}>
          {t('visits.yesCancel')}
        </Button>
        <Button size="lg" variant="ghost" colorPalette="ink" onClick={() => setConfirming(false)}>
          {t('visits.close')}
        </Button>
        <ErrorAlert code={error} />
      </Stack>
    )
  }
  return (
    <Stack gap="2">
      <Button size="lg" variant="ghost" colorPalette="ink" loading={pending !== null} onClick={costly ? () => setConfirming(true) : cancel}>
        {t('visits.cancel')}
      </Button>
      <ErrorAlert code={error} />
    </Stack>
  )
}

// Report shows a no-show or not-at-home report, and the dispute: the form
// for the reported side within 24 hours, or the answer given.
function Report({ visit, viewer, onDone }: { visit: Visit; viewer: Role; onDone: () => void }) {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const { pending, error, run } = useAction(onDone)
  const [note, setNote] = useState('')
  const absent = visit.status === VisitStatus.CUSTOMER_ABSENT
  if (!absent && visit.status !== VisitStatus.NO_SHOW) return null
  // The reported side: the customer for "not at home", the repairman for a no-show.
  const reported = absent ? viewer === Role.CUSTOMER : viewer === Role.REPAIRMAN
  const otherName = viewer === Role.CUSTOMER ? visit.repairmanName : visit.customerName
  const reportedAt = absent ? visit.absentAt : visit.noShowAt
  const canDispute = reported && !visit.disputedAt && Date.now() < timeOf(visit.disputeUntil)

  return (
    <Stack gap="2" data-testid="report">
      <Text color="red.700" fontWeight="semibold">
        {reported
          ? t(absent ? 'visits.absentReportedAboutYou' : 'visits.noShowReportedAboutYou', { name: otherName, time: formatDate(reportedAt) })
          : absent
            ? t('visits.absentReportedByYou', { time: formatDate(reportedAt) })
            : t('visits.noShow', { time: formatDate(reportedAt) })}
      </Text>
      {visit.disputedAt && (
        <Box bg="white" rounded="lg" p="3">
          <Text fontSize="sm">
            {visit.disputeOutcome === DisputeOutcome.POINTS_STAND
              ? `${t('visits.disputedOnly')} ${t('visits.outcomeStand')}`
              : visit.disputeOutcome === DisputeOutcome.POINTS_REMOVED
                ? `${t('visits.disputedOnly')} ${t('visits.outcomeRemoved')}`
                : reported
                  ? t('visits.disputedByYou')
                  : t('visits.disputedByOther', { name: otherName })}
          </Text>
          {visit.disputeNote && (
            <Text fontSize="sm" color="ink.600" mt="1">
              “{visit.disputeNote}”
            </Text>
          )}
        </Box>
      )}
      {canDispute && (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            void run('dispute', () => visitClient.disputeReport({ visitId: visit.id, note }))
          }}
        >
          <Stack gap="2">
            <Text fontSize="sm" color="ink.600">
              {t('visits.disputeUntil', { time: formatDate(visit.disputeUntil) })}
            </Text>
            <Textarea
              size="lg"
              rows={2}
              value={note}
              onChange={(e) => setNote(e.currentTarget.value)}
              placeholder={t('visits.disputeNote')}
              aria-label={t('visits.disputeNote')}
              maxLength={500}
            />
            <ErrorAlert code={error} />
            <Button type="submit" size="lg" variant="outline" colorPalette="ink" loading={pending !== null}>
              {absent ? t('visits.disputeAbsent') : t('visits.disputeNoShow')}
            </Button>
          </Stack>
        </form>
      )}
    </Stack>
  )
}

// AbsentButton is the repairman's report that nobody was at home: from the
// window's start until 30 minutes after it.
function AbsentButton({ visit, onDone }: { visit: Visit; onDone: () => void }) {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const { pending, error, run } = useAction(onDone)
  const [confirming, setConfirming] = useState(false)
  const now = Date.now()
  const from = timeOf(visit.window?.start)
  const until = timeOf(visit.noShowAfter)
  if (now < from) {
    return (
      <Text fontSize="sm" color="ink.600">
        {t('visits.absentFrom', { time: formatDate(visit.window?.start) })}
      </Text>
    )
  }
  if (now > until) return null
  if (confirming) {
    return (
      <Stack gap="2">
        <Text fontSize="sm">{t('visits.confirmAbsent')}</Text>
        <Button size="lg" colorPalette="red" loading={pending !== null} onClick={() => void run('absent', () => visitClient.reportCustomerAbsent({ visitId: visit.id }))}>
          {t('visits.yesReport')}
        </Button>
        <Button size="lg" variant="ghost" colorPalette="ink" onClick={() => setConfirming(false)}>
          {t('visits.close')}
        </Button>
        <ErrorAlert code={error} />
      </Stack>
    )
  }
  return (
    <Button size="lg" variant="outline" colorPalette="red" onClick={() => setConfirming(true)}>
      {t('visits.reportAbsent')}
    </Button>
  )
}

// Ended is one line for a visit that is over.
function Ended({ visit, viewer, otherName }: { visit: Visit; viewer: Role; otherName: string }) {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  switch (visit.status) {
    case VisitStatus.ARRIVED:
      return <Text color="green.700" fontWeight="semibold">{t('visits.arrived', { time: formatDate(visit.arrivedAt) })}</Text>
    case VisitStatus.CANCELLED:
      return <Text color="ink.500">{t('visits.cancelledBy', { who: visit.cancelledBy === viewer ? t('visits.you') : otherName })}</Text>
  }
  return null
}

// CustomerVisit is the visit on the customer's request card.
// EarlierReports are the no-show and not-at-home reports of a request's
// earlier visits: the card shows its newest visit, but a report stays
// open to dispute, and its outcome, after a new visit is arranged.
export function EarlierReports({ visits, onChange }: { visits: Visit[]; onChange: () => void }) {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const reported = visits.filter((v) => v.status === VisitStatus.NO_SHOW || v.status === VisitStatus.CUSTOMER_ABSENT)
  if (reported.length === 0) return null
  return (
    <Stack gap="3">
      {reported.map((v) => (
        <Box key={v.id.toString()} bg="red.50" rounded="lg" p="3" data-visit={v.id.toString()}>
          <Text fontSize="xs" color="ink.600" mb="1">
            {t('visits.earlierVisit', { time: formatDate(v.window?.start) })}
          </Text>
          <Report visit={v} viewer={Role.CUSTOMER} onDone={onChange} />
        </Box>
      ))}
    </Stack>
  )
}

export function CustomerVisit({ visit, onChange }: { visit: Visit; onChange: () => void }) {
  const { t } = useTranslation()
  const windowText = useWindowText()
  const formatDate = useFormatDate()
  const { pending, error, run } = useAction(onChange)
  const [confirmingNoShow, setConfirmingNoShow] = useState(false)
  const noShowAfter = visit.noShowAfter ? timestampDate(visit.noShowAfter) : null

  return (
    <Box bg="ochre.50" borderWidth="1px" borderColor="ochre.200" rounded="xl" p="4">
      <Stack gap="3">
        <Text fontWeight="semibold">
          {visit.status === VisitStatus.PROPOSED
            ? t('visits.proposedTo', { name: visit.repairmanName })
            : visit.status === VisitStatus.CONFIRMED
              ? t('visits.confirmed')
              : t('visits.title')}
        </Text>
        <Text>{windowText(visit.window)}</Text>
        {visit.status !== VisitStatus.PROPOSED && <Text fontSize="sm" color="ink.600">{t('visits.with', { name: visit.repairmanName })}</Text>}
        {visit.status === VisitStatus.PROPOSED &&
          (visit.yourTurn ? (
            <>
              <Text fontSize="sm">{t('visits.proposedBy', { name: visit.repairmanName })}</Text>
              <Answer visit={visit} cancelLabel={t('visits.cancel')} onDone={onChange} />
            </>
          ) : (
            <>
              <Text fontSize="sm" color="ink.600">{t('visits.waitingFor', { name: visit.repairmanName })}</Text>
              <CancelButton visit={visit} onDone={onChange} />
            </>
          ))}
        {visit.status === VisitStatus.CONFIRMED && (
          <>
            <Box bg="white" rounded="lg" p="3" textAlign="center">
              <Text fontSize="sm" color="ink.600">{t('visits.arrivalCode')}</Text>
              <Text fontSize="3xl" fontWeight="bold" letterSpacing="0.3em" data-testid="arrival-code">
                {visit.arrivalCode}
              </Text>
              <Text fontSize="xs" color="ink.600">{t('visits.arrivalCodeHelp')}</Text>
            </Box>
            {noShowAfter && Date.now() >= noShowAfter.getTime() ? (
              confirmingNoShow ? (
                <Stack gap="2">
                  <Text fontSize="sm">{t('visits.confirmNoShow')}</Text>
                  <Button size="lg" colorPalette="red" loading={pending === 'noShow'} onClick={() => void run('noShow', () => visitClient.reportNoShow({ visitId: visit.id }))}>
                    {t('visits.yesReport')}
                  </Button>
                  <Button size="lg" variant="ghost" colorPalette="ink" onClick={() => setConfirmingNoShow(false)}>
                    {t('visits.close')}
                  </Button>
                </Stack>
              ) : (
                <Button size="lg" variant="outline" colorPalette="red" onClick={() => setConfirmingNoShow(true)}>
                  {t('visits.reportNoShow')}
                </Button>
              )
            ) : (
              <Text fontSize="sm" color="ink.600">{t('visits.noShowFrom', { time: formatDate(visit.noShowAfter) })}</Text>
            )}
            <ErrorAlert code={error} />
            <CancelButton visit={visit} onDone={onChange} />
          </>
        )}
        <Report visit={visit} viewer={Role.CUSTOMER} onDone={onChange} />
        <Ended visit={visit} viewer={Role.CUSTOMER} otherName={visit.repairmanName} />
      </Stack>
    </Box>
  )
}

// RepairmanVisits is the repairman's schedule: proposals to answer,
// confirmed visits to arrive at, and past ones.
export function RepairmanVisits() {
  const { t } = useTranslation()
  const { list, error, load } = useVisits()
  return (
    <Stack gap="5">
      <ErrorAlert code={error} />
      {list && list.length === 0 && <Text color="ink.500">{t('visits.none')}</Text>}
      {list && [...list].sort(latestFirst).map((v) => <RepairmanVisit key={v.id.toString()} visit={v} onChange={load} />)}
    </Stack>
  )
}

function RepairmanVisit({ visit, onChange }: { visit: Visit; onChange: () => void }) {
  const { t } = useTranslation()
  const windowText = useWindowText()
  return (
    <Card.Root variant="outline" bg="white" data-visit={visit.id.toString()}>
      <Card.Body gap="3">
        <Flex justify="space-between" align="center" gap="2" wrap="wrap">
          <Badge colorPalette="ochre" variant="subtle" size="md">
            {serviceName(t, visit.service)}
          </Badge>
          {visit.status === VisitStatus.CONFIRMED && (
            <Badge colorPalette="green" variant="subtle" size="md">
              {t('visits.confirmed')}
            </Badge>
          )}
        </Flex>
        <Text fontWeight="semibold">{windowText(visit.window)}</Text>
        <Text color="ink.600" fontSize="sm">
          {t('repairman.from', { name: visit.customerName })} · {visit.area}
        </Text>
        <Text>{visit.description}</Text>
        {visit.status === VisitStatus.PROPOSED &&
          (visit.yourTurn ? (
            <>
              <Text fontSize="sm">{t('visits.proposedBy', { name: visit.customerName })}</Text>
              <Answer visit={visit} cancelLabel={t('visits.decline')} onDone={onChange} />
            </>
          ) : (
            <>
              <Text fontSize="sm" color="ink.600">{t('visits.waitingFor', { name: visit.customerName })}</Text>
              <CancelButton visit={visit} onDone={onChange} />
            </>
          ))}
        {visit.status === VisitStatus.CONFIRMED && (
          <>
            {visit.customerPhone && (
              <WhatsAppButton
                phone={visit.customerPhone}
                text={t('repairman.contactText', { name: visit.customerName, me: visit.repairmanName, service: serviceName(t, visit.service) })}
              />
            )}
            <ArrivalForm visit={visit} onDone={onChange} />
            <AbsentButton visit={visit} onDone={onChange} />
            <CancelButton visit={visit} onDone={onChange} />
          </>
        )}
        <Report visit={visit} viewer={Role.REPAIRMAN} onDone={onChange} />
        <Ended visit={visit} viewer={Role.REPAIRMAN} otherName={visit.customerName} />
      </Card.Body>
    </Card.Root>
  )
}

function ArrivalForm({ visit, onDone }: { visit: Visit; onDone: () => void }) {
  const { t } = useTranslation()
  const [code, setCode] = useState('')
  const { pending, error, run } = useAction(onDone)
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    void run('arrive', () => visitClient.confirmArrival({ visitId: visit.id, code }))
  }
  return (
    <form onSubmit={submit}>
      <Stack gap="3" borderTopWidth="1px" borderColor="ink.100" pt="3">
        <Field.Root required>
          <Field.Label>{t('visits.enterCode')}</Field.Label>
          <Input
            size="lg"
            inputMode="numeric"
            autoComplete="off"
            pattern="[0-9]{4}"
            value={code}
            onChange={(e) => setCode(e.currentTarget.value.replace(/\D/g, '').slice(0, 4))}
            fontSize="xl"
            letterSpacing="0.3em"
            required
          />
          <Field.HelperText>{t('visits.arriveHelp')}</Field.HelperText>
        </Field.Root>
        <ErrorAlert code={error} />
        <Button type="submit" size="lg" colorPalette="ink" loading={pending !== null}>
          {t('visits.arrive')}
        </Button>
      </Stack>
    </form>
  )
}
