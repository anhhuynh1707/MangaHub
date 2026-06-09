import { apiClient } from './client'

export interface Manga {
  id: string
  title: string
  author: string
  description: string
  genres: string[]
  status: string
  total_chapters: number
  cover_image?: string
  average_rating?: number
  year?: number
}

export interface MangaListResponse {
  data: Manga[]
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

export const mangaApi = {
  list: (params?: { search?: string; genre?: string; page?: number; limit?: number }) =>
    apiClient.get<{ data: MangaListResponse }>('/manga', { params }),

  get: (id: string) =>
    apiClient.get<{ data: Manga }>(`/manga/${id}`),

  search: (filters: SearchFilters) =>
    apiClient.post<{ data: Manga[]; total: number }>('/manga/search', filters),

  recommendations: (limit = 10) =>
    apiClient.get<{ data: { recommendations: Manga[] } }>('/users/recommendations', {
      params: { limit },
    }),
}
