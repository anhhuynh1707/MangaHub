import { toast } from 'sonner'
import { X } from 'lucide-react'
import { notifMeta } from './notificationMeta'

interface EventToastOptions {
  type?: string // notification type ("new_chapter", "system", …) or "progress"
  title: string
  message: string
}

/**
 * showEventToast renders an on-theme toast for live SSE events: a clean surface
 * card (matching the app's other cards) with a colored left accent bar + tinted
 * icon chip per event type — colorful, but not a loud full-bleed gradient.
 */
export function showEventToast({ type, title, message }: EventToastOptions) {
  const meta = notifMeta(type)
  const Icon = meta.icon

  toast.custom(
    (id) => (
      <div className="pointer-events-auto relative flex w-[356px] max-w-[calc(100vw-2rem)] items-start gap-3 overflow-hidden rounded-2xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-4 pl-5 shadow-lg shadow-black/5">
        {/* Colored accent bar — the only strong color, keeps it on-theme */}
        <span className={`absolute left-0 top-0 h-full w-1.5 ${meta.accent}`} />

        <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${meta.tint} ${meta.fg}`}>
          <Icon className="h-5 w-5" strokeWidth={2.25} />
        </div>

        <div className="min-w-0 flex-1 pt-0.5">
          <p className="font-semibold leading-tight tracking-tight text-[var(--color-text)]">{title}</p>
          <p className="mt-0.5 text-sm leading-snug text-[var(--color-text2)]">{message}</p>
        </div>

        <button
          onClick={() => toast.dismiss(id)}
          className="-mr-1 -mt-1 shrink-0 rounded-lg p-1.5 text-[var(--color-muted-raw)] transition hover:bg-[var(--color-surface2)] hover:text-[var(--color-text2)]"
          aria-label="Dismiss"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    ),
    { unstyled: true, duration: 5000 }
  )
}
