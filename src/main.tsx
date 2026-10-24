import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource-variable/inter'
import App from './App'
import GuestApp from './GuestApp'
import { applyTheme, loadTheme } from './lib/theme'
import './styles.css'

const isGuest = window.location.pathname.startsWith('/guest')
document.documentElement.classList.toggle('guest-mode', isGuest)
if (!isGuest) applyTheme(loadTheme())
const RootApp = isGuest ? GuestApp : App

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RootApp />
  </StrictMode>,
)
