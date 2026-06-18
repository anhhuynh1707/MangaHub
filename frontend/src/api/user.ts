import { apiClient } from './client'

export interface UserProfile {
  id: string
  username: string
  created_at: string
}

export const userApi = {
  profile: () => apiClient.get<{ data: UserProfile }>('/users/profile'),
}
