import { Clipboard, Link2, LoaderCircle, Plus, X } from 'lucide-react'
import { useRef, useState } from 'react'

interface LinkEntryProps {
  onAdd: (url: string) => Promise<void>
  disabled?: boolean
  label?: string
  hideLabel?: boolean
  allowPaste?: boolean
  buttonVariant?: 'primary' | 'secondary'
}

export function LinkEntry({
  onAdd,
  disabled,
  label = 'Add a YouTube link',
  hideLabel = false,
  allowPaste = false,
  buttonVariant = 'primary',
}: LinkEntryProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [url, setUrl] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    const value = url.trim()
    if (!value) return
    setLoading(true)
    setError('')
    setSuccess('')
    try {
      await onAdd(value)
      setUrl('')
      setSuccess('Added to the queue')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not add this link')
      setUrl('')
    } finally {
      setLoading(false)
    }
  }

  async function pasteLink() {
    const input = inputRef.current
    input?.focus()
    setError('')
    setSuccess('')
    try {
      if (!navigator.clipboard?.readText) throw new Error('Clipboard access unavailable')
      const value = (await navigator.clipboard.readText()).trim()
      if (!value) {
        setError('Your clipboard is empty.')
        return
      }
      setUrl(value)
    } catch {
      setError("Clipboard access is blocked here. Use the field's Paste command.")
    }
  }

  return (
    <form className="link-entry" onSubmit={submit}>
      <label className={hideLabel ? 'sr-only' : undefined} htmlFor="youtube-link">{label}</label>
      <div className="link-entry__row">
        <span className="link-entry__icon" aria-hidden="true">
          <Link2 size={20} />
        </span>
        <input
          ref={inputRef}
          id="youtube-link"
          type="url"
          inputMode="url"
          value={url}
          onChange={(event) => {
            setUrl(event.target.value)
            setError('')
            setSuccess('')
          }}
          placeholder="Paste youtube.com or youtu.be link"
          autoComplete="off"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          disabled={loading || disabled}
        />
        {url && (
          <button
            className="link-entry__clear"
            type="button"
            onClick={() => {
              setUrl('')
              setError('')
            }}
            aria-label="Clear YouTube link"
          >
            <X />
          </button>
        )}
        {!url && allowPaste && (
          <button
            className="link-entry__paste"
            type="button"
            onClick={() => void pasteLink()}
            disabled={loading || disabled}
          >
            <Clipboard />
            Paste
          </button>
        )}
        <button
          className={`button button--${buttonVariant}`}
          disabled={!url.trim() || loading || disabled}
        >
          {loading ? <LoaderCircle className="spin" size={20} /> : <Plus size={20} />}
          <span>{loading ? 'Adding' : 'Add'}</span>
        </button>
      </div>
      <div className="link-entry__feedback" aria-live="polite">
        {error && (
          <p className="field-error" role="alert">
            {error}
          </p>
        )}
        {success && <p className="field-success">{success}</p>}
      </div>
    </form>
  )
}
