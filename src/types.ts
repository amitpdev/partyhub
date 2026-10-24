export type TrackSource = 'youtube'

export interface Track {
  queueId: string
  source: TrackSource
  providerId: string
  url: string
  title: string
  channel: string
  thumbnail: string
  duration: number
  addedBy?: string
  canRemove?: boolean
  canMove?: boolean
  canPlay?: boolean
}

export type YouTubeSearchResult = Pick<
  Track,
  'source' | 'providerId' | 'url' | 'title' | 'channel' | 'thumbnail' | 'duration'
>

export type PlaybackState =
  | 'idle'
  | 'loading'
  | 'playing'
  | 'paused'
  | 'stopped'
  | 'ended'
  | 'error'

export interface PlayerStatus {
  available: boolean
  missing: string[]
  state: PlaybackState
  generation: number
  trackId: string
  position: number
  duration: number
  volume: number
  error?: string
}

export interface QueueState {
  tracks: Track[]
  currentIndex: number
  volume: number
}

export interface PartySnapshot extends QueueState {
  playlist?: PlaylistLink
  revision: number
  player: PlayerStatus
  storageError?: string
}

export interface ShareInfo {
  active: boolean
  publicUrl: string
  guestUrl: string
}

export interface PlaylistLink {
  id: string
  name: string
  dirty: boolean
}
export interface Playlist {
  id: string
  name: string
  createdAt: string
  updatedAt: string
  tracks: YouTubeSearchResult[]
}
export interface PlaylistBackup {
  version: 1
  playlists: Playlist[]
}
