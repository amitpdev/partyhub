import { Check, Palette } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { loadTheme, selectTheme, themes, type ThemeID } from '../lib/theme'

export function ThemeSwitcher() {
  const [open, setOpen] = useState(false)
  const [selected, setSelected] = useState<ThemeID>(() => loadTheme())
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return

    const closeOnOutsidePress = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false)
    }
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setOpen(false)
      triggerRef.current?.focus()
    }
    window.addEventListener('mousedown', closeOnOutsidePress)
    window.addEventListener('keydown', closeOnEscape)
    return () => {
      window.removeEventListener('mousedown', closeOnOutsidePress)
      window.removeEventListener('keydown', closeOnEscape)
    }
  }, [open])

  function choose(theme: ThemeID) {
    setSelected(theme)
    selectTheme(theme)
  }

  return (
    <div ref={rootRef} className="theme-switcher">
      <button
        ref={triggerRef}
        className="theme-trigger"
        type="button"
        aria-haspopup="true"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >
        <Palette aria-hidden="true" />
        <span>Theme</span>
      </button>

      {open && (
        <div className="theme-menu" role="radiogroup" aria-label="Dashboard theme">
          <div className="theme-menu__heading">
            <strong>Dashboard theme</strong>
            <span>Saved on this laptop</span>
          </div>
          <div className="theme-menu__options">
            {themes.map((theme) => (
              <button
                className="theme-option"
                type="button"
                role="radio"
                aria-checked={selected === theme.id}
                key={theme.id}
                onClick={() => choose(theme.id)}
              >
                <span className="theme-option__swatches" aria-hidden="true">
                  {theme.swatches.map((color) => (
                    <i key={color} style={{ backgroundColor: color }} />
                  ))}
                </span>
                <span className="theme-option__copy">
                  <strong>{theme.name}</strong>
                  <small>{theme.description}</small>
                </span>
                <Check className="theme-option__check" aria-hidden="true" />
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
