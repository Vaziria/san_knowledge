import { createSystem, defaultConfig, defineConfig } from '@chakra-ui/react'

// Hai Tukang's brand (logo direction 07): charcoal ink, warm cream, ochre.
// Use colorPalette="ink" for primary actions and "ochre" for small accents;
// ochre is not for small body text (too little contrast on cream).
const config = defineConfig({
  globalCss: {
    'html, body': {
      bg: 'cream',
      color: 'ink.900',
    },
    body: {
      minW: '320px',
      m: 0,
    },
  },
  theme: {
    tokens: {
      fonts: {
        heading: { value: "'Plus Jakarta Sans Variable', system-ui, sans-serif" },
        body: { value: "'Plus Jakarta Sans Variable', system-ui, sans-serif" },
      },
      colors: {
        cream: { value: '#FCF6E9' },
        ink: {
          50: { value: '#F4F4F2' },
          100: { value: '#E6E7E3' },
          200: { value: '#CDCFC8' },
          300: { value: '#A9ACA3' },
          400: { value: '#7E8378' },
          500: { value: '#5B6259' },
          600: { value: '#474D45' },
          700: { value: '#3B4139' },
          800: { value: '#2E332C' },
          900: { value: '#242922' },
          950: { value: '#171A16' },
        },
        ochre: {
          50: { value: '#FDF6EA' },
          100: { value: '#FAE9CC' },
          200: { value: '#F3D29A' },
          300: { value: '#EBBA69' },
          400: { value: '#E2A74F' },
          500: { value: '#D99B3D' },
          600: { value: '#B97F2A' },
          700: { value: '#9B6820' },
          800: { value: '#7A511A' },
          900: { value: '#5C3D14' },
          950: { value: '#3A260C' },
        },
      },
    },
    semanticTokens: {
      colors: {
        ink: {
          solid: { value: '{colors.ink.900}' },
          contrast: { value: '{colors.cream}' },
          fg: { value: '{colors.ink.900}' },
          muted: { value: '{colors.ink.200}' },
          subtle: { value: '{colors.ink.50}' },
          emphasized: { value: '{colors.ink.100}' },
          focusRing: { value: '{colors.ochre.500}' },
        },
        ochre: {
          solid: { value: '{colors.ochre.500}' },
          contrast: { value: '{colors.ink.900}' },
          fg: { value: '{colors.ochre.700}' },
          muted: { value: '{colors.ochre.200}' },
          subtle: { value: '{colors.ochre.50}' },
          emphasized: { value: '{colors.ochre.100}' },
          focusRing: { value: '{colors.ochre.500}' },
        },
      },
    },
  },
})

export const system = createSystem(defaultConfig, config)
