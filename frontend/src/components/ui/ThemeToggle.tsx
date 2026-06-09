import { Moon, Sun } from 'lucide-react'
import { useUIStore } from '@/store/uiStore'

export function ThemeToggle() {
  const { theme, toggleTheme } = useUIStore()
  const isDark = theme === 'dark'

  return (
    <button
      onClick={toggleTheme}
      aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
      className="relative flex h-8 w-14 items-center rounded-full border border-[var(--color-border-raw)] bg-[var(--color-surface2)] p-1 transition-colors duration-200 hover:border-[var(--brand-red)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand-red)]"
    >
      {/* Track fill */}
      <span
        className={`absolute inset-0 rounded-full transition-colors duration-300 ${
          isDark ? 'bg-[var(--color-surface2)]' : 'bg-amber-50'
        }`}
      />

      {/* Thumb */}
      <span
        className={`relative z-10 flex h-6 w-6 items-center justify-center rounded-full shadow-sm transition-all duration-300 ${
          isDark
            ? 'translate-x-6 bg-[var(--color-surface)]'
            : 'translate-x-0 bg-white'
        }`}
      >
        {isDark ? (
          <Moon className="h-3.5 w-3.5 text-[var(--brand-teal)]" />
        ) : (
          <Sun className="h-3.5 w-3.5 text-amber-500" />
        )}
      </span>
    </button>
  )
}
