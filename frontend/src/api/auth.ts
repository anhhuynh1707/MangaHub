import { apiClient } from './client'

export interface LoginPayload    { username: string; password: string }
export interface RegisterPayload { username: string; password: string; email?: string }
export interface AuthResponse    { token: string; user_id: string; username: string }
export interface ChangePasswordPayload { old_password: string; new_password: string }

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
