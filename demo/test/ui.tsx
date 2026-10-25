import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { Demo } from '../src/Demo'
import { engine, resetDemo, setDemoView } from '../src/api'

beforeEach(() => {
  window.history.replaceState(null, '', '/')
  setDemoView('dj')
  resetDemo()
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', '') }
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open') }
  window.confirm = vi.fn(() => true)
  window.scrollTo = vi.fn()
})
afterEach(cleanup)

it('reuses the real dashboard with simulated playback and working themes', async () => {
  render(<Demo />)
  await screen.findByText('Glasshouse Groove', { selector: '.track-copy h2' })
  expect(screen.getByText('Sample songs · Playback simulated')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Theme' }))
  fireEvent.click(screen.getByRole('radio', { name: /Sneaky Yellow/ }))
  expect(document.documentElement.dataset.theme).toBe('sneaky-yellow')
  fireEvent.click(screen.getByRole('button', { name: 'Theme' }))
  fireEvent.click(screen.getByRole('button', { name: 'Play' }))
  await waitFor(() => expect(engine.snapshot().player.state).toBe('playing'))
})

it('lets a guest search and add a song that appears in DJ view', async () => {
  render(<Demo />)
  await screen.findByRole('heading', { name: 'Now playing' })
  fireEvent.click(screen.getByRole('button', { name: 'Guest view' }))
  await screen.findByRole('heading', { name: 'Add your song' })
  const search = screen.getByRole('button', { name: 'Search YouTube' })
  await waitFor(() => expect(search).toBeEnabled())
  fireEvent.click(search)
  const dialog = screen.getByRole('dialog', { name: 'Search YouTube' })
  expect(within(dialog).getByText('Demo search shows sample songs. The full app searches YouTube.')).toBeInTheDocument()
  fireEvent.change(within(dialog).getByRole('searchbox'), { target: { value: 'Daft Punk' } })
  fireEvent.click(within(dialog).getByRole('button', { name: 'Search' }))
  const results = await within(dialog).findByRole('list', { name: 'YouTube search results' })
  const blueHour = within(results).getByText('Blue Hour').closest('li')!
  fireEvent.click(within(blueHour).getByRole('button', { name: 'Add to queue' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'DJ view' }))
  await screen.findByText('Blue Hour', { selector: '.queue-row__copy strong' })
  expect(screen.getByRole('button', { name: 'Save playlist' }).querySelector('i')).toBeInTheDocument()
})

it('loads a sample playlist and can reset to the starting set', async () => {
  render(<Demo />)
  await screen.findByRole('heading', { name: 'Now playing' })
  fireEvent.click(screen.getByRole('button', { name: 'Playlist' }))
  fireEvent.click(await screen.findByRole('button', { name: 'Load After hours' }))
  fireEvent.click(screen.getByRole('button', { name: 'Yes' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  expect(engine.snapshot().playlist?.name).toBe('After hours')
  expect(engine.snapshot().player.state).toBe('stopped')
  fireEvent.click(screen.getByRole('button', { name: 'Reset' }))
  await screen.findByText('Glasshouse Groove', { selector: '.track-copy h2' })
  expect(engine.snapshot().playlist?.name).toBe('Rooftop warm-up')
})

it('simulates an AI pick without asking for credentials', async () => {
  render(<Demo />)
  const pick = await screen.findByRole('button', { name: 'Let AI pick' })
  await waitFor(() => expect(pick).toBeEnabled())
  fireEvent.click(pick)
  expect(await screen.findByText(/No AI request sent/)).toBeInTheDocument()
  expect(screen.queryByRole('dialog', { name: 'AI settings' })).not.toBeInTheDocument()
  expect(engine.snapshot().tracks[2].providerId).toBe('demo0000008')
})
