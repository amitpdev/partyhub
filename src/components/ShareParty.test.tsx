// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ShareParty } from './ShareParty'

const { endSharing, getShareInfo } = vi.hoisted(() => ({
  endSharing: vi.fn(),
  getShareInfo: vi.fn(),
}))

vi.mock('../lib/api', () => ({
  endSharing: (...args: unknown[]) => endSharing(...args),
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

    fireEvent.click(screen.getByRole('button', { name: /Share party|Guests can join/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'Copy link' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Select and copy it below')
    expect(screen.getByLabelText('Guest link to copy manually')).toHaveValue(activeShare.guestUrl)
  })

  it('warns that guests may retain access when ending sharing fails', async () => {
    endSharing.mockRejectedValue(new Error('server unavailable'))
    render(<ShareParty />)

    fireEvent.click(screen.getByRole('button', { name: /Share party|Guests can join/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'End guest access' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Guests may still be able to join',
    )
    expect(screen.getByRole('button', { name: 'End guest access' })).toBeEnabled()
  })

  it('shows a retry when sharing status cannot be loaded', async () => {
    getShareInfo.mockRejectedValue(new Error('offline'))
    render(<ShareParty />)

    fireEvent.click(screen.getByRole('button', { name: /Share party|Guests can join/ }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Sharing status unavailable')
    getShareInfo.mockResolvedValue(activeShare)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy link' })).toBeEnabled())
  })
})
