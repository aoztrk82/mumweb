import { mount } from 'svelte'
import '../app.css'
import './admin.css'
import { apply as applyLang, language } from '../i18n/index.svelte'
import { applyTheme, prefs } from '../lib/stores/prefs.svelte'
import Admin from './Admin.svelte'

applyTheme(prefs.value.theme)
applyLang(language.current)

// No service worker here: the panel must never be served from the app-shell cache.
const app = mount(Admin, { target: document.getElementById('app')! })

export default app
