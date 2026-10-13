import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'
import { apply as applyLang, language } from './i18n/index.svelte'
import { applyTheme, prefs } from './lib/stores/prefs.svelte'

applyTheme(prefs.value.theme)
applyLang(language.current)

const app = mount(App, {
  target: document.getElementById('app')!,
})

// Static-file cache for "add to home screen" (production builds only).
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch((e) => console.warn('service worker registration failed', e))
  })
}

export default app
