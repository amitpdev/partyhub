import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { THEME_STORAGE_KEY } from '../lib/theme'
import { ThemeSwitcher } from './ThemeSwitcher'

beforeEach(() => {
  localStorage.clear()
  document.documentElement.dataset.theme = 'midnight-violet'
})

afterEach(cleanup)

it('applies and saves the selected dashboard theme', () => {
  render(<ThemeSwitcher />)

  fireEvent.click(screen.getByRole('button', { name: 'Theme' }))
  fireEvent.click(screen.getByRole('radio', { name: /Signal Orange/ }))

  expect(document.documentElement).toHaveAttribute('data-theme', 'signal-orange')
  expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('signal-orange')
  expect(screen.getByRole('radio', { name: /Signal Orange/ })).toHaveAttribute(
    'aria-checked',
    'true',
  )
})
