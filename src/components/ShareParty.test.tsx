// @vitest-environment jsdom

import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ShareParty } from './ShareParty'

const { endSharing, getShareInfo, startSharing } = vi.hoisted(() => ({
  endSharing: vi.fn(),
  startSharing: vi.fn(),
  getShareInfo: vi.fn(),
}))

vi.mock('../lib/api', () => ({
  endSharing: (...args: unknown[]) => endSharing(...args),
  startSharing: (...args: unknown[]) => startSharing(...args),
  getShareInfo: (...args: unknown[]) => getShareInfo(...args),
}))

const activeShare = {
  active: true,
  publicUrl: 'https://party.ngrok.app',
  guestUrl: 'https://party.ngrok.app/guest#token=secret',
}

describe('ShareParty recovery feedback', () => {
  beforeEach(() => {
    getShareInfo.mockReset()
    endSharing.mockReset()
    startSharing.mockReset()
    getShareInfo.mockResolvedValue(activeShare)
    window.confirm = vi.fn(() => true)
  })

  afterEach(cleanup)

  it('offers the full link for manual copying when clipboard access fails', async () => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn().mockRejectedValue(new Error('permission denied')) },
    })
    render(<ShareParty />)

    fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Copy link' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Select and copy it below')
    expect(screen.getByLabelText('Guest link to copy manually')).toHaveValue(activeShare.guestUrl)
  })

  it('warns that guests may retain access when ending sharing fails', async () => {
    endSharing.mockRejectedValue(new Error('server unavailable'))
    render(<ShareParty />)

    fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))
    fireEvent.click(await screen.findByRole('button', { name: 'End guest access' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Guests may still be able to join',
    )
    expect(screen.getByRole('button', { name: 'End guest access' })).toBeEnabled()
  })

  it('shows a retry when sharing status cannot be loaded', async () => {
    getShareInfo.mockRejectedValue(new Error('offline'))
    render(<ShareParty />)

    fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Sharing status unavailable')
    fireEvent.click(screen.getByRole('button', { name: 'More info' }))
    expect(screen.getByRole('dialog', { name: 'Get the player running' })).toHaveTextContent('yarn party')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Got it' }))
    expect(screen.getByRole('dialog', { name: 'Share this party' })).toBeInTheDocument()
    getShareInfo.mockResolvedValue(activeShare)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy link' })).toBeEnabled())
  })

  it('can end and re-enable guest access from the dashboard', async () => {
    getShareInfo.mockResolvedValue(activeShare)
    endSharing.mockResolvedValue({ active: false, publicUrl: activeShare.publicUrl, guestUrl: '' })
    startSharing.mockResolvedValue({ ...activeShare, guestUrl: 'https://party.ngrok.app/guest#token=new' })
    window.confirm = vi.fn(() => true)
    render(<ShareParty />)
    fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))
    fireEvent.click(await screen.findByRole('button', { name: 'End guest access' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Allow guests' }))
    await screen.findByRole('button', { name: 'Copy link' })
    expect(startSharing).toHaveBeenCalledTimes(1)
    expect(screen.queryByText(/Run/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Invite friends' })).toHaveClass('is-active')
  })

  it('offers enabling access after reopening an inactive configured party', async () => {
    getShareInfo.mockResolvedValue({ active: false, publicUrl: activeShare.publicUrl, guestUrl: '' })
    render(<ShareParty />)
    fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))
    expect(await screen.findByRole('button', { name: 'Allow guests' })).toBeInTheDocument()
    expect(screen.queryByText(/yarn party/)).not.toBeInTheDocument()
  })

  it('keeps the enable action available when enabling fails', async () => {
    getShareInfo.mockResolvedValue({ active: false, publicUrl: activeShare.publicUrl, guestUrl: '' })
    startSharing.mockRejectedValue(new Error('offline'))
    render(<ShareParty />)
    fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Allow guests' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not enable guest access')
    expect(screen.getByRole('button', { name: 'Allow guests' })).toBeEnabled()
  })

  it('shows initial setup instructions only when no tunnel is configured', async () => {
    getShareInfo.mockResolvedValue({ active: false, publicUrl: '', guestUrl: '' })
    render(<ShareParty />)
    fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))
    expect(await screen.findByText('yarn party')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Allow guests' })).not.toBeInTheDocument()
  })

  it('automatically refreshes sharing when the service reconnects', async () => {
    getShareInfo.mockRejectedValue(new Error('offline'))
    const { rerender } = render(<ShareParty recoveryVersion={0} />)
    fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Sharing status unavailable')
    fireEvent.click(screen.getByRole('button', { name: 'More info' }))
    getShareInfo.mockResolvedValue(activeShare)
    rerender(<ShareParty recoveryVersion={1} />)
    expect(await screen.findByRole('button', { name: 'Copy link' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('picks up guest sharing when ngrok finishes starting after the service', async () => {
    vi.useFakeTimers()
    try {
      getShareInfo.mockResolvedValue({ active: false, publicUrl: '', guestUrl: '' })
      render(<ShareParty />)
      await act(async () => {})
      fireEvent.click(screen.getByRole('button', { name: 'Invite friends' }))
      await act(async () => {})
      getShareInfo.mockResolvedValue(activeShare)
      await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
      expect(screen.getByRole('button', { name: 'Copy link' })).toBeInTheDocument()
    } finally {
      cleanup()
      vi.useRealTimers()
    }
  })

})
