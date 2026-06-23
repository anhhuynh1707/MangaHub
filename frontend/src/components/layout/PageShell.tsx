import { Outlet, useLocation } from 'react-router-dom'
import { Navbar } from './Navbar'
import { Sidebar } from './Sidebar'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { useAuthStore } from '@/store/authStore'

export function PageShell() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const { pathname } = useLocation()

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
