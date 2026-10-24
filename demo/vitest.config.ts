import { defineConfig, mergeConfig } from 'vitest/config'
import vite from './vite.config'

export default mergeConfig(vite, defineConfig({
  test: {
    environment: 'jsdom',
    include: ['test/ui.tsx'],
    setupFiles: ['../src/test/setup.ts'],
  },
}))
