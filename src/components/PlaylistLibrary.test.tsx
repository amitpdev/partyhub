import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { PlaylistLibrary } from './PlaylistLibrary'
import { changePlaylist, fetchPlaylists, loadPlaylist } from '../lib/api'
import type { PartySnapshot, Playlist } from '../types'

vi.mock('../lib/api', () => ({ changePlaylist: vi.fn(), fetchPlaylists: vi.fn(), loadPlaylist: vi.fn() }))
const track = { source: 'youtube' as const, providerId: 'aaaaaaaaaaa', url: 'https://www.youtube.com/watch?v=aaaaaaaaaaa', title: 'First song', channel: 'Artist', thumbnail: '', duration: 180 }
const playlist: Playlist = { id: 'p1', name: 'Summer Party', createdAt: '', updatedAt: '', tracks: [track] }
const party: PartySnapshot = { revision: 1, tracks: [{ ...track, queueId: 'q1' }], currentIndex: 0, volume: 80, player: { available: true, missing: [], state: 'stopped', generation: 1, trackId: '', position: 0, duration: 0, volume: 80 } }
const snapshot = vi.fn()
beforeEach(() => {
  vi.clearAllMocks()
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', '') }
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open') }
  vi.mocked(fetchPlaylists).mockResolvedValue({ version: 1, playlists: [playlist] })
  vi.mocked(changePlaylist).mockResolvedValue({ ...party, playlist: { id: 'p1', name: 'Summer Party', dirty: false } })
  vi.mocked(loadPlaylist).mockResolvedValue({ party, skipped: 0 })
})
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals() })
async function openLibrary() {
  fireEvent.click(screen.getByRole('button', { name: 'Playlist' }))
  await screen.findByRole('button', { name: 'Load Summer Party' })
}
it('shows each saved playlist once and keeps current-queue actions out of the library', async () => {
  render(<PlaylistLibrary party={{ ...party, playlist: { id: 'p1', name: 'Summer Party', dirty: false } }} onSnapshot={snapshot} />)
  await openLibrary()
  const library = within(screen.getByRole('dialog', { name: 'Playlists' }))
  expect(library.getAllByText('Summer Party')).toHaveLength(1)
  expect(library.getByRole('button', { name: 'Load' })).toBeInTheDocument()
  expect(library.queryByRole('button', { name: 'Export all' })).not.toBeInTheDocument()
  expect(library.queryByRole('button', { name: /Save|Update|Add to queue|Export|Rename|Delete/ })).not.toBeInTheDocument()
  expect(library.queryByText('All playlist changes saved')).not.toBeInTheDocument()
})
it('saves the queue in a separate dialog', async () => {
  render(<PlaylistLibrary party={party} onSnapshot={snapshot} />)
  expect(screen.getByRole('button', { name: 'Save playlist' }).querySelector('i')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Playlist' })).not.toHaveClass('is-dirty')
  fireEvent.click(screen.getByRole('button', { name: 'Save playlist' }))
  expect(screen.queryByRole('dialog', { name: 'Playlists' })).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Playlist name'), { target: { value: ' New set ' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(snapshot).toHaveBeenCalled())
  expect(changePlaylist).toHaveBeenCalledWith({ action: 'save', id: undefined, name: 'New set' })
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
it('offers save or discard when selecting a playlist with dirty changes', async () => {
  render(<PlaylistLibrary party={{ ...party, playlist: { id: 'p1', name: 'Summer Party', dirty: true } }} onSnapshot={snapshot} />)
  await openLibrary()
  fireEvent.click(screen.getByRole('button', { name: 'Load Summer Party' }))
  expect(screen.getByRole('button', { name: 'Save and load' })).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
  expect(loadPlaylist).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Load Summer Party' }))
  fireEvent.click(screen.getByRole('button', { name: 'Load without saving' }))
  await waitFor(() => expect(loadPlaylist).toHaveBeenCalledWith('p1', 'replace', true))
})
it('saves before loading and leaves the dialog open on save failure', async () => {
  vi.mocked(changePlaylist).mockRejectedValueOnce(new Error('Disk full'))
  render(<PlaylistLibrary party={{ ...party, playlist: { id: 'p1', name: 'Summer Party', dirty: true } }} onSnapshot={snapshot} />)
  await openLibrary()
  fireEvent.click(screen.getByRole('button', { name: 'Load Summer Party' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save and load' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('Disk full')
  expect(loadPlaylist).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Save and load' }))
  await waitFor(() => expect(loadPlaylist).toHaveBeenCalledWith('p1', 'replace', false))
})
it('keeps rename and delete behind the playlist actions button', async () => {
  render(<PlaylistLibrary party={party} onSnapshot={snapshot} />)
  await openLibrary()
  fireEvent.click(screen.getByRole('button', { name: 'Actions for Summer Party' }))
  fireEvent.click(screen.getByRole('button', { name: 'Rename' }))
  fireEvent.change(screen.getByLabelText('Rename playlist'), { target: { value: 'New title' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(changePlaylist).toHaveBeenCalledWith({ action: 'rename', id: 'p1', name: 'New title' }))
  await waitFor(() => expect(screen.queryByLabelText('Rename playlist')).not.toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Actions for Summer Party' }))
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
  expect(changePlaylist).toHaveBeenCalledTimes(1)
  fireEvent.click(screen.getByRole('button', { name: 'Delete playlist' }))
  await waitFor(() => expect(changePlaylist).toHaveBeenCalledWith({ action: 'delete', id: 'p1' }))
})
it('exports one playlist and loads a file into the library', async () => {
  const createURL = vi.fn(() => 'blob:test')
  vi.stubGlobal('URL', { createObjectURL: createURL, revokeObjectURL: vi.fn() })
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
  render(<PlaylistLibrary party={party} onSnapshot={snapshot} />)
  await openLibrary()
  fireEvent.click(screen.getByRole('button', { name: 'Actions for Summer Party' }))
  fireEvent.click(screen.getByRole('button', { name: 'Export' }))
  expect(createURL).toHaveBeenCalledWith(expect.any(Blob))
  const backup = { version: 1 as const, playlists: [playlist] }
  const file = new File([JSON.stringify(backup)], 'backup.json', { type: 'application/json' })
  Object.defineProperty(file, 'text', { value: async () => JSON.stringify(backup) })
  fireEvent.change(screen.getByLabelText('Load playlist file'), { target: { files: [file] } })
  await within(screen.getByRole('dialog', { name: 'Playlists' })).findByText('File added. Select a playlist to use it.')
  expect(changePlaylist).toHaveBeenCalledWith({ action: 'import', backup })
})
it('disables saving an empty queue', () => {
  render(<PlaylistLibrary party={{ ...party, tracks: [] }} onSnapshot={snapshot} />)
  expect(screen.getByRole('button', { name: 'Save playlist' }).querySelector('i')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Save playlist' })).toBeDisabled()
})

it('shares the selected playlist file through the native share sheet', async () => {
  const share = vi.fn().mockResolvedValue(undefined)
  const canShare = vi.fn().mockReturnValue(true)
  vi.stubGlobal('navigator', { share, canShare })
  render(<PlaylistLibrary party={party} onSnapshot={snapshot} />)
  await openLibrary()
  fireEvent.click(screen.getByRole('button', { name: 'Actions for Summer Party' }))
  fireEvent.click(screen.getByRole('button', { name: 'Share' }))
  await waitFor(() => expect(share).toHaveBeenCalled())
  const data = share.mock.calls[0][0] as ShareData
  expect(data.title).toBe('Summer Party')
  expect(data.files).toHaveLength(1)
  expect(data.files?.[0].name).toBe('Summer-Party.json')
  expect(data.files?.[0].type).toBe('application/json')
  expect(canShare).toHaveBeenCalledWith({ files: data.files })
})
it('downloads a playlist for manual sharing when native file sharing is unsupported', async () => {
  const share = vi.fn()
  vi.stubGlobal('navigator', { share, canShare: () => false })
  const createURL = vi.fn(() => 'blob:test')
  vi.stubGlobal('URL', { createObjectURL: createURL, revokeObjectURL: vi.fn() })
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
  render(<PlaylistLibrary party={party} onSnapshot={snapshot} />)
  await openLibrary()
  fireEvent.click(screen.getByRole('button', { name: 'Actions for Summer Party' }))
  fireEvent.click(screen.getByRole('button', { name: 'Share' }))
  await waitFor(() => expect(createURL).toHaveBeenCalledWith(expect.any(File)))
  expect(share).not.toHaveBeenCalled()
  expect(within(screen.getByRole('dialog', { name: 'Playlists' })).getByRole('status')).toHaveTextContent('downloaded')
})
it('treats dismissing the share sheet as cancellation', async () => {
  vi.stubGlobal('navigator', {
    canShare: () => true,
    share: vi.fn().mockRejectedValue(new DOMException('Canceled', 'AbortError')),
  })
  render(<PlaylistLibrary party={party} onSnapshot={snapshot} />)
  await openLibrary()
  fireEvent.click(screen.getByRole('button', { name: 'Actions for Summer Party' }))
  fireEvent.click(screen.getByRole('button', { name: 'Share' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Share' })).toBeEnabled())
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})
