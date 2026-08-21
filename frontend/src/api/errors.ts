import axios from 'axios'

interface ApiErrorPayload {
  error?: string
  message?: string
}

export function apiErrorMessage(error: unknown, fallback: string): string {
  if (!axios.isAxiosError<ApiErrorPayload>(error)) return fallback
  return error.response?.data?.error ?? error.response?.data?.message ?? fallback
}
