import type { PartySnapshot, Playlist, PlaylistBackup, QueueState, Track, YouTubeSearchResult } from '../../src/types'
import { catalog } from './catalog.ts'

const storageKey = 'partyhub.demo.v1'
const copy = <T>(value: T): T => JSON.parse(JSON.stringify(value)) as T
const id = () => crypto.randomUUID()
const now = () => new Date().toISOString()

type State = {
  version: 1
  party: PartySnapshot
  playlists: Playlist[]
  sharing: boolean
  guestName: string
}

function savedPlaylist(name: string, songs: YouTubeSearchResult[]): Playlist {
  return { id: id(), name, tracks: copy(songs), createdAt: now(), updatedAt: now() }
}

function freshState(): State {
  const playlists = [
    savedPlaylist('Rooftop warm-up', catalog.slice(0, 7)),
    savedPlaylist('After hours', [catalog[7], catalog[2], catalog[10], catalog[11], catalog[15]]),
    savedPlaylist('Weekend sunshine', [catalog[4], catalog[9], catalog[5], catalog[13], catalog[3]]),
  ]
  const tracks: Track[] = playlists[0].tracks.map((track, index) => ({ ...track, queueId: id(), addedBy: index === 3 ? 'Maya' : index === 5 ? 'Noam' : 'DJ' }))
  return {
    version: 1, playlists, sharing: true, guestName: 'Demo friend',
    party: {
      revision: 1, tracks, currentIndex: 1, volume: 70,
      playlist: { id: playlists[0].id, name: playlists[0].name, dirty: false },
      player: { available: true, missing: [], state: 'paused', generation: 1, trackId: tracks[1].queueId, position: 62, duration: tracks[1].duration, volume: 70 },
    },
  }
}

export class DemoEngine {
  private state: State
  private listeners = new Set<() => void>()
  private storage?: Pick<Storage, 'getItem' | 'setItem'>

  constructor(storage?: Pick<Storage, 'getItem' | 'setItem'>) {
    this.storage = storage
    this.state = freshState()
    try {
      const saved: State = JSON.parse(storage?.getItem(storageKey) || 'null')
      if (saved?.version === 1 && Array.isArray(saved.party?.tracks) && Array.isArray(saved.playlists) && saved.party.player) {
        this.state = saved
        this.state.party.player.state = saved.party.tracks.length ? 'paused' : 'idle'
        this.state.party.player.available = true
      }
    } catch { /* A broken or unavailable browser store starts a fresh demo. */ }
  }

  subscribe(listener: () => void) {
    this.listeners.add(listener)
    return () => { this.listeners.delete(listener) }
  }

  private changed(editorial = false, persist = true) {
    if (editorial && this.state.party.playlist) this.state.party.playlist.dirty = true
    this.state.party.revision++
    if (persist) {
      try { this.storage?.setItem(storageKey, JSON.stringify(this.state)) } catch { /* The demo can run in memory. */ }
    }
    for (const listener of this.listeners) listener()
  }

  reset() {
    this.state = freshState()
    this.changed()
  }

  snapshot(guest = false): PartySnapshot {
    if (guest && !this.state.sharing) throw new Error('Guest access is off. Return to the DJ view to allow guests.')
    const result = copy(this.state.party)
    result.tracks = result.tracks.map((track, index) => ({
      ...track,
      canPlay: !guest,
      canMove: !guest && index > result.currentIndex,
      canRemove: index !== result.currentIndex && (!guest || (index > result.currentIndex && track.addedBy === this.state.guestName)),
    }))
    if (guest) delete result.playlist
    return result
  }

  library(): PlaylistBackup { return { version: 1, playlists: copy(this.state.playlists) } }
  isSharing() { return this.state.sharing }
  name() { return this.state.guestName }

  search(query: string) {
    const words = query.toLowerCase().trim().split(/\s+/)
    const matches = catalog.filter((song) => words.every((word) => `${song.title} ${song.channel}`.toLowerCase().includes(word)))
    if (matches.length) return copy(matches)
    const queued = new Set(this.state.party.tracks.map((track) => track.providerId))
    const available = catalog.filter((song) => !queued.has(song.providerId))
    return copy((available.length ? available : catalog).slice(0, 6))
  }

  add(url: string, guest = false) {
    this.snapshot(guest)
    const parsed = new URL(url)
    if (!['www.youtube.com', 'youtube.com', 'm.youtube.com', 'youtu.be'].includes(parsed.hostname)) throw new Error('Paste a YouTube video link, or search the sample catalog.')
    const providerId = parsed.hostname === 'youtu.be' ? parsed.pathname.slice(1) : parsed.searchParams.get('v') || ''
    if (!/^[A-Za-z0-9_-]{11}$/.test(providerId) || parsed.searchParams.has('list')) throw new Error('Use a single YouTube video link.')
    const party = this.state.party
    const upcoming = party.tracks.slice(Math.max(0, party.currentIndex))
    if (upcoming.some((track) => track.providerId === providerId)) throw new Error('This song is already in the queue.')
    if (party.tracks.length - party.currentIndex - 1 >= 100) throw new Error('The queue is full.')
    if (guest && party.tracks.slice(party.currentIndex + 1).filter((track) => track.addedBy === this.state.guestName).length >= 3) throw new Error('You can have up to three upcoming tracks.')
    const song = catalog.find((song) => song.providerId === providerId) || {
      source: 'youtube' as const, providerId, url: `https://www.youtube.com/watch?v=${providerId}`,
      title: 'Your YouTube pick', channel: 'Demo track · playback simulated', duration: 240, thumbnail: catalog[0].thumbnail,
    }
    party.tracks.push({ ...song, queueId: id(), addedBy: guest ? this.state.guestName : 'DJ' })
    if (party.currentIndex < 0) this.select(0, 'stopped')
    this.changed(true)
    return this.snapshot(guest)
  }

  remove(queueId: string, guest = false) {
    const view = this.snapshot(guest)
    const index = view.tracks.findIndex((track) => track.queueId === queueId)
    if (index < 0 || !view.tracks[index].canRemove) throw new Error('This track cannot be removed.')
    this.state.party.tracks.splice(index, 1)
    if (index < this.state.party.currentIndex) this.state.party.currentIndex--
    this.changed(true)
    return this.snapshot(guest)
  }

  private select(index: number, state: 'playing' | 'stopped') {
    const party = this.state.party
    const track = party.tracks[index]
    if (!track) throw new Error('There is no track in that direction.')
    party.currentIndex = index
    party.player = { ...party.player, generation: party.player.generation + 1, state, trackId: track.queueId, duration: track.duration, position: 0 }
  }

  action(action: 'play' | 'next' | 'previous' | 'move' | 'clear', options: { index?: number; queueId?: string; direction?: -1 | 1 } = {}) {
    const party = this.state.party
    if (action === 'play') this.select(options.index ?? party.currentIndex, 'playing')
    if (action === 'next' || action === 'previous') this.select(party.currentIndex + (action === 'next' ? 1 : -1), 'playing')
    if (action === 'move') {
      const index = party.tracks.findIndex((track) => track.queueId === options.queueId)
      const next = index + (options.direction || 0)
      if (index <= party.currentIndex || next <= party.currentIndex || next >= party.tracks.length) throw new Error('That track cannot be moved there.')
      ;[party.tracks[index], party.tracks[next]] = [party.tracks[next], party.tracks[index]]
    }
    if (action === 'clear') {
      party.tracks = []
      party.currentIndex = -1
      party.player = { ...party.player, state: 'idle', trackId: '', position: 0, duration: 0 }
    }
    this.changed(action === 'clear' || action === 'move')
    return this.snapshot()
  }

  control(action: 'pause' | 'resume' | 'stop' | 'seek' | 'seekTo' | 'volume', value = 0) {
    const party = this.state.party
    if (!Number.isFinite(value)) throw new Error('Use a valid playback value.')
    if (action === 'pause') party.player.state = 'paused'
    if (action === 'resume' && party.currentIndex >= 0) party.player.state = 'playing'
    if (action === 'stop') party.player.state = 'stopped'
    if (action === 'seek' || action === 'seekTo') party.player.position = Math.max(0, Math.min(party.player.duration, action === 'seek' ? party.player.position + value : value))
    if (action === 'volume') party.volume = party.player.volume = Math.max(0, Math.min(100, value))
    this.changed()
    return copy(party.player)
  }

  tick() {
    const party = this.state.party
    if (party.player.state !== 'playing') return
    party.player.position++
    if (party.player.position >= party.player.duration) {
      if (party.currentIndex + 1 < party.tracks.length) this.select(party.currentIndex + 1, 'playing')
      else { party.player.state = 'ended'; party.player.position = party.player.duration }
    }
    this.changed(false, false)
  }

  setSharing(active: boolean) {
    this.state.sharing = active
    this.changed()
  }

  join(name: string) {
    if (!this.state.sharing) throw new Error('Guest access is off.')
    if (!name.trim()) throw new Error('Enter a display name.')
    this.state.guestName = name.trim().slice(0, 30)
    this.changed()
    return { name: this.state.guestName }
  }

  savePlaylist(playlistId: string | undefined, name: string, rename = false) {
    const cleanName = name.trim()
    if (!cleanName || [...cleanName].length > 80) throw new Error('Use a name between 1 and 80 characters.')
    const existing = this.state.playlists.find((item) => item.id === playlistId)
    if (playlistId && !existing) throw new Error('Playlist not found.')
    const songs = rename ? existing!.tracks : this.state.party.tracks.map(({ source, providerId, url, title, channel, thumbnail, duration }) => ({ source, providerId, url, title, channel, thumbnail, duration }))
    if (!songs.length) throw new Error('Add a song before saving the queue.')
    const playlist = { ...(existing || savedPlaylist(cleanName, songs)), name: cleanName, tracks: copy(songs), updatedAt: now() }
    if (existing) this.state.playlists[this.state.playlists.indexOf(existing)] = playlist
    else this.state.playlists.push(playlist)
    if (!rename) this.state.party.playlist = { id: playlist.id, name: playlist.name, dirty: false }
    else if (this.state.party.playlist?.id === playlist.id) this.state.party.playlist.name = playlist.name
    this.changed()
    return this.snapshot()
  }

  deletePlaylist(playlistId: string) {
    if (!this.state.playlists.some((item) => item.id === playlistId)) throw new Error('Playlist not found.')
    this.state.playlists = this.state.playlists.filter((item) => item.id !== playlistId)
    if (this.state.party.playlist?.id === playlistId) delete this.state.party.playlist
    this.changed()
    return this.snapshot()
  }

  importPlaylists(backup: PlaylistBackup) {
    if (backup?.version !== 1 || !Array.isArray(backup.playlists) || backup.playlists.length > 1000) throw new Error('Choose a Party Hub playlist JSON file.')
    const imports = backup.playlists.map((playlist) => {
      if (typeof playlist.name !== 'string' || !playlist.name.trim() || [...playlist.name.trim()].length > 80 || !Array.isArray(playlist.tracks) || !playlist.tracks.length || playlist.tracks.length > 201) throw new Error('The file contains an invalid playlist.')
      const tracks = playlist.tracks.map((track) => {
        if (track.source !== 'youtube' || !/^[A-Za-z0-9_-]{11}$/.test(track.providerId) || typeof track.title !== 'string' || !track.title.trim() || !Number.isFinite(track.duration) || track.duration < 0) throw new Error('The file contains invalid track metadata.')
        return { source: 'youtube' as const, providerId: track.providerId, title: track.title, channel: typeof track.channel === 'string' ? track.channel : '', duration: track.duration, url: `https://www.youtube.com/watch?v=${track.providerId}`, thumbnail: catalog.find((song) => song.providerId === track.providerId)?.thumbnail || catalog[0].thumbnail }
      })
      return savedPlaylist(playlist.name.trim(), tracks)
    })
    if (this.state.playlists.length + imports.length > 1000) throw new Error('The demo library is full.')
    this.state.playlists.push(...imports)
    this.changed()
    return this.snapshot()
  }

  loadPlaylist(playlistId: string, mode: 'append' | 'replace', discard = false) {
    const selected = this.state.playlists.find((item) => item.id === playlistId)
    if (!selected) throw new Error('Playlist not found.')
    const party = this.state.party
    if (mode === 'replace' && party.playlist?.dirty && !discard) throw new Error('Save or discard playlist changes before loading another set.')
    const seen = new Set(mode === 'append' ? party.tracks.slice(Math.max(0, party.currentIndex)).map((track) => track.providerId) : [])
    let skipped = 0
    const tracks: Track[] = []
    for (const track of selected.tracks) {
      if (seen.has(track.providerId)) { skipped++; continue }
      seen.add(track.providerId)
      tracks.push({ ...copy(track), queueId: id(), addedBy: 'DJ' })
    }
    const upcoming = party.currentIndex < 0 ? 0 : party.tracks.length - party.currentIndex - 1
    if (tracks.length > (mode === 'replace' ? 101 : 100 - upcoming + (party.currentIndex < 0 ? 1 : 0))) throw new Error('That playlist exceeds the queue capacity.')
    if (mode === 'replace') {
      party.tracks = tracks
      party.currentIndex = -1
      party.playlist = { id: selected.id, name: selected.name, dirty: skipped > 0 }
      if (tracks.length) this.select(0, 'stopped')
    } else {
      party.tracks.push(...tracks)
      if (party.currentIndex < 0 && tracks.length) this.select(0, 'stopped')
      if (tracks.length && party.playlist) party.playlist.dirty = true
    }
    this.changed()
    return { party: this.snapshot(), skipped }
  }

  importQueue(queue: QueueState) {
    if (this.state.party.tracks.length) return this.snapshot()
    this.state.party.tracks = copy(queue.tracks)
    if (queue.tracks.length) this.select(Math.max(0, Math.min(queue.tracks.length - 1, queue.currentIndex)), 'stopped')
    this.state.party.volume = this.state.party.player.volume = queue.volume
    this.changed()
    return this.snapshot()
  }
}
