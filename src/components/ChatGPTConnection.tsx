import { useEffect, useId, useRef, useState } from 'react'
import { cancelChatGPTLogin, fetchChatGPTStatus, signOutChatGPT, startChatGPTLogin, welcomeChatGPT } from '../lib/ai'
import type { ChatGPTAccount } from '../lib/ai'

interface Props {
  accountId: string
  secureStorageAvailable: boolean
  disabled: boolean
  open: boolean
  onAccount: (id: string) => void
  onReady: (ready: boolean) => void
  onSignedOut: () => void
}
export function ChatGPTConnection({ accountId, secureStorageAvailable, disabled, open, onAccount, onReady, onSignedOut }: Props) {
  const [accounts, setAccounts] = useState<ChatGPTAccount[]>([])
  const [remember, setRemember] = useState(secureStorageAvailable)
  const [pending, setPending] = useState(false)
  const [working, setWorking] = useState(false)
  const [link, setLink] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [welcome, setWelcome] = useState('')
  const welcomeDialog = useRef<HTMLDialogElement>(null)
  const initiated = useRef(false)
  const generation = useRef(0)
  const callbacks = useRef({ onAccount, onReady })
  useEffect(() => { callbacks.current = { onAccount, onReady } }, [onAccount, onReady])
  const selected = accounts.find((account) => account.id === accountId)
  const id = useId()

  useEffect(() => { setRemember(secureStorageAvailable && (selected?.id ? Boolean(selected.remembered) : true)) }, [secureStorageAvailable, selected?.id, selected?.remembered])
  useEffect(() => { onReady(Boolean(selected?.connected) && !pending) }, [selected?.connected, pending, onReady])
  useEffect(() => {
    if (!open) return
    const lifecycle = generation
    let stopped = false
    let timer: ReturnType<typeof setTimeout>
    async function refresh() {
      try {
        const status = await fetchChatGPTStatus()
        if (stopped) return
        setAccounts(status.accounts)
        if (initiated.current) {
          if (status.login.status === 'success' && status.login.accountId) {
            initiated.current = false
            setPending(false)
            setLink('')
            callbacks.current.onAccount(status.login.accountId)
            const account = status.accounts.find((account) => account.id === status.login.accountId)
            if (!account?.welcomed) setWelcome(status.login.accountId)
            setNotice('Connected. Choose a model to continue.')
          } else if (status.login.status === 'error' || status.login.status === 'idle') {
            initiated.current = false
            setPending(false)
            setLink('')
            setError(status.login.message || 'Sign-in canceled.')
          }
        }
      } catch (error) { if (!stopped) setError(errorMessage(error)) }
      if (!stopped) timer = setTimeout(() => void refresh(), 1500)
    }
    void refresh()
    return () => {
      stopped = true
      lifecycle.current++
      clearTimeout(timer)
      if (initiated.current) { initiated.current = false; void cancelChatGPTLogin().catch(() => {}) }
      setPending(false)
      setLink('')
    }
  }, [open])
  useEffect(() => { if (open && selected?.connected && !selected.welcomed) setWelcome(selected.id) }, [open, selected?.connected, selected?.welcomed, selected?.id])
  useEffect(() => { if (welcome && open) welcomeDialog.current?.showModal(); else welcomeDialog.current?.close() }, [welcome, open])

  async function login() {
    const attemptGeneration = generation.current
    setError(''); setNotice(''); setWorking(true)
    // Open synchronously so the browser does not block the sign-in tab.
    const tab = window.open('about:blank', '_blank')
    if (tab) tab.opener = null
    try {
      const result = await startChatGPTLogin(accountId, remember)
      if (generation.current !== attemptGeneration) { tab?.close(); await cancelChatGPTLogin(); return }
      initiated.current = true
      setPending(true)
      setLink(result.url)
      if (tab) tab.location.href = result.url
    } catch (error) { tab?.close(); setError(errorMessage(error)) }
    finally { setWorking(false) }
  }
  async function acknowledge() {
    setWorking(true)
    try { await welcomeChatGPT(welcome); setAccounts((accounts) => accounts.map((account) => account.id === welcome ? { ...account, welcomed: true } : account)); setWelcome('') }
    catch (error) { setError(errorMessage(error)); setWelcome('') }
    finally { setWorking(false) }
  }
  return <div className="chatgpt-connection">
    <label htmlFor={id}>ChatGPT account</label>
    <select id={id} value={accountId} disabled={disabled || working || pending} onChange={(event) => { onAccount(event.target.value); setError(''); setNotice('') }}>
      <option value="">Add a ChatGPT account</option>
      {accounts.map((account) => <option key={account.id} value={account.id}>{account.label}{account.connected ? '' : ' · Signed out'}</option>)}
    </select>
    <>
      <label className="ai-remember"><input type="checkbox" checked={remember} disabled={!secureStorageAvailable || disabled || working || pending} onChange={(event) => setRemember(event.target.checked)} />Remember sign-in securely</label>
      <small className="playlist-hint">{secureStorageAvailable ? 'For your next sign-in. Uncheck for this server session only.' : 'Session only. Sign in again after restarting the Go service.'}</small>
    </>
    <div className="playlist-actions">
      {!pending && <button type="button" className="button button--secondary chatgpt-signin" disabled={disabled || working} onClick={() => void login()}>{selected?.connected ? 'Sign in again' : 'Continue with ChatGPT'}</button>}
      {pending && <>
        <span role="status">Finish sign-in in your browser…</span>
        {link && <a href={link} target="_blank" rel="noreferrer">Open sign-in</a>}
        <button type="button" className="text-button" disabled={working} onClick={async () => {
          setWorking(true)
          try { await cancelChatGPTLogin(); initiated.current = false; setPending(false); setLink('') }
          catch (error) { setError(errorMessage(error)) }
          finally { setWorking(false) }
        }}>Cancel sign-in</button>
      </>}
      {selected?.connected && <button type="button" className="text-button" disabled={disabled || working || pending} onClick={async () => {
        if (!window.confirm('Sign out of this ChatGPT account in Party Hub?')) return
        setWorking(true); setError('')
        try {
          const result = await signOutChatGPT(accountId)
          setAccounts((accounts) => accounts.map((account) => account.id === accountId ? { ...account, connected: false } : account))
          callbacks.current.onReady(false)
          onSignedOut()
          setNotice(result.warning || 'Signed out.')
        } catch (error) { setError(errorMessage(error)) }
        finally { setWorking(false) }
      }}>Sign out</button>}
    </div>
    {selected?.connected && <small className="playlist-hint">{selected.remembered ? 'Sign-in stored securely.' : 'Signed in for this server session.'}</small>}
    <p className="playlist-hint">Eligible picks use your ChatGPT plan. <a href="https://chatgpt.com/settings/usage" target="_blank" rel="noreferrer">Manage usage</a></p>
    {error && <p className="playlist-error" role="alert">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    <dialog ref={welcomeDialog} className="playlist-dialog chatgpt-welcome" aria-labelledby={`${id}-welcome`} onCancel={(event) => { event.preventDefault(); void acknowledge() }}>
      <h2 id={`${id}-welcome`}>You’re using your ChatGPT plan</h2>
      <p>AI picks use your plan. Manage usage in ChatGPT settings.</p>
      <button type="button" className="button button--primary" disabled={working} onClick={() => void acknowledge()}>Got it</button>
    </dialog>
  </div>
}
function errorMessage(error: unknown) { return error instanceof Error ? error.message : 'ChatGPT connection failed. Try again.' }
