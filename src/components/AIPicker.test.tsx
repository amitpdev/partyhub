import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { AIPicker } from './AIPicker'
import { clearAISettings, fetchAIModels, fetchAISettings, pickAISong, saveAISettings, testAIConnection } from '../lib/ai'
import { removeTrack } from '../lib/api'
import type { AISettings } from '../lib/ai'
import type { PartySnapshot } from '../types'

vi.mock('../lib/ai', () => ({ fetchAISettings: vi.fn(), saveAISettings: vi.fn(), testAIConnection: vi.fn(), fetchAIModels: vi.fn(), clearAISettings: vi.fn(), pickAISong: vi.fn() }))
vi.mock('../lib/api', () => ({ removeTrack: vi.fn() }))
const initial: AISettings = { provider: 'openai', model: '', baseUrl: 'https://api.openai.com/v1', hasKey: false, rememberKey: false, configured: false, secureStorageAvailable: true }
const configured: AISettings = { ...initial, model: 'text-model', hasKey: true, configured: true, rememberKey: true }
const party = { tracks: [], currentIndex: -1, revision: 1, volume: 80, player: { available: true, missing: [], state: 'paused', generation: 1, trackId: 'current', duration: 200, position: 50, volume: 80 } } satisfies PartySnapshot
const onSnapshot = vi.fn()
beforeEach(() => {
  vi.resetAllMocks()
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', '') }
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open') }
  vi.mocked(fetchAISettings).mockResolvedValue(initial)
  vi.mocked(testAIConnection).mockResolvedValue({ ok: true })
  vi.mocked(saveAISettings).mockResolvedValue(configured)
  vi.mocked(fetchAIModels).mockResolvedValue({ models: ['text-model'] })
  vi.mocked(pickAISong).mockResolvedValue({ party, title: 'New song', queueId: 'new-song' })
  vi.mocked(removeTrack).mockResolvedValue(party)
})
afterEach(cleanup)

it('opens setup on first use and saves only after a successful connection test', async () => {
  render(<AIPicker onSnapshot={onSnapshot} />)
  fireEvent.click(screen.getByRole('button', { name: 'Let AI pick' }))
  await screen.findByRole('dialog', { name: 'AI settings' })
  expect(pickAISong).not.toHaveBeenCalled()
  expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('API key'), { target: { value: 'test-key' } })
  fireEvent.click(screen.getByRole('button', { name: 'Load models' }))
  await screen.findByText('Choose a model below, or enter its ID.')
  fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'text-model' } })
  fireEvent.click(screen.getByRole('button', { name: 'Test connection' }))
  await screen.findByText('Connection works. Save to use it.')
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  expect(saveAISettings).toHaveBeenCalledWith({ provider: 'openai', model: 'text-model', baseUrl: 'https://api.openai.com/v1', apiKey: 'test-key', rememberKey: true })
})
it('picks a song with saved settings and supports Undo', async () => {
  vi.mocked(fetchAISettings).mockResolvedValue(configured)
  render(<AIPicker onSnapshot={onSnapshot} />)
  fireEvent.click(screen.getByRole('button', { name: 'Let AI pick' }))
  await screen.findByText('New song')
  expect(onSnapshot).toHaveBeenCalledWith(party)
  fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
  await waitFor(() => expect(removeTrack).toHaveBeenCalledWith('new-song'))
})
it('keeps failed tests and picks visible without changing playback', async () => {
  vi.mocked(fetchAISettings).mockResolvedValue(configured)
  vi.mocked(pickAISong).mockRejectedValue(new Error('Provider is unavailable'))
  render(<AIPicker onSnapshot={onSnapshot} />)
  fireEvent.click(screen.getByRole('button', { name: 'Let AI pick' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('Provider is unavailable')
  expect(onSnapshot).not.toHaveBeenCalled()
})
it('allows local providers without a key and invalidates tests when the model changes', async () => {
  render(<AIPicker onSnapshot={onSnapshot} />)
  fireEvent.click(screen.getByRole('button', { name: 'AI settings' }))
  await screen.findByRole('dialog', { name: 'AI settings' })
  fireEvent.change(screen.getByLabelText('Provider'), { target: { value: 'ollama' } })
  expect(screen.getByLabelText('Server URL')).toHaveValue('http://127.0.0.1:11434')
  fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'local-model' } })
  fireEvent.click(screen.getByRole('button', { name: 'Test connection' }))
  await screen.findByText('Connection works. Save to use it.')
  expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'another-model' } })
  expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
})
it('uses session-only storage when OS secure storage is unavailable', async () => {
  vi.mocked(fetchAISettings).mockResolvedValue({ ...initial, secureStorageAvailable: false })
  render(<AIPicker onSnapshot={onSnapshot} />)
  fireEvent.click(screen.getByRole('button', { name: 'AI settings' }))
  await screen.findByRole('dialog', { name: 'AI settings' })
  expect(screen.getByRole('checkbox', { name: 'Remember key securely' })).toBeDisabled()
  expect(screen.getByRole('checkbox', { name: 'Remember key securely' })).not.toBeChecked()
})
it('disconnects only after confirmation', async () => {
  vi.mocked(fetchAISettings).mockResolvedValue(configured)
  vi.mocked(clearAISettings).mockResolvedValue(initial)
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  render(<AIPicker onSnapshot={onSnapshot} />)
  fireEvent.click(screen.getByRole('button', { name: 'AI settings' }))
  await screen.findByRole('dialog', { name: 'AI settings' })
  fireEvent.click(screen.getByRole('button', { name: 'Disconnect AI' }))
  await waitFor(() => expect(clearAISettings).toHaveBeenCalledTimes(1))
  vi.restoreAllMocks()
})
