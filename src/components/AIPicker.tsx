import { Check, LoaderCircle, Settings2, Sparkles, X } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { clearAISettings, fetchAIModels, fetchAISettings, pickAISong, saveAISettings, testAIConnection } from '../lib/ai'
import { fetchChatGPTModels } from '../lib/ai'
import { ChatGPTConnection } from './ChatGPTConnection'
import type { AIDraft, AIPick, AIProvider, AISettings, ModelChoice } from '../lib/ai'
import { removeTrack } from '../lib/api'
import type { PartySnapshot } from '../types'

export interface AIPickerProps {
  disabled?: boolean
  onSnapshot: (party: PartySnapshot) => void
}
const providers: { id: AIProvider; name: string; url: string; optionalKey?: boolean }[] = [
  { id: 'chatgpt', name: 'ChatGPT plan', url: 'https://api.openai.com/v1', optionalKey: true },
  { id: 'openai', name: 'OpenAI', url: 'https://api.openai.com/v1' },
  { id: 'anthropic', name: 'Anthropic', url: 'https://api.anthropic.com/v1' },
  { id: 'gemini', name: 'Google Gemini', url: 'https://generativelanguage.googleapis.com/v1beta' },
  { id: 'ollama', name: 'Ollama (local)', url: 'http://127.0.0.1:11434', optionalKey: true },
  { id: 'compatible', name: 'OpenAI-compatible server', url: '', optionalKey: true },
]
const emptyDraft: AIDraft = { provider: 'openai', model: '', baseUrl: 'https://api.openai.com/v1', apiKey: '', rememberKey: false }

export function AIPicker({ disabled, onSnapshot }: AIPickerProps) {
  const dialog = useRef<HTMLDialogElement>(null)
  const controller = useRef<AbortController | null>(null)
  const fieldID = useId()
  const [settings, setSettings] = useState<AISettings | null>(null)
  const [draft, setDraft] = useState<AIDraft>(emptyDraft)
  const [models, setModels] = useState<string[]>([])
  const [chatModels, setChatModels] = useState<ModelChoice[]>([])
  const [modelLoading, setModelLoading] = useState(false)
  const [modelError, setModelError] = useState('')
  const [modelNotice, setModelNotice] = useState('')
  const [refreshModels, setRefreshModels] = useState(0)
  const manualRefresh = useRef(false)
  const [chatReady, setChatReady] = useState(false)
  const [isOpen, setIsOpen] = useState(false)
  const [tested, setTested] = useState(false)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [formError, setFormError] = useState('')
  const [feedback, setFeedback] = useState('')
  const [picked, setPicked] = useState<AIPick | null>(null)
  const provider = providers.find((provider) => provider.id === draft.provider)!
  const savedKey = Boolean(settings?.hasKey && settings.provider === draft.provider && settings.baseUrl === draft.baseUrl.replace(/\/$/, ''))
  const isChatGPT = draft.provider === 'chatgpt'
  const canConnect = isChatGPT ? chatReady : provider.optionalKey || Boolean(draft.apiKey.trim()) || savedKey

  useEffect(() => { let active = true; void fetchAISettings().then((settings) => { if (active) setSettings(settings) }).catch(() => {}); return () => { active = false; controller.current?.abort() } }, [])

  const { provider: modelProvider, baseUrl, apiKey, accountId } = draft
  useEffect(() => {
    setModelLoading(false)
    setModelError('')
    setModelNotice('')
    setModels([])
    setChatModels([])
    if (!isOpen || !canConnect) return
    try { new URL(baseUrl) } catch { return }
    const request = new AbortController()
    const immediate = manualRefresh.current
    manualRefresh.current = false
    // Wait until the DJ pauses typing; model discovery needs no generation request.
    const timer = setTimeout(async () => {
      setModelLoading(true)
      try {
        if (modelProvider === 'chatgpt') {
          const result = await fetchChatGPTModels(accountId || '', request.signal)
          if (request.signal.aborted) return
          setChatModels(result.models)
          setModelNotice(result.models.length ? '' : 'No models available for this ChatGPT account.')
        } else {
          const result = await fetchAIModels({ provider: modelProvider, baseUrl, apiKey, accountId, model: '', rememberKey: false }, request.signal)
          if (request.signal.aborted) return
          setModels(result.models)
          setModelNotice(result.models.length ? '' : 'No models listed. Enter the model ID manually.')
        }
      } catch (error) {
        if (!request.signal.aborted) setModelError(message(error))
      } finally { if (!request.signal.aborted) setModelLoading(false) }
    }, !immediate && modelProvider !== 'chatgpt' && apiKey.trim() ? 600 : 0)
    return () => { clearTimeout(timer); request.abort() }
  }, [isOpen, canConnect, modelProvider, baseUrl, apiKey, accountId, refreshModels])

  function apply(settings: AISettings) {
    setSettings(settings)
    setDraft({ ...(settings.accountId ? { accountId: settings.accountId } : {}), provider: settings.provider, model: settings.model, baseUrl: settings.baseUrl, apiKey: '', rememberKey: settings.rememberKey || (!settings.configured && settings.secureStorageAvailable) })
    setModels([])
    setChatModels([])
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
    finally { setBusy(''); setIsOpen(true); dialog.current?.showModal() }
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
        setIsOpen(true)
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
    setFeedback('')
    if (action === 'test') setTested(false)
    try { await operation() } catch (error) { setFormError(message(error)) }
    finally { setBusy('') }
  }

  function close() {
    if (busy) return
    setDraft((draft) => ({ ...draft, apiKey: '' }))
    setIsOpen(false)
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
      {settings?.provider === 'chatgpt' && settings.configured && <small className="ai-plan-label">Using ChatGPT plan · <a href="https://chatgpt.com/settings/usage" target="_blank" rel="noreferrer">Manage usage</a></small>}
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
      <dialog ref={dialog} className="playlist-dialog ai-settings-dialog" aria-labelledby={`${fieldID}-title`} onClose={(event) => { if (event.target === event.currentTarget) setIsOpen(false) }} onCancel={(event) => {
        if (event.target !== event.currentTarget) return
        if (busy) event.preventDefault()
        else { setDraft((draft) => ({ ...draft, apiKey: '' })); setIsOpen(false) }
      }}>
        <header className="playlist-dialog-heading">
          <h2 id={`${fieldID}-title`}>AI settings</h2>
          <button className="playlist-close" disabled={Boolean(busy)} aria-label="Close AI settings" onClick={close}><X /></button>
        </header>
        <p className="playlist-hint">Choose your provider and model. Credentials are stored locally.</p>
        {formError && <p className="playlist-error" role="alert">{formError}</p>}
        {feedback && !tested && <p role="status">{feedback}</p>}
        <form onSubmit={(event) => {
          event.preventDefault()
          void run('save', async () => { apply(await saveAISettings(draft)); setIsOpen(false); dialog.current?.close() })
        }}>
          <fieldset className="playlist-controls ai-fields" disabled={Boolean(busy)}>
            <label htmlFor={`${fieldID}-provider`}>Provider</label>
            <select id={`${fieldID}-provider`} value={draft.provider} onChange={(event) => {
              const selected = providers.find((provider) => provider.id === event.target.value)!
              update({ provider: selected.id, model: '', baseUrl: selected.url, apiKey: '', accountId: undefined })
              setChatModels([])
              setChatReady(false)
              setModels([])
            }}>{providers.map((provider) => <option key={provider.id} value={provider.id}>{provider.name}</option>)}</select>
            {['ollama', 'compatible'].includes(draft.provider) && <>
              <label htmlFor={`${fieldID}-url`}>Server URL</label>
              <input id={`${fieldID}-url`} type="url" required placeholder={draft.provider === 'compatible' ? 'https://your-provider.example/v1' : 'http://127.0.0.1:11434'} value={draft.baseUrl} onChange={(event) => update({ baseUrl: event.target.value })} />
            </>}
            {isChatGPT ? <ChatGPTConnection accountId={draft.accountId || ''} secureStorageAvailable={Boolean(settings?.secureStorageAvailable)} disabled={Boolean(busy)} open={isOpen} onReady={setChatReady} onSignedOut={() => { void fetchAISettings().then(setSettings).catch(() => {}) }} onAccount={(accountId) => { update({ accountId, model: '' }); setChatModels([]) }} /> : <>
            <label htmlFor={`${fieldID}-key`}>API key{provider.optionalKey ? ' (optional)' : ''}</label>
            <input id={`${fieldID}-key`} type="password" autoComplete="off" spellCheck={false} value={draft.apiKey} placeholder={savedKey ? 'Saved key · leave blank to keep' : 'Paste your API key'} onChange={(event) => update({ apiKey: event.target.value })} />
            </>}
            <div className="ai-model-heading"><label htmlFor={`${fieldID}-model`}>Model</label><button type="button" className="text-button sharing-help-trigger" disabled={!canConnect || modelLoading} onClick={() => { manualRefresh.current = true; setRefreshModels((value) => value + 1) }}>{modelLoading ? 'Loading…' : 'Refresh models'}</button></div>
            {modelLoading && <small className="playlist-hint" role="status">Loading models…</small>}
            {modelError && <p className="playlist-error" role="alert">{modelError}</p>}
            {modelNotice && <small className="playlist-hint" role="status">{modelNotice}</small>}
            {isChatGPT ? <select id={`${fieldID}-model`} required value={draft.model} onChange={(event) => update({ model: event.target.value })}>
              <option value="">Choose a model</option>
              {draft.model && !chatModels.some((model) => model.id === draft.model) && <option value={draft.model}>{draft.model}</option>}
              {chatModels.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
            </select> : <input id={`${fieldID}-model`} required list={`${fieldID}-models`} placeholder="Select or enter a text model ID" value={draft.model} onChange={(event) => update({ model: event.target.value })} />}
            <datalist id={`${fieldID}-models`}>{models.map((model) => <option key={model} value={model} />)}</datalist>
            {!isChatGPT && <>
            <label className="ai-remember"><input type="checkbox" checked={draft.rememberKey} disabled={!settings?.secureStorageAvailable && !draft.rememberKey} onChange={(event) => setDraft((draft) => ({ ...draft, rememberKey: event.target.checked }))} />Remember key securely</label>
            <small className="playlist-hint">{settings?.secureStorageAvailable ? 'Stored in your OS credential store. Uncheck to keep it for this server session only.' : 'Secure storage is unavailable. Keys are kept for this server session only.'}</small>
            </>}
            <div className="playlist-actions">
              <button type="button" className="button button--secondary" disabled={!canConnect || !draft.model.trim()} onClick={() => void run('test', async () => {
                await testAIConnection(draft)
                setTested(true)
                setFeedback('Connection works. Save to use it.')
              })}>{busy === 'test' ? 'Testing…' : tested ? 'Test again' : 'Test connection'}</button>
              <button className="button button--primary" disabled={isChatGPT ? !canConnect || !draft.model : !tested}>{busy === 'save' ? 'Saving…' : 'Save'}</button>
              {!isChatGPT && settings?.model && <button type="button" className="text-button" onClick={() => {
                if (window.confirm('Disconnect AI and remove the saved key?')) void run('clear', async () => apply(await clearAISettings()))
              }}>Disconnect AI</button>}
            </div>
            {tested && <p className="ai-test-success" role="status"><Check size={18} aria-hidden="true" />{feedback}</p>}
            <small className="playlist-hint">Testing sends a small AI request. Picks send recent song titles and artists; your provider or ChatGPT plan limits apply.</small>
          </fieldset>
        </form>
      </dialog>
    </div>
  )
}

function message(error: unknown) { return error instanceof Error ? error.message : 'The AI connection failed. Try again.' }
