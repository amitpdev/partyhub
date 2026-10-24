import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { usePartyPlayer } from './usePartyPlayer'
import { controlPlayer, fetchParty, importQueue, queueAction, subscribeToParty } from '../lib/api'
import type { PartySnapshot } from '../types'

vi.mock('../lib/api', () => ({
  fetchParty: vi.fn(), subscribeToParty: vi.fn(), importQueue: vi.fn(),
  controlPlayer: vi.fn(), queueAction: vi.fn(), addTrack: vi.fn(), removeTrack: vi.fn(),
}))
const snapshot: PartySnapshot = {
  revision: 1, tracks: [], currentIndex: -1, volume: 80,
  player: { available: true, missing: [], state: 'stopped', generation: 1, trackId: '', position: 0, duration: 0, volume: 80 },
}
const closeStream = vi.fn()
let receive: (snapshot: PartySnapshot) => void
let disconnect: () => void
beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers()
  localStorage.clear()
  vi.mocked(subscribeToParty).mockImplementation((onSnapshot, onDisconnect) => {
    receive = onSnapshot
    disconnect = onDisconnect
    return closeStream
  })
})
afterEach(() => { cleanup(); vi.useRealTimers() })

it('recovers from an offline startup every three seconds and reconnects the stream', async () => {
  vi.mocked(fetchParty).mockRejectedValueOnce(new Error('offline')).mockResolvedValue(snapshot)
  const { result } = renderHook(() => usePartyPlayer())
  await act(async () => {})
  expect(result.current.status.available).toBe(false)
  await act(async () => { await vi.advanceTimersByTimeAsync(2999) })
  expect(fetchParty).toHaveBeenCalledTimes(1)
  await act(async () => { await vi.advanceTimersByTimeAsync(1) })
  expect(result.current.status.available).toBe(true)
  expect(result.current.recoveryVersion).toBe(1)
  expect(subscribeToParty).toHaveBeenCalledTimes(2)
  expect(closeStream).toHaveBeenCalledTimes(1)
  await act(async () => { await vi.advanceTimersByTimeAsync(9000) })
  expect(fetchParty).toHaveBeenCalledTimes(2)
  expect(controlPlayer).not.toHaveBeenCalled()
  expect(queueAction).not.toHaveBeenCalled()
  expect(importQueue).not.toHaveBeenCalled()
})

it('recovers after a running service stops without discarding the displayed queue', async () => {
  const running = { ...snapshot, revision: 4, tracks: [{ source: 'youtube' as const, providerId: 'aaaaaaaaaaa', queueId: 'q1', url: 'https://www.youtube.com/watch?v=aaaaaaaaaaa', title: 'Song', channel: 'Artist', thumbnail: '', duration: 180 }], currentIndex: 0 }
  vi.mocked(fetchParty).mockResolvedValueOnce(running).mockResolvedValueOnce(snapshot)
  const { result } = renderHook(() => usePartyPlayer())
  await act(async () => {})
  act(() => disconnect())
  expect(result.current.status.available).toBe(false)
  expect(result.current.party.tracks).toEqual(running.tracks)
  await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
  expect(result.current.status.available).toBe(true)
  expect(result.current.party).toEqual(snapshot)
  expect(result.current.recoveryVersion).toBe(2)
})

it('keeps a newer stream snapshot when an older recovery request finishes late', async () => {
  let resolve: (value: PartySnapshot) => void = () => {}
  vi.mocked(fetchParty).mockResolvedValueOnce(snapshot).mockImplementationOnce(() => new Promise((done) => { resolve = done }))
  const { result } = renderHook(() => usePartyPlayer())
  await act(async () => {})
  act(() => disconnect())
  await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
  const newer = { ...snapshot, revision: 12, volume: 35 }
  act(() => receive(newer))
  await act(async () => { resolve(snapshot) })
  expect(result.current.party).toEqual(newer)
  expect(subscribeToParty).toHaveBeenCalledTimes(1)
})

it('aborts and cleans up outstanding recovery work on unmount', async () => {
  vi.mocked(fetchParty).mockRejectedValueOnce(new Error('offline')).mockImplementationOnce(() => new Promise(() => {}))
  const { unmount } = renderHook(() => usePartyPlayer())
  await act(async () => {})
  await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
  const signal = vi.mocked(fetchParty).mock.calls[1][0]
  unmount()
  expect(signal?.aborted).toBe(true)
  expect(closeStream).toHaveBeenCalledTimes(1)
  await act(async () => { await vi.advanceTimersByTimeAsync(9000) })
  expect(fetchParty).toHaveBeenCalledTimes(2)
})

it('does not poll a connected service that reports missing playback tools', async () => {
  vi.mocked(fetchParty).mockResolvedValue({ ...snapshot, player: { ...snapshot.player, available: false, missing: ['mpv'] } })
  const { result } = renderHook(() => usePartyPlayer())
  await act(async () => {})
  expect(result.current.status.missing).toEqual(['mpv'])
  await act(async () => { await vi.advanceTimersByTimeAsync(9000) })
  expect(fetchParty).toHaveBeenCalledTimes(1)
})
