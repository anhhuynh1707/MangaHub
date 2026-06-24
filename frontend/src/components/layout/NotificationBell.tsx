import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { Bell, BellOff } from 'lucide-react'
import { useNotificationStore } from '@/store/notificationStore'
import { notifMeta } from '@/lib/notificationMeta'

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

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.14, ease: 'easeOut' }}
            className="absolute right-0 top-full mt-2 w-80 origin-top-right overflow-hidden rounded-2xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] shadow-xl shadow-black/10"
          >
            {/* Header — simple, on-theme (brand-red accent) */}
            <div className="flex items-center justify-between border-b border-[var(--color-border-raw)] px-4 py-3">
              <span className="flex items-center gap-2 text-sm font-semibold text-[var(--color-text)]">
                <Bell className="h-4 w-4 text-[var(--brand-red)]" /> Notifications
              </span>
              {items.length > 0 && (
                <button
                  onClick={clear}
                  className="rounded-md px-2 py-0.5 text-xs font-medium text-[var(--color-muted-raw)] transition hover:bg-[var(--color-surface2)] hover:text-[var(--color-text2)]"
                >
                  Clear
                </button>
              )}
            </div>

            <div className="max-h-96 overflow-y-auto">
              {items.length === 0 ? (
                <div className="flex flex-col items-center gap-2 px-4 py-10 text-center text-[var(--color-muted-raw)]">
                  <BellOff className="h-7 w-7 opacity-60" />
                  <span className="text-sm">No notifications yet</span>
                </div>
              ) : (
                items.map((n) => {
                  const meta = notifMeta(n.type)
                  const Icon = meta.icon
                  return (
                    <div
                      key={n.id}
                      className="flex items-start gap-3 border-b border-[var(--color-border-raw)] px-4 py-3 last:border-b-0 hover:bg-[var(--color-surface2)]"
                    >
                      <div
                        className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl ${meta.tint} ${meta.fg}`}
                      >
                        <Icon className="h-5 w-5" strokeWidth={2.25} />
                      </div>
                      <div className="min-w-0 flex-1">
                        {n.title && (
                          <p className="text-sm font-semibold text-[var(--color-text)]">{n.title}</p>
                        )}
                        <p className="text-sm leading-snug text-[var(--color-text2)]">{n.message}</p>
                        <p className="mt-1 text-xs text-[var(--color-muted-raw)]">{timeAgo(n.receivedAt)}</p>
                      </div>
                    </div>
                  )
                })
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
