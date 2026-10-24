export function DevDataSwitcher() {
  const params = new URLSearchParams(window.location.search)
  if (!import.meta.env.DEV || !params.has('data') || params.get('switcher') !== '1') return null
  const current = params.get('data') || 'empty'

  function select(mode: string) {
    const url = new URL(window.location.href)
    url.searchParams.set('data', mode)
    window.location.href = url.toString()
  }

  return (
    <div className="dev-data-switcher" aria-label="Development fixture">
      {['empty', 'one', 'worst'].map((mode) => (
        <button
          key={mode}
          className={current === mode ? 'is-active' : ''}
          onClick={() => select(mode)}
        >
          {mode === 'worst' ? 'Worst case' : mode[0].toUpperCase() + mode.slice(1)}
        </button>
      ))}
    </div>
  )
}
