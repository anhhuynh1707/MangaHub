import axios from 'axios'
import { useAuthStore } from '@/store/authStore'
import { notify } from '@/lib/notify'

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
    const status = error.response?.status as number | undefined

    if (status === 401 && !isCredentialEndpoint) {
      useAuthStore.getState().clearAuth()
      window.location.href = '/auth'
      return Promise.reject(error)
    }

    // Global toast for UNEXPECTED errors only — network failures and 5xx.
    // Ordinary 4xx (validation/conflict) and credential errors are shown inline
    // by the relevant form/component, so we don't double-notify here.
    if (!isCredentialEndpoint) {
      if (!error.response) {
        notify.error('Network error', 'Could not reach the server. Check your connection.')
      } else if (status && status >= 500) {
        notify.error('Server error', 'Something went wrong on our end. Please try again.')
      }
    }
    return Promise.reject(error)
  }
)
