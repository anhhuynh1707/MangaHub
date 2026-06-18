import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { motion } from 'framer-motion'
import {
  ArrowLeft, BookOpen, Star, ThumbsUp, BookMarked,
  Trash2, CheckCircle2, MessageSquare, Plus, ChevronDown,
} from 'lucide-react'
import { mangaApi } from '@/api/manga'
import { libraryApi, LIBRARY_STATUSES, type LibraryStatus } from '@/api/library'
import { reviewApi, type Review } from '@/api/review'
import { mangaRoomId } from '@/api/chat'
import { useAuthStore } from '@/store/authStore'

// ── Helpers ────────────────────────────────────────────────────────

const STATUS_STYLE: Record<string, string> = {
  ongoing:   'bg-emerald-500 text-white',
  completed: 'bg-blue-500 text-white',
  hiatus:    'bg-amber-500 text-white',
}

function RatingBadge({ rating }: { rating: number }) {
  return (
    <span className="inline-flex items-center gap-1 text-sm font-semibold text-amber-400">
      <Star className="h-4 w-4 fill-amber-400" />
      <span>
        {rating}
        <span className="text-xs font-normal text-[var(--color-muted-raw)]">/10</span>
      </span>
    </span>
  )
}

// ── Star picker ────────────────────────────────────────────────────

function StarPicker({ value, onChange }: { value: number; onChange: (n: number) => void }) {
  const [hover, setHover] = useState(0)
  return (
    <div className="flex items-center gap-0.5">
      {Array.from({ length: 10 }, (_, i) => {
        const n = i + 1
        const active = n <= (hover || value)
        return (
          <button
            key={n}
            type="button"
            onClick={() => onChange(n)}
            onMouseEnter={() => setHover(n)}
            onMouseLeave={() => setHover(0)}
          >
            <Star
              className={`h-5 w-5 transition-colors ${
                active ? 'fill-amber-400 text-amber-400' : 'text-[var(--color-border-raw)]'
              }`}
            />
          </button>
        )
      })}
      {value > 0 && (
        <span className="ml-2 text-sm font-semibold text-[var(--color-text)]">{value}/10</span>
      )}
    </div>
  )
}

// ── Library action ─────────────────────────────────────────────────

function LibraryAction({ mangaId, totalChapters }: { mangaId: string; totalChapters: number }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const qc = useQueryClient()
  const [selectedStatus, setSelectedStatus] = useState<LibraryStatus>('reading')

  // Same queryFn shape as LibraryPage so they share the cache cleanly
  const { data: libData } = useQuery({
    queryKey: ['library'],
    queryFn: () => libraryApi.get().then((r) => r.data.data),
    enabled: isAuthenticated,
  })

  const lists = libData?.reading_lists
  const allEntries = [
    ...(lists?.reading ?? []),
    ...(lists?.completed ?? []),
    ...(lists?.plan_to_read ?? []),
  ]
  const entry = allEntries.find((e) => e.manga_id === mangaId)

  // When adding as "completed", set chapter to total immediately after
  const addMutation = useMutation({
    mutationFn: async (status: LibraryStatus) => {
      await libraryApi.add(mangaId, status)
      if (status === 'completed' && totalChapters > 0) {
        await libraryApi.updateProgress(mangaId, totalChapters, 'completed')
      }
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['library'] }),
  })

  // When changing status on an existing entry, also update chapter if completing
  const updateStatusMutation = useMutation({
    mutationFn: (status: LibraryStatus) => {
      const chapter =
        status === 'completed' && totalChapters > 0
          ? totalChapters
          : (entry?.current_chapter ?? 0)
      return libraryApi.updateProgress(mangaId, chapter, status)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['library'] }),
  })

  const removeMutation = useMutation({
    mutationFn: () => libraryApi.remove(mangaId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['library'] }),
  })

  if (!isAuthenticated) {
    return (
      <Link
        to="/auth"
        className="inline-flex items-center gap-2 rounded-lg border border-dashed border-[var(--color-border-raw)] px-4 py-2 text-sm text-[var(--color-muted-raw)] transition-colors hover:border-[var(--brand-red)] hover:text-[var(--brand-red)]"
      >
        <BookMarked className="h-4 w-4" />
        Sign in to add to library
      </Link>
    )
  }

  if (entry) {
    const statusLabel = LIBRARY_STATUSES.find((s) => s.value === entry.status)?.label ?? entry.status
    const isBusy = updateStatusMutation.isPending || removeMutation.isPending
    return (
      <div className="flex flex-wrap items-center gap-3">
        <span className="inline-flex items-center gap-1.5 rounded-lg bg-[var(--brand-teal)]/15 px-3 py-1.5 text-sm font-medium text-[var(--brand-teal)]">
          <BookMarked className="h-3.5 w-3.5" />
          {statusLabel}
        </span>
        <div className="relative">
          <select
            value={entry.status}
            onChange={(e) => updateStatusMutation.mutate(e.target.value as LibraryStatus)}
            disabled={isBusy}
            className="appearance-none rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface)] pl-3 pr-8 py-1.5 text-sm text-[var(--color-text)] focus:outline-none focus:ring-2 focus:ring-[var(--brand-red)]/40 disabled:opacity-50"
          >
            {LIBRARY_STATUSES.map((s) => (
              <option key={s.value} value={s.value}>{s.label}</option>
            ))}
          </select>
          <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-muted-raw)]" />
        </div>
        <button
          onClick={() => removeMutation.mutate()}
          disabled={isBusy}
          className="inline-flex items-center gap-1.5 rounded-lg border border-red-200 px-3 py-1.5 text-sm text-red-500 transition-colors hover:bg-red-50 disabled:opacity-50 dark:border-red-900/40 dark:hover:bg-red-900/20"
        >
          <Trash2 className="h-3.5 w-3.5" />
          {removeMutation.isPending ? 'Removing…' : 'Remove'}
        </button>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-2">
      <div className="relative">
        <select
          value={selectedStatus}
          onChange={(e) => setSelectedStatus(e.target.value as LibraryStatus)}
          className="appearance-none rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface)] pl-3 pr-8 py-2 text-sm text-[var(--color-text)] focus:outline-none focus:ring-2 focus:ring-[var(--brand-red)]/40"
        >
          {LIBRARY_STATUSES.map((s) => (
            <option key={s.value} value={s.value}>{s.label}</option>
          ))}
        </select>
        <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-muted-raw)]" />
      </div>
      <button
        onClick={() => addMutation.mutate(selectedStatus)}
        disabled={addMutation.isPending}
        className="inline-flex items-center gap-2 rounded-lg bg-[var(--brand-red)] px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-[var(--brand-red-hover)] disabled:opacity-50"
      >
        <Plus className="h-4 w-4" />
        {addMutation.isPending ? 'Adding…' : 'Add to Library'}
      </button>
    </div>
  )
}

// ── localStorage helpers for helpful-vote dedup ───────────────────

const VOTED_KEY = 'mangahub-helpful-votes'

function hasVotedFor(reviewId: string): boolean {
  try {
    const ids: string[] = JSON.parse(localStorage.getItem(VOTED_KEY) ?? '[]')
    return ids.includes(reviewId)
  } catch {
    return false
  }
}

function persistVote(reviewId: string) {
  try {
    const ids: string[] = JSON.parse(localStorage.getItem(VOTED_KEY) ?? '[]')
    if (!ids.includes(reviewId)) {
      localStorage.setItem(VOTED_KEY, JSON.stringify([...ids, reviewId]))
    }
  } catch { /* ignore */ }
}

// ── Review card ────────────────────────────────────────────────────

function ReviewCard({ review, mangaId }: { review: Review; mangaId: string }) {
  const userId = useAuthStore((s) => s.userId)
  const username = useAuthStore((s) => s.username)
  const qc = useQueryClient()
  // Match by username (populated via JOIN) or user_id as fallback
  const isOwn =
    (!!username && username === review.username) ||
    (!!userId && userId === review.user_id)

  // Helpful — persisted across refreshes in localStorage
  const [helpfulCount, setHelpfulCount] = useState(review.helpful)
  const [hasVoted, setHasVoted] = useState(() => hasVotedFor(review.id))

  const helpfulMutation = useMutation({
    mutationFn: () => reviewApi.markHelpful(review.id),
    onSuccess: () => {
      setHelpfulCount((c) => c + 1)
      setHasVoted(true)
      persistVote(review.id)
    },
  })

  // Edit
  const [isEditing, setIsEditing] = useState(false)
  const [editRating, setEditRating] = useState(review.rating)
  const [editText, setEditText] = useState(review.text)
  const [editError, setEditError] = useState('')

  const updateMutation = useMutation({
    mutationFn: () => reviewApi.update(review.id, { rating: editRating, text: editText }),
    onSuccess: () => {
      setIsEditing(false)
      setEditError('')
      qc.invalidateQueries({ queryKey: ['reviews', mangaId] })
    },
    onError: (e: any) => {
      setEditError(e?.response?.data?.message ?? 'Failed to update review')
    },
  })

  // Delete
  const [confirmDelete, setConfirmDelete] = useState(false)

  const deleteMutation = useMutation({
    mutationFn: () => reviewApi.delete(review.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['reviews', mangaId] }),
  })

  return (
    <div className="rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-card)] p-4">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold text-[var(--color-text)]">
              {review.username}
            </span>
            {isOwn && (
              <span className="rounded-full bg-[var(--brand-teal)]/15 px-2 py-0.5 text-[10px] font-medium text-[var(--brand-teal)]">
                You
              </span>
            )}
          </div>
          <p className="text-xs text-[var(--color-muted-raw)]">
            {new Date(review.created_at).toLocaleDateString(undefined, {
              year: 'numeric',
              month: 'short',
              day: 'numeric',
            })}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <RatingBadge rating={isEditing ? editRating : review.rating} />
          {isOwn && !isEditing && !confirmDelete && (
            <div className="flex items-center gap-1.5">
              <button
                onClick={() => setIsEditing(true)}
                className="rounded-lg border border-[var(--color-border-raw)] px-2.5 py-1 text-xs text-[var(--color-muted-raw)] transition-colors hover:border-[var(--brand-red)] hover:text-[var(--brand-red)]"
              >
                Edit
              </button>
              <button
                onClick={() => setConfirmDelete(true)}
                className="rounded-lg border border-[var(--color-border-raw)] px-2.5 py-1 text-xs text-[var(--color-muted-raw)] transition-colors hover:border-red-400 hover:text-red-500"
              >
                Delete
              </button>
            </div>
          )}
          {isOwn && confirmDelete && (
            <div className="flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-2.5 py-1 dark:border-red-900/40 dark:bg-red-900/20">
              <span className="text-xs text-red-600 dark:text-red-400">Delete this review?</span>
              <button
                onClick={() => deleteMutation.mutate()}
                disabled={deleteMutation.isPending}
                className="text-xs font-semibold text-red-600 hover:text-red-700 disabled:opacity-50 dark:text-red-400"
              >
                {deleteMutation.isPending ? 'Deleting…' : 'Yes'}
              </button>
              <button
                onClick={() => setConfirmDelete(false)}
                className="text-xs text-[var(--color-muted-raw)] hover:text-[var(--color-text)]"
              >
                No
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Inline edit form */}
      {isEditing ? (
        <div className="mt-3 flex flex-col gap-3">
          <StarPicker value={editRating} onChange={setEditRating} />
          <textarea
            value={editText}
            onChange={(e) => setEditText(e.target.value)}
            rows={4}
            className="w-full resize-none rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface)] px-3 py-2.5 text-sm text-[var(--color-text)] placeholder-[var(--color-muted-raw)] focus:outline-none focus:ring-2 focus:ring-[var(--brand-red)]/40"
          />
          {editError && <p className="text-xs text-red-500">{editError}</p>}
          <div className="flex items-center gap-2">
            <button
              onClick={() => {
                if (editRating === 0) { setEditError('Please select a rating'); return }
                if (editText.trim().length < 10) { setEditError('Review must be at least 10 characters'); return }
                updateMutation.mutate()
              }}
              disabled={updateMutation.isPending}
              className="rounded-lg bg-[var(--brand-red)] px-4 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-[var(--brand-red-hover)] disabled:opacity-50"
            >
              {updateMutation.isPending ? 'Saving…' : 'Save'}
            </button>
            <button
              onClick={() => { setIsEditing(false); setEditRating(review.rating); setEditText(review.text); setEditError('') }}
              className="text-xs text-[var(--color-muted-raw)] transition-colors hover:text-[var(--color-text)]"
            >
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <p className="mt-3 text-sm leading-relaxed text-[var(--color-text2)]">{review.text}</p>
      )}

      {/* Helpful */}
      {!isEditing && (
        <div className="mt-3">
          <button
            onClick={() => helpfulMutation.mutate()}
            disabled={hasVoted || isOwn || helpfulMutation.isPending}
            className={`inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs transition-colors disabled:cursor-not-allowed disabled:opacity-40 ${
              hasVoted
                ? 'border-[var(--brand-teal)] text-[var(--brand-teal)]'
                : 'border-[var(--color-border-raw)] text-[var(--color-muted-raw)] hover:border-[var(--brand-teal)] hover:text-[var(--brand-teal)]'
            }`}
          >
            <ThumbsUp className={`h-3.5 w-3.5 ${hasVoted ? 'fill-current' : ''}`} />
            Helpful{helpfulCount > 0 ? ` (${helpfulCount})` : ''}
          </button>
        </div>
      )}
    </div>
  )
}

// ── Write review form ──────────────────────────────────────────────

function WriteReviewForm({
  mangaId,
  onSuccess,
}: {
  mangaId: string
  onSuccess: () => void
}) {
  const [rating, setRating] = useState(0)
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)
  const qc = useQueryClient()

  const mutation = useMutation({
    mutationFn: () => reviewApi.create(mangaId, { rating, text }),
    onSuccess: () => {
      setDone(true)
      qc.invalidateQueries({ queryKey: ['reviews', mangaId] })
      setTimeout(onSuccess, 1500)
    },
    onError: (e: any) => {
      setError(e?.response?.data?.message ?? 'Failed to submit review')
    },
  })

  if (done) {
    return (
      <div className="flex items-center gap-2 rounded-xl border border-[var(--brand-teal)]/30 bg-[var(--brand-teal)]/10 p-4 text-sm text-[var(--brand-teal)]">
        <CheckCircle2 className="h-4 w-4 flex-shrink-0" />
        Review submitted! Thank you.
      </div>
    )
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        if (rating === 0) { setError('Please select a rating'); return }
        if (text.trim().length < 10) { setError('Review must be at least 10 characters'); return }
        setError('')
        mutation.mutate()
      }}
      className="flex flex-col gap-4 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-card)] p-5"
    >
      <h3 className="font-semibold text-[var(--color-text)]">Write a Review</h3>

      <div>
        <p className="mb-2 text-sm text-[var(--color-muted-raw)]">Your rating</p>
        <StarPicker value={rating} onChange={setRating} />
      </div>

      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={4}
        placeholder="Share your thoughts about this manga…"
        className="w-full resize-none rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface)] px-3 py-2.5 text-sm text-[var(--color-text)] placeholder-[var(--color-muted-raw)] focus:outline-none focus:ring-2 focus:ring-[var(--brand-red)]/40"
      />

      {error && <p className="text-sm text-red-500">{error}</p>}

      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={mutation.isPending}
          className="rounded-lg bg-[var(--brand-red)] px-5 py-2 text-sm font-semibold text-white transition-colors hover:bg-[var(--brand-red-hover)] disabled:opacity-50"
        >
          {mutation.isPending ? 'Submitting…' : 'Submit Review'}
        </button>
        <button
          type="button"
          onClick={onSuccess}
          className="text-sm text-[var(--color-muted-raw)] hover:text-[var(--color-text)] transition-colors"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}

// ── Page ───────────────────────────────────────────────────────────

export default function MangaDetailPage() {
  const { id } = useParams<{ id: string }>()
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const userId = useAuthStore((s) => s.userId)
  const [showReviewForm, setShowReviewForm] = useState(false)

  const { data: manga, isLoading, isError } = useQuery({
    queryKey: ['manga', id],
    queryFn: () => mangaApi.get(id!).then((r) => r.data.data),
    enabled: !!id,
  })

  const { data: reviewsResp } = useQuery({
    queryKey: ['reviews', id],
    queryFn: () => reviewApi.list(id!),
    enabled: !!id,
  })
  const reviews: Review[] = reviewsResp?.data?.data?.reviews ?? []
  const userReview = reviews.find((r) => r.user_id === userId)
  const avgRating =
    reviews.length > 0
      ? Math.round((reviews.reduce((s, r) => s + r.rating, 0) / reviews.length) * 10) / 10
      : null

  // ── Loading ──────────────────────────────────────────────────────
  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-[var(--brand-red)] border-t-transparent" />
      </div>
    )
  }

  // ── Error / not found ────────────────────────────────────────────
  if (isError || !manga) {
    return (
      <div className="flex h-64 flex-col items-center justify-center gap-3 text-center">
        <BookOpen className="h-10 w-10 text-[var(--color-muted-raw)] opacity-40" />
        <p className="text-[var(--color-muted-raw)]">Manga not found</p>
        <Link to="/" className="text-sm text-[var(--brand-red)] hover:underline">
          Back to Browse
        </Link>
      </div>
    )
  }

  const statusStyle = STATUS_STYLE[manga.status?.toLowerCase() ?? ''] ?? 'bg-zinc-500 text-white'

  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
      className="mx-auto max-w-5xl space-y-10 p-4 pb-16"
    >
      {/* Back */}
      <Link
        to="/"
        className="inline-flex items-center gap-1.5 text-sm text-[var(--color-muted-raw)] transition-colors hover:text-[var(--brand-red)]"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to Browse
      </Link>

      {/* ── Hero ──────────────────────────────────────────────────── */}
      <div className="flex flex-col gap-6 sm:flex-row sm:gap-8">
        {/* Cover */}
        <div className="mx-auto w-44 flex-shrink-0 sm:mx-0 sm:w-52">
          <div className="aspect-[2/3] overflow-hidden rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface2)] shadow-lg">
            {manga.cover_url ? (
              <img
                src={manga.cover_url}
                alt={manga.title}
                className="h-full w-full object-cover"
              />
            ) : (
              <div className="flex h-full items-center justify-center text-[var(--color-muted-raw)]">
                <BookOpen className="h-12 w-12 opacity-30" />
              </div>
            )}
          </div>
        </div>

        {/* Meta */}
        <div className="flex flex-1 flex-col gap-4">
          <div>
            <h1 className="text-2xl font-bold leading-tight text-[var(--color-text)] sm:text-3xl">
              {manga.title}
            </h1>
            {manga.author && (
              <p className="mt-1 text-[var(--color-muted-raw)]">{manga.author}</p>
            )}
          </div>

          {/* Badges */}
          <div className="flex flex-wrap items-center gap-2">
            <span className={`rounded-lg px-2.5 py-1 text-xs font-bold uppercase tracking-wide ${statusStyle}`}>
              {manga.status}
            </span>
            <span className="rounded-lg bg-[var(--color-surface2)] px-2.5 py-1 text-xs text-[var(--color-text2)]">
              {manga.total_chapters} ch
            </span>
            {avgRating !== null && <RatingBadge rating={avgRating} />}
          </div>

          {/* Genres */}
          {manga.genres?.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {manga.genres.map((g) => (
                <span
                  key={g}
                  className="rounded-md bg-[var(--brand-red)]/10 px-2 py-0.5 text-xs font-medium text-[var(--brand-red)]"
                >
                  {g}
                </span>
              ))}
            </div>
          )}

          {/* Synopsis */}
          {manga.description && (
            <p className="text-sm leading-relaxed text-[var(--color-text2)]">
              {manga.description}
            </p>
          )}

          {/* Library action + discuss */}
          <div className="flex flex-wrap items-center gap-3 pt-1">
            <LibraryAction mangaId={id!} totalChapters={manga.total_chapters ?? 0} />
            <Link
              to={`/chat/${mangaRoomId(id!)}`}
              className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-border-raw)] px-4 py-2 text-sm font-medium text-[var(--color-text2)] transition-colors hover:border-[var(--brand-teal)] hover:text-[var(--brand-teal)]"
            >
              <MessageSquare className="h-4 w-4" />
              Discuss
            </Link>
          </div>
        </div>
      </div>

      {/* ── Reviews ───────────────────────────────────────────────── */}
      <div>
        <div className="mb-5 flex items-center justify-between">
          <h2 className="flex items-center gap-2 text-xl font-bold text-[var(--color-text)]">
            <MessageSquare className="h-5 w-5 text-[var(--brand-teal)]" />
            Reviews
            {reviews.length > 0 && (
              <span className="rounded-full bg-[var(--color-surface2)] px-2.5 py-0.5 text-sm font-normal text-[var(--color-muted-raw)]">
                {reviews.length}
              </span>
            )}
          </h2>

          {isAuthenticated && !showReviewForm && !userReview && (
            <button
              onClick={() => setShowReviewForm(true)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-[var(--color-border-raw)] px-3 py-1.5 text-sm text-[var(--color-text2)] transition-colors hover:border-[var(--brand-red)] hover:text-[var(--brand-red)]"
            >
              <Plus className="h-4 w-4" />
              Write a review
            </button>
          )}
        </div>

        {/* Write form */}
        {isAuthenticated && showReviewForm && (
          <div className="mb-6">
            <WriteReviewForm
              mangaId={id!}
              onSuccess={() => setShowReviewForm(false)}
            />
          </div>
        )}

        {/* Review list */}
        {reviews.length === 0 ? (
          <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed border-[var(--color-border-raw)] py-14 text-center">
            <MessageSquare className="h-8 w-8 text-[var(--color-muted-raw)] opacity-30" />
            <p className="text-sm text-[var(--color-muted-raw)]">
              No reviews yet. Be the first!
            </p>
            {!isAuthenticated && (
              <Link to="/auth" className="text-sm text-[var(--brand-red)] hover:underline">
                Sign in to write a review
              </Link>
            )}
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            {reviews.map((r) => (
              <ReviewCard key={r.id} review={r} mangaId={id!} />
            ))}
          </div>
        )}
      </div>
    </motion.div>
  )
}
