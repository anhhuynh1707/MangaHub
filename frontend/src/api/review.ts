import { apiClient } from './client'

export interface Review {
  id: string
  user_id: string
  username: string
  manga_id: string
  rating: number
  text: string
  helpful: number
  created_at: string
  updated_at: string
}

export interface ReviewsResponse {
  reviews: Review[]
  total: number
  limit: number
  offset: number
  pages: number
}

export const reviewApi = {
  list: (manga_id: string) =>
    apiClient.get<{ data: ReviewsResponse }>(`/manga/${manga_id}/reviews`),

  create: (manga_id: string, payload: { rating: number; text: string }) =>
    apiClient.post(`/manga/${manga_id}/reviews`, { manga_id, ...payload }),

  update: (review_id: string, payload: { rating: number; text: string }) =>
    apiClient.put(`/reviews/${review_id}`, payload),

  delete: (review_id: string) =>
    apiClient.delete(`/reviews/${review_id}`),

  markHelpful: (review_id: string) =>
    apiClient.post(`/reviews/${review_id}/helpful`, { review_id }),

  myReviews: (page = 1, limit = 50) =>
    apiClient.get<{ data: { reviews: Review[]; total: number; page: number; limit: number; pages: number } }>(
      '/users/reviews',
      { params: { page, limit } }
    ),
}
