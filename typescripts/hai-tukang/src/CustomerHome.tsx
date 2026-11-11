import { Badge, Box, Button, Card, Field, Flex, Heading, Input, Stack, Tabs, Text, Textarea } from '@chakra-ui/react'
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { accounts, errorCode, repairRequests } from './api'
import { serviceName } from './catalog'
import { ErrorAlert, ServiceSelect, SuccessAlert, useFormatDate, WhatsAppButton } from './components'
import type { Repairman } from './gen/haitukang/v1/account_pb'
import type { RepairRequest } from './gen/haitukang/v1/repair_request_pb'
import type { ErrorCode } from './i18n/id'
import { CustomerVisit, EarlierReports, isActive, latestFirst, ScheduleVisit, useVisits } from './Visits'

// CustomerHome: find a repairman and contact him (HT-003), or post a
// request, see who responded (HT-004) and arrange a visit (HT-005 to HT-007).
export function CustomerHome() {
  const { t } = useTranslation()
  const [tab, setTab] = useState('find')
  return (
    <Tabs.Root value={tab} onValueChange={(e) => setTab(e.value)} fitted colorPalette="ink" lazyMount>
      <Tabs.List mb="5">
        <Tabs.Trigger value="find">{t('customer.tabFind')}</Tabs.Trigger>
        <Tabs.Trigger value="requests">{t('customer.tabRequests')}</Tabs.Trigger>
      </Tabs.List>
      <Tabs.Content value="find">
        <FindRepairmen />
      </Tabs.Content>
      <Tabs.Content value="requests">
        <MyRequests />
      </Tabs.Content>
    </Tabs.Root>
  )
}

function FindRepairmen() {
  const { t } = useTranslation()
  const [service, setService] = useState('')
  const [area, setArea] = useState('')
  const [list, setList] = useState<Repairman[] | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [pending, setPending] = useState(false)

  const search = useCallback(async (service: string, area: string) => {
    setPending(true)
    setError(null)
    try {
      setList((await accounts.listRepairmen({ service, area })).repairmen)
    } catch (err) {
      setError(errorCode(err))
    } finally {
      setPending(false)
    }
  }, [])
  useEffect(() => void search('', ''), [search])

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    void search(service, area)
  }

  return (
    <Stack gap="5">
      <form onSubmit={submit}>
        <Stack gap="4">
          <ServiceSelect value={service} onChange={setService} allowAll />
          <Field.Root>
            <Field.Label>{t('common.area')}</Field.Label>
            <Input size="lg" value={area} onChange={(e) => setArea(e.currentTarget.value)} placeholder={t('common.areaPlaceholder')} />
          </Field.Root>
          <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
            {t('customer.search')}
          </Button>
        </Stack>
      </form>
      <ErrorAlert code={error} />
      {list && list.length === 0 && <Text color="ink.500">{t('customer.noRepairmen')}</Text>}
      {list?.map((r) => (
        <Card.Root key={r.id.toString()} variant="outline" bg="white">
          <Card.Body gap="3">
            <Box>
              <Heading as="h3" size="md">
                {r.fullName}
              </Heading>
              <Text color="ink.500" fontSize="sm">
                {r.profile?.area}
              </Text>
            </Box>
            <Flex wrap="wrap" gap="2">
              {r.profile?.services.map((code) => (
                <Badge key={code} colorPalette="ochre" variant="subtle" size="md">
                  {serviceName(t, code)}
                </Badge>
              ))}
            </Flex>
            {r.profile?.about && <Text fontSize="sm">{r.profile.about}</Text>}
            <WhatsAppButton
              phone={r.phone}
              text={t('customer.contactText', { name: r.fullName, service: serviceName(t, service || r.profile?.services[0] || '') })}
            />
          </Card.Body>
        </Card.Root>
      ))}
    </Stack>
  )
}

function MyRequests() {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const [list, setList] = useState<RepairRequest[] | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)

  const load = useCallback(async () => {
    try {
      setList((await repairRequests.listMyRepairRequests({})).requests)
      setError(null)
    } catch (err) {
      setError(errorCode(err))
    }
  }, [])
  useEffect(() => void load(), [load])
  const { list: visits, error: visitsError, load: loadVisits } = useVisits()
  const reload = () => {
    void load()
    void loadVisits()
  }
  const visitsOf = (requestId: bigint) => (visits ?? []).filter((v) => v.requestId === requestId).sort(latestFirst)

  return (
    <Stack gap="6">
      <NewRequestForm onPosted={load} />
      <ErrorAlert code={error ?? visitsError} />
      {list && list.length === 0 && <Text color="ink.500">{t('customer.noRequests')}</Text>}
      {list?.map((r) => (
        <Card.Root key={r.id.toString()} variant="outline" bg="white">
          <Card.Body gap="3">
            <Flex justify="space-between" align="center" gap="2" wrap="wrap">
              <Badge colorPalette="ochre" variant="subtle" size="md">
                {serviceName(t, r.service)}
              </Badge>
              <Text color="ink.500" fontSize="xs">
                {t('common.postedAt', { date: formatDate(r.createdAt) })}
              </Text>
            </Flex>
            <Text color="ink.500" fontSize="sm">
              {r.area}
            </Text>
            <Text>{r.description}</Text>
            {visitsOf(r.id)[0] && <CustomerVisit visit={visitsOf(r.id)[0]} onChange={reload} />}
            <EarlierReports visits={visitsOf(r.id).slice(1)} onChange={reload} />
            <Box borderTopWidth="1px" borderColor="ink.100" pt="3">
              {r.responses.length === 0 ? (
                <Text color="ink.500" fontSize="sm">
                  {t('customer.noResponses')}
                </Text>
              ) : (
                <Stack gap="4">
                  <Text fontWeight="semibold" fontSize="sm">
                    {t('customer.responses', { count: r.responses.length })}
                  </Text>
                  {r.responses.map((resp) => (
                    <Stack key={resp.id.toString()} gap="2">
                      <Text fontWeight="semibold">{resp.repairmanName}</Text>
                      <Text fontSize="sm">{resp.message}</Text>
                      <WhatsAppButton
                        phone={resp.repairmanPhone}
                        text={t('customer.replyText', { name: resp.repairmanName, service: serviceName(t, r.service) })}
                      />
                      {visits && !visitsOf(r.id).some(isActive) && (
                        <ScheduleVisit requestId={r.id} repairmanId={resp.repairmanId} onDone={reload} />
                      )}
                    </Stack>
                  ))}
                </Stack>
              )}
            </Box>
          </Card.Body>
        </Card.Root>
      ))}
    </Stack>
  )
}

function NewRequestForm({ onPosted }: { onPosted: () => void }) {
  const { t } = useTranslation()
  const [service, setService] = useState('')
  const [area, setArea] = useState('')
  const [description, setDescription] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [sent, setSent] = useState(false)

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPending(true)
    setError(null)
    setSent(false)
    try {
      await repairRequests.createRepairRequest({ service, area: area.trim(), description: description.trim() })
      setDescription('')
      setSent(true)
      onPosted()
    } catch (err) {
      setError(errorCode(err))
    } finally {
      setPending(false)
    }
  }

  return (
    <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="2xl" p="5">
      <form onSubmit={submit}>
        <Stack gap="4">
          <Heading as="h2" size="md">
            {t('customer.newRequest')}
          </Heading>
          <ServiceSelect value={service} onChange={setService} />
          <Field.Root required>
            <Field.Label>{t('common.area')}</Field.Label>
            <Input
              size="lg"
              value={area}
              onChange={(e) => setArea(e.currentTarget.value)}
              placeholder={t('common.areaPlaceholder')}
              minLength={2}
              maxLength={80}
              required
            />
          </Field.Root>
          <Field.Root required>
            <Field.Label>{t('customer.description')}</Field.Label>
            <Textarea
              size="lg"
              rows={4}
              value={description}
              onChange={(e) => setDescription(e.currentTarget.value)}
              placeholder={t('customer.descriptionPlaceholder')}
              minLength={10}
              maxLength={1000}
              required
            />
          </Field.Root>
          <ErrorAlert code={error} />
          <SuccessAlert text={sent ? t('customer.requestSent') : null} />
          <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
            {t('customer.submitRequest')}
          </Button>
        </Stack>
      </form>
    </Box>
  )
}
