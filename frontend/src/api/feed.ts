import { apiClient } from './client'

// Mirrors the Go Activity struct (pkg/models/models.go)
export interface Activity {
  id: string
  user_id: string
  username: string
  type: ActivityType
  manga_id?: string
  manga_title?: string
  review_id?: string
  friend_id?: string
  message: string
  created_at: string
}

export type ActivityType =
  | 'started_manga'
  | 'completed_manga'
  | 'wrote_review'
  | 'added_friend'
  | string // tolerate unknown future types

export interface ActivityFeedResponse {
  activities: Activity[]
  total: number
  page: number
  limit: number
  pages: number
}

// Filter chips shown above the feed
export const ACTIVITY_FILTERS = [
  { value: '',                label: 'All' },
  { value: 'started_manga',   label: 'Started' },
  { value: 'completed_manga', label: 'Completed' },
  { value: 'wrote_review',    label: 'Reviews' },
  { value: 'added_friend',    label: 'Friends' },
] as const

export const feedApi = {
  list: (params?: { page?: number; limit?: number; type?: string }) =>
    apiClient.get<{ data: ActivityFeedResponse }>('/feed/activities', { params }),

  stats: () =>
    apiClient.get<{ data: Record<string, number> }>('/feed/stats'),

  post: (message: string) =>
    apiClient.post('/feed/activities', { message }),

  clear: () =>
    apiClient.delete('/feed/clear'),

  userActivities: (userId: string, params?: { page?: number; limit?: number }) =>
    apiClient.get<{ data: ActivityFeedResponse }>(`/users/${userId}/activities`, { params }),
}
