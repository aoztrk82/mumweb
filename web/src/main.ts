import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'
import { applyTheme, prefs } from './lib/stores/prefs.svelte'

applyTheme(prefs.value.theme)

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
