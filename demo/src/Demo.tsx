import { useEffect, useState } from 'react'
import { ArrowUpRight, Headphones, RotateCcw, Users } from 'lucide-react'
import '@fontsource-variable/inter'
import App from '../../src/App'
import GuestApp from '../../src/GuestApp'
import { applyTheme, loadTheme, selectTheme } from '../../src/lib/theme'
import { demoInfo, resetDemo, setDemoView, subscribeToDemo } from './api'
import '../../src/styles.css'
import './style.css'

applyTheme(loadTheme())

type View = 'dj' | 'guest'
const initialView: View = new URLSearchParams(window.location.search).get('view') === 'guest' ? 'guest' : 'dj'
setDemoView(initialView)
document.documentElement.classList.toggle('guest-mode', initialView === 'guest')

export function Demo() {
  const [view, setView] = useState<View>(initialView)
  const [generation, setGeneration] = useState(0)
  const [info, setInfo] = useState(demoInfo)
  const [announcement, setAnnouncement] = useState('')

  useEffect(() => subscribeToDemo(() => setInfo(demoInfo())), [])

  function choose(next: View) {
    if (next === view) return
    setDemoView(next)
    setView(next)
    document.documentElement.classList.toggle('guest-mode', next === 'guest')
    const url = new URL(window.location.href)
    if (next === 'guest') url.searchParams.set('view', 'guest')
    else url.searchParams.delete('view')
    url.hash = ''
    window.history.replaceState(null, '', url)
    setAnnouncement(next === 'guest' ? 'Guest preview. Add a song, then return to DJ view to see it in the queue.' : 'DJ view. You control the set.')
  }

  function reset() {
    if (!window.confirm('Reset this demo? Your demo queue and playlists will return to the starting set.')) return
    resetDemo()
    selectTheme('midnight-violet')
    setGeneration((value) => value + 1)
    setAnnouncement('Demo reset. Your starting set is ready.')
  }

  return (
    <div className="demo-shell" onClickCapture={(event) => {
      const target = event.target as HTMLElement
      if (target.closest('a.brand')) { event.preventDefault(); choose('dj') }
    }}>
      <aside className="demo-controls" aria-label="Interactive demo controls">
        <div className="demo-label"><span className="demo-label-dot" /><strong>Interactive demo</strong><span className="demo-description">Sample songs · Playback simulated</span></div>
        <div className="demo-options">
          <div className="demo-tabs" role="group" aria-label="Demo view">
            <button aria-pressed={view === 'dj'} onClick={() => choose('dj')}><Headphones size={15} aria-hidden="true" />DJ view</button>
            <button aria-pressed={view === 'guest'} onClick={() => choose('guest')}><Users size={15} aria-hidden="true" />Guest view</button>
          </div>
          <button className="demo-reset" onClick={reset} title="Reset the demo"><RotateCcw size={15} aria-hidden="true" /><span>Reset</span></button>
          <a className="demo-source" href="https://github.com/amitpdev/partyhub" target="_blank" rel="noreferrer">Get Party Hub<ArrowUpRight size={15} aria-hidden="true" /></a>
        </div>
      </aside>
      <span className="sr-only" role="status">{announcement}</span>
      {view === 'dj' ? <App key={`dj-${generation}`} /> : (
        <div className="demo-guest-stage" key={`guest-${generation}`}>
          <div className="demo-guest-caption"><span>THE OTHER SIDE OF THE PARTY</span><h1>Your friends pick the next track.</h1><p>Search “neon”, “summer”, or “night”. Add a song, then switch to DJ view to see it join the set.</p></div>
          <div className="demo-phone">
            {info.sharing ? <GuestApp /> : <div className="demo-guest-off"><Users size={32} /><h2>Guest access is off</h2><p>Switch to DJ view and select Invite friends → Allow guests.</p><button onClick={() => choose('dj')}>Back to DJ view</button></div>}
          </div>
          <p className="demo-footnote">Everything stays in this browser. No accounts, installs, or audio playback.</p>
        </div>
      )}
    </div>
  )
}
