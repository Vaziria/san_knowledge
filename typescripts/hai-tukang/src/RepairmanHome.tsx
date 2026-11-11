import { Alert, Badge, Box, Button, Card, Field, Flex, Input, Stack, Tabs, Text, Textarea } from '@chakra-ui/react'
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { accounts, errorCode, repairRequests } from './api'
import { serviceName, services } from './catalog'
import { ErrorAlert, ServiceSelect, SuccessAlert, useFormatDate, WhatsAppButton } from './components'
import type { Account, Profile } from './gen/haitukang/v1/account_pb'
import type { RepairRequest } from './gen/haitukang/v1/repair_request_pb'
import type { ErrorCode } from './i18n/id'
import { RepairmanVisits } from './Visits'

// RepairmanHome: describe his work so customers find him (HT-003), and
// respond to requests (HT-004). Without a profile he starts on it.
export function RepairmanHome({
  account,
  profile,
  onProfileSaved,
}: {
  account: Account
  profile: Profile | undefined
  onProfileSaved: (profile: Profile) => void
}) {
  const { t } = useTranslation()
  const [tab, setTab] = useState(profile ? 'requests' : 'profile')
  return (
    <Stack gap="5">
      {!profile && (
        <Alert.Root status="info" role="status">
          <Alert.Indicator />
          <Alert.Title>{t('repairman.profileNeeded')}</Alert.Title>
        </Alert.Root>
      )}
      <Tabs.Root value={tab} onValueChange={(e) => setTab(e.value)} fitted colorPalette="ink" lazyMount>
        <Tabs.List mb="5">
          <Tabs.Trigger value="requests">{t('repairman.tabRequests')}</Tabs.Trigger>
          <Tabs.Trigger value="visits">{t('visits.tab')}</Tabs.Trigger>
          <Tabs.Trigger value="profile">{t('repairman.tabProfile')}</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="requests">
          <OpenRequests account={account} defaultService={profile?.services.length === 1 ? profile.services[0] : ''} />
        </Tabs.Content>
        <Tabs.Content value="visits">
          <RepairmanVisits />
        </Tabs.Content>
        <Tabs.Content value="profile">
          <ProfileForm profile={profile} onSaved={onProfileSaved} />
        </Tabs.Content>
      </Tabs.Root>
    </Stack>
  )
}

function ProfileForm({ profile, onSaved }: { profile: Profile | undefined; onSaved: (p: Profile) => void }) {
  const { t } = useTranslation()
  const [chosen, setChosen] = useState<string[]>(profile?.services ?? [])
  const [area, setArea] = useState(profile?.area ?? '')
  const [about, setAbout] = useState(profile?.about ?? '')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [saved, setSaved] = useState(false)

  const toggle = (code: string) =>
    setChosen((list) => (list.includes(code) ? list.filter((c) => c !== code) : [...list, code]))

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPending(true)
    setError(null)
    setSaved(false)
    try {
      const res = await accounts.saveProfile({ profile: { services: chosen, area: area.trim(), about: about.trim() } })
      setSaved(true)
      if (res.profile) onSaved(res.profile)
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
          <Box role="group" aria-label={t('repairman.services')}>
            <Text fontWeight="semibold" mb="3">
              {t('repairman.services')}
            </Text>
            <Flex wrap="wrap" gap="2">
              {services.map((code) => (
                <Button
                  key={code}
                  type="button"
                  size="md"
                  minH="11"
                  colorPalette="ink"
                  variant={chosen.includes(code) ? 'solid' : 'outline'}
                  aria-pressed={chosen.includes(code)}
                  onClick={() => toggle(code)}
                >
                  {t(`services.${code}`)}
                </Button>
              ))}
            </Flex>
          </Box>
          <Field.Root required>
            <Field.Label>{t('repairman.area')}</Field.Label>
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
          <Field.Root>
            <Field.Label>{t('repairman.about')}</Field.Label>
            <Textarea
              size="lg"
              rows={3}
              value={about}
              onChange={(e) => setAbout(e.currentTarget.value)}
              placeholder={t('repairman.aboutPlaceholder')}
              maxLength={500}
            />
          </Field.Root>
          <ErrorAlert code={error} />
          <SuccessAlert text={saved ? t('repairman.profileSaved') : null} />
          <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
            {t('repairman.saveProfile')}
          </Button>
        </Stack>
      </form>
    </Box>
  )
}

function OpenRequests({ account, defaultService }: { account: Account; defaultService: string }) {
  const { t } = useTranslation()
  const formatDate = useFormatDate()
  const [service, setService] = useState(defaultService)
  const [list, setList] = useState<RepairRequest[] | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)

  const load = useCallback(async (service: string) => {
    try {
      setList((await repairRequests.listOpenRepairRequests({ service })).requests)
      setError(null)
    } catch (err) {
      setError(errorCode(err))
    }
  }, [])
  useEffect(() => void load(service), [load, service])

  return (
    <Stack gap="5">
      <ServiceSelect value={service} onChange={setService} allowAll />
      <ErrorAlert code={error} />
      {list && list.length === 0 && <Text color="ink.500">{t('repairman.noOpen')}</Text>}
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
              {t('repairman.from', { name: r.customerName })} · {r.area}
            </Text>
            <Text>{r.description}</Text>
            {r.responded ? (
              <Stack gap="2" borderTopWidth="1px" borderColor="ink.100" pt="3">
                <Text color="green.700" fontSize="sm" fontWeight="semibold">
                  {t('repairman.responded')}
                </Text>
                {r.customerPhone && (
                  <WhatsAppButton
                    phone={r.customerPhone}
                    text={t('repairman.contactText', { name: r.customerName, me: account.fullName, service: serviceName(t, r.service) })}
                  />
                )}
              </Stack>
            ) : (
              <RespondForm requestId={r.id} onResponded={() => void load(service)} />
            )}
          </Card.Body>
        </Card.Root>
      ))}
    </Stack>
  )
}

function RespondForm({ requestId, onResponded }: { requestId: bigint; onResponded: () => void }) {
  const { t } = useTranslation()
  const [message, setMessage] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPending(true)
    setError(null)
    try {
      await repairRequests.respondToRepairRequest({ requestId, message: message.trim() })
      onResponded()
    } catch (err) {
      setError(errorCode(err))
      setPending(false)
    }
  }

  return (
    <form onSubmit={submit}>
      <Stack gap="3" borderTopWidth="1px" borderColor="ink.100" pt="3">
        <Textarea
          size="lg"
          rows={2}
          value={message}
          onChange={(e) => setMessage(e.currentTarget.value)}
          placeholder={t('repairman.responsePlaceholder')}
          aria-label={t('repairman.respond')}
          minLength={2}
          maxLength={500}
          required
        />
        <ErrorAlert code={error} />
        <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
          {t('repairman.respond')}
        </Button>
      </Stack>
    </form>
  )
}
