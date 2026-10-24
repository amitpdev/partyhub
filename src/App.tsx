import { AlertTriangle, Keyboard, WifiOff, X } from 'lucide-react'
import { useEffect } from 'react'
import { LinkEntry } from './components/LinkEntry'
import { PlayerDeck } from './components/PlayerDeck'
import { PlaylistLibrary } from './components/PlaylistLibrary'
import { QueuePanel } from './components/QueuePanel'
import { SearchDialog } from './components/SearchDialog'
import { ShareParty } from './components/ShareParty'
import { ThemeSwitcher } from './components/ThemeSwitcher'
import { usePartyPlayer } from './hooks/usePartyPlayer'

export default function App() {
  const player = usePartyPlayer()
  const queue = player.party
  const currentTrack = queue.tracks[queue.currentIndex]

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const target = event.target as HTMLElement
      if (document.querySelector('dialog[open]')) return
      if (['INPUT', 'TEXTAREA', 'BUTTON'].includes(target.tagName)) return
      if (event.code === 'Space') {
        event.preventDefault()
        if (currentTrack) {
          if (['playing', 'paused'].includes(player.status.state)) void player.pauseOrResume()
          else void player.play()
        }
      } else if (event.key.toLowerCase() === 'n') {
        void player.next()
      } else if (event.key.toLowerCase() === 'b') {
        void player.previous()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [currentTrack, player])

  return (
    <div className="app-shell">
      <header className="topbar">
        <a className="brand" href="/" aria-label="Party Hub home">
          <span className="brand__mark" aria-hidden="true">
            <span />
            <span />
            <span />
          </span>
          <span>Party Hub</span>
        </a>
        <PlaylistLibrary party={queue} onSnapshot={player.acceptSnapshot} />
        <ThemeSwitcher />
        <ShareParty />
        <div className={`service-state ${player.status.available ? 'is-ready' : ''}`}>
          <span aria-hidden="true" />
          {player.status.available ? 'Player connected' : 'Player offline'}
        </div>
      </header>

      {!player.status.available && (
        <div className="service-banner" role="status">
          <WifiOff aria-hidden="true" />
          <div>
            <strong>The local player is not ready.</strong>
            <span>
              {player.status.missing.length
                ? ` Install ${player.status.missing.join(' and ')}, then restart Party Hub.`
                : ' Start the Go helper with yarn dev.'}
            </span>
          </div>
        </div>
      )}

      {queue.storageError && (
        <div className="recovery-banner" role="alert">
          <AlertTriangle aria-hidden="true" />
          <div>
            <strong>Queue changes are not being saved.</strong>
            <span>{queue.storageError}</span>
          </div>
        </div>
      )}

      {player.notice && (
        <div className="playback-notice" role="status">
          <span>{player.notice}</span>
          <button onClick={player.clearNotice} aria-label="Dismiss playback message">
            <X />
          </button>
        </div>
      )}

      <main className="dashboard">
        <PlayerDeck
          track={currentTrack}
          status={player.status}
          volume={queue.volume}
          onPlay={() => void player.play()}
          onPauseResume={() => void player.pauseOrResume()}
          onPrevious={() => void player.previous()}
          onNext={() => void player.next()}
          onSeek={(seconds) => void player.seek(seconds)}
          onSeekTo={(position) => void player.seekTo(position)}
          onVolume={(volume) => void player.setVolume(volume)}
        />

        <div className="dashboard-side">
          <section className="song-adder" aria-labelledby="song-adder-title">
            <div className="song-adder__heading">
              <h2 id="song-adder-title">Add your song</h2>
            </div>
            <SearchDialog
              tracks={queue.tracks}
              currentIndex={queue.currentIndex}
              onAdd={player.add}
              disabled={!player.status.available}
            />
            <div className="song-adder__divider"><span>or paste a link</span></div>
            <LinkEntry
              onAdd={player.add}
              disabled={!player.status.available}
              hideLabel
              allowPaste
              buttonVariant="secondary"
            />
          </section>

          <QueuePanel
            tracks={queue.tracks}
            currentIndex={queue.currentIndex}
            onPlay={(index) => void player.play(index)}
            onMove={(queueId, direction) => void player.move(queueId, direction)}
            onRemove={(queueId) => void player.remove(queueId)}
            onClear={() => void player.clear()}
          />
        </div>
      </main>

      <footer className="shortcut-bar">
        <span>
          <Keyboard aria-hidden="true" /> Keyboard
        </span>
        <span><kbd>Space</kbd> play / pause</span>
        <span><kbd>N</kbd> next</span>
        <span><kbd>B</kbd> previous</span>
      </footer>
    </div>
  )
}
