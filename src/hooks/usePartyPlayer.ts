import { useCallback, useEffect, useRef, useState } from 'react'
import {
  addTrack,
  controlPlayer,
  fetchParty,
  importQueue,
  queueAction,
  removeTrack,
  subscribeToParty,
} from '../lib/api'
import { loadQueue } from '../state/queue'
import type { PartySnapshot, PlayerStatus } from '../types'

const disconnectedStatus: PlayerStatus = {
  available: false,
  missing: [],
  state: 'idle',
  generation: 0,
  trackId: '',
  position: 0,
  duration: 0,
  volume: 80,
}

const initialParty: PartySnapshot = {
  revision: 0,
  tracks: [],
  currentIndex: -1,
  volume: 80,
  player: disconnectedStatus,
}

export function usePartyPlayer() {
  const [party, setParty] = useState<PartySnapshot>(initialParty)
  const [notice, setNotice] = useState('')
  const [recoveryVersion, setRecoveryVersion] = useState(0)
  const partyRef = useRef(party)
  partyRef.current = party

  useEffect(() => {
    let alive = true
    let disconnected = true
    let pending = false
    let snapshotVersion = 0
    let streamGeneration = 0
    let unsubscribe = () => {}
    let controller: AbortController | undefined

    function receive(snapshot: PartySnapshot) {
      if (!alive) return
      snapshotVersion++
      if (disconnected) setRecoveryVersion((version) => version + 1)
      disconnected = false
      setParty(snapshot)
    }

    function connectEvents() {
      unsubscribe()
      const generation = ++streamGeneration
      unsubscribe = subscribeToParty(
        (snapshot) => {
          if (generation === streamGeneration) receive(snapshot)
        },
        () => {
          if (!alive || generation !== streamGeneration) return
          disconnected = true
          setParty((current) => ({
            ...current,
            player: { ...current.player, available: false },
          }))
        },
      )
    }

    async function recover(bootstrap = false) {
      if (!alive || pending || !disconnected) return
      pending = true
      const version = snapshotVersion
      const request = new AbortController()
      controller = request
      const timeout = window.setTimeout(() => request.abort(), 2500)
      try {
        let snapshot = await fetchParty(request.signal)
        if (!alive || request.signal.aborted || version !== snapshotVersion) return
        if (bootstrap && !window.location.pathname.startsWith('/guest') && snapshot.tracks.length === 0) {
          const localQueue = loadQueue()
          if (localQueue.tracks.length > 0) snapshot = await importQueue(localQueue)
        }
        // A newer stream event wins over a slower recovery request.
        if (!alive || request.signal.aborted || version !== snapshotVersion) return
        receive(snapshot)
        if (!bootstrap) connectEvents()
      } catch {
        // Keep the queue visible and retry while the service is unreachable.
      } finally {
        window.clearTimeout(timeout)
        pending = false
      }
    }

    void recover(true)
    connectEvents()
    const retry = window.setInterval(() => void recover(), 3000)
    return () => {
      alive = false
      streamGeneration++
      window.clearInterval(retry)
      controller?.abort()
      unsubscribe()
    }
  }, [])

  const update = useCallback(async (operation: () => Promise<PartySnapshot>) => {
    try {
      setParty(await operation())
    } catch (error) {
      setNotice(messageFrom(error))
    }
  }, [])

  const play = useCallback(
    async (index = partyRef.current.currentIndex) => update(() => queueAction('play', { index })),
    [update],
  )

  const next = useCallback(
    async () => update(() => queueAction('next')),
    [update],
  )

  const previous = useCallback(
    async () => update(() => queueAction('previous')),
    [update],
  )

  const pauseOrResume = useCallback(async () => {
    const action = party.player.state === 'paused' ? 'resume' : 'pause'
    try {
      const player = await controlPlayer(action)
      setParty((current) => ({ ...current, player }))
    } catch (error) {
      setNotice(messageFrom(error))
    }
  }, [party.player.state])

  const seek = useCallback(async (offset: number) => {
    try {
      const player = await controlPlayer('seek', offset)
      setParty((current) => ({ ...current, player }))
    } catch (error) {
      setNotice(messageFrom(error))
    }
  }, [])

  const seekTo = useCallback(async (position: number) => {
    setParty((current) => ({ ...current, player: { ...current.player, position } }))
    try {
      const player = await controlPlayer('seekTo', position)
      setParty((current) => ({ ...current, player }))
    } catch (error) {
      setNotice(messageFrom(error))
    }
  }, [])

  const stop = useCallback(async () => {
    try {
      const player = await controlPlayer('stop')
      setParty((current) => ({ ...current, player }))
    } catch {
      setParty((current) => ({ ...current, player: disconnectedStatus }))
    }
  }, [])

  const setVolume = useCallback(async (volume: number) => {
    setParty((current) => ({ ...current, volume }))
    try {
      const player = await controlPlayer('volume', volume)
      setParty((current) => ({ ...current, player, volume }))
    } catch (error) {
      setNotice(`${messageFrom(error)}. The volume will apply when playback starts.`)
    }
  }, [])

  const add = useCallback(
    async (url: string) => {
      try {
        setParty(await addTrack(url))
      } catch (error) {
        setNotice(messageFrom(error))
        throw error
      }
    },
    [],
  )

  const remove = useCallback(
    async (queueId: string) => update(() => removeTrack(queueId)),
    [update],
  )

  const move = useCallback(
    async (queueId: string, direction: -1 | 1) =>
      update(() => queueAction('move', { queueId, direction })),
    [update],
  )

  const clear = useCallback(async () => update(() => queueAction('clear')), [update])

  return {
    party,
    recoveryVersion,
    acceptSnapshot: setParty,
    status: party.player,
    notice,
    clearNotice: () => setNotice(''),
    add,
    remove,
    move,
    clear,
    play,
    next,
    previous,
    pauseOrResume,
    seek,
    seekTo,
    stop,
    setVolume,
  }
}

function messageFrom(error: unknown) {
  return error instanceof Error ? error.message : 'The player could not complete that action'
}
