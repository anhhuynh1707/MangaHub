import { Link, useNavigate } from 'react-router-dom'
import { BookOpen, Menu, LogOut, User } from 'lucide-react'
import { ThemeToggle } from '@/components/ui/ThemeToggle'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { authApi } from '@/api/auth'

export function Navbar() {
  const { isAuthenticated, username, clearAuth } = useAuthStore()
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)
  const navigate = useNavigate()

  async function handleLogout() {
    try { await authApi.logout() } catch { /* ignore */ }
    clearAuth()
    navigate('/auth')
  }

  return (
    <header className="sticky top-0 z-50 flex h-14 items-center gap-3 border-b border-[var(--color-border-raw)] bg-[var(--color-surface)] px-4 shadow-sm">
      {/* Sidebar toggle (only when authenticated) */}
      {isAuthenticated && (
        <button
          onClick={toggleSidebar}
          className="rounded-lg p-1.5 text-[var(--color-muted-raw)] hover:bg-[var(--color-surface2)] hover:text-[var(--color-text)]"
          aria-label="Toggle sidebar"
        >
          <Menu className="h-5 w-5" />
        </button>
      )}

      {/* Logo */}
      <Link to="/" className="flex items-center gap-2 font-bold text-[var(--color-text)] no-underline">
        <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-[var(--brand-red)] text-white">
          <BookOpen className="h-4 w-4" />
        </div>
        <span>
          Manga<span className="text-[var(--brand-red)]"> Hub</span>
        </span>
      </Link>

      <div className="flex-1" />

      {/* Theme toggle */}
      <ThemeToggle />

      {/* Auth controls */}
      {isAuthenticated ? (
        <div className="flex items-center gap-2">
          <Link
            to="/profile"
            className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-[var(--color-text2)] hover:bg-[var(--color-surface2)] no-underline"
          >
            <User className="h-4 w-4" />
            {username}
          </Link>
          <button
            onClick={handleLogout}
            className="rounded-lg p-1.5 text-[var(--color-muted-raw)] hover:bg-[var(--color-surface2)] hover:text-[var(--color-error)]"
            aria-label="Logout"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      ) : (
        <Link
          to="/auth"
          className="rounded-lg bg-[var(--brand-red)] px-4 py-1.5 text-sm font-semibold text-white no-underline hover:bg-[var(--brand-red-hover)]"
        >
          Sign in
        </Link>
      )}
    </header>
  )
}
