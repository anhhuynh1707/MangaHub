import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AuthState {
  token: string | null
  userId: string | null
  username: string | null
  isAuthenticated: boolean
  setAuth: (token: string, userId: string, username: string) => void
  clearAuth: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      userId: null,
      username: null,
      isAuthenticated: false,

      setAuth: (token, userId, username) =>
        set({ token, userId, username, isAuthenticated: true }),

      clearAuth: () =>
        set({ token: null, userId: null, username: null, isAuthenticated: false }),
    }),
    {
      name: 'mangahub-auth',
      onRehydrateStorage: () => (state) => {
        // Clear sessions saved before the login fix (missing userId or username)
        if (state?.isAuthenticated && (!state.userId || !state.username)) {
          state.clearAuth()
        }
      },
    }
  )
)
