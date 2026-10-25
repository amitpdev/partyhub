import { LoaderCircle, Settings2, Sparkles, X } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { clearAISettings, fetchAIModels, fetchAISettings, pickAISong, saveAISettings, testAIConnection } from '../lib/ai'
import type { AIDraft, AIPick, AIProvider, AISettings } from '../lib/ai'
import { removeTrack } from '../lib/api'
import type { PartySnapshot } from '../types'

export interface AIPickerProps {
  disabled?: boolean
  onSnapshot: (party: PartySnapshot) => void
}
const providers: { id: AIProvider; name: string; url: string; optionalKey?: boolean }[] = [
  { id: 'openai', name: 'OpenAI', url: 'https://api.openai.com/v1' },
  { id: 'anthropic', name: 'Anthropic', url: 'https://api.anthropic.com/v1' },
  { id: 'gemini', name: 'Google Gemini', url: 'https://generativelanguage.googleapis.com/v1beta' },
  { id: 'ollama', name: 'Ollama (local)', url: 'http://127.0.0.1:11434', optionalKey: true },
  { id: 'compatible', name: 'OpenAI-compatible server', url: '', optionalKey: true },
]
const emptyDraft: AIDraft = { provider: 'openai', model: '', baseUrl: providers[0].url, apiKey: '', rememberKey: false }

export function AIPicker({ disabled, onSnapshot }: AIPickerProps) {
  const dialog = useRef<HTMLDialogElement>(null)
  const controller = useRef<AbortController | null>(null)
  const fieldID = useId()
  const [settings, setSettings] = useState<AISettings | null>(null)
  const [draft, setDraft] = useState<AIDraft>(emptyDraft)
  const [models, setModels] = useState<string[]>([])
  const [tested, setTested] = useState(false)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [formError, setFormError] = useState('')
  const [feedback, setFeedback] = useState('')
  const [picked, setPicked] = useState<AIPick | null>(null)
  const provider = providers.find((provider) => provider.id === draft.provider)!
  const savedKey = Boolean(settings?.hasKey && settings.provider === draft.provider && settings.baseUrl === draft.baseUrl.replace(/\/$/, ''))
  const canConnect = provider.optionalKey || Boolean(draft.apiKey.trim()) || savedKey

  useEffect(() => () => controller.current?.abort(), [])

  function apply(settings: AISettings) {
    setSettings(settings)
    setDraft({ provider: settings.provider, model: settings.model, baseUrl: settings.baseUrl, apiKey: '', rememberKey: settings.rememberKey || (!settings.configured && settings.secureStorageAvailable) })
    setModels([])
    setTested(false)
    setFeedback('')
  }

  function update(value: Partial<AIDraft>) {
    setDraft((draft) => ({ ...draft, ...value }))
    setTested(false)
    setFeedback('')
    setFormError('')
  }

  async function openSettings() {
    setBusy('settings')
    setFormError('')
    try { apply(await fetchAISettings()) } catch (error) { setFormError(message(error)) }
    finally { setBusy(''); dialog.current?.showModal() }
  }

  async function pick() {
    setBusy('pick')
    setError('')
    setPicked(null)
    controller.current = new AbortController()
    try {
      const config = await fetchAISettings()
      if (!config.configured) {
        apply(config)
        dialog.current?.showModal()
        return
      }
      const result = await pickAISong(controller.current.signal)
      onSnapshot(result.party)
      setPicked(result)
    } catch (error) {
      if (!(error instanceof Error && error.name === 'AbortError')) setError(message(error))
    } finally { setBusy(''); controller.current = null }
  }

  async function run(action: string, operation: () => Promise<void>) {
    setBusy(action)
    setFormError('')
    try { await operation() } catch (error) { setFormError(message(error)) }
    finally { setBusy('') }
  }

  function close() {
    if (busy) return
    setDraft((draft) => ({ ...draft, apiKey: '' }))
    dialog.current?.close()
  }

  return (
    <div className="ai-picker">
      <div className="ai-pick-controls">
        <button className="song-search-trigger ai-pick-trigger" disabled={disabled || Boolean(busy)} onClick={() => void pick()}>
          {busy === 'pick' ? <LoaderCircle className="spin" aria-hidden="true" /> : <Sparkles aria-hidden="true" />}
          <strong>{busy === 'pick' ? 'Picking…' : 'Let AI pick'}</strong>
        </button>
        <button className="ai-settings-trigger" aria-label="AI settings" title="AI settings" disabled={Boolean(busy)} onClick={() => void openSettings()}><Settings2 size={18} aria-hidden="true" /></button>
      </div>
      {error && <p className="ai-notice playlist-error" role="alert">{error}</p>}
      {picked && <div className="ai-notice" role="status">
        <span>AI picked: <strong>{picked.title}</strong></span>
        <button className="text-button" disabled={Boolean(busy)} onClick={async () => {
          setBusy('undo')
          try { onSnapshot(await removeTrack(picked.queueId)); setPicked(null) } catch (error) { setError(message(error)) }
          finally { setBusy('') }
        }}>Undo</button>
        <button className="text-button" aria-label="Dismiss AI pick" onClick={() => setPicked(null)}><X size={14} /></button>
      </div>}
      <dialog ref={dialog} className="playlist-dialog ai-settings-dialog" aria-labelledby={`${fieldID}-title`} onCancel={(event) => {
        if (busy) event.preventDefault()
        else setDraft((draft) => ({ ...draft, apiKey: '' }))
      }}>
        <header className="playlist-dialog-heading">
          <h2 id={`${fieldID}-title`}>AI settings</h2>
          <button className="playlist-close" disabled={Boolean(busy)} aria-label="Close AI settings" onClick={close}><X /></button>
        </header>
        <p className="playlist-hint">Choose your provider and model. Your key is stored locally.</p>
        {formError && <p className="playlist-error" role="alert">{formError}</p>}
        {feedback && <p role="status">{feedback}</p>}
        <form onSubmit={(event) => {
          event.preventDefault()
          void run('save', async () => { apply(await saveAISettings(draft)); dialog.current?.close() })
        }}>
          <fieldset className="playlist-controls ai-fields" disabled={Boolean(busy)}>
            <label htmlFor={`${fieldID}-provider`}>Provider</label>
            <select id={`${fieldID}-provider`} value={draft.provider} onChange={(event) => {
              const selected = providers.find((provider) => provider.id === event.target.value)!
              update({ provider: selected.id, model: '', baseUrl: selected.url, apiKey: '' })
              setModels([])
            }}>{providers.map((provider) => <option key={provider.id} value={provider.id}>{provider.name}</option>)}</select>
            {['ollama', 'compatible'].includes(draft.provider) && <>
              <label htmlFor={`${fieldID}-url`}>Server URL</label>
              <input id={`${fieldID}-url`} type="url" required placeholder={draft.provider === 'compatible' ? 'https://your-provider.example/v1' : 'http://127.0.0.1:11434'} value={draft.baseUrl} onChange={(event) => update({ baseUrl: event.target.value })} />
            </>}
            <label htmlFor={`${fieldID}-key`}>API key{provider.optionalKey ? ' (optional)' : ''}</label>
            <input id={`${fieldID}-key`} type="password" autoComplete="off" spellCheck={false} value={draft.apiKey} placeholder={savedKey ? 'Saved key · leave blank to keep' : 'Paste your API key'} onChange={(event) => update({ apiKey: event.target.value })} />
            <div className="ai-model-heading"><label htmlFor={`${fieldID}-model`}>Model</label><button type="button" className="text-button sharing-help-trigger" disabled={!canConnect} onClick={() => void run('models', async () => {
              const result = await fetchAIModels(draft)
              setModels(result.models)
              setFeedback(result.models.length ? 'Choose a model below, or enter its ID.' : 'No models listed. Enter the model ID manually.')
            })}>{busy === 'models' ? 'Loading…' : 'Load models'}</button></div>
            <input id={`${fieldID}-model`} required list={`${fieldID}-models`} placeholder="Select or enter a text model ID" value={draft.model} onChange={(event) => update({ model: event.target.value })} />
            <datalist id={`${fieldID}-models`}>{models.map((model) => <option key={model} value={model} />)}</datalist>
            <label className="ai-remember"><input type="checkbox" checked={draft.rememberKey} disabled={!settings?.secureStorageAvailable && !draft.rememberKey} onChange={(event) => setDraft((draft) => ({ ...draft, rememberKey: event.target.checked }))} />Remember key securely</label>
            <small className="playlist-hint">{settings?.secureStorageAvailable ? 'Stored in your OS credential store. Uncheck to keep it for this server session only.' : 'Secure storage is unavailable. Keys are kept for this server session only.'}</small>
            <div className="playlist-actions">
              <button type="button" className="button button--secondary" disabled={!canConnect || !draft.model.trim()} onClick={() => void run('test', async () => {
                await testAIConnection(draft)
                setTested(true)
                setFeedback('Connection works. Save to use it.')
              })}>{busy === 'test' ? 'Testing…' : 'Test connection'}</button>
              <button className="button button--primary" disabled={!tested}>{busy === 'save' ? 'Saving…' : 'Save'}</button>
              {settings?.model && <button type="button" className="text-button" onClick={() => {
                if (window.confirm('Disconnect AI and remove the saved key?')) void run('clear', async () => apply(await clearAISettings()))
              }}>Disconnect AI</button>}
            </div>
            <small className="playlist-hint">Testing sends a small AI request. Picks send recent song titles and artists; provider charges may apply.</small>
          </fieldset>
        </form>
      </dialog>
    </div>
  )
}

function message(error: unknown) { return error instanceof Error ? error.message : 'The AI connection failed. Try again.' }
