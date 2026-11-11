import { ChakraProvider } from '@chakra-ui/react'
import '@fontsource-variable/plus-jakarta-sans'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './i18n' // before App renders: sets the language (Bahasa Indonesia by default)
import { listenForPush } from './push'
import { system } from './theme'

listenForPush()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ChakraProvider value={system}>
      <App />
    </ChakraProvider>
  </StrictMode>,
)
