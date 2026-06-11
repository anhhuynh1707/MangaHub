import { useState } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient, useQueries } from '@tanstack/react-query'
import { motion, AnimatePresence } from 'framer-motion'
import {
  BookOpen, BookMarked, Download, ChevronLeft, ChevronRight,
  Trash2, Loader2, Check, Clock,
} from 'lucide-react'
import { libraryApi, LIBRARY_STATUSES } from '@/api/library'
import type { LibraryStatus, UserProgress } from '@/api/library'
import { mangaApi } from '@/api/manga'
import type { Manga } from '@/api/manga'
import { useAuthStore } from '@/store/authStore'

type Tab = LibraryStatus

const TAB_ICONS: Record<Tab, React.ElementType> = {
  reading: BookOpen,
  completed: Check,
  plan_to_read: Clock,
}

const STATUS_COLORS: Record<Tab, string> = {
  reading: 'bg-blue-500 text-white',
  completed: 'bg-emerald-500 text-white',
  plan_to_read: 'bg-amber-500 text-white',
}

export default function LibraryPage() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const [activeTab, setActiveTab] = useState<Tab>('reading')

  if (!isAuthenticated) return <Navigate to="/auth" replace />

  const { data: libraryData, isLoading } = useQuery({
    queryKey: ['library'],
    queryFn: () => libraryApi.get().then((r) => r.data.data),
    refetchOnMount: 'always',
  })

  const lists = libraryData?.reading_lists
  const allEntries = [
    ...(lists?.reading ?? []),
    ...(lists?.completed ?? []),
    ...(lists?.plan_to_read ?? []),
  ]
  const uniqueIds = [...new Set(allEntries.map((p) => p.manga_id))]

  const mangaQueries = useQueries({
    queries: uniqueIds.map((id) => ({
      queryKey: ['manga', id],
      queryFn: () => mangaApi.get(id).then((r) => r.data.data),
      staleTime: 5 * 60 * 1000,
    })),
  })

  const mangaMap = Object.fromEntries(
    uniqueIds.map((id, i) => [id, mangaQueries[i].data])
  ) as Record<string, Manga | undefined>

  const currentList = lists?.[activeTab] ?? []
  const totalCount = allEntries.length

  function handleExport() {
    const payload = {
      exported_at: new Date().toISOString(),
      username: libraryData?.username ?? '',
      library: {
        reading: (lists?.reading ?? []).map((p) => ({
          manga_id: p.manga_id,
          title: mangaMap[p.manga_id]?.title ?? p.manga_id,
          current_chapter: p.current_chapter,
          status: p.status,
          updated_at: p.updated_at,
        })),
        completed: (lists?.completed ?? []).map((p) => ({
          manga_id: p.manga_id,
          title: mangaMap[p.manga_id]?.title ?? p.manga_id,
          current_chapter: p.current_chapter,
          status: p.status,
          updated_at: p.updated_at,
        })),
        plan_to_read: (lists?.plan_to_read ?? []).map((p) => ({
          manga_id: p.manga_id,
          title: mangaMap[p.manga_id]?.title ?? p.manga_id,
          current_chapter: p.current_chapter,
          status: p.status,
          updated_at: p.updated_at,
        })),
      },
    }
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `mangahub-library-${payload.username || 'export'}.json`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  return (
    <div className="min-h-screen bg-[var(--color-bg)] px-4 py-8 md:px-8">
      <div className="mx-auto max-w-5xl">
        {/* Header */}
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-[var(--color-text)]">My Library</h1>
            <p className="mt-0.5 text-sm text-[var(--color-muted-raw)]">
              {totalCount} manga tracked
            </p>
          </div>
          <button
            onClick={handleExport}
            className="flex items-center gap-2 rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface)] px-4 py-2 text-sm font-medium text-[var(--color-text2)] transition hover:border-[var(--brand-red)] hover:text-[var(--brand-red)]"
          >
            <Download className="h-4 w-4" />
            Export
          </button>
        </div>

        {/* Tabs */}
        <div className="mb-6 flex gap-1 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-1">
          {LIBRARY_STATUSES.map(({ value, label }) => {
            const Icon = TAB_ICONS[value]
            const count = lists?.[value]?.length ?? 0
            return (
              <button
                key={value}
                onClick={() => setActiveTab(value)}
                className={`flex flex-1 items-center justify-center gap-2 rounded-lg px-3 py-2 text-sm font-semibold transition-all ${
                  activeTab === value
                    ? 'bg-[var(--brand-red)] text-white shadow-sm'
                    : 'text-[var(--color-muted-raw)] hover:text-[var(--color-text2)]'
                }`}
              >
                <Icon className="h-4 w-4" />
                <span className="hidden sm:inline">{label}</span>
                <span
                  className={`rounded-full px-1.5 py-0.5 text-xs font-bold ${
                    activeTab === value
                      ? 'bg-white/20 text-white'
                      : 'bg-[var(--color-surface2)] text-[var(--color-text2)]'
                  }`}
                >
                  {count}
                </span>
              </button>
            )
          })}
        </div>

        {/* Content */}
        {isLoading ? (
          <LibrarySkeletons />
        ) : currentList.length === 0 ? (
          <EmptyState tab={activeTab} />
        ) : (
          <AnimatePresence mode="wait">
            <motion.div
              key={activeTab}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.18 }}
              className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
            >
              {currentList.map((progress) => (
                <LibraryCard
                  key={progress.manga_id}
                  progress={progress}
                  manga={mangaMap[progress.manga_id]}
                />
              ))}
            </motion.div>
          </AnimatePresence>
        )}
      </div>
    </div>
  )
}

/* ── Library Card ──────────────────────────────────────────────────── */
function LibraryCard({ progress, manga }: { progress: UserProgress; manga: Manga | undefined }) {
  const queryClient = useQueryClient()
  const [confirmRemove, setConfirmRemove] = useState(false)

  const updateMutation = useMutation({
    mutationFn: ({ chapter, status }: { chapter: number; status: LibraryStatus }) =>
      libraryApi.updateProgress(progress.manga_id, chapter, status),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['library'] }),
  })

  const removeMutation = useMutation({
    mutationFn: () => libraryApi.remove(progress.manga_id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['library'] }),
  })

  const total = manga?.total_chapters ?? 0
  const current = progress.current_chapter
  const pct = total > 0 ? Math.min(100, Math.round((current / total) * 100)) : 0
  const statusLabel = LIBRARY_STATUSES.find((s) => s.value === progress.status)?.label ?? progress.status

  function adjustChapter(delta: number) {
    const next = Math.max(0, Math.min(total > 0 ? total : 9999, current + delta))
    if (next === current) return
    // Auto-complete when the last chapter is reached
    const newStatus: LibraryStatus =
      total > 0 && next >= total ? 'completed' : (progress.status as LibraryStatus)
    updateMutation.mutate({ chapter: next, status: newStatus })
  }

  return (
    <motion.div
      layout
      className="flex flex-col overflow-hidden rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)]"
    >
      {/* Cover */}
      <Link
        to={`/manga/${progress.manga_id}`}
        className="relative block aspect-[2/3] overflow-hidden bg-[var(--color-surface2)]"
      >
        {manga?.cover_url ? (
          <img
            src={manga.cover_url}
            alt={manga.title}
            className="h-full w-full object-cover transition-transform duration-300 hover:scale-105"
          />
        ) : (
          <div className="flex h-full items-center justify-center">
            <BookOpen className="h-12 w-12 text-[var(--color-muted-raw)]" />
          </div>
        )}
        {/* Status badge */}
        <div
          className={`absolute left-2 top-2 rounded-full px-2 py-0.5 text-xs font-semibold backdrop-blur-sm ${
            STATUS_COLORS[progress.status as Tab]
          }`}
        >
          {statusLabel}
        </div>
        {/* Progress overlay at bottom */}
        {total > 0 && (
          <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/70 to-transparent px-3 pb-2 pt-4">
            <div className="h-1 overflow-hidden rounded-full bg-white/30">
              <div
                className="h-full rounded-full bg-white transition-all duration-300"
                style={{ width: `${pct}%` }}
              />
            </div>
            <p className="mt-1 text-right text-[10px] font-medium text-white/80">{pct}%</p>
          </div>
        )}
      </Link>

      {/* Info */}
      <div className="flex flex-1 flex-col gap-3 p-3">
        <Link to={`/manga/${progress.manga_id}`}>
          <h3 className="line-clamp-1 text-sm font-semibold text-[var(--color-text)] transition-colors hover:text-[var(--brand-red)]">
            {manga?.title ?? progress.manga_id}
          </h3>
        </Link>

        {/* Chapter progress */}
        <div className="flex items-center justify-between text-xs text-[var(--color-muted-raw)]">
          <span>Chapter</span>
          <span>
            {current} / {total > 0 ? total : '?'}
          </span>
        </div>

        {/* Chapter controls + remove */}
        <div className="flex items-center gap-2">
          <button
            onClick={() => adjustChapter(-1)}
            disabled={current <= 0 || updateMutation.isPending}
            className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--color-border-raw)] bg-[var(--color-surface2)] text-[var(--color-text2)] transition hover:border-[var(--brand-red)] hover:text-[var(--brand-red)] disabled:opacity-40"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>

          <span className="flex flex-1 items-center justify-center text-sm font-semibold text-[var(--color-text)]">
            {updateMutation.isPending ? (
              <Loader2 className="h-4 w-4 animate-spin text-[var(--color-muted-raw)]" />
            ) : (
              `Ch. ${current}`
            )}
          </span>

          <button
            onClick={() => adjustChapter(1)}
            disabled={(total > 0 && current >= total) || updateMutation.isPending}
            className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--color-border-raw)] bg-[var(--color-surface2)] text-[var(--color-text2)] transition hover:border-[var(--brand-red)] hover:text-[var(--brand-red)] disabled:opacity-40"
          >
            <ChevronRight className="h-4 w-4" />
          </button>

          {/* Remove with inline confirmation */}
          {confirmRemove ? (
            <div className="ml-auto flex gap-1">
              <button
                onClick={() => removeMutation.mutate()}
                disabled={removeMutation.isPending}
                className="flex h-7 items-center rounded border border-red-500/30 px-2 text-xs font-semibold text-red-500 hover:bg-red-500/10 disabled:opacity-50"
              >
                {removeMutation.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : 'Yes'}
              </button>
              <button
                onClick={() => setConfirmRemove(false)}
                className="flex h-7 items-center rounded border border-[var(--color-border-raw)] px-2 text-xs font-semibold text-[var(--color-muted-raw)]"
              >
                No
              </button>
            </div>
          ) : (
            <button
              onClick={() => setConfirmRemove(true)}
              className="ml-auto flex h-7 w-7 items-center justify-center rounded-md text-[var(--color-muted-raw)] transition hover:text-red-500"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>
    </motion.div>
  )
}

/* ── Loading Skeleton ──────────────────────────────────────────────── */
function LibrarySkeletons() {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="animate-pulse overflow-hidden rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)]"
        >
          <div className="aspect-[2/3] bg-[var(--color-surface2)]" />
          <div className="flex flex-col gap-2 p-3">
            <div className="h-4 w-3/4 rounded bg-[var(--color-surface2)]" />
            <div className="h-3 w-full rounded bg-[var(--color-surface2)]" />
            <div className="h-7 w-full rounded bg-[var(--color-surface2)]" />
          </div>
        </div>
      ))}
    </div>
  )
}

/* ── Empty State ───────────────────────────────────────────────────── */
function EmptyState({ tab }: { tab: Tab }) {
  const messages: Record<Tab, { title: string; sub: string }> = {
    reading: {
      title: 'Nothing in progress',
      sub: 'Open a manga and start tracking your reading progress.',
    },
    completed: {
      title: 'No completed manga yet',
      sub: 'Finish a series and mark it as completed to see it here.',
    },
    plan_to_read: {
      title: 'Watchlist is empty',
      sub: 'Add manga to your plan-to-read list so you never forget them.',
    },
  }
  const { title, sub } = messages[tab]

  return (
    <div className="flex flex-col items-center gap-3 py-20 text-center">
      <BookMarked className="h-12 w-12 text-[var(--color-muted-raw)]" />
      <h3 className="font-semibold text-[var(--color-text)]">{title}</h3>
      <p className="max-w-xs text-sm text-[var(--color-muted-raw)]">{sub}</p>
      <Link
        to="/"
        className="mt-2 rounded-lg bg-[var(--brand-red)] px-4 py-2 text-sm font-semibold text-white transition hover:bg-[var(--brand-red-hover)]"
      >
        Browse Manga
      </Link>
    </div>
  )
}
