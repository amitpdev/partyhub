import type { PartySnapshot, PlaylistBackup, QueueState } from '../../src/types'
import { DemoEngine } from './model.ts'

const storage = (() => { try { return window.localStorage } catch { return undefined } })()
export const engine = new DemoEngine(storage)
let guest = new URLSearchParams(window.location.search).get('view') === 'guest'
let clock: number | undefined
let observers = 0

function observe(listener: () => void) {
  const unsubscribe = engine.subscribe(listener)
  observers++
  if (clock === undefined) clock = window.setInterval(() => engine.tick(), 1000)
  return () => {
    unsubscribe()
    observers--
    if (!observers) { window.clearInterval(clock); clock = undefined }
  }
}
function djOnly() { if (guest) throw new Error('DJ controls are available in the DJ view.') }
export function setDemoView(view: 'dj' | 'guest') { guest = view === 'guest' }
export function resetDemo() { engine.reset() }
export function subscribeToDemo(listener: () => void) { return observe(listener) }
export function demoInfo() { return { sharing: engine.isSharing(), name: engine.name() } }

export async function fetchParty(signal?: AbortSignal) {
  if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
  return engine.snapshot(guest)
}
export async function addTrack(url: string) { return engine.add(url, guest) }
export async function searchYouTube(query: string) { return engine.search(query) }
export async function importQueue(queue: QueueState) { djOnly(); return engine.importQueue(queue) }
export async function removeTrack(queueId: string) { return engine.remove(queueId, guest) }
export async function queueAction(action: 'play' | 'next' | 'previous' | 'move' | 'clear', options: { queueId?: string; index?: number; direction?: -1 | 1 } = {}) {
  djOnly()
  return engine.action(action, options)
}
export async function controlPlayer(action: 'pause' | 'resume' | 'stop' | 'seek' | 'seekTo' | 'volume', value?: number) {
  djOnly()
  return engine.control(action, value)
}
export function subscribeToParty(onSnapshot: (snapshot: PartySnapshot) => void, onDisconnect: () => void) {
  let active = true
  const emit = () => {
    if (!active) return
    try { onSnapshot(engine.snapshot(guest)) } catch { onDisconnect() }
  }
  const unsubscribe = observe(emit)
  queueMicrotask(emit)
  return () => { active = false; unsubscribe() }
}
export async function createGuestSession(_token: string, name: string) { return engine.join(name) }
export async function getShareInfo() {
  djOnly()
  const link = new URL(window.location.href)
  link.search = '?view=guest'
  link.hash = ''
  return { active: engine.isSharing(), publicUrl: window.location.origin, guestUrl: engine.isSharing() ? link.href : '' }
}
export async function startSharing() { djOnly(); engine.setSharing(true); return getShareInfo() }
export async function endSharing() { djOnly(); engine.setSharing(false); return getShareInfo() }
export async function fetchPlaylists() { djOnly(); return engine.library() }
export async function changePlaylist(body: { action: 'save' | 'rename' | 'delete' | 'import'; id?: string; name?: string; backup?: PlaylistBackup }) {
  djOnly()
  if (body.action === 'delete') return engine.deletePlaylist(body.id || '')
  if (body.action === 'import') return engine.importPlaylists(body.backup!)
  return engine.savePlaylist(body.id, body.name || '', body.action === 'rename')
}
export async function loadPlaylist(id: string, mode: 'append' | 'replace', discard = false) { djOnly(); return engine.loadPlaylist(id, mode, discard) }
