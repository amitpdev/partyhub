// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Track, YouTubeSearchResult } from '../types'
import { SearchDialog } from './SearchDialog'

const { searchYouTube } = vi.hoisted(() => ({
  searchYouTube: vi.fn(),
}))

vi.mock('../lib/api', () => ({
  searchYouTube: (...args: unknown[]) => searchYouTube(...args),
}))

const result = (
  providerId: string,
  title: string,
): YouTubeSearchResult => ({
  source: 'youtube',
  providerId,
  url: `https://www.youtube.com/watch?v=${providerId}`,
  title,
  channel: 'Party Channel',
  thumbnail: '',
  duration: 183,
})

const queuedTrack = (providerId: string, title: string): Track => ({
  ...result(providerId, title),
  queueId: `queue-${providerId}`,
})

function openDialog(options: {
  tracks?: Track[]
  currentIndex?: number
  onAdd?: (url: string) => Promise<void>
} = {}) {
  const onAdd = options.onAdd ?? vi.fn().mockResolvedValue(undefined)
  render(
    <SearchDialog
      tracks={options.tracks ?? []}
      currentIndex={options.currentIndex ?? -1}
      onAdd={onAdd}
    />,
  )
  const trigger = screen.getByRole('button', { name: /Search YouTube/ })
  fireEvent.click(trigger)
  return { onAdd, trigger }
}

async function searchFor(query = 'dance floor') {
  const input = screen.getByRole('searchbox', { name: 'Search YouTube' })
  fireEvent.change(input, { target: { value: query } })
  fireEvent.submit(screen.getByRole('search'))
}

describe('SearchDialog', () => {
  beforeEach(() => {
    searchYouTube.mockReset()
  })

  afterEach(cleanup)

  it('searches only after submit and shows empty and retry states', async () => {
    searchYouTube
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce([])
    openDialog()

    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'karaoke' } })
    expect(searchYouTube).not.toHaveBeenCalled()
    fireEvent.submit(screen.getByRole('search'))

    expect(await screen.findByRole('alert')).toHaveTextContent('Check the connection')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByText('No matching videos')).toBeInTheDocument()
    expect(searchYouTube).toHaveBeenNthCalledWith(2, 'karaoke')
  })

  it('shows loading feedback while a search is pending', async () => {
    let resolveSearch: (results: YouTubeSearchResult[]) => void = () => undefined
    searchYouTube.mockReturnValue(
      new Promise<YouTubeSearchResult[]>((resolve) => {
        resolveSearch = resolve
      }),
    )
    openDialog()
    await searchFor()

    expect(screen.getByRole('status')).toHaveTextContent('Searching YouTube')
    resolveSearch([])
    expect(await screen.findByText('No matching videos')).toBeInTheDocument()
  })

  it('disables current and upcoming duplicates but allows replaying history', async () => {
    searchYouTube.mockResolvedValue([
      result('aaaaaaaaaaa', 'Played earlier'),
      result('bbbbbbbbbbb', 'Playing now'),
    ])
    openDialog({
      tracks: [
        queuedTrack('aaaaaaaaaaa', 'Played earlier'),
        queuedTrack('bbbbbbbbbbb', 'Playing now'),
      ],
      currentIndex: 1,
    })
    await searchFor()

    const results = await screen.findByRole('list', { name: 'YouTube search results' })
    const rows = within(results).getAllByRole('listitem')
    expect(within(rows[0]).getByRole('button', { name: 'Add to queue' })).toBeEnabled()
    expect(within(rows[1]).getByRole('button', { name: 'Already queued' })).toBeDisabled()
  })

  it('adds a selected result, closes, clears, and restores trigger focus', async () => {
    searchYouTube.mockResolvedValue([result('aaaaaaaaaaa', 'Dance Floor')])
    const { onAdd, trigger } = openDialog()
    await searchFor()

    fireEvent.click(await screen.findByRole('button', { name: 'Add to queue' }))
    await waitFor(() => expect(onAdd).toHaveBeenCalledWith('https://www.youtube.com/watch?v=aaaaaaaaaaa'))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(trigger).toHaveFocus()

    fireEvent.click(trigger)
    expect(screen.getByRole('searchbox')).toHaveValue('')
  })

  it('keeps the dialog open when adding fails and closes on Escape', async () => {
    searchYouTube.mockResolvedValue([result('aaaaaaaaaaa', 'Dance Floor')])
    openDialog({ onAdd: vi.fn().mockRejectedValue(new Error('Queue is full')) })
    await searchFor()

    fireEvent.click(await screen.findByRole('button', { name: 'Add to queue' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Queue is full')
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
