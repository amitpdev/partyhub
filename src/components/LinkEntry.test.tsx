// @vitest-environment jsdom

import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { LinkEntry } from './LinkEntry'

afterEach(cleanup)

it('clears a rejected link after showing its error', async () => {
  const onAdd = vi.fn().mockRejectedValue(new Error('Only YouTube links are supported'))
  render(<LinkEntry onAdd={onAdd} />)

  const input = screen.getByLabelText('Add a YouTube link')
  fireEvent.change(input, { target: { value: 'https://example.com/song' } })
  fireEvent.click(screen.getByRole('button', { name: 'Add' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('Only YouTube links are supported')
  await waitFor(() => expect(input).toHaveValue(''))
})

it('lets the user clear a pasted link', () => {
  render(<LinkEntry onAdd={vi.fn()} />)

  const input = screen.getByLabelText('Add a YouTube link')
  fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=dQw4w9WgXcQ' } })
  fireEvent.click(screen.getByRole('button', { name: 'Clear YouTube link' }))

  expect(input).toHaveValue('')
  expect(screen.queryByRole('button', { name: 'Clear YouTube link' })).not.toBeInTheDocument()
})

it('pastes a link through the clipboard button when supported', async () => {
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { readText: vi.fn().mockResolvedValue('https://youtu.be/dQw4w9WgXcQ') },
  })
  render(<LinkEntry onAdd={vi.fn()} allowPaste />)

  fireEvent.click(screen.getByRole('button', { name: 'Paste' }))

  await waitFor(() =>
    expect(screen.getByLabelText('Add a YouTube link')).toHaveValue(
      'https://youtu.be/dQw4w9WgXcQ',
    ),
  )
  expect(screen.queryByRole('button', { name: 'Paste' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Clear YouTube link' })).toBeInTheDocument()
})

it('focuses the field and explains when clipboard access is blocked', async () => {
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { readText: vi.fn().mockRejectedValue(new Error('blocked')) },
  })
  render(<LinkEntry onAdd={vi.fn()} allowPaste />)

  fireEvent.click(screen.getByRole('button', { name: 'Paste' }))

  const input = screen.getByLabelText('Add a YouTube link')
  expect(input).toHaveFocus()
  expect(await screen.findByRole('alert')).toHaveTextContent('Clipboard access is blocked')
})
