import { beforeEach, describe, expect, it } from 'vitest'
import {
  applyTheme,
  DEFAULT_THEME,
  loadTheme,
  saveTheme,
  THEME_STORAGE_KEY,
} from './theme'

describe('dashboard themes', () => {
  beforeEach(() => {
    document.documentElement.removeAttribute('data-theme')
  })

  it('falls back to Midnight Violet for missing and unknown values', () => {
    expect(loadTheme({ getItem: () => null })).toBe(DEFAULT_THEME)
    expect(loadTheme({ getItem: () => 'unknown-theme' })).toBe(DEFAULT_THEME)
  })

  it('loads and applies a saved theme', () => {
    expect(loadTheme({ getItem: () => 'terminal-green' })).toBe('terminal-green')
    applyTheme('terminal-green')
    expect(document.documentElement).toHaveAttribute('data-theme', 'terminal-green')
  })

  it('recognizes the yellow and wine palettes', () => {
    expect(loadTheme({ getItem: () => 'sneaky-yellow' })).toBe('sneaky-yellow')
    expect(loadTheme({ getItem: () => 'wine-red' })).toBe('wine-red')
  })

  it('persists a selected theme', () => {
    const values = new Map<string, string>()
    saveTheme('signal-orange', {
      setItem: (key, value) => values.set(key, value),
    })
    expect(values.get(THEME_STORAGE_KEY)).toBe('signal-orange')
  })
})
