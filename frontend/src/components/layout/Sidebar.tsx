import { NavLink } from 'react-router-dom'
import { Home, Library, MessageSquare, Activity, User, Search } from 'lucide-react'
import { useUIStore } from '@/store/uiStore'

const navItems = [
  { to: '/',        icon: Home,          label: 'Browse' },
  { to: '/library', icon: Library,       label: 'My Library' },
  { to: '/chat',    icon: MessageSquare, label: 'Chat' },
  { to: '/feed',    icon: Activity,      label: 'Feed' },
  { to: '/profile', icon: User,          label: 'Profile' },
]

export function Sidebar() {
  const sidebarOpen = useUIStore((s) => s.sidebarOpen)

  return (
    <aside
      className={`flex flex-col gap-1 border-r border-[var(--color-border-raw)] bg-[var(--color-surface)] p-2 transition-all duration-300 ${
        sidebarOpen ? 'w-52' : 'w-0 overflow-hidden p-0'
      }`}
    >
      {/* Quick search shortcut */}
      <div className="mb-1 flex items-center gap-2 rounded-lg bg-[var(--color-surface2)] px-3 py-2 text-sm text-[var(--color-muted-raw)]">
        <Search className="h-4 w-4 flex-shrink-0" />
        <span className="truncate">Search manga…</span>
        <kbd className="ml-auto text-xs opacity-60">⌘K</kbd>
      </div>

      {navItems.map(({ to, icon: Icon, label }) => (
        <NavLink
          key={to}
          to={to}
          end={to === '/'}
          className={({ isActive }) =>
            `flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium no-underline transition-colors ${
              isActive
                ? 'bg-[var(--brand-red)]/10 text-[var(--brand-red)]'
                : 'text-[var(--color-text2)] hover:bg-[var(--color-surface2)] hover:text-[var(--color-text)]'
            }`
          }
        >
          <Icon className="h-4 w-4 flex-shrink-0" />
          <span className="truncate">{label}</span>
        </NavLink>
      ))}
    </aside>
  )
}
