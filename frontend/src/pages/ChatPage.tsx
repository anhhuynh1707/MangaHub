import { useState, useRef, useEffect, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { motion, AnimatePresence } from 'framer-motion'
import { Send, Hash, Users, Wifi, WifiOff, Loader2, Info, BookOpen } from 'lucide-react'
import { CHAT_ROOMS, MANGA_ROOM_PREFIX, type ChatMessage } from '@/api/chat'
import { mangaApi } from '@/api/manga'
import { useChat } from '@/hooks/useChat'
import { useAuthStore } from '@/store/authStore'

export default function ChatPage() {
  const { room: roomParam } = useParams<{ room: string }>()
  const room = roomParam ?? 'general'
  const navigate = useNavigate()
  const username = useAuthStore((s) => s.username)

  const { messages, users, status, send } = useChat(room)
  const [draft, setDraft] = useState('')
  const scrollRef = useRef<HTMLDivElement>(null)

  const mangaRoom = room.startsWith(MANGA_ROOM_PREFIX)
  const mangaId = mangaRoom ? room.slice(MANGA_ROOM_PREFIX.length) : ''

  // Resolve the manga title for the header when this is a manga discussion room
  const { data: roomManga } = useQuery({
    queryKey: ['manga', mangaId],
    queryFn: () => mangaApi.get(mangaId).then((r) => r.data.data),
    enabled: mangaRoom && !!mangaId,
    staleTime: 5 * 60 * 1000,
  })

  const roomMeta = useMemo(() => {
    if (mangaRoom) {
      return { value: room, label: roomManga?.title ?? 'Manga Discussion', emoji: '📖' }
    }
    return CHAT_ROOMS.find((r) => r.value === room) ?? { value: room, label: room, emoji: '💬' }
  }, [room, mangaRoom, roomManga])

  // Auto-scroll to the newest message
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' })
  }, [messages])

  function handleSend(e: React.FormEvent) {
    e.preventDefault()
    const text = draft.trim()
    if (!text || status !== 'open') return
    send(text)
    setDraft('')
  }

  return (
    <div className="mx-auto flex h-[calc(100vh-7rem)] max-w-6xl gap-4 p-4">
      {/* ── Room list ─────────────────────────────────────────────── */}
      <aside className="hidden w-48 flex-shrink-0 flex-col gap-1 sm:flex">
        <h2 className="px-2 pb-2 text-xs font-semibold uppercase tracking-wide text-[var(--color-muted-raw)]">
          Rooms
        </h2>
        {CHAT_ROOMS.map((r) => (
          <button
            key={r.value}
            onClick={() => navigate(`/chat/${r.value}`)}
            className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
              r.value === room
                ? 'bg-[var(--brand-red)] text-white'
                : 'text-[var(--color-text2)] hover:bg-[var(--color-surface2)]'
            }`}
          >
            <span>{r.emoji}</span>
            {r.label}
          </button>
        ))}

        {/* Active manga discussion room (not part of the fixed topic list) */}
        {mangaRoom && (
          <>
            <h2 className="px-2 pb-2 pt-4 text-xs font-semibold uppercase tracking-wide text-[var(--color-muted-raw)]">
              Manga
            </h2>
            <div className="flex items-center gap-2 rounded-lg bg-[var(--brand-red)] px-3 py-2 text-sm font-medium text-white">
              <span>📖</span>
              <span className="truncate">{roomMeta.label}</span>
            </div>
          </>
        )}
      </aside>

      {/* ── Chat panel ────────────────────────────────────────────── */}
      <div className="flex flex-1 flex-col overflow-hidden rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)]">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-[var(--color-border-raw)] px-4 py-3">
          <div className="flex items-center gap-2">
            {mangaRoom ? (
              <BookOpen className="h-4 w-4 text-[var(--brand-red)]" />
            ) : (
              <Hash className="h-4 w-4 text-[var(--color-muted-raw)]" />
            )}
            <span className="font-semibold text-[var(--color-text)]">{roomMeta.label}</span>
          </div>
          <ConnectionBadge status={status} />
        </div>

        {/* Messages */}
        <div ref={scrollRef} className="flex-1 space-y-1 overflow-y-auto px-4 py-3">
          {messages.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center gap-2 text-center text-[var(--color-muted-raw)]">
              <Info className="h-8 w-8 opacity-30" />
              <p className="text-sm">No messages yet — say hello! 👋</p>
              <p className="text-xs">Type <code className="rounded bg-[var(--color-surface2)] px-1">/help</code> for commands.</p>
            </div>
          ) : (
            <AnimatePresence initial={false}>
              {messages.map((m, i) => (
                <MessageBubble key={i} msg={m} isOwn={!!username && m.username === username} />
              ))}
            </AnimatePresence>
          )}
        </div>

        {/* Input */}
        <form onSubmit={handleSend} className="flex items-center gap-2 border-t border-[var(--color-border-raw)] p-3">
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={status === 'open' ? `Message #${roomMeta.label}…` : 'Connecting…'}
            disabled={status !== 'open'}
            className="flex-1 rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface2)] px-3 py-2 text-sm text-[var(--color-text)] placeholder-[var(--color-muted-raw)] focus:outline-none focus:ring-2 focus:ring-[var(--brand-red)]/40 disabled:opacity-60"
          />
          <button
            type="submit"
            disabled={status !== 'open' || !draft.trim()}
            className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[var(--brand-red)] text-white transition-colors hover:bg-[var(--brand-red-hover)] disabled:opacity-40"
          >
            <Send className="h-4 w-4" />
          </button>
        </form>
      </div>

      {/* ── Online users ──────────────────────────────────────────── */}
      <aside className="hidden w-48 flex-shrink-0 flex-col gap-1 lg:flex">
        <h2 className="flex items-center gap-1.5 px-2 pb-2 text-xs font-semibold uppercase tracking-wide text-[var(--color-muted-raw)]">
          <Users className="h-3.5 w-3.5" />
          Online — {users.length}
        </h2>
        {users.length === 0 ? (
          <p className="px-2 text-xs text-[var(--color-muted-raw)]">No one else here.</p>
        ) : (
          users.map((u) => (
            <div
              key={u}
              className="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm text-[var(--color-text2)]"
            >
              <span className="h-2 w-2 rounded-full bg-emerald-500" />
              {u}
              {u === username && <span className="text-xs text-[var(--color-muted-raw)]">(you)</span>}
            </div>
          ))
        )}
      </aside>
    </div>
  )
}

/* ── Connection status badge ───────────────────────────────────────── */
function ConnectionBadge({ status }: { status: string }) {
  if (status === 'open') {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-emerald-500">
        <Wifi className="h-3.5 w-3.5" />
        Connected
      </span>
    )
  }
  if (status === 'connecting') {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-[var(--color-muted-raw)]">
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        Connecting…
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1.5 text-xs font-medium text-red-500">
      <WifiOff className="h-3.5 w-3.5" />
      {status === 'error' ? 'Connection error' : 'Disconnected'}
    </span>
  )
}

/* ── Message bubble ────────────────────────────────────────────────── */
function MessageBubble({ msg, isOwn }: { msg: ChatMessage; isOwn: boolean }) {
  const time = new Date(msg.timestamp * 1000).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  })

  // System / join / leave / error — centered status lines
  if (msg.type === 'system' || msg.type === 'join' || msg.type === 'leave' || msg.type === 'error') {
    const color =
      msg.type === 'error'
        ? 'text-red-500'
        : msg.type === 'join'
        ? 'text-emerald-500'
        : 'text-[var(--color-muted-raw)]'
    return (
      <motion.div
        initial={{ opacity: 0, y: 4 }}
        animate={{ opacity: 1, y: 0 }}
        className="py-1 text-center"
      >
        <span className={`text-xs ${color} whitespace-pre-line`}>{msg.message}</span>
      </motion.div>
    )
  }

  // Private message — distinct styling
  if (msg.type === 'pm') {
    return (
      <motion.div initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} className="py-1">
        <div className="mx-auto max-w-[80%] rounded-lg border border-purple-400/40 bg-purple-500/10 px-3 py-2">
          <p className="text-[10px] font-semibold uppercase tracking-wide text-purple-400">
            {isOwn ? `PM → ${msg.recipient}` : `PM from ${msg.username}`}
          </p>
          <p className="mt-0.5 text-sm text-[var(--color-text)]">{msg.message}</p>
        </div>
      </motion.div>
    )
  }

  // Regular message bubble
  return (
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      className={`flex ${isOwn ? 'justify-end' : 'justify-start'}`}
    >
      <div className={`flex max-w-[75%] flex-col ${isOwn ? 'items-end' : 'items-start'}`}>
        {!isOwn && (
          <span className="mb-0.5 px-1 text-xs font-medium text-[var(--brand-teal)]">
            {msg.username}
          </span>
        )}
        <div
          className={`rounded-2xl px-3.5 py-2 text-sm ${
            isOwn
              ? 'rounded-br-sm bg-[var(--brand-red)] text-white'
              : 'rounded-bl-sm bg-[var(--color-surface2)] text-[var(--color-text)]'
          }`}
        >
          {msg.message}
        </div>
        <span className="mt-0.5 px-1 text-[10px] text-[var(--color-muted-raw)]">{time}</span>
      </div>
    </motion.div>
  )
}
