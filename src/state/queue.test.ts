import { describe, expect, it } from 'vitest'
import type { Track } from '../types'
import {
  STORAGE_KEY,
  initialQueueState,
  loadQueue,
  queueReducer,
} from './queue'

const first: Track = {
  queueId: 'first',
  source: 'youtube',
  providerId: 'dQw4w9WgXcQ',
  url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
  title: 'First',
  channel: 'Channel',
  thumbnail: '',
  duration: 60,
}

const second = { ...first, queueId: 'second', title: 'Second' }

describe('queueReducer', () => {
  it('adds the first track and selects it', () => {
    expect(queueReducer(initialQueueState, { type: 'add', track: first })).toMatchObject({
      tracks: [first],
      currentIndex: 0,
    })
  })

  it('keeps the current track selected when rows move around it', () => {
    const state = { tracks: [first, second], currentIndex: 1, volume: 80 }
    const result = queueReducer(state, { type: 'move', queueId: first.queueId, direction: 1 })
    expect(result.tracks).toEqual([second, first])
    expect(result.currentIndex).toBe(0)
  })

  it('adjusts the cursor when an earlier track is removed', () => {
    const state = { tracks: [first, second], currentIndex: 1, volume: 80 }
    const result = queueReducer(state, { type: 'remove', queueId: first.queueId })
    expect(result.tracks).toEqual([second])
    expect(result.currentIndex).toBe(0)
  })
})

describe('loadQueue', () => {
  it('recovers from corrupt storage', () => {
    const storage = { getItem: () => '{not json' }
    expect(loadQueue(storage)).toEqual(initialQueueState)
  })

  it('clamps persisted values', () => {
    const storage = {
      getItem: (key: string) =>
        key === STORAGE_KEY
          ? JSON.stringify({ tracks: [first], currentIndex: 99, volume: 200 })
          : null,
    }
    expect(loadQueue(storage)).toMatchObject({ currentIndex: 0, volume: 100 })
  })
})
