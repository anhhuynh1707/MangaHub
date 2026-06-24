import { create } from 'zustand'

// Mirrors the udp.Notification payload the backend bridges over SSE
// (internal/udp/server.go). All fields optional except message — the broadcast
// endpoint only requires type/manga_id/message.
export interface ServerNotification {
  type?: string
  manga_id?: string
  title?: string
  message: string
  timestamp?: number
}

// One item as shown in the bell dropdown. `id` is client-generated so React has
// a stable key and we can keep a bounded, ordered list.
export interface NotificationItem extends ServerNotification {
  id: string
  receivedAt: number
}

const MAX_ITEMS = 50

interface NotificationState {
  items: NotificationItem[]
  unread: number
  add: (n: ServerNotification) => void
  markAllRead: () => void
  clear: () => void
}

export const useNotificationStore = create<NotificationState>((set) => ({
  items: [],
  unread: 0,

  add: (n) =>
    set((state) => ({
      items: [
        { ...n, id: crypto.randomUUID(), receivedAt: Date.now() },
        ...state.items,
      ].slice(0, MAX_ITEMS),
      unread: state.unread + 1,
    })),

  markAllRead: () => set({ unread: 0 }),

  clear: () => set({ items: [], unread: 0 }),
}))
