import { MangaCard, MangaCardSkeleton } from './MangaCard'
import type { Manga } from '@/api/manga'

interface Props {
  manga: Manga[]
  loading?: boolean
  skeletonCount?: number
}

export function MangaGrid({ manga, loading = false, skeletonCount = 18 }: Props) {
  if (loading) {
    return (
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        {Array.from({ length: skeletonCount }).map((_, i) => (
          <MangaCardSkeleton key={i} />
        ))}
      </div>
    )
  }

  if (manga.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 py-20 text-center">
        <span className="text-4xl">📭</span>
        <p className="font-semibold text-[var(--color-text)]">No manga found</p>
        <p className="text-sm text-[var(--color-muted-raw)]">Try adjusting your search or filters</p>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
      {manga.map((m) => (
        <MangaCard key={m.id} manga={m} />
      ))}
    </div>
  )
}
