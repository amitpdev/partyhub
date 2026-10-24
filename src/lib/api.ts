import type {
  PartySnapshot,
  PlaylistBackup,
  PlayerStatus,
  QueueState,
  ShareInfo,
  YouTubeSearchResult,
} from '../types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: string } | null
    throw new Error(payload?.error || `Request failed (${response.status})`)
  }
  return response.json() as Promise<T>
}

export function fetchParty() {
  return request<PartySnapshot>('/api/party')
}

export function addTrack(url: string) {
  return request<PartySnapshot>('/api/queue/tracks', {
    method: 'POST',
    body: JSON.stringify({ url }),
  })
}

export function searchYouTube(query: string) {
  return request<{ results: YouTubeSearchResult[] }>('/api/youtube/search', {
    method: 'POST',
    body: JSON.stringify({ query }),
  }).then((payload) => payload.results)
}

export function importQueue(queue: QueueState) {
  const tracks = queue.tracks.map(
    ({ source, providerId, url, title, channel, thumbnail, duration }) => ({
      source,
      providerId,
      url,
      title,
      channel,
      thumbnail,
      duration,
    }),
  )
  return request<PartySnapshot>('/api/queue/import', {
    method: 'POST',
    body: JSON.stringify({
      tracks,
      currentIndex: queue.currentIndex,
      volume: queue.volume,
    }),
  })
}

export function removeTrack(queueId: string) {
  return request<PartySnapshot>(`/api/queue/tracks/${encodeURIComponent(queueId)}`, {
    method: 'DELETE',
  })
}

export function queueAction(
  action: 'play' | 'next' | 'previous' | 'move' | 'clear',
  options: { queueId?: string; index?: number; direction?: -1 | 1 } = {},
) {
  return request<PartySnapshot>('/api/queue/action', {
    method: 'POST',
    body: JSON.stringify({ action, ...options }),
  })
}

export function controlPlayer(
  action: 'pause' | 'resume' | 'stop' | 'seek' | 'seekTo' | 'volume',
  value?: number,
) {
  return request<PlayerStatus>('/api/player/control', {
    method: 'POST',
    body: JSON.stringify({ action, value }),
  })
}

export function subscribeToParty(
  onSnapshot: (snapshot: PartySnapshot) => void,
  onDisconnect: () => void,
) {
  const events = new EventSource('/api/events')
  events.addEventListener('party', (event) => {
    onSnapshot(JSON.parse((event as MessageEvent<string>).data) as PartySnapshot)
  })
  events.onerror = onDisconnect
  return () => events.close()
}

export function createGuestSession(token: string, name: string) {
  return request<{ name: string }>('/api/guest/session', {
    method: 'POST',
    body: JSON.stringify({ token, name }),
  })
}

export function getShareInfo() {
  return request<ShareInfo>('/api/share')
}

export function endSharing() {
  return request<ShareInfo>('/api/share/end', { method: 'POST' })
}

export function fetchPlaylists() {
  return request<PlaylistBackup>('/api/playlists')
}
export function changePlaylist(body: {
  action: 'save' | 'rename' | 'delete' | 'import'
  id?: string
  name?: string
  backup?: PlaylistBackup
}) {
  return request<PartySnapshot>('/api/playlists', { method: 'POST', body: JSON.stringify(body) })
}
export function loadPlaylist(id: string, mode: 'append' | 'replace', discard = false) {
  return request<{ party: PartySnapshot; skipped: number }>('/api/playlists', {
    method: 'POST', body: JSON.stringify({ action: 'load', id, mode, discard }),
  })
}
