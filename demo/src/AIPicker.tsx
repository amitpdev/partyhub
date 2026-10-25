import { Sparkles, X } from 'lucide-react'
import { useState } from 'react'
import type { AIPickerProps } from '../../src/components/AIPicker'
import { engine } from './api'

export function AIPicker({ disabled, onSnapshot }: AIPickerProps) {
  const [pick, setPick] = useState<{ queueId: string; title: string } | null>(null)
  const [error, setError] = useState('')
  return <div className="ai-picker">
    <div className="ai-pick-controls">
      <button className="song-search-trigger ai-pick-trigger" disabled={disabled} title="Demo pick: adds a sample song next without calling an AI service" onClick={() => {
        setError('')
        try { const result = engine.pickNext(); onSnapshot(result.party); setPick(result) }
        catch (error) { setError(error instanceof Error ? error.message : 'Could not pick a sample song.') }
      }}><Sparkles aria-hidden="true" /><strong>Let AI pick</strong></button>
    </div>
    {pick && <div className="ai-notice" role="status"><span>Demo pick: <strong>{pick.title}</strong> · No AI request sent.</span><button className="text-button" onClick={() => {
      try { onSnapshot(engine.remove(pick.queueId)); setPick(null) } catch (error) { setError(error instanceof Error ? error.message : 'Could not undo this pick.') }
    }}>Undo</button><button className="text-button" aria-label="Dismiss AI pick" onClick={() => setPick(null)}><X size={14} /></button></div>}
    {error && <p className="ai-notice playlist-error" role="alert">{error}</p>}
  </div>
}
