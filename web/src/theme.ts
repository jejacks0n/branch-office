import { ref, watch } from 'vue'

export type ThemePref = 'system' | 'light' | 'dark'

// Also read by the inline script in index.html, which sets the theme before first paint.
const STORAGE_KEY = 'broffice_theme'
const THEME_COLORS = { light: '#ffffff', dark: '#09090b' }

const media = window.matchMedia('(prefers-color-scheme: light)')

function readPref(): ThemePref {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'light' || saved === 'dark') return saved
  } catch {
    // Storage unavailable (private mode); fall back to the system preference.
  }
  return 'system'
}

export const themePref = ref<ThemePref>(readPref())

function apply() {
  const theme = themePref.value === 'system' ? (media.matches ? 'light' : 'dark') : themePref.value
  document.documentElement.dataset.theme = theme
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', THEME_COLORS[theme])
}

watch(themePref, (pref) => {
  try {
    if (pref === 'system') localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, pref)
  } catch {
    // ignore
  }
  apply()
})

media.addEventListener('change', apply)
apply()
