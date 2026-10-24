export const THEME_STORAGE_KEY = 'partyhub.theme.v1'

export const themes = [
  {
    id: 'midnight-violet',
    name: 'Midnight Violet',
    description: 'Nightlife energy',
    swatches: ['#07070b', '#1b1928', '#6d4aff'],
  },
  {
    id: 'signal-orange',
    name: 'Signal Orange',
    description: 'Warm and direct',
    swatches: ['#050505', '#242424', '#ff6b2c'],
  },
  {
    id: 'electric-blue',
    name: 'Electric Blue',
    description: 'Cool club light',
    swatches: ['#06080c', '#18202d', '#1473e6'],
  },
  {
    id: 'terminal-green',
    name: 'Terminal Green',
    description: 'Classic phosphor glow',
    swatches: ['#020704', '#102017', '#4fe07d'],
  },
  {
    id: 'sneaky-yellow',
    name: 'Sneaky Yellow',
    description: 'Low-key mischief',
    swatches: ['#080704', '#27240f', '#f5da4e'],
  },
  {
    id: 'wine-red',
    name: 'Wine Red',
    description: 'Deep after-hours warmth',
    swatches: ['#090305', '#29151b', '#d45a78'],
  },
  {
    id: 'high-contrast',
    name: 'High Contrast',
    description: 'Maximum clarity',
    swatches: ['#000000', '#202020', '#f5f5f5'],
  },
] as const

export type ThemeID = (typeof themes)[number]['id']

export const DEFAULT_THEME: ThemeID = 'midnight-violet'

const themeGrounds: Record<ThemeID, string> = {
  'midnight-violet': '#07070b',
  'signal-orange': '#050505',
  'electric-blue': '#06080c',
  'terminal-green': '#020704',
  'sneaky-yellow': '#080704',
  'wine-red': '#090305',
  'high-contrast': '#000000',
}

export function isThemeID(value: unknown): value is ThemeID {
  return themes.some((theme) => theme.id === value)
}

export function loadTheme(storage: Pick<Storage, 'getItem'> = window.localStorage): ThemeID {
  try {
    const saved = storage.getItem(THEME_STORAGE_KEY)
    return isThemeID(saved) ? saved : DEFAULT_THEME
  } catch {
    return DEFAULT_THEME
  }
}

export function saveTheme(
  theme: ThemeID,
  storage: Pick<Storage, 'setItem'> = window.localStorage,
) {
  try {
    storage.setItem(THEME_STORAGE_KEY, theme)
  } catch {
    // The selected theme still applies for this session when storage is unavailable.
  }
}

export function applyTheme(
  theme: ThemeID,
  root: HTMLElement = document.documentElement,
) {
  root.dataset.theme = theme
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', themeGrounds[theme])
}

export function selectTheme(theme: ThemeID) {
  applyTheme(theme)
  saveTheme(theme)
}
