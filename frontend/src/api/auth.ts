import { apiClient } from './client'
import type { LoginRequest, RegisterRequest, ChangePasswordRequest } from './generated'

// Request payload types come from the generated OpenAPI schema (single source of
// truth — regenerate with `npm run gen:api` when the backend spec changes).
export type LoginPayload = LoginRequest
export type RegisterPayload = RegisterRequest
export type ChangePasswordPayload = ChangePasswordRequest

export interface AuthUser     { id: string; username: string }
export interface AuthResponse { token: string; user: AuthUser }

export const authApi = {
  login: (data: LoginPayload) =>
    apiClient.post<{ data: AuthResponse }>('/auth/login', data),

  register: (data: RegisterPayload) =>
    apiClient.post<{ data: AuthResponse }>('/auth/register', data),

  logout: () =>
    apiClient.post('/auth/logout'),

  changePassword: (data: ChangePasswordPayload) =>
    apiClient.put('/auth/change-password', data),
}
