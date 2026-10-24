import {
  Pause,
  Play,
  RotateCcw,
  RotateCw,
  SkipBack,
  SkipForward,
  Volume2,
} from 'lucide-react'
import { useState, type CSSProperties } from 'react'
import { formatDuration } from '../lib/format'
import type { PlayerStatus, Track } from '../types'
import { Artwork } from './Artwork'

interface PlayerDeckProps {
  track?: Track
  status: PlayerStatus
  volume: number
  onPlay: () => void
  onPauseResume: () => void
  onPrevious: () => void
  onNext: () => void
  onSeek: (seconds: number) => void
  onSeekTo: (position: number) => void
  onVolume: (volume: number) => void
}

export function PlayerDeck({
  track,
  status,
  volume,
  onPlay,
  onPauseResume,
  onPrevious,
  onNext,
  onSeek,
  onSeekTo,
  onVolume,
}: PlayerDeckProps) {
  const [pendingSeek, setPendingSeek] = useState<number | null>(null)
  const duration = status.duration || track?.duration || 0
  const displayedPosition = pendingSeek ?? status.position
  const progress = duration > 0 ? Math.min(100, (displayedPosition / duration) * 100) : 0
  const isActive = ['playing', 'paused', 'loading'].includes(status.state)
  const canSeek = isActive && duration > 0

  const commitSeek = (position: number) => {
    if (!canSeek) return
    setPendingSeek(null)
    onSeekTo(position)
  }

  return (
    <section className="player-deck" aria-labelledby="now-playing-title">
      <div className="section-heading">
        <div>
          <h1 id="now-playing-title">Now playing</h1>
          <p>{statusLine(status, track)}</p>
        </div>
        <span className={`status-dot status-dot--${status.state}`}>
          <span aria-hidden="true" />
          {status.state === 'idle' ? 'Ready' : labelState(status.state)}
        </span>
      </div>

      <div className="now-playing">
        <div className="cover">
          <Artwork src={track?.thumbnail || ''} />
        </div>
        <div className="track-copy">
          <p className="track-copy__source">YouTube audio</p>
          <h2 title={track?.title}>{track ? compactTitle(track.title) : 'Your set starts here'}</h2>
          <p>{track?.channel || 'Add a link to build the queue.'}</p>
        </div>
      </div>

      <div className="timeline">
        <div
          className="timeline__control"
          style={{ '--timeline-progress': `${progress}%` } as CSSProperties}
        >
          <span aria-hidden="true" />
          <input
            className="timeline__slider"
            type="range"
            min="0"
            max={Math.max(1, duration)}
            step="0.1"
            value={Math.min(displayedPosition, Math.max(1, duration))}
            disabled={!canSeek}
            aria-label="Seek playback"
            aria-valuetext={`${formatDuration(displayedPosition)} of ${formatDuration(duration)}`}
            onChange={(event) => setPendingSeek(Number(event.target.value))}
            onPointerUp={(event) => commitSeek(Number(event.currentTarget.value))}
            onKeyUp={(event) => {
              if (pendingSeek !== null) commitSeek(Number(event.currentTarget.value))
            }}
            onBlur={() => {
              if (pendingSeek !== null) commitSeek(pendingSeek)
            }}
          />
        </div>
        <div className="timeline__time">
          <span>{formatDuration(displayedPosition)}</span>
          <span>{formatDuration(duration)}</span>
        </div>
      </div>

      <div className="transport" aria-label="Playback controls">
        <button className="icon-button" onClick={onPrevious} disabled={!track} aria-label="Previous">
          <SkipBack />
        </button>
        <button className="icon-button" onClick={() => onSeek(-10)} disabled={!isActive} aria-label="Back 10 seconds">
          <RotateCcw />
          <span className="icon-button__count">10</span>
        </button>
        <button
          className="play-button"
          onClick={isActive ? onPauseResume : onPlay}
          disabled={!track || status.state === 'loading'}
          aria-label={status.state === 'playing' ? 'Pause' : 'Play'}
        >
          {status.state === 'playing' ? <Pause fill="currentColor" /> : <Play fill="currentColor" />}
        </button>
        <button className="icon-button" onClick={() => onSeek(10)} disabled={!isActive} aria-label="Forward 10 seconds">
          <RotateCw />
          <span className="icon-button__count">10</span>
        </button>
        <button className="icon-button" onClick={onNext} disabled={!track} aria-label="Next">
          <SkipForward />
        </button>
      </div>

      <label className="volume-control">
        <Volume2 aria-hidden="true" />
        <span>Volume</span>
        <input
          type="range"
          min="0"
          max="100"
          value={volume}
          onChange={(event) => onVolume(Number(event.target.value))}
        />
        <output>{volume}%</output>
      </label>
    </section>
  )
}

function labelState(state: PlayerStatus['state']) {
  if (state === 'loading') return 'Loading'
  if (state === 'playing') return 'Playing'
  if (state === 'paused') return 'Paused'
  if (state === 'error') return 'Playback issue'
  if (state === 'ended') return 'Set complete'
  return 'Stopped'
}

function statusLine(status: PlayerStatus, track?: Track) {
  if (!status.available) return 'Waiting for the local player'
  if (status.error) return status.error
  if (track) return `${track.channel} · ${formatDuration(track.duration)}`
  return 'Queue a track to begin'
}

function compactTitle(title: string) {
  const characters = Array.from(title)
  return characters.length > 22 ? `${characters.slice(0, 21).join('')}…` : title
}

