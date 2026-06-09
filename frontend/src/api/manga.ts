import { apiClient } from './client'

export interface Manga {
  id: string
  title: string
  author: string
  description: string
  genres: string[]
  status: string
  total_chapters: number
  cover_url?: string
}

export interface MangaSearchResponse {
  manga: Manga[]
  total: number
  page: number
  limit: number
}

export interface SearchFilters {
  search?: string
  genres?: string[]
  status?: string
  min_rating?: number
  sort_by?: string
  page?: number
  limit?: number
}

export const GENRES = [
  'Action', 'Adventure', 'Comedy', 'Drama', 'Fantasy',
  'Horror', 'Mystery', 'Romance', 'Sci-Fi', 'Shounen',
  'Shoujo', 'Slice of Life', 'Sports', 'Supernatural', 'Thriller',
]

export const SORT_OPTIONS = [
  { value: 'title',      label: 'Title A–Z' },
  { value: 'popularity', label: 'Popularity' },
  { value: 'rating',     label: 'Top Rated' },
  { value: 'recent',     label: 'Recent' },
]

export const mangaApi = {
  list: (params?: {
    search?: string
    genre?: string
    status?: string
    sort_by?: string
    min_rating?: number
    page?: number
    limit?: number
  }) =>
    apiClient.get<{ data: MangaSearchResponse }>('/manga', { params }),

  get: (id: string) =>
    apiClient.get<{ data: Manga }>(`/manga/${id}`),

  advancedSearch: (filters: SearchFilters) =>
    apiClient.post<{ data: { manga: Manga[]; total: number } }>('/manga/search', filters),

  recommendations: (limit = 10) =>
    apiClient.get<{ data: { recommendations: Manga[] } }>('/users/recommendations', {
      params: { limit },
    }),
}
