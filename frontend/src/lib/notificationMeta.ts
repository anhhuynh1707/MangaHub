import { BellRing, BookOpen, Megaphone, RefreshCw, Sparkles, type LucideIcon } from 'lucide-react'

// Visual identity for each live-event type, shared by the toast (eventToast.tsx)
// and the notification bell dropdown so they always look consistent.
export interface NotifMeta {
  icon: LucideIcon
  // Solid accent (left accent bar on the toast)
  accent: string
  // Subtle tint (icon chip background)
  tint: string
  // Icon/text color on the chip
  fg: string
}

const META: Record<string, NotifMeta> = {
  new_chapter: {
    icon: Sparkles,
    accent: 'bg-rose-500',
    tint: 'bg-rose-500/10',
    fg: 'text-rose-500',
  },
  manga_update: {
    icon: RefreshCw,
    accent: 'bg-violet-500',
    tint: 'bg-violet-500/10',
    fg: 'text-violet-500',
  },
  system: {
    icon: Megaphone,
    accent: 'bg-sky-500',
    tint: 'bg-sky-500/10',
    fg: 'text-sky-500',
  },
  progress: {
    icon: BookOpen,
    accent: 'bg-cyan-500',
    tint: 'bg-cyan-500/10',
    fg: 'text-cyan-500',
  },
}

const FALLBACK: NotifMeta = {
  icon: BellRing,
  accent: 'bg-slate-500',
  tint: 'bg-slate-500/10',
  fg: 'text-slate-500',
}

export function notifMeta(type?: string): NotifMeta {
  return (type && META[type]) || FALLBACK
}
