import { useEffect, useRef, useState } from 'react'
import { Bell, BellOff } from 'lucide-react'
import { useNotificationStore } from '@/store/notificationStore'

function timeAgo(ms: number): string {
  const secs = Math.max(0, Math.floor((Date.now() - ms) / 1000))
  if (secs < 60) return 'just now'
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return new Date(ms).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

/**
 * NotificationBell shows a bell with an unread badge and a dropdown of recent
 * live notifications (UDP events bridged over SSE). Opening it marks all read.
 */
export function NotificationBell() {
  const [open, setOpen] = useState(false)
  const items = useNotificationStore((s) => s.items)
  const unread = useNotificationStore((s) => s.unread)
  const markAllRead = useNotificationStore((s) => s.markAllRead)
  const clear = useNotificationStore((s) => s.clear)
  const ref = useRef<HTMLDivElement>(null)

  // Close on outside click.
  useEffect(() => {
    if (!open) return
    function onClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onClick)
    return () => document.removeEventListener('mousedown', onClick)
  }, [open])

  function toggle() {
    setOpen((o) => {
      if (!o) markAllRead()
      return !o
    })
  }

  return (
    <div ref={ref} className="relative">
      <button
        onClick={toggle}
        className="relative rounded-lg p-1.5 text-[var(--color-muted-raw)] hover:bg-[var(--color-surface2)] hover:text-[var(--color-text2)]"
        aria-label="Notifications"
      >
        <Bell className="h-4 w-4" />
        {unread > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--brand-red)] px-1 text-[10px] font-bold leading-none text-white">
            {unread > 9 ? '9+' : unread}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-2 w-80 overflow-hidden rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] shadow-lg">
          <div className="flex items-center justify-between border-b border-[var(--color-border-raw)] px-4 py-2.5">
            <span className="text-sm font-semibold text-[var(--color-text)]">Notifications</span>
            {items.length > 0 && (
              <button
                onClick={clear}
                className="text-xs text-[var(--color-muted-raw)] hover:text-[var(--color-text2)]"
              >
                Clear
              </button>
            )}
          </div>

          <div className="max-h-96 overflow-y-auto">
            {items.length === 0 ? (
              <div className="flex flex-col items-center gap-2 px-4 py-8 text-center text-[var(--color-muted-raw)]">
                <BellOff className="h-6 w-6" />
                <span className="text-sm">No notifications yet</span>
              </div>
            ) : (
              items.map((n) => (
                <div
                  key={n.id}
                  className="border-b border-[var(--color-border-raw)] px-4 py-3 last:border-b-0 hover:bg-[var(--color-surface2)]"
                >
                  {n.title && (
                    <p className="text-sm font-medium text-[var(--color-text)]">{n.title}</p>
                  )}
                  <p className="text-sm text-[var(--color-text2)]">{n.message}</p>
                  <p className="mt-0.5 text-xs text-[var(--color-muted-raw)]">{timeAgo(n.receivedAt)}</p>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
