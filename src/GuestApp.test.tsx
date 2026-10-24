// @vitest-environment jsdom

import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import GuestApp from './GuestApp'
import type { Track } from './types'

const {
  createGuestSession,
  fetchParty,
  searchYouTube,
  guestAdd,
  guestRemove,
  playerAvailable,
  guestTracks,
  guestCurrentIndex,
} = vi.hoisted(() => ({
  createGuestSession: vi.fn(),
  fetchParty: vi.fn(),
  searchYouTube: vi.fn(),
  guestAdd: vi.fn(),
  guestRemove: vi.fn(),
  playerAvailable: { value: true },
  guestTracks: { value: [] as Track[] },
  guestCurrentIndex: { value: -1 },
}))

vi.mock('./lib/api', () => ({
  createGuestSession: (...args: unknown[]) => createGuestSession(...args),
  fetchParty: (...args: unknown[]) => fetchParty(...args),
  searchYouTube: (...args: unknown[]) => searchYouTube(...args),
}))

vi.mock('./hooks/usePartyPlayer', () => ({
  usePartyPlayer: () => ({
    party: {
      revision: 1,
      tracks: guestTracks.value,
      currentIndex: guestCurrentIndex.value,
      volume: 80,
      player: {
        available: playerAvailable.value,
        missing: [],
        state: 'idle',
        generation: 0,
        trackId: '',
        position: 0,
        duration: 0,
        volume: 80,
      },
    },
    status: { available: playerAvailable.value },
    notice: '',
    add: guestAdd,
    remove: guestRemove,
  }),
}))

describe('GuestApp', () => {
  beforeEach(() => {
    localStorage.clear()
    window.scrollTo = vi.fn()
    createGuestSession.mockReset()
    fetchParty.mockReset()
    searchYouTube.mockReset()
    guestAdd.mockReset()
    guestRemove.mockReset()
    playerAvailable.value = true
    guestTracks.value = []
    guestCurrentIndex.value = -1
    window.history.replaceState(null, '', '/guest#token=invite-token')
  })

  afterEach(cleanup)

  it('requires a name and exchanges the invite before showing the queue', async () => {
    createGuestSession.mockResolvedValue({ name: 'Maya' })
    render(<GuestApp />)

    fireEvent.change(screen.getByLabelText('Your name'), { target: { value: 'Maya' } })
    fireEvent.click(screen.getByRole('button', { name: 'Join party' }))

    await waitFor(() =>
      expect(createGuestSession).toHaveBeenCalledWith('invite-token', 'Maya'),
    )
    expect(await screen.findByRole('heading', { name: 'Add your song' })).toBeInTheDocument()
    expect(localStorage.getItem('partyhub.guestName')).toBe('Maya')
    expect(window.location.hash).toBe('')
  })

  it('places guest search before the paste-link option', async () => {
    window.history.replaceState(null, '', '/guest')
    fetchParty.mockResolvedValue({})
    render(<GuestApp />)

    const heading = await screen.findByRole('heading', { name: 'Add your song' })
    const section = heading.closest('section')
    const search = screen.getByRole('button', { name: 'Search YouTube' })
    const link = screen.getByLabelText('YouTube link')

    expect(section).toContainElement(search)
    expect(section).toContainElement(link)
    expect(section).toHaveTextContent('or paste a link')
    expect(search.compareDocumentPosition(link) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('disables guest search and link adding while reconnecting', async () => {
    playerAvailable.value = false
    window.history.replaceState(null, '', '/guest')
    fetchParty.mockResolvedValue({})
    render(<GuestApp />)

    expect(await screen.findByRole('button', { name: 'Search YouTube' })).toBeDisabled()
    expect(screen.getByLabelText('YouTube link')).toBeDisabled()
  })

  it('keeps removal limited to the guest-owned upcoming track', async () => {
    const track = (queueId: string, title: string, canRemove: boolean): Track => ({
      queueId,
      source: 'youtube',
      providerId: `${queueId}00000000000`.slice(0, 11),
      url: `https://youtube.com/watch?v=${queueId}`,
      title,
      channel: 'Party Channel',
      thumbnail: '',
      duration: 180,
      addedBy: canRemove ? 'Maya' : 'Noah',
      canRemove,
      canMove: false,
      canPlay: false,
    })
    guestTracks.value = [
      track('current', 'Playing now', false),
      track('other', 'Other guest song', false),
      track('mine', 'My song', true),
    ]
    guestCurrentIndex.value = 0
    window.history.replaceState(null, '', '/guest')
    fetchParty.mockResolvedValue({})
    render(<GuestApp />)

    expect(await screen.findByRole('button', { name: 'Remove My song' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: 'Remove Other guest song' })).not.toBeInTheDocument()
  })

  it('rejects a QR URL without its capability token', async () => {
    window.history.replaceState(null, '', '/guest')
    fetchParty.mockRejectedValue(new Error('expired'))
    render(<GuestApp />)

    expect(await screen.findByText('This invite is incomplete')).toBeInTheDocument()
  })
})
