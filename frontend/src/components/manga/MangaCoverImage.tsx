import type { ComponentProps } from 'react'

const MANGADEX_COVER_PREFIX = 'https://uploads.mangadex.org/covers/'

type Props = Omit<ComponentProps<'img'>, 'referrerPolicy'>

// MangaDex replaces covers with an anti-hotlink placeholder when a third-party
// web origin is sent as the Referer. Keep the privacy override scoped to its
// cover CDN so unrelated images retain the application's normal policy.
export function MangaCoverImage({ src, ...props }: Props) {
  const referrerPolicy = typeof src === 'string' && src.startsWith(MANGADEX_COVER_PREFIX)
    ? 'no-referrer'
    : undefined

  return <img {...props} src={src} referrerPolicy={referrerPolicy} />
}
