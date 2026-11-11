import { Box, Button, Field, Heading, HStack, Input, Separator, Spinner, Stack, Tabs, Text } from '@chakra-ui/react'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { accounts, errorCode } from './api'
import { ErrorAlert, PhoneField, SuccessAlert } from './components'
import { Role } from './gen/haitukang/v1/account_pb'
import { loadGoogle } from './google'
import type { ErrorCode } from './i18n/id'
import { countryCode, fullPhone, localPhone } from './phone'
import { TermsAgree } from './terms/TermsAgree'
import { TERMS_VERSION } from './terms/version'

// A Google account without a Hai Tukang account, signing up.
type GoogleNewcomer = { idToken: string; name: string; email: string }

// AuthScreen signs up (acceptance HT-001, HT-002, HT-009, HT-013, HT-014) or
// in, with a password or with Google. Each signs the browser in; onSignedIn
// then loads the account.
export function AuthScreen({ onSignedIn }: { onSignedIn: () => void }) {
  const { t } = useTranslation()
  const [tab, setTab] = useState('register')
  const [googleClientId, setGoogleClientId] = useState('')
  const [newcomer, setNewcomer] = useState<GoogleNewcomer | null>(null)
  const [googlePending, setGooglePending] = useState(false)
  const [googleError, setGoogleError] = useState<ErrorCode | null>(null)
  useEffect(() => {
    accounts.getSignInOptions({}).then(
      (options) => setGoogleClientId(options.googleClientId),
      () => setGoogleClientId(''),
    )
  }, [])

  // A Google account with an account is signed in; one without signs up.
  const withGoogle = async (idToken: string) => {
    setGooglePending(true)
    setGoogleError(null)
    try {
      const res = await accounts.signInWithGoogle({ idToken })
      if (res.registered) {
        onSignedIn()
        return
      }
      setNewcomer({ idToken, name: res.fullName, email: res.email })
    } catch (err) {
      setGoogleError(errorCode(err))
    }
    setGooglePending(false)
  }

  if (newcomer) {
    return (
      <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="2xl" p={{ base: '5', sm: '8' }}>
        <GoogleSignUp newcomer={newcomer} onSignedIn={onSignedIn} onCancel={() => setNewcomer(null)} />
      </Box>
    )
  }
  return (
    <Stack gap="6">
      <Box>
        <Heading as="h1" size="2xl" letterSpacing="-0.03em">
          {t('auth.title')}
        </Heading>
        <Text mt="3" color="ink.500" lineHeight="1.65">
          {tab === 'register' ? t('auth.intro') : t('auth.loginIntro')}
        </Text>
      </Box>
      <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="2xl" p={{ base: '5', sm: '8' }}>
        {googleClientId && (
          <Stack gap="4" mb="5">
            {googlePending ? (
              <HStack justify="center" minH="11" color="ink.500" gap="3">
                <Spinner size="sm" />
                <Text>{t('app.loading')}</Text>
              </HStack>
            ) : (
              <GoogleButton clientId={googleClientId} onCredential={(token) => void withGoogle(token)} />
            )}
            <ErrorAlert code={googleError} />
            <HStack gap="3">
              <Separator flex="1" />
              <Text color="ink.500" fontSize="sm">
                {t('auth.or')}
              </Text>
              <Separator flex="1" />
            </HStack>
          </Stack>
        )}
        <Tabs.Root value={tab} onValueChange={(e) => setTab(e.value)} fitted colorPalette="ink">
          <Tabs.List mb="5">
            <Tabs.Trigger value="register">{t('auth.tabRegister')}</Tabs.Trigger>
            <Tabs.Trigger value="login">{t('auth.tabLogin')}</Tabs.Trigger>
          </Tabs.List>
          <Tabs.Content value="register">
            <RegisterForm onSignedIn={onSignedIn} />
          </Tabs.Content>
          <Tabs.Content value="login">
            <LoginForm onSignedIn={onSignedIn} />
          </Tabs.Content>
        </Tabs.Root>
      </Box>
    </Stack>
  )
}

// GoogleButton is Google's own "Continue with Google" button, in the
// reader's language. Some apps' built-in browsers block it: the phone and
// password forms stay for them.
function GoogleButton({ clientId, onCredential }: { clientId: string; onCredential: (idToken: string) => void }) {
  const { t, i18n } = useTranslation()
  const box = useRef<HTMLDivElement>(null)
  const latest = useRef(onCredential)
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    latest.current = onCredential
  })
  useEffect(() => {
    let cancelled = false
    loadGoogle().then(
      (google) => {
        const el = box.current
        if (cancelled || !el) return
        google.initialize({ client_id: clientId, callback: (res) => latest.current(res.credential), ux_mode: 'popup' })
        el.replaceChildren()
        google.renderButton(el, {
          type: 'standard',
          theme: 'outline',
          size: 'large',
          text: 'continue_with',
          shape: 'pill',
          width: Math.max(200, Math.min(400, el.offsetWidth)),
          locale: i18n.resolvedLanguage,
        })
      },
      () => {
        if (!cancelled) setFailed(true)
      },
    )
    return () => {
      cancelled = true
    }
  }, [clientId, i18n.resolvedLanguage])
  if (failed) {
    return (
      <Text color="ink.500" fontSize="sm" textAlign="center">
        {t('auth.googleUnavailable')}
      </Text>
    )
  }
  return <Box ref={box} minH="11" display="flex" justifyContent="center" data-testid="google-button" />
}

// RoleChoice picks customer or repairman.
function RoleChoice({ role, onRole }: { role: Role; onRole: (role: Role) => void }) {
  const { t } = useTranslation()
  return (
    <Box>
      <Text fontWeight="semibold" mb="3">
        {t('auth.rolePrompt')}
      </Text>
      <HStack gap="3">
        {[Role.CUSTOMER, Role.REPAIRMAN].map((value) => (
          <Button
            key={value}
            type="button"
            flex="1"
            size="lg"
            colorPalette="ink"
            variant={role === value ? 'solid' : 'outline'}
            aria-pressed={role === value}
            onClick={() => onRole(value)}
          >
            {value === Role.CUSTOMER ? t('auth.roleCustomer') : t('auth.roleRepairman')}
          </Button>
        ))}
      </HStack>
    </Box>
  )
}

// GoogleSignUp makes the account of a Google account (HT-013): the role, a
// name (Google's, editable) and the phone with its WhatsApp code, as any
// sign-up, and the terms; no password.
function GoogleSignUp({ newcomer, onSignedIn, onCancel }: { newcomer: GoogleNewcomer; onSignedIn: () => void; onCancel: () => void }) {
  const { t } = useTranslation()
  const [role, setRole] = useState<Role>(Role.CUSTOMER)
  const [fullName, setFullName] = useState(newcomer.name)
  const [phone, setPhone] = useState('')
  const [agreed, setAgreed] = useState(false)
  const [step, setStep] = useState<'details' | 'code'>('details')
  const [code, setCode] = useState('')
  const [resendAt, setResendAt] = useState(0)
  const [resent, setResent] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)
  const resendIn = useSecondsUntil(resendAt)

  const sendCode = async () => {
    const res = await accounts.sendSignUpCode({ phone: fullPhone(localPhone(phone)) })
    setResendAt(Date.now() + res.resendAfterSeconds * 1000)
  }

  const submitDetails = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPhone(localPhone(phone))
    if (!agreed) {
      setError('terms_not_accepted')
      return
    }
    setPending(true)
    setError(null)
    try {
      await sendCode()
      setCode('')
      setResent(false)
      setStep('code')
    } catch (err) {
      setError(errorCode(err))
    } finally {
      setPending(false)
    }
  }

  const submitCode = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPending(true)
    setError(null)
    setResent(false)
    try {
      await accounts.registerWithGoogle({
        idToken: newcomer.idToken,
        role,
        fullName: fullName.trim(),
        phone: fullPhone(localPhone(phone)),
        code,
        acceptedTerms: TERMS_VERSION,
      })
      onSignedIn()
    } catch (err) {
      setError(errorCode(err))
      setPending(false)
    }
  }

  const resend = async () => {
    setError(null)
    setResent(false)
    try {
      await sendCode()
      setCode('')
      setResent(true)
    } catch (err) {
      setError(errorCode(err))
    }
  }

  if (step === 'code') {
    return (
      <CodeStep
        phone={phone}
        code={code}
        onCode={setCode}
        submitLabel={role === Role.CUSTOMER ? t('auth.submitCustomer') : t('auth.submitRepairman')}
        pending={pending}
        error={error}
        resent={resent}
        resendIn={resendIn}
        onSubmit={submitCode}
        onBack={() => {
          setStep('details')
          setError(null)
        }}
        onResend={() => void resend()}
      />
    )
  }

  return (
    <form onSubmit={submitDetails} data-testid="google-signup">
      <Stack gap="4">
        <Box>
          <Heading as="h2" size="lg">
            {t('auth.googleTitle')}
          </Heading>
          <Text mt="2" color="ink.500" lineHeight="1.6" wordBreak="break-word">
            {t('auth.googleIntro', { email: newcomer.email })}
          </Text>
        </Box>
        <RoleChoice role={role} onRole={setRole} />
        <Field.Root required>
          <Field.Label>{t('auth.fullName')}</Field.Label>
          <Input
            size="lg"
            autoComplete="name"
            value={fullName}
            onChange={(event) => setFullName(event.currentTarget.value)}
            placeholder={t('auth.fullNamePlaceholder')}
            minLength={2}
            maxLength={80}
            required
          />
        </Field.Root>
        <PhoneField value={phone} onChange={setPhone} />
        <TermsAgree checked={agreed} onChange={setAgreed} />
        <ErrorAlert code={error} />
        <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
          {t('auth.sendCode')}
        </Button>
        <Button type="button" size="lg" variant="ghost" colorPalette="ink" onClick={onCancel}>
          {t('auth.googleCancel')}
        </Button>
      </Stack>
    </form>
  )
}

// RegisterForm takes the details, sends a code to the phone on WhatsApp,
// then signs up with that code (HT-009). Going back to the details keeps
// what was typed.
function RegisterForm({ onSignedIn }: { onSignedIn: () => void }) {
  const { t } = useTranslation()
  const [role, setRole] = useState<Role>(Role.CUSTOMER)
  const [fullName, setFullName] = useState('')
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [agreed, setAgreed] = useState(false)
  const [step, setStep] = useState<'details' | 'code'>('details')
  const [code, setCode] = useState('')
  const [resendAt, setResendAt] = useState(0)
  const [resent, setResent] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)
  const resendIn = useSecondsUntil(resendAt)

  const sendCode = async () => {
    const res = await accounts.sendSignUpCode({ phone: fullPhone(localPhone(phone)) })
    setResendAt(Date.now() + res.resendAfterSeconds * 1000)
  }

  // No code is sent before the terms are agreed to (HT-014).
  const submitDetails = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPhone(localPhone(phone))
    if (!agreed) {
      setError('terms_not_accepted')
      return
    }
    setPending(true)
    setError(null)
    try {
      await sendCode()
      setCode('')
      setResent(false)
      setStep('code')
    } catch (err) {
      setError(errorCode(err))
    } finally {
      setPending(false)
    }
  }

  const submitCode = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPending(true)
    setError(null)
    setResent(false)
    try {
      await accounts.register({ role, fullName: fullName.trim(), phone: fullPhone(localPhone(phone)), password, code, acceptedTerms: TERMS_VERSION })
      onSignedIn()
    } catch (err) {
      setError(errorCode(err))
      setPending(false)
    }
  }

  const resend = async () => {
    setError(null)
    setResent(false)
    try {
      await sendCode()
      setCode('')
      setResent(true)
    } catch (err) {
      setError(errorCode(err))
    }
  }

  if (step === 'code') {
    return (
      <CodeStep
        phone={phone}
        code={code}
        onCode={setCode}
        submitLabel={role === Role.CUSTOMER ? t('auth.submitCustomer') : t('auth.submitRepairman')}
        pending={pending}
        error={error}
        resent={resent}
        resendIn={resendIn}
        onSubmit={submitCode}
        onBack={() => {
          setStep('details')
          setError(null)
        }}
        onResend={() => void resend()}
      />
    )
  }

  return (
    <form onSubmit={submitDetails}>
      <Stack gap="4">
        <RoleChoice role={role} onRole={setRole} />
        <Field.Root required>
          <Field.Label>{t('auth.fullName')}</Field.Label>
          <Input
            size="lg"
            autoComplete="name"
            value={fullName}
            onChange={(event) => setFullName(event.currentTarget.value)}
            placeholder={t('auth.fullNamePlaceholder')}
            minLength={2}
            maxLength={80}
            required
          />
        </Field.Root>
        <PhoneField value={phone} onChange={setPhone} />
        <Field.Root required>
          <Field.Label>{t('auth.password')}</Field.Label>
          <Input
            size="lg"
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.currentTarget.value)}
            minLength={8}
            placeholder={t('auth.passwordPlaceholder')}
            required
          />
        </Field.Root>
        <TermsAgree checked={agreed} onChange={setAgreed} />
        <ErrorAlert code={error} />
        <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
          {t('auth.sendCode')}
        </Button>
      </Stack>
    </form>
  )
}

// LoginForm signs in with phone and password. An admin then gets a code on
// WhatsApp and signs in with it too, every time.
function LoginForm({ onSignedIn }: { onSignedIn: () => void }) {
  const { t } = useTranslation()
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [step, setStep] = useState<'password' | 'code'>('password')
  const [code, setCode] = useState('')
  const [resendAt, setResendAt] = useState(0)
  const [resent, setResent] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)
  const resendIn = useSecondsUntil(resendAt)

  // login signs in, or, for an admin without a code, has one sent and
  // answers true.
  const login = async (withCode: string) => {
    const res = await accounts.login({ phone: fullPhone(localPhone(phone)), password, code: withCode })
    if (res.codeSent) {
      setResendAt(Date.now() + res.resendAfterSeconds * 1000)
      return true
    }
    onSignedIn()
    return false
  }

  const submitPassword = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPhone(localPhone(phone))
    setPending(true)
    setError(null)
    try {
      if (await login('')) {
        setCode('')
        setResent(false)
        setStep('code')
        setPending(false)
      }
    } catch (err) {
      setError(errorCode(err))
      setPending(false)
    }
  }

  const submitCode = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPending(true)
    setError(null)
    setResent(false)
    try {
      await login(code)
    } catch (err) {
      setError(errorCode(err))
      setPending(false)
    }
  }

  const resend = async () => {
    setError(null)
    setResent(false)
    try {
      await login('')
      setCode('')
      setResent(true)
    } catch (err) {
      setError(errorCode(err))
    }
  }

  if (step === 'code') {
    return (
      <CodeStep
        phone={phone}
        code={code}
        onCode={setCode}
        submitLabel={t('auth.submitLogin')}
        pending={pending}
        error={error}
        resent={resent}
        resendIn={resendIn}
        onSubmit={submitCode}
        onBack={() => {
          setStep('password')
          setError(null)
        }}
        onResend={() => void resend()}
      />
    )
  }

  return (
    <form onSubmit={submitPassword}>
      <Stack gap="4">
        <PhoneField value={phone} onChange={setPhone} />
        <Field.Root required>
          <Field.Label>{t('auth.password')}</Field.Label>
          <Input
            size="lg"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.currentTarget.value)}
            required
          />
        </Field.Root>
        <ErrorAlert code={error} />
        <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
          {t('auth.submitLogin')}
        </Button>
      </Stack>
    </form>
  )
}

// CodeStep asks for the 6-digit code sent to phone on WhatsApp.
function CodeStep(props: {
  phone: string
  code: string
  onCode: (code: string) => void
  submitLabel: string
  pending: boolean
  error: ErrorCode | null
  resent: boolean
  resendIn: number
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onBack: () => void
  onResend: () => void
}) {
  const { t } = useTranslation()
  return (
    <form onSubmit={props.onSubmit}>
      <Stack gap="4">
        <Text>{t('auth.codeSent', { phone: `${countryCode} ${localPhone(props.phone)}` })}</Text>
        <Field.Root required>
          <Field.Label>{t('auth.code')}</Field.Label>
          {/* Digits only, cut to 6 after cleaning, so a pasted "Kode: 123 456" works. */}
          <Input
            size="lg"
            inputMode="numeric"
            autoComplete="one-time-code"
            pattern="[0-9]{6}"
            value={props.code}
            onChange={(event) => props.onCode(event.currentTarget.value.replace(/\D/g, '').slice(0, 6))}
            fontSize="xl"
            letterSpacing="0.3em"
            autoFocus
            required
          />
        </Field.Root>
        <ErrorAlert code={props.error} />
        <SuccessAlert text={props.resent ? t('auth.codeResent') : null} />
        <Button type="submit" size="lg" colorPalette="ink" loading={props.pending}>
          {props.submitLabel}
        </Button>
        <HStack justify="space-between" gap="2" wrap="wrap">
          <Button type="button" variant="ghost" size="sm" minH="11" colorPalette="ink" onClick={props.onBack}>
            {t('auth.editDetails')}
          </Button>
          <Button type="button" variant="ghost" size="sm" minH="11" colorPalette="ink" disabled={props.resendIn > 0} onClick={props.onResend}>
            {props.resendIn > 0 ? t('auth.resendIn', { seconds: props.resendIn }) : t('auth.resend')}
          </Button>
        </HStack>
      </Stack>
    </form>
  )
}

// useSecondsUntil counts down, once a second, to a time (ms since epoch).
// It reads the clock on every render: a time kept in state would be stale
// when `at` changes, and the count would start above the real wait.
function useSecondsUntil(at: number) {
  const [, tick] = useState(0)
  const left = Math.max(0, Math.ceil((at - Date.now()) / 1000))
  useEffect(() => {
    if (left <= 0) return
    const id = window.setTimeout(() => tick((n) => n + 1), 1000)
    return () => window.clearTimeout(id)
  })
  return left
}
