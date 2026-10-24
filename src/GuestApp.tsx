import { Headphones, LoaderCircle, Music2, Trash2, UserRound } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Artwork } from './components/Artwork'
import { LinkEntry } from './components/LinkEntry'
import { SearchDialog } from './components/SearchDialog'
import { usePartyPlayer } from './hooks/usePartyPlayer'
import { createGuestSession, fetchParty } from './lib/api'
import { formatDuration } from './lib/format'

function tokenFromLocation() {
  return new URLSearchParams(window.location.hash.slice(1)).get('token') || ''
}

export default function GuestApp() {
  const [joined, setJoined] = useState(false)
  const [name, setName] = useState(localStorage.getItem('partyhub.guestName') || '')
  const [token] = useState(tokenFromLocation)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [checkingSession, setCheckingSession] = useState(!token)

  useEffect(() => {
    if (token) return
    void fetchParty()
      .then(() => setJoined(true))
      .catch(() => undefined)
      .finally(() => setCheckingSession(false))
  }, [token])

  async function join(event: React.FormEvent) {
    event.preventDefault()
    setLoading(true)
    setError('')
    try {
      await createGuestSession(token, name)
      localStorage.setItem('partyhub.guestName', name.trim())
      window.history.replaceState(null, '', '/guest')
      setJoined(true)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not join this party')
    } finally {
      setLoading(false)
    }
  }

  if (joined) return <GuestParty />
  if (checkingSession) {
    return (
      <main className="guest-join guest-join--loading">
        <LoaderCircle className="spin" aria-label="Checking party invitation" />
      </main>
    )
  }

  return (
    <main className="guest-join">
      <div className="guest-brand">
        <span className="brand__mark" aria-hidden="true">
          <span />
          <span />
          <span />
        </span>
        Party Hub
      </div>
      <section className="guest-join__card">
        <div className="guest-join__icon">
          <Headphones aria-hidden="true" />
        </div>
        <h1>Join the party queue</h1>
        <p>Pick a name so everyone knows who added each song.</p>
        {token ? (
          <form onSubmit={join}>
            <label htmlFor="guest-name">Your name</label>
            <div className="guest-name-field">
              <UserRound aria-hidden="true" />
              <input
                id="guest-name"
                value={name}
                maxLength={30}
                autoFocus
                autoComplete="off"
                onChange={(event) => setName(event.target.value)}
                placeholder="e.g. Maya"
              />
            </div>
            <button className="button button--primary guest-join__button" disabled={!name.trim() || loading}>
              {loading && <LoaderCircle className="spin" />}
              {loading ? 'Joining' : 'Join party'}
            </button>
          </form>
        ) : (
          <div className="guest-link-error">
            <strong>This invite is incomplete</strong>
            <p>Scan the QR code on the host laptop again.</p>
          </div>
        )}
        {error && <p className="field-error" role="alert">{error}</p>}
      </section>
    </main>
  )
}

function GuestParty() {
  const party = usePartyPlayer()
  const { tracks, currentIndex } = party.party
  const current = tracks[currentIndex]
  const upcoming = tracks.slice(Math.max(0, currentIndex + 1))

  useEffect(() => {
    window.scrollTo(0, 0)
    const afterKeyboardCloses = window.setTimeout(() => window.scrollTo(0, 0), 150)
    return () => window.clearTimeout(afterKeyboardCloses)
  }, [])

  return (
    <main className="guest-app">
      <header className="guest-header">
        <div className="guest-brand">
          <span className="brand__mark" aria-hidden="true">
            <span />
            <span />
            <span />
          </span>
          Party Hub
        </div>
        <span className={`guest-connection ${party.status.available ? 'is-ready' : ''}`}>
          <i aria-hidden="true" />
          {party.status.available ? 'Live' : 'Reconnecting'}
        </span>
      </header>

      <section className="guest-add">
        <h1>Add your song</h1>
        <SearchDialog
          tracks={tracks}
          currentIndex={currentIndex}
          onAdd={party.add}
          disabled={!party.status.available}
        />
        <div className="song-adder__divider"><span>or paste a link</span></div>
        <LinkEntry
          onAdd={party.add}
          label="YouTube link"
          hideLabel
          allowPaste
          disabled={!party.status.available}
          buttonVariant="secondary"
        />
        <p className="guest-add__instructions">
          Videos must be 15 minutes or shorter. You can have three upcoming songs.
        </p>
      </section>

      <section className="guest-now-playing">
        <h2>Now playing</h2>
        {current ? (
          <div>
            <Artwork src={current.thumbnail} compact />
            <span>
              <strong>{current.title}</strong>
              <small>{current.channel}</small>
            </span>
          </div>
        ) : (
          <p>The queue is waiting for its first song.</p>
        )}
      </section>

      {party.notice && <div className="guest-notice" role="status">{party.notice}</div>}

      <section className="guest-queue">
        <div className="guest-section-heading">
          <div>
            <h2>Up next</h2>
            <p>{upcoming.length ? `${upcoming.length} in the queue` : 'The queue is open'}</p>
          </div>
          <Music2 aria-hidden="true" />
        </div>
        <div className="guest-track-list">
          {upcoming.length === 0 && (
            <p className="guest-queue__empty">Search YouTube or paste a link to request the next song.</p>
          )}
          {upcoming.map((track, offset) => (
            <article className="guest-track" key={track.queueId}>
              <span className="guest-track__position">{offset + 1}</span>
              <Artwork src={track.thumbnail} compact />
              <span className="guest-track__copy">
                <strong>{track.title}</strong>
                <small>Added by {track.addedBy || 'DJ'} · {formatDuration(track.duration)}</small>
              </span>
              {track.canRemove && (
                <button onClick={() => void party.remove(track.queueId)} aria-label={`Remove ${track.title}`}>
                  <Trash2 />
                </button>
              )}
            </article>
          ))}
        </div>
      </section>
    </main>
  )
}
