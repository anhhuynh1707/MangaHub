import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '@/store/authStore'
import { useNotificationStore, type ServerNotification } from '@/store/notificationStore'
import { showEventToast } from '@/lib/eventToast'

// The SSE envelope mirrors internal/sse/hub.go Event.
interface ServerEvent {
  type: 'notification' | 'progress'
  data: unknown
  timestamp: number
}

// Payload for "progress" events (cmd/api-server/sync.go).
interface ProgressData {
  user_id: string
  username: string
  manga_id: string
  chapter: number
}

function eventStreamUrl(token: string): string {
  const apiBase = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
  return `${apiBase}/events/stream?token=${encodeURIComponent(token)}`
}

/**
 * useServerEvents bridges the backend's SSE stream into the SPA: UDP
 * notifications become toasts + bell items, and TCP progress updates surface as
 * subtle toasts while invalidating the activity feed so it refreshes live.
 *
 * EventSource reconnects on its own (no manual backoff needed, unlike useChat);
 * we just open it when authenticated and close it on logout/unmount. The current
 * user's own progress updates are skipped so you don't get toasted by yourself.
 */
export function useServerEvents() {
  const token = useAuthStore((s) => s.token)
  const userId = useAuthStore((s) => s.userId)
  const addNotification = useNotificationStore((s) => s.add)
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!token) return

    const source = new EventSource(eventStreamUrl(token))

    source.onmessage = (e) => {
      let event: ServerEvent
      try {
        event = JSON.parse(e.data)
      } catch {
        return
      }

      if (event.type === 'notification') {
        const data = event.data as ServerNotification
        addNotification(data)
        showEventToast({
          type: data.type,
          title: data.title || 'New notification',
          message: data.message,
        })
      } else if (event.type === 'progress') {
        const data = event.data as ProgressData
        // Don't notify users about their own reading activity.
        if (data.user_id === userId) {
          queryClient.invalidateQueries({ queryKey: ['feed'] })
          return
        }
        showEventToast({
          type: 'progress',
          title: 'Reading activity',
          message: `${data.username} read a manga → ch. ${data.chapter}`,
        })
        queryClient.invalidateQueries({ queryKey: ['feed'] })
      }
    }

    // EventSource auto-reconnects on transient errors; nothing to do here beyond
    // letting it retry. (It only fires onerror, never throws.)

    return () => source.close()
  }, [token, userId, addNotification, queryClient])
}
