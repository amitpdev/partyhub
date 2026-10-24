import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import App from './App'

const player = {
  party: {
    revision: 1,
    tracks: [],
    currentIndex: -1,
    volume: 80,
    player: {
      available: true,
      missing: [],
      state: 'idle' as const,
      generation: 0,
      trackId: '',
      position: 0,
      duration: 0,
      volume: 80,
    },
  },
  status: {
    available: true,
    missing: [],
    state: 'idle' as const,
    generation: 0,
    trackId: '',
    position: 0,
    duration: 0,
    volume: 80,
  },
  notice: '',
  clearNotice: vi.fn(),
  add: vi.fn(),
  remove: vi.fn(),
  move: vi.fn(),
  clear: vi.fn(),
  play: vi.fn(),
  next: vi.fn(),
  previous: vi.fn(),
  pauseOrResume: vi.fn(),
  seek: vi.fn(),
  seekTo: vi.fn(),
  stop: vi.fn(),
  setVolume: vi.fn(),
}

vi.mock('./hooks/usePartyPlayer', () => ({
  usePartyPlayer: () => player,
}))

vi.mock('./components/ShareParty', () => ({
  ShareParty: () => <button>Share party</button>,
}))

vi.mock('./components/ThemeSwitcher', () => ({
  ThemeSwitcher: () => <button>Theme</button>,
}))

afterEach(cleanup)

it('places the standalone song adder before and outside the queue', () => {
  render(<App />)

  const songAdder = screen.getByRole('region', { name: 'Add your song' })
  const queue = screen.getByRole('region', { name: 'Up next' })
  const input = screen.getByLabelText('Add a YouTube link')
  const search = screen.getByRole('button', { name: /Search YouTube/ })

  expect(songAdder).toContainElement(input)
  expect(songAdder).toContainElement(search)
  expect(songAdder).toHaveTextContent('or paste a link')
  expect(queue).not.toContainElement(input)
  expect(
    songAdder.compareDocumentPosition(queue) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy()
})
