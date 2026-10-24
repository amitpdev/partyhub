import type { QueueState, Track } from '../types'

export const STORAGE_KEY = 'partyhub.queue.v1'

export const initialQueueState: QueueState = {
  tracks: [],
  currentIndex: -1,
  volume: 80,
}

export type QueueAction =
  | { type: 'add'; track: Track }
  | { type: 'play'; index: number }
  | { type: 'next' }
  | { type: 'previous' }
  | { type: 'remove'; queueId: string }
  | { type: 'move'; queueId: string; direction: -1 | 1 }
  | { type: 'clear' }
  | { type: 'volume'; volume: number }

export function queueReducer(state: QueueState, action: QueueAction): QueueState {
  switch (action.type) {
    case 'add': {
      const tracks = [...state.tracks, action.track]
      return {
        ...state,
        tracks,
        currentIndex: state.currentIndex === -1 ? 0 : state.currentIndex,
      }
    }
    case 'play':
      return action.index >= 0 && action.index < state.tracks.length
        ? { ...state, currentIndex: action.index }
        : state
    case 'next':
      return state.currentIndex + 1 < state.tracks.length
        ? { ...state, currentIndex: state.currentIndex + 1 }
        : state
    case 'previous':
      return state.currentIndex > 0
        ? { ...state, currentIndex: state.currentIndex - 1 }
        : state
    case 'remove': {
      const index = state.tracks.findIndex((track) => track.queueId === action.queueId)
      if (index < 0) return state
      const tracks = state.tracks.filter((track) => track.queueId !== action.queueId)
      let currentIndex = state.currentIndex
      if (tracks.length === 0) currentIndex = -1
      else if (index < currentIndex) currentIndex -= 1
      else if (index === currentIndex) currentIndex = Math.min(currentIndex, tracks.length - 1)
      return { ...state, tracks, currentIndex }
    }
    case 'move': {
      const index = state.tracks.findIndex((track) => track.queueId === action.queueId)
      const destination = index + action.direction
      if (index < 0 || destination < 0 || destination >= state.tracks.length) return state
      const tracks = [...state.tracks]
      ;[tracks[index], tracks[destination]] = [tracks[destination], tracks[index]]
      let currentIndex = state.currentIndex
      if (currentIndex === index) currentIndex = destination
      else if (currentIndex === destination) currentIndex = index
      return { ...state, tracks, currentIndex }
    }
    case 'clear':
      return { ...state, tracks: [], currentIndex: -1 }
    case 'volume':
      return { ...state, volume: Math.max(0, Math.min(100, action.volume)) }
  }
}

export function loadQueue(storage: Pick<Storage, 'getItem'> = localStorage): QueueState {
  try {
    const raw = storage.getItem(STORAGE_KEY)
    if (!raw) return initialQueueState
    const parsed = JSON.parse(raw) as Partial<QueueState>
    if (!Array.isArray(parsed.tracks)) return initialQueueState
    const tracks = parsed.tracks.filter(isTrack)
    const currentIndex =
      typeof parsed.currentIndex === 'number' && tracks.length > 0
        ? Math.max(0, Math.min(tracks.length - 1, parsed.currentIndex))
        : tracks.length > 0
          ? 0
          : -1
    const volume =
      typeof parsed.volume === 'number'
        ? Math.max(0, Math.min(100, parsed.volume))
        : initialQueueState.volume
    return { tracks, currentIndex, volume }
  } catch {
    return initialQueueState
  }
}

export function saveQueue(state: QueueState, storage: Pick<Storage, 'setItem'> = localStorage) {
  storage.setItem(STORAGE_KEY, JSON.stringify(state))
}

function isTrack(value: unknown): value is Track {
  if (!value || typeof value !== 'object') return false
  const track = value as Partial<Track>
  return (
    track.source === 'youtube' &&
    typeof track.queueId === 'string' &&
    typeof track.providerId === 'string' &&
    typeof track.url === 'string' &&
    typeof track.title === 'string'
  )
}
