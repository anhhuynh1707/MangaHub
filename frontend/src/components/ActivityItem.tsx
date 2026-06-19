import { Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import { BookOpen, CheckCircle2, Star, UserPlus, Activity as ActivityIcon } from 'lucide-react'
import type { Activity } from '@/api/feed'

const TYPE_META: Record<string, { icon: React.ElementType; color: string; verb: string }> = {
  started_manga:   { icon: BookOpen,     color: 'text-blue-500 bg-blue-500/10',     verb: 'started reading' },
  completed_manga: { icon: CheckCircle2, color: 'text-emerald-500 bg-emerald-500/10', verb: 'completed' },
  wrote_review:    { icon: Star,         color: 'text-amber-500 bg-amber-500/10',   verb: 'reviewed' },
  added_friend:    { icon: UserPlus,     color: 'text-purple-500 bg-purple-500/10', verb: 'added a friend' },
}

function timeAgo(iso: string): string {
  const then = new Date(iso).getTime()
  const secs = Math.max(0, Math.floor((Date.now() - then) / 1000))
  if (secs < 60) return 'just now'
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  if (days < 7) return `${days}d ago`
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

export function ActivityItem({ activity }: { activity: Activity }) {
  const meta = TYPE_META[activity.type] ?? {
    icon: ActivityIcon,
    color: 'text-[var(--color-muted-raw)] bg-[var(--color-surface2)]',
    verb: '',
  }
  const Icon = meta.icon

  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      className="flex items-start gap-3 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-3.5"
    >
      <div className={`flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full ${meta.color}`}>
        <Icon className="h-4 w-4" />
      </div>

      <div className="min-w-0 flex-1">
        {/* The backend stores a self-contained message ("alice started reading X")
            but not username/manga_title separately, so the message is the source
            of truth. Link the whole line to the manga when we have its id. */}
        <ActivityText
          message={activity.message || meta.verb}
          mangaId={activity.manga_id}
        />
        <p className="mt-0.5 text-xs text-[var(--color-muted-raw)]">{timeAgo(activity.created_at)}</p>
      </div>
    </motion.div>
  )
}

function ActivityText({ message, mangaId }: { message: string; mangaId?: string }) {
  if (mangaId) {
    return (
      <Link
        to={`/manga/${mangaId}`}
        className="text-sm leading-snug text-[var(--color-text)] hover:text-[var(--brand-red)]"
      >
        {message}
      </Link>
    )
  }
  return <p className="text-sm leading-snug text-[var(--color-text)]">{message}</p>
}
