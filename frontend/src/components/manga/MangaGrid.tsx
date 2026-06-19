import { motion, type Variants } from 'framer-motion'
import { MangaCard, MangaCardSkeleton } from './MangaCard'
import type { Manga } from '@/api/manga'

interface Props {
  manga: Manga[]
  loading?: boolean
  skeletonCount?: number
}

const GRID = 'grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6'

// Stagger the cards in as the grid mounts / changes page.
const container: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.04 } },
}
const item: Variants = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.25, ease: 'easeOut' } },
}

export function MangaGrid({ manga, loading = false, skeletonCount = 18 }: Props) {
  if (loading) {
    return (
      <div className={GRID}>
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
    <motion.div
      // Re-key on the rendered set so the stagger replays on page / filter change
      key={manga.map((m) => m.id).join(',')}
      className={GRID}
      variants={container}
      initial="hidden"
      animate="show"
    >
      {manga.map((m) => (
        <motion.div key={m.id} variants={item}>
          <MangaCard manga={m} />
        </motion.div>
      ))}
    </motion.div>
  )
}
