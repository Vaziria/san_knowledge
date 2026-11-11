import { Badge, Box, Button, chakra, Field, Flex, Heading, Image, Input, Stack, Text } from '@chakra-ui/react'
import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { errorCode, storeClient } from './api'
import { serviceName } from './catalog'
import { ErrorAlert, ServiceSelect, WhatsAppButton } from './components'
import type { Product, ProductListing, ProductPhoto } from './gen/haitukang/v1/store_pb'
import type { ErrorCode } from './i18n/id'
import { useFormatRupiah } from './money'

// Shop: find repair materials partner stores sell (acceptance HT-015), by
// words, kind of work and area, and ask the store on WhatsApp. Orders come
// later (HT-016); payment is between the buyer and the store.
export function Shop() {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const [service, setService] = useState('')
  const [area, setArea] = useState('')
  const [list, setList] = useState<ProductListing[] | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [pending, setPending] = useState(false)
  const [open, setOpen] = useState<ProductListing | null>(null)
  const scrolled = useRef(0)

  const search = useCallback(async (query: string, service: string, area: string) => {
    setPending(true)
    setError(null)
    try {
      setList((await storeClient.searchProducts({ query, service, area })).listings)
    } catch (err) {
      setError(errorCode(err))
    } finally {
      setPending(false)
    }
  }, [])
  useEffect(() => void search('', '', ''), [search])

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    void search(query.trim(), service, area.trim())
  }
  const show = (listing: ProductListing) => {
    scrolled.current = window.scrollY
    setOpen(listing)
    window.scrollTo(0, 0)
  }
  const back = () => {
    setOpen(null)
    window.requestAnimationFrame(() => window.scrollTo(0, scrolled.current))
  }

  if (open) return <ProductDetail listing={open} onBack={back} />
  return (
    <Stack gap="5">
      <form onSubmit={submit}>
        <Stack gap="4">
          <Field.Root>
            <Field.Label>{t('shop.query')}</Field.Label>
            <Input size="lg" type="search" value={query} onChange={(e) => setQuery(e.currentTarget.value)} placeholder={t('shop.queryPlaceholder')} />
          </Field.Root>
          <ServiceSelect value={service} onChange={setService} allowAll />
          <Field.Root>
            <Field.Label>{t('common.area')}</Field.Label>
            <Input size="lg" value={area} onChange={(e) => setArea(e.currentTarget.value)} placeholder={t('common.areaPlaceholder')} />
          </Field.Root>
          <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
            {t('shop.search')}
          </Button>
        </Stack>
      </form>
      <ErrorAlert code={error} />
      {list && list.length === 0 && <Text color="ink.500">{t('shop.none')}</Text>}
      <Stack gap="3">
        {list?.map((l) => l.product && <ProductCard key={l.product.id.toString()} listing={l} onOpen={() => show(l)} />)}
      </Stack>
    </Stack>
  )
}

// PhotoBox is a product's photo, or a plain box saying it has none (or
// that it could not be loaded).
export function PhotoBox({ photo, alt, size }: { photo?: ProductPhoto; alt: string; size?: string }) {
  const { t } = useTranslation()
  const [failed, setFailed] = useState<string | null>(null)
  const shown = photo && failed !== photo.url ? photo : undefined
  return (
    <Box
      flexShrink={0}
      w={size ?? 'full'}
      h={size}
      aspectRatio={size ? undefined : 4 / 3}
      bg="ink.50"
      rounded="lg"
      overflow="hidden"
      display="flex"
      alignItems="center"
      justifyContent="center"
    >
      {shown ? (
        <Image src={shown.url} alt={alt} w="full" h="full" objectFit={size ? 'cover' : 'contain'} loading="lazy" onError={() => setFailed(shown.url)} />
      ) : (
        <Text fontSize="xs" color="ink.500" textAlign="center" px="2">
          {t('shop.noPhoto')}
        </Text>
      )}
    </Box>
  )
}

// PriceLine is "Rp 15.000 / batang".
export function PriceLine({ product, size }: { product: Product; size?: string }) {
  const { t } = useTranslation()
  const rupiah = useFormatRupiah()
  return (
    <Text fontWeight="bold" fontSize={size ?? 'md'}>
      {t('shop.perUnit', { price: rupiah(product.details?.price ?? 0n), unit: product.details?.unit ?? '' })}
    </Text>
  )
}

function ProductCard({ listing, onOpen }: { listing: ProductListing; onOpen: () => void }) {
  const { t } = useTranslation()
  const product = listing.product!
  const store = listing.store?.profile
  return (
    <chakra.button
      type="button"
      onClick={onOpen}
      textAlign="start"
      w="full"
      bg="white"
      borderWidth="1px"
      borderColor="ink.100"
      rounded="xl"
      p="3"
      _hover={{ borderColor: 'ink.300' }}
      data-testid="product-card"
    >
      <Flex gap="3" align="flex-start">
        <PhotoBox photo={product.photos[0]} alt={product.details?.name ?? ''} size="24" />
        <Stack gap="1" minW="0" flex="1">
          <Text fontWeight="semibold" lineClamp={2}>
            {product.details?.name}
          </Text>
          <PriceLine product={product} />
          <Text fontSize="sm" color="ink.600" truncate>
            {store?.name} · {store?.area}
          </Text>
          {!product.details?.available && (
            <Badge alignSelf="flex-start" colorPalette="red" variant="subtle">
              {t('shop.soldOut')}
            </Badge>
          )}
        </Stack>
      </Flex>
    </chakra.button>
  )
}

function ProductDetail({ listing, onBack }: { listing: ProductListing; onBack: () => void }) {
  const { t } = useTranslation()
  const rupiah = useFormatRupiah()
  const product = listing.product!
  const details = product.details
  const store = listing.store
  const profile = store?.profile
  return (
    <Stack gap="5" data-testid="product-detail">
      <Button alignSelf="flex-start" variant="ghost" colorPalette="ink" minH="11" px="2" onClick={onBack}>
        ← {t('shop.back')}
      </Button>
      {product.photos.length === 0 ? (
        <PhotoBox alt={details?.name ?? ''} />
      ) : (
        <Stack gap="2">
          <Flex overflowX="auto" gap="2" scrollSnapType="x mandatory" rounded="lg">
            {product.photos.map((photo, i) => (
              <Box key={photo.id.toString()} flex="0 0 100%" scrollSnapAlign="start">
                <PhotoBox photo={photo} alt={t('shop.photo', { n: i + 1, count: product.photos.length })} />
              </Box>
            ))}
          </Flex>
          {product.photos.length > 1 && (
            <Text fontSize="sm" color="ink.600">
              {t('shop.swipe')}
            </Text>
          )}
        </Stack>
      )}
      <Stack gap="2">
        <Heading as="h2" size="lg">
          {details?.name}
        </Heading>
        <PriceLine product={product} size="xl" />
        <Flex gap="2" wrap="wrap">
          <Badge colorPalette="ochre" variant="subtle" size="md">
            {serviceName(t, details?.service ?? '')}
          </Badge>
          {!details?.available && (
            <Badge colorPalette="red" variant="subtle" size="md">
              {t('shop.soldOut')}
            </Badge>
          )}
        </Flex>
        {details?.description && <Text whiteSpace="pre-wrap">{details.description}</Text>}
      </Stack>
      <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="2xl" p="5">
        <Stack gap="2">
          <Heading as="h3" size="md">
            {profile?.name}
          </Heading>
          <Text fontSize="sm" color="ink.600">
            {profile?.area} · {profile?.address}
          </Text>
          {profile?.pickup && <Text fontSize="sm">{t('shop.pickup')}</Text>}
          {profile?.delivers && (
            <Text fontSize="sm">
              {profile.deliveryFee > 0n ? t('shop.delivers', { fee: rupiah(profile.deliveryFee) }) : t('shop.deliversFree')}
              {profile.deliveryAreas && (
                <>
                  <br />
                  {t('shop.deliveryAreas', { areas: profile.deliveryAreas })}
                </>
              )}
            </Text>
          )}
          <Text fontSize="sm" color="ink.600" pt="1">
            {t('shop.payNote')}
          </Text>
          {store && (
            <Box pt="2">
              <WhatsAppButton
                phone={store.phone}
                label={t('shop.contact')}
                text={t('shop.contactText', { store: profile?.name ?? '', product: details?.name ?? '' })}
              />
            </Box>
          )}
        </Stack>
      </Box>
    </Stack>
  )
}
