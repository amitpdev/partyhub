import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { PlayerRecovery } from './PlayerRecovery'

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', '') }
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open') }
})
afterEach(cleanup)

it('opens recovery instructions from the warning and can dismiss them', () => {
  render(<PlayerRecovery missing={[]} />)
  expect(screen.getByRole('status')).toHaveTextContent('cannot reach the local playback service')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'More info' }))
  const dialog = screen.getByRole('dialog', { name: 'Get the player running' })
  expect(within(dialog).getByText('yarn party', { selector: 'pre code' })).toBeInTheDocument()
  expect(within(dialog).getByText('yarn dev:server')).toBeInTheDocument()
  expect(dialog).toHaveTextContent('reconnects automatically')
  expect(dialog).toHaveTextContent('new QR code')
  fireEvent.click(screen.getByRole('button', { name: 'Got it' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('shows targeted installation instructions for missing playback tools', () => {
  render(<PlayerRecovery missing={['mpv', 'yt-dlp']} />)
  expect(screen.getByRole('status')).toHaveTextContent('Missing mpv and yt-dlp')
  fireEvent.click(screen.getByRole('button', { name: 'More info' }))
  expect(screen.getByText('brew install mpv yt-dlp')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Close player help' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('does not suggest reinstalling tools for a lost service connection', () => {
  render(<PlayerRecovery missing={[]} />)
  fireEvent.click(screen.getByRole('button', { name: 'More info' }))
  expect(screen.queryByRole('heading', { name: 'Install missing playback tools first' })).not.toBeInTheDocument()
})
