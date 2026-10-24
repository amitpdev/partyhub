import type { QueueState } from '../../src/types'

// Never migrate the real app's browser queue into the isolated demo.
export function loadQueue(): QueueState {
  return { tracks: [], currentIndex: -1, volume: 70 }
}
