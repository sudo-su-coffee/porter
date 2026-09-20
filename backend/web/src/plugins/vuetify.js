// Vuetify 3 theme for the Porter control plane.
import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'

export default createVuetify({
  theme: {
    defaultTheme: 'dark',
    themes: {
      dark: {
        colors: {
          background: '#0f1620',
          surface: '#151d2a',
          'surface-variant': '#1a2434',
          primary: '#5b8cff',
          secondary: '#8f9bb3',
          accent: '#2dd4bf',
          error: '#f87171',
          warning: '#fbbf24',
          info: '#38bdf8',
          success: '#34d399'
        }
      }
    }
  },
  defaults: {
    VBtn: { rounded: 'lg', textTransform: 'none' },
    VTextField: { variant: 'outlined', density: 'comfortable' },
    VCard: { rounded: 'lg' }
  }
})