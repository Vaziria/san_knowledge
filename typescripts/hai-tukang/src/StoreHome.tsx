import { Alert, Badge, Box, Button, Checkbox, Field, Flex, Heading, Input, InputGroup, SimpleGrid, Stack, Tabs, Text, Textarea } from '@chakra-ui/react'
import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { errorCode, storeClient } from './api'
import { ErrorAlert, ServiceSelect, SuccessAlert } from './components'
import type { GetMyStoreResponse, Product, StoreProfile } from './gen/haitukang/v1/store_pb'
import type { ErrorCode } from './i18n/id'
import { rupiahInput, useFormatRupiah } from './money'
import { shrinkPhoto } from './photo'
import { PhotoBox, PriceLine } from './Shop'

const maxPhotos = 5

// StoreHome: a partner store keeps its profile and products, with up to 5
// photos each (acceptance HT-015). Without a profile it starts on it.
export function StoreHome() {
  const { t } = useTranslation()
  const [mine, setMine] = useState<GetMyStoreResponse | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [tab, setTab] = useState<string | null>(null)
  const load = useCallback(async () => {
    try {
      const res = await storeClient.getMyStore({})
      setMine(res)
      setError(null)
      setTab((tab) => tab ?? (res.profile ? 'products' : 'profile'))
    } catch (err) {
      setError(errorCode(err))
    }
  }, [])
  useEffect(() => void load(), [load])

  if (!mine) return <ErrorAlert code={error} />
  return (
    <Stack gap="5">
      {!mine.profile && (
        <Alert.Root status="info" role="status">
          <Alert.Indicator />
          <Alert.Title>{t('store.profileNeeded')}</Alert.Title>
        </Alert.Root>
      )}
      <ErrorAlert code={error} />
      <Tabs.Root value={tab ?? 'products'} onValueChange={(e) => setTab(e.value)} fitted colorPalette="ink" lazyMount>
        <Tabs.List mb="5">
          <Tabs.Trigger value="products">{t('store.tabProducts')}</Tabs.Trigger>
          <Tabs.Trigger value="profile">{t('store.tabProfile')}</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="products">
          <Products mine={mine} onChange={() => void load()} />
        </Tabs.Content>
        <Tabs.Content value="profile">
          <StoreProfileForm
            profile={mine.profile}
            onSaved={() => {
              if (!mine.profile) setTab('products')
              void load()
            }}
          />
        </Tabs.Content>
      </Tabs.Root>
    </Stack>
  )
}

function Tick({ checked, onChange, children }: { checked: boolean; onChange: (checked: boolean) => void; children: ReactNode }) {
  return (
    <Checkbox.Root checked={checked} onCheckedChange={(e) => onChange(e.checked === true)} colorPalette="ink" minH="11">
      <Checkbox.HiddenInput />
      <Checkbox.Control />
      <Checkbox.Label fontWeight="normal">{children}</Checkbox.Label>
    </Checkbox.Root>
  )
}

// RupiahField is whole rupiah, typed as digits, written back as "Rp 15.000".
function RupiahField({ label, value, onChange, help, required }: { label: string; value: string; onChange: (v: string) => void; help?: string; required?: boolean }) {
  const rupiah = useFormatRupiah()
  return (
    <Field.Root required={required}>
      <Field.Label>{label}</Field.Label>
      <InputGroup startAddon="Rp" startAddonProps={{ fontWeight: 'semibold' }}>
        <Input size="lg" inputMode="numeric" value={value} onChange={(e) => onChange(rupiahInput(e.currentTarget.value))} required={required} />
      </InputGroup>
      <Field.HelperText>
        {value ? rupiah(BigInt(value)) : ''}
        {value && help ? ' · ' : ''}
        {help}
      </Field.HelperText>
    </Field.Root>
  )
}

function StoreProfileForm({ profile, onSaved }: { profile: StoreProfile | undefined; onSaved: () => void }) {
  const { t } = useTranslation()
  const [name, setName] = useState(profile?.name ?? '')
  const [area, setArea] = useState(profile?.area ?? '')
  const [address, setAddress] = useState(profile?.address ?? '')
  const [pickup, setPickup] = useState(profile?.pickup ?? true)
  const [delivers, setDelivers] = useState(profile?.delivers ?? false)
  const [fee, setFee] = useState(profile?.delivers ? profile.deliveryFee.toString() : '0')
  const [deliveryAreas, setDeliveryAreas] = useState(profile?.deliveryAreas ?? '')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [saved, setSaved] = useState(false)

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError(null)
    setSaved(false)
    if (!pickup && !delivers) {
      setError('no_handover')
      return
    }
    setPending(true)
    try {
      await storeClient.saveStoreProfile({
        profile: {
          name: name.trim(),
          area: area.trim(),
          address: address.trim(),
          pickup,
          delivers,
          deliveryFee: delivers ? BigInt(fee || '0') : 0n,
          deliveryAreas: delivers ? deliveryAreas.trim() : '',
        },
      })
      setSaved(true)
      onSaved()
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
          <Field.Root required>
            <Field.Label>{t('store.name')}</Field.Label>
            <Input size="lg" value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder={t('store.namePlaceholder')} minLength={2} maxLength={80} required />
          </Field.Root>
          <Field.Root required>
            <Field.Label>{t('store.area')}</Field.Label>
            <Input size="lg" value={area} onChange={(e) => setArea(e.currentTarget.value)} placeholder={t('common.areaPlaceholder')} minLength={2} maxLength={80} required />
          </Field.Root>
          <Field.Root required>
            <Field.Label>{t('store.address')}</Field.Label>
            <Textarea size="lg" rows={2} value={address} onChange={(e) => setAddress(e.currentTarget.value)} placeholder={t('store.addressPlaceholder')} minLength={5} maxLength={300} required />
          </Field.Root>
          <Box role="group" aria-label={t('store.handover')}>
            <Text fontWeight="semibold" mb="1">
              {t('store.handover')}
            </Text>
            <Stack gap="0">
              <Tick checked={pickup} onChange={setPickup}>
                {t('store.pickup')}
              </Tick>
              <Tick checked={delivers} onChange={setDelivers}>
                {t('store.delivers')}
              </Tick>
            </Stack>
          </Box>
          {delivers && (
            <>
              <RupiahField label={t('store.deliveryFee')} value={fee} onChange={setFee} help={t('store.deliveryFeeHelp')} />
              <Field.Root>
                <Field.Label>{t('store.deliveryAreas')}</Field.Label>
                <Input size="lg" value={deliveryAreas} onChange={(e) => setDeliveryAreas(e.currentTarget.value)} placeholder={t('store.deliveryAreasPlaceholder')} maxLength={200} />
              </Field.Root>
            </>
          )}
          <ErrorAlert code={error} />
          <SuccessAlert text={saved ? t('store.profileSaved') : null} />
          <Button type="submit" size="lg" colorPalette="ink" loading={pending}>
            {t('store.saveProfile')}
          </Button>
        </Stack>
      </form>
    </Box>
  )
}

function Products({ mine, onChange }: { mine: GetMyStoreResponse; onChange: () => void }) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState<Product | 'new' | null>(null)
  const scrolled = useRef(0)
  const edit = (product: Product | 'new') => {
    scrolled.current = window.scrollY
    setEditing(product)
    window.scrollTo(0, 0)
  }
  if (editing) {
    return (
      <ProductEditor
        product={editing === 'new' ? undefined : editing}
        onClose={() => {
          setEditing(null)
          onChange()
          window.requestAnimationFrame(() => window.scrollTo(0, scrolled.current))
        }}
      />
    )
  }
  return (
    <Stack gap="4">
      <Button size="lg" colorPalette="ink" onClick={() => edit('new')} disabled={!mine.profile}>
        + {t('store.addProduct')}
      </Button>
      {mine.profile && mine.products.length === 0 && <Text color="ink.500">{t('store.noProducts')}</Text>}
      {mine.products.map((p) => (
        <Box key={p.id.toString()} bg="white" borderWidth="1px" borderColor="ink.100" rounded="xl" p="3" data-testid="my-product">
          <Flex gap="3" align="flex-start">
            <PhotoBox photo={p.photos[0]} alt={p.details?.name ?? ''} size="20" />
            <Stack gap="1" minW="0" flex="1">
              <Text fontWeight="semibold" lineClamp={2}>
                {p.details?.name}
              </Text>
              <PriceLine product={p} />
              <Flex gap="2" wrap="wrap">
                {!p.details?.available && (
                  <Badge colorPalette="red" variant="subtle">
                    {t('shop.soldOut')}
                  </Badge>
                )}
                {p.hidden && (
                  <Badge colorPalette="orange" variant="solid">
                    {t('store.hidden')}
                  </Badge>
                )}
              </Flex>
            </Stack>
            <Button size="sm" minH="11" variant="outline" colorPalette="ink" onClick={() => edit(p)}>
              {t('store.edit')}
            </Button>
          </Flex>
        </Box>
      ))}
    </Stack>
  )
}

function ProductEditor({ product, onClose }: { product: Product | undefined; onClose: () => void }) {
  const { t } = useTranslation()
  const [current, setCurrent] = useState(product)
  const d = product?.details
  const [name, setName] = useState(d?.name ?? '')
  const [service, setService] = useState(d?.service ?? '')
  const [price, setPrice] = useState(d ? d.price.toString() : '')
  const [unit, setUnit] = useState(d?.unit ?? '')
  const [available, setAvailable] = useState(d?.available ?? true)
  const [description, setDescription] = useState(d?.description ?? '')
  const [pending, setPending] = useState<'save' | 'photo' | 'delete' | null>(null)
  const [error, setError] = useState<ErrorCode | null>(null)
  const [saved, setSaved] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const fileInput = useRef<HTMLInputElement>(null)

  const run = async (kind: 'save' | 'photo' | 'delete', work: () => Promise<void>) => {
    setPending(kind)
    setError(null)
    setSaved(false)
    try {
      await work()
    } catch (err) {
      setError(errorCode(err))
    } finally {
      setPending(null)
    }
  }
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    void run('save', async () => {
      const res = await storeClient.saveProduct({
        productId: current?.id ?? 0n,
        details: { name: name.trim(), service, price: BigInt(price || '0'), unit: unit.trim(), available, description: description.trim() },
      })
      setCurrent(res.product)
      setSaved(true)
    })
  }
  const addPhotos = (files: FileList | null) => {
    if (!current || !files) return
    const room = maxPhotos - current.photos.length
    const chosen = Array.from(files).slice(0, room)
    void run('photo', async () => {
      for (const file of chosen) {
        let data: Uint8Array
        try {
          data = await shrinkPhoto(file)
        } catch {
          setError('invalid_photo')
          return
        }
        const res = await storeClient.addProductPhoto({ productId: current.id, data })
        if (res.product) setCurrent(res.product)
      }
    })
  }
  const removePhoto = (photoId: bigint) =>
    void run('photo', async () => {
      const res = await storeClient.deleteProductPhoto({ productId: current!.id, photoId })
      if (res.product) setCurrent(res.product)
    })
  const remove = () =>
    void run('delete', async () => {
      await storeClient.deleteProduct({ productId: current!.id })
      onClose()
    })

  return (
    <Stack gap="5" data-testid="product-editor">
      <Heading as="h2" size="md">
        {current ? t('store.editProduct') : t('store.newProduct')}
      </Heading>
      {current?.hidden && (
        <Alert.Root status="warning" role="status">
          <Alert.Indicator />
          <Alert.Content>
            <Alert.Title>{t('store.hidden')}</Alert.Title>
            <Alert.Description>{t('store.hiddenHelp')}</Alert.Description>
          </Alert.Content>
        </Alert.Root>
      )}
      <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="2xl" p="5">
        <form onSubmit={submit}>
          <Stack gap="4">
            <Field.Root required>
              <Field.Label>{t('store.productName')}</Field.Label>
              <Input size="lg" value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder={t('store.productNamePlaceholder')} minLength={2} maxLength={120} required />
            </Field.Root>
            <ServiceSelect value={service} onChange={setService} />
            <RupiahField label={t('store.price')} value={price} onChange={setPrice} required />
            <Field.Root required>
              <Field.Label>{t('store.unit')}</Field.Label>
              <Input size="lg" value={unit} onChange={(e) => setUnit(e.currentTarget.value)} placeholder={t('store.unitPlaceholder')} maxLength={20} required />
            </Field.Root>
            <Tick checked={available} onChange={setAvailable}>
              {t('store.available')}
            </Tick>
            <Field.Root>
              <Field.Label>{t('store.description')}</Field.Label>
              <Textarea size="lg" rows={3} value={description} onChange={(e) => setDescription(e.currentTarget.value)} placeholder={t('store.descriptionPlaceholder')} maxLength={1000} />
            </Field.Root>
            <SuccessAlert text={saved ? t('store.saved') : null} />
            <Button type="submit" size="lg" colorPalette="ink" loading={pending === 'save'} disabled={pending !== null && pending !== 'save'}>
              {t('store.save')}
            </Button>
          </Stack>
        </form>
      </Box>
      <Box bg="white" borderWidth="1px" borderColor="ink.100" rounded="2xl" p="5">
        <Stack gap="3">
          <Text fontWeight="semibold">{t('store.photos', { count: current?.photos.length ?? 0 })}</Text>
          {!current ? (
            <Text fontSize="sm" color="ink.600">
              {t('store.photosAfterSave')}
            </Text>
          ) : (
            <>
              {current.photos.length > 0 && (
                <SimpleGrid columns={3} gap="2">
                  {current.photos.map((photo, i) => (
                    <Stack key={photo.id.toString()} gap="1">
                      <PhotoBox photo={photo} alt={t('shop.photo', { n: i + 1, count: current.photos.length })} />
                      <Button
                        size="xs"
                        minH="9"
                        variant="outline"
                        colorPalette="red"
                        aria-label={t('store.removePhoto', { n: i + 1 })}
                        disabled={pending !== null}
                        onClick={() => removePhoto(photo.id)}
                      >
                        ✕
                      </Button>
                    </Stack>
                  ))}
                </SimpleGrid>
              )}
              <input
                ref={fileInput}
                type="file"
                accept="image/jpeg,image/png,image/webp,image/*"
                multiple
                hidden
                onChange={(e) => {
                  addPhotos(e.currentTarget.files)
                  e.currentTarget.value = ''
                }}
                data-testid="photo-input"
              />
              <Button
                size="lg"
                variant="outline"
                colorPalette="ink"
                loading={pending === 'photo'}
                disabled={current.photos.length >= maxPhotos || (pending !== null && pending !== 'photo')}
                onClick={() => fileInput.current?.click()}
              >
                + {t('store.addPhoto')}
              </Button>
              <Text fontSize="xs" color="ink.600">
                {t('store.photosHelp')}
              </Text>
            </>
          )}
        </Stack>
      </Box>
      <ErrorAlert code={error} />
      <Button size="lg" variant="outline" colorPalette="ink" onClick={onClose} disabled={pending !== null}>
        {t('store.done')}
      </Button>
      {current &&
        (confirmDelete ? (
          <Stack gap="2" bg="red.50" rounded="xl" p="4">
            <Text>{t('store.confirmDelete')}</Text>
            <Button size="lg" colorPalette="red" loading={pending === 'delete'} onClick={remove}>
              {t('store.yesDelete')}
            </Button>
            <Button size="lg" variant="ghost" colorPalette="ink" onClick={() => setConfirmDelete(false)}>
              {t('store.cancel')}
            </Button>
          </Stack>
        ) : (
          <Button size="lg" variant="ghost" colorPalette="red" onClick={() => setConfirmDelete(true)} disabled={pending !== null}>
            {t('store.deleteProduct')}
          </Button>
        ))}
    </Stack>
  )
}
