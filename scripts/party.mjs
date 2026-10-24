import { spawn } from 'node:child_process'

const localUrl = 'http://127.0.0.1:8787'
const inspectorUrl = 'http://127.0.0.1:4040/api/tunnels'
const children = new Set()

function start(command, args, options = {}) {
  const child = spawn(command, args, { stdio: 'inherit', ...options })
  children.add(child)
  child.once('exit', () => children.delete(child))
  return child
}

function run(command, args) {
  return new Promise((resolve, reject) => {
    const child = start(command, args)
    child.once('exit', (code) => {
      if (code === 0) resolve()
      else reject(new Error(`${command} exited with code ${code}`))
    })
    child.once('error', reject)
  })
}

async function waitFor(url, timeout = 20_000) {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url)
      if (response.ok) return response
    } catch {
      // The process is still starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 300))
  }
  throw new Error(`Timed out waiting for ${url}`)
}

async function discoverTunnel() {
  const deadline = Date.now() + 20_000
  while (Date.now() < deadline) {
    try {
      const response = await fetch(inspectorUrl)
      if (response.ok) {
        const payload = await response.json()
        const tunnel = payload.tunnels?.find(
          (item) => item.proto === 'https' && String(item.config?.addr).includes('8787'),
        )
        if (tunnel?.public_url) return tunnel.public_url
      }
    } catch {
      // ngrok's local inspection API is not ready yet.
    }
    await new Promise((resolve) => setTimeout(resolve, 350))
  }
  throw new Error('Could not discover the ngrok HTTPS tunnel')
}

function stop() {
  for (const child of children) child.kill('SIGTERM')
}

process.on('SIGINT', () => {
  stop()
  process.exit(0)
})
process.on('SIGTERM', () => {
  stop()
  process.exit(0)
})

try {
  console.log('Building Party Hub…')
  await run('yarn', ['build'])

  console.log('Starting the local player…')
  const server = start('go', ['run', './server'])
  server.once('exit', (code) => {
    if (code && code !== 0) {
      console.error(`Party Hub server stopped with code ${code}`)
      stop()
      process.exit(code)
    }
  })
  await waitFor(`${localUrl}/api/health`)

  console.log('Starting ngrok…')
  const ngrok = start('ngrok', ['http', '8787', '--log=stdout', '--log-format=json'])
  ngrok.once('error', () => {
    console.error('ngrok is not installed. Install it with: brew install ngrok')
  })
  const publicUrl = await discoverTunnel()

  const response = await fetch(`${localUrl}/api/share/configure`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ publicUrl }),
  })
  if (!response.ok) throw new Error(`Could not configure sharing (${response.status})`)

  console.log(`\nDJ dashboard: ${localUrl}`)
  console.log(`Guest access: ${publicUrl}/guest`)
  console.log('Open the DJ dashboard and click “Invite friends” to show the QR code.')
  console.log('Press Ctrl+C to end sharing.\n')

  await new Promise(() => {})
} catch (error) {
  console.error(error instanceof Error ? error.message : error)
  stop()
  process.exit(1)
}
