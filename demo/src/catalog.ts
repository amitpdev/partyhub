import type { YouTubeSearchResult } from '../../src/types'

const songs = [
  ['Neon Afterglow', 'Orbit Club', 248],
  ['Glasshouse Groove', 'The Night Shift', 214],
  ['Velvet Skies', 'Luna Arcade', 267],
  ['Rooftop Radio', 'Sunday People', 192],
  ['Electric Summer', 'Palma Social', 231],
  ['Golden Hour Drive', 'Coastline FM', 283],
  ['After Midnight', 'Disco Department', 256],
  ['Blue Hour', 'Paper Satellites', 224],
  ['Stay for One More', 'Orbit Club', 209],
  ['Poolside Polaroids', 'Palma Social', 238],
  ['City Lights', 'Luna Arcade', 271],
  ['Last Dance Home', 'The Night Shift', 294],
  ['Neon Heartbeat', 'Disco Department', 246],
  ['Weekend Frequency', 'Coastline FM', 218],
  ['Under the Mirrorball', 'Sunday People', 265],
  ['Soft Landing', 'Paper Satellites', 203],
] as const

export const catalog: YouTubeSearchResult[] = songs.map(([title, channel, duration], index) => ({
  source: 'youtube',
  providerId: `demo${String(index + 1).padStart(7, '0')}`,
  url: `https://www.youtube.com/watch?v=demo${String(index + 1).padStart(7, '0')}`,
  title,
  channel,
  duration,
  thumbnail: `./artwork/cover-${String(index % 8 + 1).padStart(2, '0')}.svg`,
}))
