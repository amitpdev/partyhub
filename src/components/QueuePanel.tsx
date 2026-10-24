import {
  ChevronDown,
  ChevronUp,
  ListMusic,
  Play,
  Trash2,
  X,
} from 'lucide-react'
import { useEffect, useRef } from 'react'
import { formatDuration } from '../lib/format'
import type { Track } from '../types'
import { Artwork } from './Artwork'

interface QueuePanelProps {
  tracks: Track[]
  currentIndex: number
  onPlay: (index: number) => void
  onMove: (queueId: string, direction: -1 | 1) => void
  onRemove: (queueId: string) => void
  onClear: () => void
}

export function QueuePanel({
  tracks,
  currentIndex,
  onPlay,
  onMove,
  onRemove,
  onClear,
}: QueuePanelProps) {
  const currentTrackRef = useRef<HTMLElement>(null)
  const queueListRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const list = queueListRef.current
    const current = currentTrackRef.current
    if (!list || !current || list.scrollHeight <= list.clientHeight) return

    const currentTop = current.offsetTop - list.offsetTop
    const currentBottom = currentTop + current.offsetHeight
    if (currentTop < list.scrollTop) {
      list.scrollTop = currentTop
    } else if (currentBottom > list.scrollTop + list.clientHeight) {
      list.scrollTop = currentBottom - list.clientHeight
    }
  }, [currentIndex])

  return (
    <section className="queue-panel" aria-labelledby="queue-title">
      <div className="section-heading queue-heading">
        <div>
          <h2 id="queue-title">Up next</h2>
          <p>{tracks.length === 1 ? '1 track in the set' : `${tracks.length} tracks in the set`}</p>
        </div>
        {tracks.length > 0 && (
          <button
            className="text-button"
            onClick={() => {
              if (window.confirm('Clear the entire queue?')) onClear()
            }}
          >
            <X size={16} />
            Clear
          </button>
        )}
      </div>

      <div ref={queueListRef} className="queue-list" aria-live="polite">
        {tracks.length === 0 ? (
          <div className="empty-state">
            <ListMusic aria-hidden="true" />
            <h3>The queue is open</h3>
            <p>Add a YouTube link to start the set. The first track becomes ready to play.</p>
          </div>
        ) : (
          tracks.map((track, index) => {
            const state = index < currentIndex ? 'played' : index === currentIndex ? 'current' : 'queued'
            return (
              <article
                className={`queue-row queue-row--${state}`}
                key={track.queueId}
                ref={state === 'current' ? currentTrackRef : undefined}
                aria-current={state === 'current' ? 'true' : undefined}
              >
                <span className="sr-only">
                  {state === 'current' ? 'Currently playing' : state === 'played' ? 'Played' : 'Queued'}
                </span>
                <div className="setlist-marker" aria-hidden="true">
                  <span />
                </div>
                <button
                  className="queue-row__main"
                  onClick={() => onPlay(index)}
                  disabled={!track.canPlay}
                >
                  <Artwork src={track.thumbnail} compact />
                  <span className="queue-row__copy">
                    <strong>{track.title}</strong>
                    <span>
                      {track.channel}
                      {track.addedBy ? ` · Added by ${track.addedBy}` : ''}
                    </span>
                  </span>
                  <span className="queue-row__duration">{formatDuration(track.duration)}</span>
                  <Play className="queue-row__play" size={18} fill="currentColor" />
                </button>
                <div className="queue-row__actions">
                  <button
                    onClick={() => onMove(track.queueId, -1)}
                    disabled={!track.canMove || !tracks[index - 1]?.canMove}
                    aria-label={`Move ${track.title} up`}
                  >
                    <ChevronUp />
                  </button>
                  <button
                    onClick={() => onMove(track.queueId, 1)}
                    disabled={!track.canMove || !tracks[index + 1]?.canMove}
                    aria-label={`Move ${track.title} down`}
                  >
                    <ChevronDown />
                  </button>
                  <button
                    onClick={() => onRemove(track.queueId)}
                    disabled={!track.canRemove}
                    aria-label={
                      !track.canRemove
                        ? `Cannot remove ${track.title}`
                        : `Remove ${track.title}`
                    }
                  >
                    <Trash2 />
                  </button>
                </div>
              </article>
            )
          })
        )}
      </div>
    </section>
  )
}
