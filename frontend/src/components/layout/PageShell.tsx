import { Outlet, useLocation } from 'react-router-dom'
import { Navbar } from './Navbar'
import { Sidebar } from './Sidebar'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { useAuthStore } from '@/store/authStore'
import { useServerEvents } from '@/hooks/useServerEvents'

export function PageShell() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const { pathname } = useLocation()

  // Bridge live SSE events (notifications + reading activity) into the SPA.
  // No-op until authenticated (the hook keys off the auth token).
  useServerEvents()

  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <Navbar />
      <div className="flex flex-1 overflow-hidden">
        {isAuthenticated && <Sidebar />}
        <main className="flex-1 overflow-y-auto bg-[var(--color-bg)] p-6">
          {/* Per-page error boundary — keyed by path so navigating away clears
              any caught error and the next page mounts fresh. */}
          <ErrorBoundary key={pathname}>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
    </div>
  )
}
