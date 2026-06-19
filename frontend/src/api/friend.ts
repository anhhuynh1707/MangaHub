import { apiClient } from './client'

export interface Friend {
  user_id: string
  username: string
  status: string // "accepted"
}

export interface FriendsResponse {
  friends: Friend[] | null
  total: number
  page: number
  limit: number
  pages: number
}

export interface PendingResponse {
  pending_requests: string[] | null // array of requester user IDs
  count: number
  page: number
  limit: number
  pages: number
}

// User IDs are slugged as `user-<lowercased-username>` (see generateUserID in
// the backend). These helpers convert between the two for display / lookup.
export const userIdFromUsername = (username: string) =>
  `user-${username.trim().toLowerCase().replace(/\s+/g, '-')}`

export const usernameFromUserId = (userId: string) =>
  userId.startsWith('user-') ? userId.slice('user-'.length) : userId

export const friendApi = {
  list: () => apiClient.get<{ data: FriendsResponse }>('/users/friends'),

  pending: () => apiClient.get<{ data: PendingResponse }>('/users/friends/pending'),

  add: (friend_id: string) => apiClient.post('/friends/add', { friend_id }),

  accept: (friend_id: string) => apiClient.post(`/friends/${friend_id}/accept`),

  decline: (friend_id: string) => apiClient.post(`/friends/${friend_id}/decline`),

  remove: (friend_id: string) => apiClient.delete(`/friends/${friend_id}`),
}
