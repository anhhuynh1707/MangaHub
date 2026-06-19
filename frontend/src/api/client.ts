import axios from 'axios'
import { useAuthStore } from '@/store/authStore'

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? 'http://localhost:8080',
  timeout: 15_000,
  headers: { 'Content-Type': 'application/json' },
})

// Attach JWT on every request
apiClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// On 401, clear auth and redirect to /auth — BUT only for an expired session.
// A 401 from a credential endpoint (login / register / change-password) means
// "wrong credentials in this request", which the form should display inline;
// redirecting there would reload the page and erase the error message.
const CREDENTIAL_ENDPOINTS = ['/auth/login', '/auth/register', '/auth/change-password']

apiClient.interceptors.response.use(
  (res) => res,
  (error) => {
    const url: string = error.config?.url ?? ''
    const isCredentialEndpoint = CREDENTIAL_ENDPOINTS.some((p) => url.includes(p))

    if (error.response?.status === 401 && !isCredentialEndpoint) {
      useAuthStore.getState().clearAuth()
      window.location.href = '/auth'
    }
    return Promise.reject(error)
  }
)
