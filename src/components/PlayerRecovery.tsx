import { WifiOff, X } from 'lucide-react'
import { useRef } from 'react'

interface Props {
  missing: string[]
}

export function PlayerRecovery({ missing }: Props) {
  const dialog = useRef<HTMLDialogElement>(null)

  return (
    <>
      <div className="service-banner" role="status">
        <WifiOff aria-hidden="true" />
        <div>
          <strong>The local player is not ready.</strong>
          <span>{missing.length ? `Missing ${missing.join(' and ')}.` : 'The dashboard cannot reach the local playback service.'}</span>
        </div>
        <button className="text-button service-help-trigger" type="button" aria-haspopup="dialog" onClick={() => dialog.current?.showModal()}>More info</button>
      </div>
      <dialog ref={dialog} className="player-help-dialog" aria-labelledby="player-help-title">
        <PlayerRecoveryHelp missing={missing} titleId="player-help-title" onClose={() => dialog.current?.close()} />
      </dialog>
    </>
  )
}


interface HelpProps extends Props {
  titleId: string
  onClose: () => void
}

export function PlayerRecoveryHelp({ missing, titleId, onClose }: HelpProps) {
  const tools = missing.filter((name) => name === 'mpv' || name === 'yt-dlp')
  return (
    <>
      <header className="playlist-dialog-heading">
        <h2 id={titleId}>Get the player running</h2>
        <button className="playlist-close" type="button" onClick={onClose} aria-label="Close player help" autoFocus><X aria-hidden="true" /></button>
      </header>
      <p>Open a terminal in the <strong>Party Hub project folder</strong>.</p>
      {missing.length > 0 && <section className="player-help-step" aria-labelledby={`${titleId}-tools`}>
        <h3 id={`${titleId}-tools`}>Install missing playback tools first</h3>
        <p>On macOS with Homebrew:</p>
        {tools.length > 0 && <pre><code>brew install {tools.join(' ')}</code></pre>}
        <p>Then restart below.</p>
      </section>}
      <section className="player-help-step" aria-labelledby={`${titleId}-party`}>
        <h3 id={`${titleId}-party`}>Hosting a party with friends</h3>
        <p>In your party terminal, press <kbd>Ctrl+C</kbd> if it’s still running, then run:</p>
        <pre><code>yarn party</code></pre>
        <p>Keep the terminal open. Guests can rejoin through <strong>Invite friends</strong> with the new QR code.</p>
      </section>
      <details className="player-help-step">
        <summary>Developing locally?</summary>
        <p>If the web app on port 5173 is still running, start only the player in a second terminal:</p>
        <pre><code>yarn dev:server</code></pre>
        <p>For guest access, use the party restart above.</p>
      </details>
      <p className="player-help-note">Your queue is saved. The dashboard reconnects automatically; press Play to resume. Still offline? Check the terminal for errors.</p>
      <button className="button button--primary" type="button" onClick={onClose}>Got it</button>
    </>
  )
}
