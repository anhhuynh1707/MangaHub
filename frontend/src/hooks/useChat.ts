import { useEffect, useRef, useState, useCallback } from 'react'
import { useAuthStore } from '@/store/authStore'
import { buildChatSocketUrl, type ChatMessage } from '@/api/chat'
import { notify } from '@/lib/notify'

export type ChatStatus = 'connecting' | 'open' | 'closed' | 'error'

interface UseChatResult {
  messages: ChatMessage[]
  users: string[]
  status: ChatStatus
  send: (text: string) => void
  clear: () => void
}

/**
 * useChat opens a WebSocket to the chat hub for a given room and exposes
 * the live message stream, online-user list, and a send() helper.
 * Reconnects automatically on unexpected disconnects.
 */
export function useChat(room: string): UseChatResult {
  const token = useAuthStore((s) => s.token)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [users, setUsers] = useState<string[]>([])
  const [status, setStatus] = useState<ChatStatus>('connecting')

  // Only used by send() — always points at the latest live socket.
  const socketRef = useRef<WebSocket | null>(null)

  const send = useCallback((text: string) => {
    const sock = socketRef.current
    if (!sock || sock.readyState !== WebSocket.OPEN) return
    sock.send(JSON.stringify({ type: 'message', message: text }))
  }, [])

  const clear = useCallback(() => setMessages([]), [])

  useEffect(() => {
    if (!token) return

    // `stopped` is LOCAL to this effect invocation. A socket closed by THIS
    // cleanup will see its own stopped=true and never reconnect — which avoids
    // the phantom-reconnect bug where a stale socket from the previous room
    // keeps re-registering after a room switch / StrictMode remount.
    let stopped = false
    let sock: WebSocket | null = null
    let timer: ReturnType<typeof setTimeout> | null = null
    let attempts = 0

    // Fresh stream when switching rooms
    setMessages([])
    setUsers([])

    function connect() {
      if (stopped) return
      setStatus('connecting')
      sock = new WebSocket(buildChatSocketUrl(token!, room))
      socketRef.current = sock

      sock.onopen = () => {
        if (attempts > 0 && !stopped) {
          notify.success('Reconnected to chat')
        }
        attempts = 0
        if (!stopped) setStatus('open')
      }

      sock.onmessage = (event) => {
        if (stopped) return
        let msg: ChatMessage
        try {
          msg = JSON.parse(event.data)
        } catch {
          return
        }

        // Drop anything addressed to a different room. Defends against late
        // deliveries (e.g. replayed history) from a socket being torn down
        // mid room-switch, so one room never shows another room's messages.
        // Exceptions: private messages (Room "pm") are cross-room by design,
        // and roomless control messages (e.g. /help, /status) have no room.
        if (msg.type !== 'pm' && msg.room && msg.room !== room) return

        // The server broadcasts an authoritative deduped user list as a
        // "users" message on every join/leave (and in the "system" welcome).
        // Trust it directly instead of tracking join/leave incrementally.
        if ((msg.type === 'system' || msg.type === 'users') && Array.isArray(msg.users)) {
          setUsers([...new Set(msg.users)])
        }

        // The 'history' separator and bare 'users' presence updates aren't bubbles
        if (msg.type === 'history' || msg.type === 'users') return

        setMessages((prev) => [...prev, msg])
      }

      sock.onerror = () => {
        if (!stopped) setStatus('error')
      }

      sock.onclose = () => {
        if (socketRef.current === sock) socketRef.current = null
        if (stopped) return // closed by cleanup — do NOT reconnect
        // Alert once per disconnect episode (on the first drop, before backoff).
        if (attempts === 0) {
          notify.warning('Disconnected from chat', 'Trying to reconnect…')
        }
        setStatus('closed')
        attempts += 1
        const delay = Math.min(1000 * 2 ** (attempts - 1), 10_000)
        timer = setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      stopped = true
      if (timer) clearTimeout(timer)
      sock?.close()
      if (socketRef.current === sock) socketRef.current = null
    }
  }, [token, room])

  return { messages, users, status, send, clear }
}
