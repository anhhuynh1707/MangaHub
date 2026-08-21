const configuredBase = import.meta.env.VITE_API_URL?.trim()
const developmentDefault = import.meta.env.DEV ? 'http://localhost:8080' : window.location.origin

/**
 * API base used by REST, WebSocket, and SSE clients.
 *
 * Local Vite development defaults to localhost:8080. Production images set the
 * value to /api, which the public edge proxy strips before forwarding requests
 * to the internal Go API.
 */
export const API_BASE_URL = (configuredBase || developmentDefault).replace(/\/+$/, '')

export function apiUrl(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${API_BASE_URL}${normalizedPath}`
}

export function webSocketUrl(path: string, params: Record<string, string>): string {
  const url = new URL(apiUrl(path), window.location.origin)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  Object.entries(params).forEach(([key, value]) => url.searchParams.set(key, value))
  return url.toString()
}
