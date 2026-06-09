import { Outlet } from 'react-router-dom'
import { Navbar } from './Navbar'
import { Sidebar } from './Sidebar'
import { useAuthStore } from '@/store/authStore'

export function PageShell() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)

  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <Navbar />
      <div className="flex flex-1 overflow-hidden">
        {isAuthenticated && <Sidebar />}
        <main className="flex-1 overflow-y-auto bg-[var(--color-bg)] p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
