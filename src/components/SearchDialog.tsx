import { AlertTriangle, LoaderCircle, Search, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { searchYouTube } from '../lib/api'
import { formatDuration } from '../lib/format'
import type { Track, YouTubeSearchResult } from '../types'
import { Artwork } from './Artwork'

interface SearchDialogProps {
  tracks: Track[]
  currentIndex: number
  onAdd: (url: string) => Promise<void>
  disabled?: boolean
}

export function SearchDialog({
  tracks,
  currentIndex,
  onAdd,
  disabled = false,
}: SearchDialogProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<YouTubeSearchResult[]>([])
  const [hasSearched, setHasSearched] = useState(false)
  const [loading, setLoading] = useState(false)
  const [searchError, setSearchError] = useState('')
  const [addError, setAddError] = useState('')
  const [addingID, setAddingID] = useState('')
  const dialogRef = useRef<HTMLElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const activeIDs = useMemo(
    () =>
      new Set(
        tracks
          .slice(Math.max(0, currentIndex))
          .map((track) => track.providerId),
      ),
    [currentIndex, tracks],
  )

  const closeDialog = useCallback(() => {
    setOpen(false)
    setQuery('')
    setResults([])
    setHasSearched(false)
    setLoading(false)
    setSearchError('')
    setAddError('')
    setAddingID('')
  }, [])

  useEffect(() => {
    if (!open) return
    const trigger = triggerRef.current
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeDialog()
    }
    window.addEventListener('keydown', closeOnEscape)
    return () => {
      document.body.style.overflow = previousOverflow
      window.removeEventListener('keydown', closeOnEscape)
      trigger?.focus()
    }
  }, [closeDialog, open])

  function openDialog() {
    setOpen(true)
  }

  async function runSearch() {
    const value = query.trim()
    if (!value) {
      setSearchError('Enter an artist, song, or karaoke title.')
      return
    }
    setLoading(true)
    setHasSearched(true)
    setSearchError('')
    setAddError('')
    try {
      setResults(await searchYouTube(value))
    } catch (error) {
      setResults([])
      setSearchError(messageFrom(error, 'Could not search YouTube. Check the connection and try again.'))
    } finally {
      setLoading(false)
    }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    await runSearch()
  }

  async function addResult(result: YouTubeSearchResult) {
    setAddingID(result.providerId)
    setAddError('')
    try {
      await onAdd(result.url)
      closeDialog()
    } catch (error) {
      setAddError(messageFrom(error, 'Could not add this track. Try again.'))
    } finally {
      setAddingID('')
    }
  }

  function trapFocus(event: React.KeyboardEvent) {
    if (event.key !== 'Tab') return
    const controls = dialogRef.current?.querySelectorAll<HTMLElement>(
      'button:not(:disabled), input:not(:disabled)',
    )
    if (!controls?.length) return
    const first = controls[0]
    const last = controls[controls.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  return (
    <>
      <button
        ref={triggerRef}
        className="song-search-trigger"
        type="button"
        onClick={openDialog}
        disabled={disabled}
      >
        <Search aria-hidden="true" />
        <strong>Search YouTube</strong>
      </button>

      {open && (
        <div className="modal-backdrop" onMouseDown={closeDialog}>
          <section
            ref={dialogRef}
            className="search-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="search-title"
            aria-describedby="search-description"
            onMouseDown={(event) => event.stopPropagation()}
            onKeyDown={trapFocus}
          >
            <button className="dialog-close" type="button" onClick={closeDialog} aria-label="Close search">
              <X />
            </button>

            <header className="search-dialog__heading">
              <h2 id="search-title">Search YouTube</h2>
              <p id="search-description">Choose a video up to 15 minutes long.</p>
            </header>

            <form className="search-form" onSubmit={submit} role="search">
              <Search aria-hidden="true" />
              <input
                type="search"
                value={query}
                onChange={(event) => {
                  setQuery(event.target.value)
                  setSearchError('')
                  setResults([])
                  setHasSearched(false)
                }}
                placeholder="Artist, song, or karaoke title"
                aria-label="Search YouTube"
                autoComplete="off"
                enterKeyHint="search"
                maxLength={200}
                disabled={loading}
                autoFocus
              />
              <button className="button button--primary" disabled={loading || !query.trim()}>
                {loading ? <LoaderCircle className="spin" aria-hidden="true" /> : <Search aria-hidden="true" />}
                {loading ? 'Searching' : 'Search'}
              </button>
            </form>

            <div className="search-dialog__feedback" aria-live="polite">
              {searchError && (
                <div className="search-message search-message--error" role="alert">
                  <AlertTriangle aria-hidden="true" />
                  <span>{searchError}</span>
                  <button type="button" onClick={() => void runSearch()} disabled={!query.trim()}>
                    Retry
                  </button>
                </div>
              )}
              {addError && (
                <div className="search-message search-message--error" role="alert">
                  <AlertTriangle aria-hidden="true" />
                  <span>{addError}</span>
                </div>
              )}
            </div>

            <div className="search-results" aria-busy={loading}>
              {loading ? (
                <div className="search-state" role="status">
                  <LoaderCircle className="spin" aria-hidden="true" />
                  <strong>Searching YouTube…</strong>
                </div>
              ) : results.length > 0 ? (
                <ul aria-label="YouTube search results">
                  {results.map((result) => {
                    const queued = activeIDs.has(result.providerId)
                    const adding = addingID === result.providerId
                    return (
                      <li key={result.providerId} className="search-result">
                        <Artwork src={result.thumbnail} compact />
                        <span className="search-result__copy">
                          <strong title={result.title}>{result.title}</strong>
                          <small>
                            {result.channel} · <span>{formatDuration(result.duration)}</span>
                          </small>
                        </span>
                        <button
                          className={`search-result__add${queued ? ' is-queued' : ''}`}
                          type="button"
                          disabled={queued || Boolean(addingID)}
                          onClick={() => void addResult(result)}
                        >
                          {adding && <LoaderCircle className="spin" aria-hidden="true" />}
                          {queued ? 'Already queued' : adding ? 'Adding…' : 'Add to queue'}
                        </button>
                      </li>
                    )
                  })}
                </ul>
              ) : hasSearched && !searchError ? (
                <div className="search-state">
                  <Search aria-hidden="true" />
                  <strong>No matching videos</strong>
                  <span>Try a different artist, title, or spelling.</span>
                </div>
              ) : (
                <div className="search-state">
                  <Search aria-hidden="true" />
                  <strong>Search for the next track</strong>
                  <span>Live streams, Shorts, and long videos stay out of the results.</span>
                </div>
              )}
            </div>
          </section>
        </div>
      )}
    </>
  )
}

function messageFrom(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? `${fallback} ${error.message}` : fallback
}
