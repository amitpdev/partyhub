import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const demoRoot = dirname(fileURLToPath(import.meta.url))
const appRoot = resolve(demoRoot, '..')
const replacements: Record<string, string> = {
  [resolve(appRoot, 'src/components/AIPicker')]: resolve(demoRoot, 'src/AIPicker.tsx'),
  [resolve(appRoot, 'src/lib/api')]: resolve(demoRoot, 'src/api.ts'),
  [resolve(appRoot, 'src/state/queue')]: resolve(demoRoot, 'src/legacyQueue.ts'),
}

export default defineConfig({
  root: demoRoot,
  base: './',
  plugins: [
    {
      name: 'partyhub-browser-demo',
      enforce: 'pre',
      transform(code, file) {
        if (file.split('?')[0] !== resolve(appRoot, 'src/components/SearchDialog.tsx')) return
        const original = 'Choose a video up to 15 minutes long.'
        if (!code.includes(original)) this.error('The shared search description changed; update the demo search note.')
        return code.replace(original, 'Demo search shows sample songs. The full app searches YouTube.')
      },
      resolveId(source, importer) {
        if (importer && source.startsWith('.')) {
          const path = resolve(dirname(importer.split('?')[0]), source).replace(/\.ts$/, '')
          if (replacements[path]) return replacements[path]
        }
      },
    },
    react(),
  ],
  server: { fs: { allow: [appRoot] } },
  build: { outDir: resolve(demoRoot, 'dist'), emptyOutDir: true },
})
