import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ChatGPTConnection } from './ChatGPTConnection'
import { cancelChatGPTLogin, fetchChatGPTStatus, signOutChatGPT, startChatGPTLogin, welcomeChatGPT } from '../lib/ai'
import type { ChatGPTStatus } from '../lib/ai'

vi.mock('../lib/ai', () => ({ fetchChatGPTStatus: vi.fn(), startChatGPTLogin: vi.fn(), cancelChatGPTLogin: vi.fn(), signOutChatGPT: vi.fn(), welcomeChatGPT: vi.fn() }))
const account = { id: 'account-one', label: 'dj@example.test · 1', connected: true, remembered: true, welcomed: false }
const onAccount = vi.fn()
const onReady = vi.fn()
const onSignedOut = vi.fn()
let status: ChatGPTStatus
const props = { accountId: '', secureStorageAvailable: true, disabled: false, open: true, onAccount, onReady, onSignedOut }
beforeEach(() => {
  vi.resetAllMocks()
  status = { accounts: [], login: { status: 'idle' } }
  vi.mocked(fetchChatGPTStatus).mockImplementation(async () => status)
  vi.mocked(startChatGPTLogin).mockResolvedValue({ url: 'https://auth.openai.com/api/accounts/authorize?state=test' })
  vi.mocked(cancelChatGPTLogin).mockResolvedValue({ ok: true })
  vi.mocked(welcomeChatGPT).mockResolvedValue({ ok: true })
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', '') }
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open') }
  vi.spyOn(window, 'open').mockReturnValue(null)
})
afterEach(() => { cleanup(); vi.restoreAllMocks() })

it('starts browser sign-in, polls completion, and confirms plan use once', async () => {
  render(<ChatGPTConnection {...props} />)
  fireEvent.click(screen.getByRole('button', { name: 'Continue with ChatGPT' }))
  await screen.findByText('Finish sign-in in your browser…')
  expect(startChatGPTLogin).toHaveBeenCalledWith('', true)
  expect(screen.getByRole('link', { name: 'Open sign-in' })).toHaveAttribute('href', expect.stringContaining('auth.openai.com'))
  status = { accounts: [account], login: { status: 'success', accountId: account.id } }
  await screen.findByRole('dialog', { name: 'You’re using your ChatGPT plan' }, { timeout: 3500 })
  expect(onAccount).toHaveBeenCalledWith(account.id)
  fireEvent.click(screen.getByRole('button', { name: 'Got it' }))
  await waitFor(() => expect(welcomeChatGPT).toHaveBeenCalledWith(account.id))
})
it('cancels pending sign-in when settings close', async () => {
  const { rerender } = render(<ChatGPTConnection {...props} />)
  fireEvent.click(screen.getByRole('button', { name: 'Continue with ChatGPT' }))
  await screen.findByText('Finish sign-in in your browser…')
  rerender(<ChatGPTConnection {...props} open={false} />)
  await waitFor(() => expect(cancelChatGPTLogin).toHaveBeenCalledOnce())
})
it('uses session-only sign-in without a credential store', async () => {
  render(<ChatGPTConnection {...props} secureStorageAvailable={false} />)
  expect(screen.getByRole('checkbox', { name: 'Remember sign-in securely' })).toBeDisabled()
  fireEvent.click(screen.getByRole('button', { name: 'Continue with ChatGPT' }))
  await waitFor(() => expect(startChatGPTLogin).toHaveBeenCalledWith('', false))
})
it('shows sign-out recovery and leaves the saved account selectable', async () => {
  status = { accounts: [account], login: { status: 'idle' } }
  vi.mocked(signOutChatGPT).mockResolvedValue({ warning: 'Signed out locally. Disconnect Party Hub in ChatGPT Settings.' })
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  render(<ChatGPTConnection {...props} accountId={account.id} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Sign out' }))
  await screen.findByText('Signed out locally. Disconnect Party Hub in ChatGPT Settings.')
  expect(signOutChatGPT).toHaveBeenCalledWith(account.id)
  expect(onSignedOut).toHaveBeenCalled()
  expect(screen.getByRole('option', { name: /Signed out/ })).toBeInTheDocument()
  expect(onReady).toHaveBeenLastCalledWith(false)
})
it('shows declined authorization without changing the selected connection', async () => {
  render(<ChatGPTConnection {...props} />)
  fireEvent.click(screen.getByRole('button', { name: 'Continue with ChatGPT' }))
  await screen.findByText('Finish sign-in in your browser…')
  status = { accounts: [], login: { status: 'error', message: 'Sign-in was declined.' } }
  expect(await screen.findByRole('alert', {}, { timeout: 3500 })).toHaveTextContent('Sign-in was declined.')
  expect(onAccount).not.toHaveBeenCalled()
})
