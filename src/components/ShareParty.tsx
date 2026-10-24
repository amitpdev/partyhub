import { AlertTriangle, Check, Copy, Share2, Users, X } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useRef, useState } from 'react'
import { endSharing, getShareInfo } from '../lib/api'
import type { ShareInfo } from '../types'

const inactiveShare: ShareInfo = { active: false, publicUrl: '', guestUrl: '' }

export function ShareParty() {
  const [open, setOpen] = useState(false)
  const [copied, setCopied] = useState(false)
  const [share, setShare] = useState<ShareInfo>(inactiveShare)
  const [loaded, setLoaded] = useState(false)
  const [loadError, setLoadError] = useState('')
  const [actionError, setActionError] = useState('')
  const [showManualCopy, setShowManualCopy] = useState(false)
  const [busy, setBusy] = useState<'copy' | 'end' | ''>('')
  const dialogRef = useRef<HTMLElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const manualLinkRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    void loadShareInfo()
  }, [])

  useEffect(() => {
    if (!open) return
    const trigger = triggerRef.current
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    window.addEventListener('keydown', closeOnEscape)
    return () => {
      window.removeEventListener('keydown', closeOnEscape)
      trigger?.focus()
    }
  }, [open])

  useEffect(() => {
    if (showManualCopy) manualLinkRef.current?.select()
  }, [showManualCopy])

  async function loadShareInfo() {
    setLoadError('')
    try {
      setShare(await getShareInfo())
      setLoaded(true)
    } catch (error) {
      setLoaded(false)
      setLoadError(`Could not load sharing status. Check the server connection and try again. ${messageFrom(error)}`)
    }
  }

  async function copyLink() {
    setBusy('copy')
    setActionError('')
    setShowManualCopy(false)
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard access is unavailable.')
      await navigator.clipboard.writeText(share.guestUrl)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1800)
    } catch (error) {
      setShowManualCopy(true)
      setActionError(`Could not copy the link automatically. Select and copy it below. ${messageFrom(error)}`)
    } finally {
      setBusy('')
    }
  }

  function openDialog() {
    setOpen(true)
    setActionError('')
    setShowManualCopy(false)
    void loadShareInfo()
  }

  async function stopSharing() {
    if (!window.confirm('End guest access? The queue will be kept.')) return
    setBusy('end')
    setActionError('')
    try {
      setShare(await endSharing())
    } catch (error) {
      setActionError(
        `Could not end guest access. Guests may still be able to join. Check the server connection and try again. ${messageFrom(error)}`,
      )
    } finally {
      setBusy('')
    }
  }

  function trapFocus(event: React.KeyboardEvent) {
    if (event.key !== 'Tab') return
    const controls = dialogRef.current?.querySelectorAll<HTMLElement>(
      'button:not(:disabled), [href], input:not(:disabled)',
    )
    if (!controls?.length) return
    const first = controls[0]
    const last = controls[controls.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  return (
    <>
      <button ref={triggerRef} className="share-trigger" onClick={openDialog}>
        <Users aria-hidden="true" />
        <span>{share.active ? 'Guests can join' : 'Share party'}</span>
        {share.active && <i aria-hidden="true" />}
      </button>

      {open && (
        <div className="modal-backdrop" onMouseDown={() => setOpen(false)}>
          <section
            ref={dialogRef}
            className="share-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="share-title"
            onMouseDown={(event) => event.stopPropagation()}
            onKeyDown={trapFocus}
          >
            <button className="dialog-close" onClick={() => setOpen(false)} aria-label="Close" autoFocus>
              <X />
            </button>
            <div className="share-dialog__heading">
              <Share2 aria-hidden="true" />
              <div>
                <h2 id="share-title">Share this party</h2>
                <p>Friends can scan the code to add songs from their phones.</p>
              </div>
            </div>

            {loadError ? (
              <div className="share-empty share-empty--error" role="alert">
                <AlertTriangle aria-hidden="true" />
                <strong>Sharing status unavailable</strong>
                <p>{loadError}</p>
                <button className="button button--primary" onClick={() => void loadShareInfo()}>
                  Retry
                </button>
              </div>
            ) : !loaded ? (
              <div className="share-empty" role="status">
                <Users aria-hidden="true" />
                <strong>Loading sharing status…</strong>
              </div>
            ) : share.active ? (
              <>
                <div className="qr-frame">
                  <QRCodeSVG value={share.guestUrl} size={220} level="M" marginSize={2} />
                </div>
                <p className="share-dialog__hint">Scan to join · The link expires when sharing ends</p>
                <div className="share-link">
                  <span>{share.publicUrl.replace('https://', '')}</span>
                  <button disabled={busy === 'copy'} onClick={() => void copyLink()}>
                    {copied ? <Check /> : <Copy />}
                    {copied ? 'Copied' : busy === 'copy' ? 'Copying…' : 'Copy link'}
                  </button>
                </div>
                {showManualCopy && (
                  <input
                    ref={manualLinkRef}
                    className="share-manual-link"
                    value={share.guestUrl}
                    readOnly
                    aria-label="Guest link to copy manually"
                    onFocus={(event) => event.currentTarget.select()}
                  />
                )}
                {actionError && <p className="share-dialog__error" role="alert">{actionError}</p>}
                <button
                  className="text-button share-dialog__end"
                  disabled={busy === 'end'}
                  onClick={() => void stopSharing()}
                >
                  {busy === 'end' ? 'Ending guest access…' : 'End guest access'}
                </button>
              </>
            ) : (
              <div className="share-empty">
                <Users aria-hidden="true" />
                <strong>Guest access is off</strong>
                <p>Run <code>yarn party</code> to start Party Hub with an ngrok guest link.</p>
              </div>
            )}
          </section>
        </div>
      )}
    </>
  )
}

function messageFrom(error: unknown) {
  return error instanceof Error ? error.message : 'Unexpected error.'
}
