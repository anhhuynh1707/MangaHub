import { Link } from 'react-router-dom'
import { BookOpen } from 'lucide-react'
import type { Manga } from '@/api/manga'

interface Props {
  manga: Manga
}

// Solid opaque backgrounds so the badge is always readable over any cover image
const STATUS_STYLE: Record<string, string> = {
  ongoing:   'bg-emerald-500 text-white',
  completed: 'bg-blue-500    text-white',
  hiatus:    'bg-amber-500   text-white',
}

export function MangaCard({ manga }: Props) {
  const statusKey = manga.status?.toLowerCase() ?? ''
  const statusStyle = STATUS_STYLE[statusKey] ?? 'bg-zinc-500 text-white'

  return (
    <Link
      to={`/manga/${manga.id}`}
      className="group flex flex-col overflow-hidden rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-card)] no-underline shadow-sm transition hover:border-[var(--brand-red)]/40 hover:shadow-md"
    >
      {/* Cover */}
      <div className="relative aspect-[2/3] overflow-hidden bg-[var(--color-surface2)]">
        {manga.cover_url ? (
          <img
            src={manga.cover_url}
            alt={manga.title}
            loading="lazy"
            className="h-full w-full object-cover transition duration-300 group-hover:scale-105"
          />
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-[var(--color-muted-raw)]">
            <BookOpen className="h-10 w-10 opacity-30" />
            <span className="text-xs opacity-40">No cover</span>
          </div>
        )}

        {/* Status badge (overlay) */}
        <span className={`absolute left-2 top-2 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide ${statusStyle}`}>
          {manga.status || 'Unknown'}
        </span>
      </div>

      {/* Info */}
      <div className="flex flex-1 flex-col gap-1.5 p-3">
        <h3 className="line-clamp-2 text-sm font-semibold leading-snug text-[var(--color-text)] group-hover:text-[var(--brand-red)]">
          {manga.title}
        </h3>
        <p className="truncate text-xs text-[var(--color-muted-raw)]">{manga.author}</p>

        {/* Genre chips — show first 2 */}
        {manga.genres?.length > 0 && (
          <div className="mt-auto flex flex-wrap gap-1 pt-1">
            {manga.genres.slice(0, 2).map((g) => (
              <span
                key={g}
                className="rounded-md bg-[var(--brand-red)]/10 px-1.5 py-0.5 text-[10px] font-medium text-[var(--brand-red)]"
              >
                {g}
              </span>
            ))}
            {manga.genres.length > 2 && (
              <span className="rounded-md bg-[var(--color-surface2)] px-1.5 py-0.5 text-[10px] text-[var(--color-muted-raw)]">
                +{manga.genres.length - 2}
              </span>
            )}
          </div>
        )}

        {/* Chapter count */}
        <p className="text-[11px] text-[var(--color-muted-raw)]">
          {manga.total_chapters} ch
        </p>
      </div>
    </Link>
  )
}

/* ── Skeleton ────────────────────────────────────────────────────── */
export function MangaCardSkeleton() {
  return (
    <div className="flex flex-col overflow-hidden rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-card)]">
      <div className="aspect-[2/3] animate-pulse bg-[var(--color-surface2)]" />
      <div className="flex flex-col gap-2 p-3">
        <div className="h-3.5 w-3/4 animate-pulse rounded bg-[var(--color-surface2)]" />
        <div className="h-3 w-1/2 animate-pulse rounded bg-[var(--color-surface2)]" />
        <div className="mt-1 flex gap-1">
          <div className="h-4 w-12 animate-pulse rounded bg-[var(--color-surface2)]" />
          <div className="h-4 w-10 animate-pulse rounded bg-[var(--color-surface2)]" />
        </div>
      </div>
    </div>
  )
}
