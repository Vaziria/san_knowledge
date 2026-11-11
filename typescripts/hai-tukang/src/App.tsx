import { Badge, Box, Button, Group, HStack, Link, Spinner, Stack, Text } from '@chakra-ui/react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { accounts, errorCode, signedOutEvent, termsEvent } from './api'
import { AdminHome } from './AdminHome'
import { AuthScreen } from './AuthScreen'
import { ErrorAlert } from './components'
import { CustomerHome } from './CustomerHome'
import { Role, type Account, type Profile } from './gen/haitukang/v1/account_pb'
import { languages, type Language } from './i18n'
import type { ErrorCode } from './i18n/id'
import { forgetPush, keepPush } from './push'
import { PushPrompt } from './PushPrompt'
import { RepairmanHome } from './RepairmanHome'
import { termsHash, useHash } from './route'
import { Standing } from './Standing'
import { TermsGate } from './terms/TermsGate'
import { TermsPage } from './terms/TermsPage'

type Session =
  | { state: 'loading' }
  | { state: 'signedOut'; error?: ErrorCode }
  | { state: 'signedIn'; account: Account; profile?: Profile }

export default function App() {
  const { t, i18n } = useTranslation()
  const [session, setSession] = useState<Session>({ state: 'loading' })

  const load = useCallback(async () => {
    try {
      const me = await accounts.getMe({})
      setSession(me.account ? { state: 'signedIn', account: me.account, profile: me.profile } : { state: 'signedOut' })
      // This device's notifications follow whoever is signed in here.
      if (me.account) void keepPush(i18n.resolvedLanguage ?? 'id')
    } catch (err) {
      const code = errorCode(err)
      setSession({ state: 'signedOut', error: code === 'unauthorized' ? undefined : code })
    }
  }, [i18n])
  useEffect(() => void load(), [load])
  useEffect(() => {
    const changed = (language: string) => {
      if (session.state === 'signedIn') void keepPush(language)
    }
    i18n.on('languageChanged', changed)
    return () => i18n.off('languageChanged', changed)
  }, [i18n, session.state])
  useEffect(() => {
    const signedOut = () => setSession({ state: 'signedOut' })
    window.addEventListener(signedOutEvent, signedOut)
    return () => window.removeEventListener(signedOutEvent, signedOut)
  }, [])
  useEffect(() => {
    const changed = () => void load()
    window.addEventListener(termsEvent, changed)
    return () => window.removeEventListener(termsEvent, changed)
  }, [load])

  // The Terms and Policies show over the app, which stays as it was (a
  // half-filled sign-up too); leaving them returns to it, scrolled back.
  const hash = useHash()
  const showTerms = hash === termsHash
  const previousHash = useRef(hash)
  const cameFromApp = useRef(false)
  const scrolled = useRef(0)
  useEffect(() => {
    const before = previousHash.current
    previousHash.current = hash
    if (hash === before) return
    if (hash === termsHash) {
      cameFromApp.current = true
      scrolled.current = window.scrollY
      window.scrollTo(0, 0)
    } else if (before === termsHash) {
      window.requestAnimationFrame(() => window.scrollTo(0, scrolled.current))
    }
  }, [hash])
  const leaveTerms = () => {
    if (cameFromApp.current) {
      window.history.back()
      return
    }
    window.history.replaceState(null, '', window.location.pathname + window.location.search)
    window.dispatchEvent(new HashChangeEvent('hashchange'))
  }
  const needsTerms = session.state === 'signedIn' && !session.account.termsAccepted

  const signOut = async () => {
    await forgetPush()
    await accounts.logout({}).catch(() => undefined)
    setSession({ state: 'signedOut' })
  }

  return (
    <Box as="main" minH="100vh" px={{ base: '4', sm: '6' }} py={{ base: '5', md: '12' }}>
      <Stack maxW="md" mx="auto" gap="6">
        <Header session={session} onSignOut={() => void signOut()} />
        {showTerms && <TermsPage onBack={leaveTerms} />}
        <Stack gap="6" hidden={showTerms}>
        {session.state === 'loading' && (
          <HStack color="ink.500" gap="3" py="10" justify="center">
            <Spinner size="sm" />
            <Text>{t('app.loading')}</Text>
          </HStack>
        )}
        {session.state === 'signedOut' && (
          <>
            <ErrorAlert code={session.error ?? null} />
            <AuthScreen onSignedIn={() => void load()} />
          </>
        )}
        {needsTerms && <TermsGate onAgreed={() => void load()} />}
        {session.state === 'signedIn' && !needsTerms && session.account.role !== Role.ADMIN && <Standing role={session.account.role} />}
        {session.state === 'signedIn' && !needsTerms && session.account.role !== Role.ADMIN && <PushPrompt />}
        {session.state === 'signedIn' && !needsTerms && session.account.role === Role.ADMIN && <AdminHome />}
        {session.state === 'signedIn' &&
          !needsTerms &&
          session.account.role !== Role.ADMIN &&
          (session.account.role === Role.REPAIRMAN ? (
            <RepairmanHome
              account={session.account}
              profile={session.profile}
              onProfileSaved={(profile) => setSession({ ...session, profile })}
            />
          ) : (
            <CustomerHome />
          ))}
        </Stack>
        <Box as="footer" textAlign="center" pt="2" pb="4" borderTopWidth="1px" borderColor="ink.100">
          <Link href={termsHash} color="ink.600" fontSize="sm" minH="11" display="inline-flex" alignItems="center" data-testid="terms-link">
            {t('app.terms')}
          </Link>
        </Box>
      </Stack>
    </Box>
  )
}

function Header({ session, onSignOut }: { session: Session; onSignOut: () => void }) {
  const { t, i18n } = useTranslation()
  return (
    <Stack gap="3">
      <HStack justify="space-between">
        <Text color="ochre.700" fontSize="xs" fontWeight="bold" letterSpacing="0.16em">
          {t('app.brand')}
        </Text>
        <Group attached aria-label={t('app.language')}>
          {(Object.keys(languages) as Language[]).map((code) => (
            <Button
              key={code}
              size="sm"
              minW="11"
              minH="11"
              colorPalette="ink"
              variant={i18n.resolvedLanguage === code ? 'solid' : 'outline'}
              aria-pressed={i18n.resolvedLanguage === code}
              lang={code}
              onClick={() => void i18n.changeLanguage(code)}
            >
              {languages[code]}
            </Button>
          ))}
        </Group>
      </HStack>
      {session.state === 'signedIn' && (
        <HStack justify="space-between" gap="3">
          <HStack gap="2" minW="0">
            <Text fontWeight="semibold" truncate>
              {session.account.fullName}
            </Text>
            <Badge colorPalette="ink" variant="subtle">
              {session.account.role === Role.ADMIN
                ? t('roles.admin')
                : session.account.role === Role.REPAIRMAN
                  ? t('roles.repairman')
                  : t('roles.customer')}
            </Badge>
          </HStack>
          <Button size="sm" minH="11" variant="ghost" colorPalette="ink" onClick={onSignOut}>
            {t('app.signOut')}
          </Button>
        </HStack>
      )}
    </Stack>
  )
}
