import { FolderOpen, ListMusic, MoreHorizontal, Save, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { changePlaylist, fetchPlaylists, loadPlaylist } from '../lib/api'
import { formatDuration } from '../lib/format'
import type { PartySnapshot, Playlist, PlaylistBackup } from '../types'
import { PlayerRecoveryHelp } from './PlayerRecovery'

interface Props {
  party: PartySnapshot
  onSnapshot: (party: PartySnapshot) => void
  recoveryVersion?: number
}

export function PlaylistLibrary({ party, onSnapshot, recoveryVersion = 0 }: Props) {
  const dialog = useRef<HTMLDialogElement>(null)
  const saveDialog = useRef<HTMLDialogElement>(null)
  const fileInput = useRef<HTMLInputElement>(null)
  const previousRecovery = useRef(recoveryVersion)
  const [items, setItems] = useState<Playlist[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [ready, setReady] = useState(false)
  const [showHelp, setShowHelp] = useState(false)
  const [menuID, setMenuID] = useState('')
  const [renaming, setRenaming] = useState<Playlist | null>(null)
  const [name, setName] = useState('')
  const [pending, setPending] = useState<Playlist | null>(null)
  const [deleting, setDeleting] = useState<Playlist | null>(null)
  const active = party.playlist
  const unsaved = active ? active.dirty : party.tracks.length > 0

  const refresh = useCallback(async () => {
    setReady(false)
    const library = await fetchPlaylists()
    setItems(library.playlists)
    setReady(true)
  }, [])

  useEffect(() => {
    if (previousRecovery.current === recoveryVersion) return
    previousRecovery.current = recoveryVersion
    if (!dialog.current?.open || ready) return
    void refresh().then(() => {
      setError('')
      setShowHelp(false)
    }).catch((error: unknown) => setError(message(error)))
  }, [recoveryVersion, ready, refresh])

  function reset() {
    setShowHelp(false)
    setError('')
    setNotice('')
    setPending(null)
    setDeleting(null)
    setRenaming(null)
    setMenuID('')
  }

  function openLibrary() {
    reset()
    dialog.current?.showModal()
    void refresh().catch((err: unknown) => setError(message(err)))
  }

  function openSave() {
    reset()
    setName(active?.name || '')
    saveDialog.current?.showModal()
  }

  async function run(operation: () => Promise<void>) {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await operation()
    } catch (err) {
      setError(message(err))
    } finally {
      setBusy(false)
    }
  }

  async function saveQueue(asNew = false) {
    onSnapshot(await changePlaylist({ action: 'save', id: asNew ? undefined : active?.id, name: name.trim() }))
    saveDialog.current?.close()
  }

  async function load(item: Playlist, discard = false) {
    const result = await loadPlaylist(item.id, 'replace', discard)
    onSnapshot(result.party)
    setPending(null)
    dialog.current?.close()
  }

  function playlistFile(item: Playlist) {
    const backup: PlaylistBackup = { version: 1, playlists: [item] }
    const filename = `${item.name.replace(/[^a-zA-Z0-9_-]+/g, '-').slice(0, 80) || 'playlist'}.json`
    return new File([JSON.stringify(backup, null, 2)], filename, { type: 'application/json' })
  }

  function download(item: Playlist) {
    const file = playlistFile(item)
    const url = URL.createObjectURL(file)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = file.name
    anchor.click()
    const revoke = URL.revokeObjectURL.bind(URL)
    setTimeout(() => revoke(url), 1000)
    setMenuID('')
  }

  async function share(item: Playlist) {
    const file = playlistFile(item)
    if (!navigator.share || !navigator.canShare?.({ files: [file] })) {
      download(item)
      setNotice('File sharing is unavailable in this browser. The playlist was downloaded so you can send it yourself.')
      return
    }
    try {
      await navigator.share({ title: item.name, files: [file] })
      setMenuID('')
    } catch (error) {
      if (typeof error === 'object' && error !== null && 'name' in error && error.name === 'AbortError') return
      throw new Error('Could not share the playlist. Try again, or use Export to download it.')
    }
  }

  async function importFile(file: File) {
    if (file.size > 8 * 1024 * 1024) throw new Error('Choose a JSON file smaller than 8 MB.')
    const backup = JSON.parse(await file.text()) as PlaylistBackup
    onSnapshot(await changePlaylist({ action: 'import', backup }))
    await refresh()
    setNotice('File added. Select a playlist to use it.')
  }

  const feedback = <>
    {error && <p role="alert" className="playlist-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
  </>

  return (
    <div className="playlist-toolbar">
      <button
        className="playlist-trigger"
        type="button"
        aria-label="Playlist"
        aria-haspopup="dialog"
        title="Playlist"
        onClick={openLibrary}
      >
        <ListMusic aria-hidden="true" />
        <span>Playlist</span>
      </button>
      <button className={`playlist-trigger playlist-save-trigger${unsaved ? ' is-dirty' : ''}`} type="button" aria-label="Save playlist" aria-describedby="playlist-save-status" title={unsaved ? 'Save playlist · Unsaved changes' : 'Save current queue'} aria-haspopup="dialog" disabled={!party.tracks.length} onClick={openSave}>
        <Save aria-hidden="true" />
        {unsaved && <i aria-hidden="true" />}
      </button>
      <span id="playlist-save-status" className="sr-only" role="status">
        {unsaved ? 'Unsaved playlist changes' : 'All playlist changes saved'}
      </span>

      <dialog ref={dialog} className={showHelp ? 'player-help-dialog' : 'playlist-dialog'} aria-labelledby={showHelp ? 'playlist-help-title' : 'playlist-title'} onCancel={(event) => {
        if (showHelp) { event.preventDefault(); setShowHelp(false) }
        else if (busy) event.preventDefault()
      }}>
        {showHelp ? (
          <PlayerRecoveryHelp missing={party.player.missing} titleId="playlist-help-title" onClose={() => setShowHelp(false)} />
        ) : (
          <>
            <header className="playlist-dialog-heading">
              <h2 id="playlist-title">Playlists</h2>
              <div className="playlist-actions">
                <button className="button button--secondary" disabled={busy} onClick={() => fileInput.current?.click()} title="Load playlists from a file"><FolderOpen size={17} aria-hidden="true" />Load</button>
                <button className="playlist-close" disabled={busy} onClick={() => dialog.current?.close()} aria-label="Close playlists"><X aria-hidden="true" /></button>
              </div>
            </header>
            <p className="playlist-hint">Choose a saved playlist, or load one from a file.</p>
            {feedback}
            <fieldset disabled={busy} className="playlist-controls">
              <input ref={fileInput} type="file" accept="application/json,.json" aria-label="Load playlist file" hidden onChange={(event) => {
                const file = event.target.files?.[0]
                event.target.value = ''
                if (file) void run(() => importFile(file))
              }} />
              {renaming && <form className="playlist-editor" onSubmit={(event) => {
                event.preventDefault()
                void run(async () => {
                  onSnapshot(await changePlaylist({ action: 'rename', id: renaming.id, name: name.trim() }))
                  setRenaming(null)
                  await refresh()
                })
              }}>
                <label htmlFor="playlist-rename">Rename playlist</label>
                <input id="playlist-rename" autoFocus required maxLength={80} value={name} onChange={(event) => setName(event.target.value)} />
                <button className="button button--primary" disabled={!name.trim()}>Save</button>
                <button type="button" className="text-button" onClick={() => setRenaming(null)}>Cancel</button>
              </form>}
              {pending && <div className="playlist-confirm" role="group" aria-label="Confirm playlist replacement">
                <p>{active?.dirty ? `Save changes to “${active.name}” before loading “${pending.name}”?` : `Load “${pending.name}”? This replaces the queue and stops playback.`}</p>
                {active?.dirty && <button disabled={!party.tracks.length} className="text-button" onClick={() => void run(async () => {
                  onSnapshot(await changePlaylist({ action: 'save', id: active.id, name: active.name }))
                  await load(pending)
                })}>Save and load</button>}
                <button className="text-button" onClick={() => void run(() => load(pending, true))}>{active?.dirty ? 'Load without saving' : 'Yes'}</button>
                <button className="text-button" onClick={() => setPending(null)}>Cancel</button>
              </div>}
              {deleting && <div className="playlist-confirm" role="group" aria-label="Confirm playlist deletion">
                <p>Delete “{deleting.name}”? The current queue will stay available.</p>
                <button className="text-button" onClick={() => void run(async () => {
                  onSnapshot(await changePlaylist({ action: 'delete', id: deleting.id }))
                  setDeleting(null)
                  await refresh()
                })}>Delete playlist</button>
                <button className="text-button" onClick={() => setDeleting(null)}>Cancel</button>
              </div>}
              <div className="playlist-list" aria-busy={busy || !ready}>
                {!ready && !error && <p>Loading playlists…</p>}
                {!ready && error && <div className="playlist-actions">
                  <button className="text-button" onClick={() => void run(refresh)}>Retry</button>
                  <button className="text-button sharing-help-trigger" onClick={() => setShowHelp(true)}>More info</button>
                </div>}
                {ready && !items.length && <p className="playlist-empty">No playlists yet. Load a file, or use the Save button in the top bar to save your queue.</p>}
                {ready && items.map((item) => <article key={item.id} className="playlist-entry">
                  <div className="playlist-entry-row">
                    <button className="playlist-entry-main" aria-label={`Load ${item.name}`} onClick={() => { setPending(item); setMenuID(''); setDeleting(null); setRenaming(null) }}>
                      <ListMusic aria-hidden="true" />
                      <span><strong>{item.name}</strong><small>{item.tracks.length} tracks · {formatDuration(item.tracks.reduce((sum, track) => sum + track.duration, 0))}</small></span>
                    </button>
                    <button className="playlist-more" aria-label={`Actions for ${item.name}`} aria-expanded={menuID === item.id} onClick={() => setMenuID(menuID === item.id ? '' : item.id)}><MoreHorizontal aria-hidden="true" /></button>
                  </div>
                  {menuID === item.id && <div className="playlist-entry-actions" role="group" aria-label={`Actions for ${item.name}`}>
                    <button className="text-button" onClick={() => { setRenaming(item); setName(item.name); setMenuID(''); setPending(null); setDeleting(null) }}>Rename</button>
                    <button className="text-button" onClick={() => download(item)}>Export</button>
                    <button className="text-button" onClick={() => void run(() => share(item))}>Share</button>
                    <button className="text-button" onClick={() => { setDeleting(item); setMenuID(''); setPending(null); setRenaming(null) }}>Delete</button>
                  </div>}
                </article>)}
              </div>
            </fieldset>
          </>
        )}
      </dialog>

      <dialog ref={saveDialog} className="playlist-dialog playlist-save-dialog" aria-labelledby="playlist-save-title" onCancel={(event) => { if (busy) event.preventDefault() }}>
        <header className="playlist-dialog-heading">
          <h2 id="playlist-save-title">Save playlist</h2>
          <button className="playlist-close" disabled={busy} onClick={() => saveDialog.current?.close()} aria-label="Close save playlist"><X aria-hidden="true" /></button>
        </header>
        <p className="playlist-hint">Save all {party.tracks.length} songs in the current queue.</p>
        {feedback}
        <form onSubmit={(event) => { event.preventDefault(); void run(() => saveQueue()) }}>
          <fieldset disabled={busy} className="playlist-controls playlist-editor">
            <label htmlFor="playlist-name">Playlist name</label>
            <input id="playlist-name" autoFocus required maxLength={80} value={name} onChange={(event) => setName(event.target.value)} />
            <div className="playlist-actions">
              <button className="button button--primary" disabled={!name.trim()}>{active ? 'Save changes' : 'Save'}</button>
              {active && <button type="button" className="text-button" disabled={!name.trim()} onClick={() => void run(() => saveQueue(true))}>Save as new</button>}
            </div>
          </fieldset>
        </form>
      </dialog>
    </div>
  )
}

function message(error: unknown) {
  return error instanceof Error ? error.message : 'The playlist operation failed. Try again.'
}
