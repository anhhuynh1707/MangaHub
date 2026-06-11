import { apiClient } from './client'

export interface UserProgress {
  user_id: string
  manga_id: string
  current_chapter: number
  status: 'reading' | 'completed' | 'plan_to_read'
  updated_at: string
}

export interface ReadingLists {
  reading: UserProgress[]
  completed: UserProgress[]
  plan_to_read: UserProgress[]
}

export const LIBRARY_STATUSES = [
  { value: 'reading',      label: 'Reading' },
  { value: 'completed',    label: 'Completed' },
  { value: 'plan_to_read', label: 'Plan to Read' },
] as const

export type LibraryStatus = (typeof LIBRARY_STATUSES)[number]['value']

export const libraryApi = {
  get: () =>
    apiClient.get<{ data: { user_id: string; username: string; reading_lists: ReadingLists } }>('/users/library'),

  add: (manga_id: string, status: LibraryStatus) =>
    apiClient.post('/users/library', { manga_id, status }),

  remove: (manga_id: string) =>
    apiClient.delete(`/users/library/${manga_id}`),

  updateProgress: (manga_id: string, current_chapter: number, status: LibraryStatus) =>
    apiClient.put('/users/progress', { manga_id, current_chapter, status }),
}
