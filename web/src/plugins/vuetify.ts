import { createVuetify, type ThemeDefinition } from 'vuetify'
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg'

const cotaLight: ThemeDefinition = {
  dark: false,
  colors: {
    background: '#EDE8DC',
    surface: '#FAF7EF',
    'surface-light': '#F1ECDF',
    'surface-variant': '#DDD6C5',
    'on-surface-variant': '#18221F',
    'on-background': '#18221F',
    'on-surface': '#18221F',
    primary: '#0A5C66',
    'on-primary': '#FFFFFF',
    secondary: '#7A4A24',
    'on-secondary': '#FFFFFF',
    accent: '#0D69A0',
    'on-accent': '#FFFFFF',
    success: '#256B3E',
    warning: '#8A5500',
    error: '#A82020',
    info: '#0D69A0',
    'water-fill': '#9CCFD3',
    'water-edge': '#4E9AA3',
    'dry-hatch': '#7A4A24',
  },
  variables: {
    'border-color': '#18221F',
    'border-opacity': 0.16,
    'medium-emphasis-opacity': 0.7,
    'disabled-opacity': 0.4,
    'hover-opacity': 0.05,
    'focus-opacity': 0.12,
  },
}

const cotaDark: ThemeDefinition = {
  dark: true,
  colors: {
    background: '#0E1514',
    surface: '#152020',
    'surface-light': '#1B2A29',
    'surface-variant': '#24332F',
    'on-surface-variant': '#E3ECE8',
    'on-background': '#E3ECE8',
    'on-surface': '#E3ECE8',
    primary: '#6FD0D8',
    'on-primary': '#06292D',
    secondary: '#D6AE86',
    'on-secondary': '#2B1A0B',
    accent: '#7CC4EA',
    'on-accent': '#08293B',
    success: '#8FD6A5',
    warning: '#F2BE6A',
    error: '#FF9A90',
    info: '#7CC4EA',
    'water-fill': '#1E5A62',
    'water-edge': '#4FA3AC',
    'dry-hatch': '#D6AE86',
  },
  variables: {
    'border-color': '#E3ECE8',
    'border-opacity': 0.16,
    'medium-emphasis-opacity': 0.7,
    'disabled-opacity': 0.4,
    'hover-opacity': 0.07,
    'focus-opacity': 0.14,
  },
}

export default createVuetify({
  icons: { defaultSet: 'mdi', aliases, sets: { mdi } },
  theme: { defaultTheme: 'cotaLight', themes: { cotaLight, cotaDark } },
  defaults: {
    VBtn: { rounded: 'sm', variant: 'flat', class: 'text-none', height: 44 },
    VCard: { rounded: 'sm', variant: 'flat', elevation: 0, border: true },
    VChip: { rounded: 'sm', variant: 'tonal', size: 'small', label: true },
    VTextField: { variant: 'outlined', density: 'comfortable', rounded: 'sm' },
    VAppBar: { flat: true, height: 56, color: 'background', border: 'b' },
    VAlert: { variant: 'tonal', rounded: 'sm', border: 'start' },
  },
})
