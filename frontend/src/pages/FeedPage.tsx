import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Activity as ActivityIcon, Send, Loader2 } from 'lucide-react'
import { feedApi, ACTIVITY_FILTERS } from '@/api/feed'
import { ActivityItem } from '@/components/ActivityItem'

export default function FeedPage() {
  const qc = useQueryClient()
  const [filter, setFilter] = useState('')
  const [draft, setDraft] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['feed', filter],
    queryFn: () =>
      feedApi.list({ limit: 50, type: filter || undefined }).then((r) => r.data.data),
    refetchOnMount: 'always',
  })

  const postMutation = useMutation({
    mutationFn: (message: string) => feedApi.post(message),
    onSuccess: () => {
      setDraft('')
      qc.invalidateQueries({ queryKey: ['feed'] })
    },
  })

  const activities = data?.activities ?? []

  function handlePost(e: React.FormEvent) {
    e.preventDefault()
    const text = draft.trim()
    if (!text) return
    postMutation.mutate(text)
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-8">
      {/* Header */}
      <div className="mb-6 flex items-center gap-2">
        <ActivityIcon className="h-6 w-6 text-[var(--brand-red)]" />
        <h1 className="text-2xl font-bold text-[var(--color-text)]">Activity Feed</h1>
      </div>

      {/* Post composer */}
      <form
        onSubmit={handlePost}
        className="mb-6 flex items-center gap-2 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-2"
      >
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          maxLength={500}
          placeholder="Share something with the community…"
          className="flex-1 bg-transparent px-2 py-1.5 text-sm text-[var(--color-text)] placeholder-[var(--color-muted-raw)] focus:outline-none"
        />
        <button
          type="submit"
          disabled={!draft.trim() || postMutation.isPending}
          className="flex h-9 items-center gap-1.5 rounded-lg bg-[var(--brand-red)] px-3 text-sm font-semibold text-white transition-colors hover:bg-[var(--brand-red-hover)] disabled:opacity-40"
        >
          {postMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
          Post
        </button>
      </form>

      {/* Filter chips */}
      <div className="mb-5 flex flex-wrap gap-2">
        {ACTIVITY_FILTERS.map((f) => (
          <button
            key={f.value}
            onClick={() => setFilter(f.value)}
            className={`rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors ${
              filter === f.value
                ? 'bg-[var(--brand-red)] text-white'
                : 'bg-[var(--color-surface2)] text-[var(--color-text2)] hover:bg-[var(--color-surface)]'
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>

      {/* Feed */}
      {isLoading ? (
        <FeedSkeletons />
      ) : activities.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-[var(--color-border-raw)] py-16 text-center">
          <ActivityIcon className="h-8 w-8 text-[var(--color-muted-raw)] opacity-30" />
          <p className="text-sm text-[var(--color-muted-raw)]">
            No activity yet. Start reading or reviewing to see it here!
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-2.5">
          {activities.map((a) => (
            <ActivityItem key={a.id} activity={a} />
          ))}
        </div>
      )}
    </div>
  )
}

function FeedSkeletons() {
  return (
    <div className="flex flex-col gap-2.5">
      {Array.from({ length: 5 }).map((_, i) => (
        <div
          key={i}
          className="flex animate-pulse items-start gap-3 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-3.5"
        >
          <div className="h-9 w-9 flex-shrink-0 rounded-full bg-[var(--color-surface2)]" />
          <div className="flex-1 space-y-2 py-1">
            <div className="h-3.5 w-3/4 rounded bg-[var(--color-surface2)]" />
            <div className="h-3 w-1/4 rounded bg-[var(--color-surface2)]" />
          </div>
        </div>
      ))}
    </div>
  )
}
