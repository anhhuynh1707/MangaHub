import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, X, SlidersHorizontal, ChevronLeft, ChevronRight } from 'lucide-react'
import { mangaApi, GENRES, SORT_OPTIONS } from '@/api/manga'
import { MangaGrid } from '@/components/manga/MangaGrid'
import { useDebounce } from '@/hooks/useDebounce'

const PAGE_SIZE = 18

export default function BrowsePage() {
  const [search, setSearch]           = useState('')
  const [genre, setGenre]             = useState('')
  const [status, setStatus]           = useState('')
  const [sortBy, setSortBy]           = useState('title')
  const [page, setPage]               = useState(1)
  const [showFilters, setShowFilters] = useState(false)

  const debouncedSearch = useDebounce(search, 300)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['manga', debouncedSearch, genre, status, sortBy, page],
    queryFn: () =>
      mangaApi.list({
        search:  debouncedSearch || undefined,
        genre:   genre          || undefined,
        status:  status         || undefined,
        sort_by: sortBy,
        page,
        limit:   PAGE_SIZE,
      }),
    placeholderData: (prev) => prev,
  })

  const manga      = data?.data?.data?.manga ?? []
  const total      = data?.data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  function resetFilters() {
    setGenre('')
    setStatus('')
    setSortBy('title')
    setPage(1)
  }

  const hasActiveFilters = genre !== '' || status !== '' || sortBy !== 'title'

  return (
    <div className="flex flex-col gap-5">

      {/* ── Page header ── */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-[var(--color-text)]">Browse Manga</h1>
          {!isLoading && (
            <p className="mt-0.5 text-sm text-[var(--color-muted-raw)]">
              {total.toLocaleString()} titles
            </p>
          )}
        </div>

        {/* Filter toggle button (mobile only) */}
        <button
          onClick={() => setShowFilters((v) => !v)}
          className={`flex items-center gap-1.5 rounded-lg border px-3 py-2 text-sm font-medium transition md:hidden ${
            showFilters
              ? 'border-[var(--brand-red)] bg-[var(--brand-red)]/10 text-[var(--brand-red)]'
              : 'border-[var(--color-border-raw)] text-[var(--color-text2)] hover:bg-[var(--color-surface2)]'
          }`}
        >
          <SlidersHorizontal className="h-4 w-4" />
          Filters
          {hasActiveFilters && (
            <span className="flex h-4 w-4 items-center justify-center rounded-full bg-[var(--brand-red)] text-[9px] font-bold text-white">
              !
            </span>
          )}
        </button>
      </div>

      {/* ── Search bar ── */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-muted-raw)]" />
        <input
          type="text"
          value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(1) }}
          placeholder="Search by title, author, or description…"
          className="w-full rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] py-2.5 pl-9 pr-10 text-sm text-[var(--color-text)] placeholder:text-[var(--color-muted-raw)] outline-none transition focus:border-[var(--brand-red)] focus:ring-2 focus:ring-[var(--brand-red)]/20"
        />
        {search && (
          <button
            onClick={() => { setSearch(''); setPage(1) }}
            className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--color-muted-raw)] hover:text-[var(--color-text)]"
          >
            <X className="h-4 w-4" />
          </button>
        )}
      </div>

      {/* ── Filters (md+ always shown, mobile toggles) ── */}
      <div className={`flex flex-col gap-3 ${showFilters ? 'flex' : 'hidden md:flex'}`}>

        {/* Genre chips */}
        <div className="flex flex-wrap gap-1.5">
          <button
            onClick={() => { setGenre(''); setPage(1) }}
            className={chipCls(genre === '')}
          >
            All Genres
          </button>
          {GENRES.map((g) => (
            <button
              key={g}
              onClick={() => { setGenre(genre === g ? '' : g); setPage(1) }}
              className={chipCls(genre === g)}
            >
              {g}
            </button>
          ))}
        </div>

        {/* Status + Sort row */}
        <div className="flex flex-wrap items-center gap-3">
          <select
            value={status}
            onChange={(e) => { setStatus(e.target.value); setPage(1) }}
            className={selectCls}
          >
            <option value="">All Status</option>
            <option value="ongoing">Ongoing</option>
            <option value="completed">Completed</option>
            <option value="hiatus">Hiatus</option>
          </select>

          <select
            value={sortBy}
            onChange={(e) => { setSortBy(e.target.value); setPage(1) }}
            className={selectCls}
          >
            {SORT_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>

          {hasActiveFilters && (
            <button
              onClick={resetFilters}
              className="flex items-center gap-1 text-sm text-[var(--color-muted-raw)] hover:text-[var(--brand-red)]"
            >
              <X className="h-3.5 w-3.5" /> Clear filters
            </button>
          )}
        </div>
      </div>

      {/* ── Error state ── */}
      {isError && (
        <div className="rounded-xl border border-[var(--color-error)]/30 bg-[var(--color-error)]/10 px-4 py-3 text-sm text-[var(--color-error)]">
          Failed to load manga. Make sure the backend is running at{' '}
          <code className="font-mono">localhost:8080</code>.
        </div>
      )}

      {/* ── Grid ── */}
      <MangaGrid manga={manga} loading={isLoading} />

      {/* ── Pagination ── */}
      {!isLoading && total > PAGE_SIZE && (
        <div className="flex items-center justify-between border-t border-[var(--color-border-raw)] pt-4">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className={paginationBtnCls}
          >
            <ChevronLeft className="h-4 w-4" /> Previous
          </button>

          <span className="text-sm text-[var(--color-muted-raw)]">
            Page <strong className="text-[var(--color-text)]">{page}</strong> of{' '}
            <strong className="text-[var(--color-text)]">{totalPages}</strong>
          </span>

          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            className={paginationBtnCls}
          >
            Next <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      )}
    </div>
  )
}

/* ── Style helpers ───────────────────────────────────────────────── */
function chipCls(active: boolean) {
  return `rounded-full border px-3 py-1 text-xs font-medium transition ${
    active
      ? 'border-[var(--brand-red)] bg-[var(--brand-red)]/10 text-[var(--brand-red)]'
      : 'border-[var(--color-border-raw)] text-[var(--color-text2)] hover:border-[var(--brand-red)]/40 hover:text-[var(--brand-red)]'
  }`
}

const selectCls =
  'rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-text)] outline-none transition focus:border-[var(--brand-red)] cursor-pointer'

const paginationBtnCls =
  'flex items-center gap-1.5 rounded-lg border border-[var(--color-border-raw)] px-4 py-2 text-sm font-medium text-[var(--color-text2)] transition hover:bg-[var(--color-surface2)] disabled:opacity-40 disabled:cursor-not-allowed'
