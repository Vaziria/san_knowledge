import { Alert, Box, Button, Heading, List, Stack, Text } from '@chakra-ui/react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import type { Language } from '../i18n'
import { terms } from './content'

// TermsPage is the Terms and Policies (acceptance HT-014), in the reader's
// language, open to anyone.
export function TermsPage({ onBack }: { onBack: () => void }) {
  const { t, i18n } = useTranslation()
  const text = terms[(i18n.resolvedLanguage as Language | undefined) ?? 'id'] ?? terms.id
  useEffect(() => {
    const before = document.title
    document.title = `${text.title} · Hai Tukang`
    return () => {
      document.title = before
    }
  }, [text.title])
  return (
    <Stack as="article" gap="6" data-testid="terms-page">
      <Box>
        <Heading as="h1" size="2xl" letterSpacing="-0.03em">
          {text.title}
        </Heading>
        <Text mt="2" color="ink.500" fontSize="sm">
          {text.updated}
        </Text>
      </Box>
      <Alert.Root status="warning" variant="subtle">
        <Alert.Indicator />
        <Alert.Description>{text.draft}</Alert.Description>
      </Alert.Root>
      <Text lineHeight="1.7">{text.intro}</Text>
      {text.sections.map((section) => (
        <Stack as="section" key={section.heading} gap="3">
          <Heading as="h2" size="lg">
            {section.heading}
          </Heading>
          {section.blocks.map((block, i) =>
            typeof block === 'string' ? (
              <Text key={i} lineHeight="1.7">
                {block}
              </Text>
            ) : (
              <List.Root key={i} gap="2" ps="5" lineHeight="1.7">
                {block.map((item) => (
                  <List.Item key={item}>{item}</List.Item>
                ))}
              </List.Root>
            ),
          )}
        </Stack>
      ))}
      <Button size="lg" colorPalette="ink" variant="outline" onClick={onBack}>
        {t('terms.back')}
      </Button>
    </Stack>
  )
}
