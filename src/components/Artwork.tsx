import { useEffect, useState } from 'react'

interface ArtworkProps {
  src: string
  compact?: boolean
}

export function Artwork({ src, compact = false }: ArtworkProps) {
  const [failed, setFailed] = useState(false)

  useEffect(() => setFailed(false), [src])

  return (
    <span className={compact ? 'artwork artwork--compact' : 'artwork'}>
      <span aria-hidden="true">PH</span>
      {src && !failed && <img src={src} alt="" onError={() => setFailed(true)} />}
    </span>
  )
}
