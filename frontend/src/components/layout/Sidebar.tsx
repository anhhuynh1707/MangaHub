import { NavLink } from 'react-router-dom'
import { Home, Library, MessageSquare, Activity } from 'lucide-react'
import { useUIStore } from '@/store/uiStore'

const navItems = [
  { to: '/',        icon: Home,          label: 'Browse'     },
  { to: '/library', icon: Library,       label: 'My Library' },
  { to: '/chat',    icon: MessageSquare, label: 'Chat'       },
  { to: '/feed',    icon: Activity,      label: 'Feed'       },
]

export function Sidebar() {
  const sidebarOpen = useUIStore((s) => s.sidebarOpen)

  return (
    <aside
      className={`flex flex-shrink-0 flex-col gap-1 border-r border-[var(--color-border-raw)] bg-[var(--color-surface)] p-2 transition-all duration-300 ${
        sidebarOpen ? 'w-52' : 'w-14'
      }`}
    >
      {navItems.map(({ to, icon: Icon, label }) => (
        <NavLink
          key={to}
          to={to}
          end={to === '/'}
          title={sidebarOpen ? undefined : label}
          className={({ isActive }) =>
            `flex items-center rounded-lg py-2 text-sm font-medium no-underline transition-colors ${
              sidebarOpen ? 'gap-2.5 px-3' : 'justify-center px-0'
            } ${
              isActive
                ? 'bg-[var(--brand-red)]/10 text-[var(--brand-red)]'
                : 'text-[var(--color-text2)] hover:bg-[var(--color-surface2)] hover:text-[var(--color-text)]'
            }`
          }
        >
          <Icon className="h-4 w-4 flex-shrink-0" />
          {sidebarOpen && <span className="truncate">{label}</span>}
        </NavLink>
      ))}
    </aside>
  )
}
