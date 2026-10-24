import type { QueueState, Track } from '../types'

function track(
  queueId: string,
  title: string,
  channel: string,
  duration: number,
  thumbnail = 'https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg',
): Track {
  return {
    queueId,
    source: 'youtube',
    providerId: 'dQw4w9WgXcQ',
    url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
    title,
    channel,
    thumbnail,
    duration,
  }
}

const worstTracks = [
  track(
    'worst-1',
    'Live at the Royal Albert Hall — The Complete Twenty-Seven Minute Extended Anniversary Performance',
    'The International Orchestra for Contemporary Music and Very Long Channel Names',
    1665,
  ),
  track('worst-2', '夜に駆ける / Racing Into The Night（ライブ・アレンジ完全版）', '音楽チャンネル公式', 261),
  track('worst-3', 'Jo', 'X', 7, ''),
  track('worst-4', 'São João, coração e verão — edição remasterizada', 'Música Brasileira • Arquivo', 3723),
  track('worst-5', 'Wedding Party Mix — Celebration Edition, Final Master v12', 'Aleksandra Wiśniewska-Kowalczyk', 8432),
  track('worst-6', 'A track with an unavailable thumbnail and unknown duration', 'Northwind Sessions', 0, ''),
]

export function devQueueFromLocation(): QueueState | null {
  if (!import.meta.env.DEV) return null
  const mode = new URLSearchParams(window.location.search).get('data')
  if (!mode) return null
  if (mode === 'empty') return { tracks: [], currentIndex: -1, volume: 80 }
  if (mode === 'one') return { tracks: [worstTracks[2]], currentIndex: 0, volume: 80 }
  if (mode === 'worst') return { tracks: worstTracks, currentIndex: 1, volume: 80 }
  return null
}
