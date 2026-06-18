import { apiClient } from './client'

// Mirrors the Go ChatMessage struct (internal/websocket/hub.go)
export interface ChatMessage {
  type: 'message' | 'system' | 'pm' | 'join' | 'leave' | 'users' | 'history' | 'error'
  user_id?: string
  username?: string
  message: string
  recipient?: string
  room?: string
  users?: string[]
  timestamp: number
}

// Predefined rooms shown in the sidebar
export const CHAT_ROOMS = [
  { value: 'general',   label: 'General',    emoji: '💬' },
  { value: 'shounen',   label: 'Shounen',    emoji: '⚔️' },
  { value: 'romance',   label: 'Romance',    emoji: '💕' },
  { value: 'isekai',    label: 'Isekai',     emoji: '🌍' },
  { value: 'recommend', label: 'Recommend',  emoji: '⭐' },
] as const

export type ChatRoom = (typeof CHAT_ROOMS)[number]['value']

// Manga discussion rooms are namespaced as `manga-<mangaId>` so they never
// collide with the predefined topic rooms.
export const MANGA_ROOM_PREFIX = 'manga-'

export const mangaRoomId = (mangaId: string) => `${MANGA_ROOM_PREFIX}${mangaId}`

export const isMangaRoom = (room: string) => room.startsWith(MANGA_ROOM_PREFIX)

export const mangaIdFromRoom = (room: string) =>
  isMangaRoom(room) ? room.slice(MANGA_ROOM_PREFIX.length) : ''

// Build the WebSocket URL from the HTTP API base, swapping the scheme.
export function buildChatSocketUrl(token: string, room: string): string {
  const apiBase = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
  const wsBase = apiBase.replace(/^http/, 'ws')
  return `${wsBase}/ws/chat?token=${encodeURIComponent(token)}&room=${encodeURIComponent(room)}`
}

export const chatApi = {
  history: (room: string, limit = 50) =>
    apiClient.get<{ data: ChatMessage[] }>('/chat/history', { params: { room, limit } }),
}
