import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Star, BookOpen, CheckCircle2, Clock, Activity as ActivityIcon, Calendar } from 'lucide-react'
import { userApi } from '@/api/user'
import { feedApi } from '@/api/feed'
import { reviewApi } from '@/api/review'
import { libraryApi } from '@/api/library'
import { ActivityItem } from '@/components/ActivityItem'
import { useAuthStore } from '@/store/authStore'

type Tab = 'reviews' | 'activity'

export default function ProfilePage() {
  const userId = useAuthStore((s) => s.userId)
  const username = useAuthStore((s) => s.username)
  const [tab, setTab] = useState<Tab>('reviews')

  const { data: profile } = useQuery({
    queryKey: ['profile'],
    queryFn: () => userApi.profile().then((r) => r.data.data),
  })

  const { data: library } = useQuery({
    queryKey: ['library'],
    queryFn: () => libraryApi.get().then((r) => r.data.data),
  })

  const { data: reviewsData } = useQuery({
    queryKey: ['my-reviews'],
    queryFn: () => reviewApi.myReviews().then((r) => r.data.data),
  })

  const { data: activityData } = useQuery({
    queryKey: ['user-activities', userId],
    queryFn: () => feedApi.userActivities(userId!).then((r) => r.data.data),
    enabled: !!userId,
  })

  const lists = library?.reading_lists
  const readingCount = lists?.reading?.length ?? 0
  const completedCount = lists?.completed?.length ?? 0
  const planCount = lists?.plan_to_read?.length ?? 0
  const libraryTotal = readingCount + completedCount + planCount

  const reviews = reviewsData?.reviews ?? []
  const activities = activityData?.activities ?? []

  const displayName = profile?.username ?? username ?? 'You'
  const initials = displayName.slice(0, 2).toUpperCase()
  const joined = profile?.created_at
    ? new Date(profile.created_at).toLocaleDateString(undefined, { year: 'numeric', month: 'long' })
    : null

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      {/* ── Header card ───────────────────────────────────────────── */}
      <div className="mb-6 flex items-center gap-4 rounded-2xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-6">
        <div className="flex h-16 w-16 flex-shrink-0 items-center justify-center rounded-full bg-[var(--brand-red)] text-xl font-bold text-white">
          {initials}
        </div>
        <div>
          <h1 className="text-2xl font-bold text-[var(--color-text)]">{displayName}</h1>
          {joined && (
            <p className="mt-0.5 flex items-center gap-1.5 text-sm text-[var(--color-muted-raw)]">
              <Calendar className="h-3.5 w-3.5" />
              Member since {joined}
            </p>
          )}
        </div>
      </div>

      {/* ── Stats ─────────────────────────────────────────────────── */}
      <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard icon={BookOpen}     label="In Library" value={libraryTotal}   color="text-[var(--brand-red)]" />
        <StatCard icon={Clock}        label="Reading"    value={readingCount}   color="text-blue-500" />
        <StatCard icon={CheckCircle2} label="Completed"  value={completedCount} color="text-emerald-500" />
        <StatCard icon={Star}         label="Reviews"    value={reviews.length} color="text-amber-500" />
      </div>

      {/* ── Tabs ──────────────────────────────────────────────────── */}
      <div className="mb-4 flex gap-1 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-1">
        <TabButton active={tab === 'reviews'} onClick={() => setTab('reviews')} icon={Star} label={`Reviews (${reviews.length})`} />
        <TabButton active={tab === 'activity'} onClick={() => setTab('activity')} icon={ActivityIcon} label="My Activity" />
      </div>

      {/* ── Content ───────────────────────────────────────────────── */}
      {tab === 'reviews' ? (
        reviews.length === 0 ? (
          <EmptyState text="You haven't written any reviews yet." />
        ) : (
          <div className="flex flex-col gap-3">
            {reviews.map((r) => (
              <Link
                key={r.id}
                to={`/manga/${r.manga_id}`}
                className="block rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-4 transition-colors hover:border-[var(--brand-red)]"
              >
                <div className="mb-1.5 flex items-center gap-1.5 text-amber-400">
                  <Star className="h-4 w-4 fill-amber-400" />
                  <span className="text-sm font-semibold">{r.rating}/10</span>
                  <span className="ml-auto text-xs text-[var(--color-muted-raw)]">
                    {new Date(r.created_at).toLocaleDateString()}
                  </span>
                </div>
                <p className="line-clamp-3 text-sm text-[var(--color-text2)]">{r.text}</p>
              </Link>
            ))}
          </div>
        )
      ) : activities.length === 0 ? (
        <EmptyState text="No activity recorded yet." />
      ) : (
        <div className="flex flex-col gap-2.5">
          {activities.map((a) => (
            <ActivityItem key={a.id} activity={a} showUser={false} />
          ))}
        </div>
      )}
    </div>
  )
}

/* ── Sub-components ─────────────────────────────────────────────────── */
function StatCard({
  icon: Icon,
  label,
  value,
  color,
}: {
  icon: React.ElementType
  label: string
  value: number
  color: string
}) {
  return (
    <div className="flex flex-col items-center gap-1 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-4">
      <Icon className={`h-5 w-5 ${color}`} />
      <span className="text-xl font-bold text-[var(--color-text)]">{value}</span>
      <span className="text-xs text-[var(--color-muted-raw)]">{label}</span>
    </div>
  )
}

function TabButton({
  active,
  onClick,
  icon: Icon,
  label,
}: {
  active: boolean
  onClick: () => void
  icon: React.ElementType
  label: string
}) {
  return (
    <button
      onClick={onClick}
      className={`flex flex-1 items-center justify-center gap-2 rounded-lg px-3 py-2 text-sm font-semibold transition-all ${
        active
          ? 'bg-[var(--brand-red)] text-white'
          : 'text-[var(--color-muted-raw)] hover:text-[var(--color-text2)]'
      }`}
    >
      <Icon className="h-4 w-4" />
      {label}
    </button>
  )
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-[var(--color-border-raw)] py-14 text-center">
      <p className="text-sm text-[var(--color-muted-raw)]">{text}</p>
      <Link to="/" className="text-sm font-medium text-[var(--brand-red)] hover:underline">
        Browse Manga
      </Link>
    </div>
  )
}
