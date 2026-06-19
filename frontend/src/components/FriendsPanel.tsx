import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { UserPlus, UserCheck, UserX, Trash2, Loader2, Users } from 'lucide-react'
import { friendApi, userIdFromUsername, usernameFromUserId } from '@/api/friend'
import { userApi } from '@/api/user'
import { useDebounce } from '@/hooks/useDebounce'

function Avatar({ name }: { name: string }) {
  return (
    <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full bg-[var(--brand-teal)] text-sm font-bold text-white">
      {name.slice(0, 2).toUpperCase()}
    </div>
  )
}

export function FriendsPanel() {
  const qc = useQueryClient()
  const [addName, setAddName] = useState('')
  const [addError, setAddError] = useState('')
  const [addOk, setAddOk] = useState('')
  const [focused, setFocused] = useState(false)

  const { data: friendsData, isLoading: friendsLoading } = useQuery({
    queryKey: ['friends'],
    queryFn: () => friendApi.list().then((r) => r.data.data),
  })

  const { data: pendingData, isLoading: pendingLoading } = useQuery({
    queryKey: ['friends-pending'],
    queryFn: () => friendApi.pending().then((r) => r.data.data),
  })

  const friends = friendsData?.friends ?? []
  const pending = pendingData?.pending_requests ?? []

  // Debounced username autocomplete
  const debouncedQuery = useDebounce(addName.trim(), 250)
  const { data: searchResults } = useQuery({
    queryKey: ['user-search', debouncedQuery],
    queryFn: () => userApi.search(debouncedQuery).then((r) => r.data.data),
    enabled: debouncedQuery.length > 0,
  })
  const suggestions = searchResults ?? []
  const showDropdown = focused && debouncedQuery.length > 0 && suggestions.length > 0

  function refresh() {
    qc.invalidateQueries({ queryKey: ['friends'] })
    qc.invalidateQueries({ queryKey: ['friends-pending'] })
  }

  // Takes the friend's real user id (from search) plus a display name for the toast.
  const addMutation = useMutation({
    mutationFn: (v: { friendId: string; name: string }) => friendApi.add(v.friendId),
    onSuccess: (_, v) => {
      setAddName('')
      setAddError('')
      setAddOk(`Friend request sent to ${v.name}.`)
      setFocused(false)
      refresh()
    },
    onError: (e: any) => {
      setAddOk('')
      setAddError(e?.response?.data?.error ?? 'Could not send friend request.')
    },
  })

  const acceptMutation = useMutation({
    mutationFn: (friendId: string) => friendApi.accept(friendId),
    onSuccess: refresh,
  })

  const declineMutation = useMutation({
    mutationFn: (friendId: string) => friendApi.decline(friendId),
    onSuccess: refresh,
  })

  // Submitting the form (typed a full username and pressed Send) falls back to
  // constructing the id from the username slug.
  function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    const name = addName.trim()
    if (!name) return
    setAddError('')
    setAddOk('')
    addMutation.mutate({ friendId: userIdFromUsername(name), name })
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Add friend */}
      <div className="rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-4">
        <h3 className="mb-2 flex items-center gap-2 text-sm font-semibold text-[var(--color-text)]">
          <UserPlus className="h-4 w-4 text-[var(--brand-red)]" />
          Add a friend
        </h3>
        <form onSubmit={handleAdd} className="flex items-center gap-2">
          <div className="relative flex-1">
            <input
              value={addName}
              onChange={(e) => setAddName(e.target.value)}
              onFocus={() => setFocused(true)}
              onBlur={() => setTimeout(() => setFocused(false), 150)}
              placeholder="Search a username (e.g. bob)"
              autoComplete="off"
              className="w-full rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface2)] px-3 py-2 text-sm text-[var(--color-text)] placeholder-[var(--color-muted-raw)] focus:outline-none focus:ring-2 focus:ring-[var(--brand-red)]/40"
            />

            {/* Autocomplete dropdown */}
            {showDropdown && (
              <ul className="absolute left-0 right-0 top-full z-20 mt-1 max-h-60 overflow-y-auto rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface)] py-1 shadow-lg">
                {suggestions.map((u) => (
                  <li key={u.id}>
                    <button
                      type="button"
                      // onMouseDown fires before the input's onBlur, so the click registers
                      onMouseDown={(e) => {
                        e.preventDefault()
                        addMutation.mutate({ friendId: u.id, name: u.username })
                      }}
                      disabled={addMutation.isPending}
                      className="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-[var(--color-text)] transition-colors hover:bg-[var(--color-surface2)] disabled:opacity-50"
                    >
                      <Avatar name={u.username} />
                      <span className="flex-1 truncate font-medium">{u.username}</span>
                      <UserPlus className="h-4 w-4 text-[var(--color-muted-raw)]" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <button
            type="submit"
            disabled={!addName.trim() || addMutation.isPending}
            className="flex h-9 items-center gap-1.5 rounded-lg bg-[var(--brand-red)] px-4 text-sm font-semibold text-white transition-colors hover:bg-[var(--brand-red-hover)] disabled:opacity-40"
          >
            {addMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Send'}
          </button>
        </form>
        {addError && <p className="mt-2 text-xs text-red-500">{addError}</p>}
        {addOk && <p className="mt-2 text-xs text-emerald-500">{addOk}</p>}
      </div>

      {/* Pending incoming requests */}
      {!pendingLoading && pending.length > 0 && (
        <div>
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--color-muted-raw)]">
            Friend Requests ({pending.length})
          </h3>
          <div className="flex flex-col gap-2">
            {pending.map((requesterId) => (
              <div
                key={requesterId}
                className="flex items-center gap-3 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-3"
              >
                <Avatar name={usernameFromUserId(requesterId)} />
                <span className="flex-1 text-sm font-medium text-[var(--color-text)]">
                  {usernameFromUserId(requesterId)}
                </span>
                <button
                  onClick={() => acceptMutation.mutate(requesterId)}
                  disabled={acceptMutation.isPending}
                  className="flex items-center gap-1 rounded-lg bg-emerald-500 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-emerald-600 disabled:opacity-50"
                >
                  <UserCheck className="h-3.5 w-3.5" />
                  Accept
                </button>
                <button
                  onClick={() => declineMutation.mutate(requesterId)}
                  disabled={declineMutation.isPending}
                  className="flex items-center gap-1 rounded-lg border border-[var(--color-border-raw)] px-3 py-1.5 text-xs font-semibold text-[var(--color-muted-raw)] transition-colors hover:border-red-400 hover:text-red-500 disabled:opacity-50"
                >
                  <UserX className="h-3.5 w-3.5" />
                  Decline
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Friends list */}
      <div>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--color-muted-raw)]">
          Friends ({friends.length})
        </h3>
        {friendsLoading ? (
          <p className="text-sm text-[var(--color-muted-raw)]">Loading…</p>
        ) : friends.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-[var(--color-border-raw)] py-12 text-center">
            <Users className="h-8 w-8 text-[var(--color-muted-raw)] opacity-30" />
            <p className="text-sm text-[var(--color-muted-raw)]">
              No friends yet. Add someone by their username above.
            </p>
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {friends.map((f) => (
              <FriendRow key={f.user_id} friendId={f.user_id} onRemoved={refresh} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

/* ── Friend row with inline remove confirmation ─────────────────────── */
function FriendRow({ friendId, onRemoved }: { friendId: string; onRemoved: () => void }) {
  const [confirm, setConfirm] = useState(false)

  const removeMutation = useMutation({
    mutationFn: () => friendApi.remove(friendId),
    onSuccess: () => {
      setConfirm(false)
      onRemoved()
    },
  })

  return (
    <div className="flex items-center gap-3 rounded-xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-3">
      <Avatar name={usernameFromUserId(friendId)} />
      <span className="flex-1 text-sm font-medium text-[var(--color-text)]">
        {usernameFromUserId(friendId)}
      </span>

      {confirm ? (
        <div className="flex items-center gap-2">
          <span className="text-xs text-[var(--color-muted-raw)]">Remove?</span>
          <button
            onClick={() => removeMutation.mutate()}
            disabled={removeMutation.isPending}
            className="flex items-center rounded border border-red-500/30 px-2 py-1 text-xs font-semibold text-red-500 hover:bg-red-500/10 disabled:opacity-50"
          >
            {removeMutation.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : 'Yes'}
          </button>
          <button
            onClick={() => setConfirm(false)}
            disabled={removeMutation.isPending}
            className="rounded border border-[var(--color-border-raw)] px-2 py-1 text-xs font-semibold text-[var(--color-muted-raw)] hover:text-[var(--color-text)] disabled:opacity-50"
          >
            No
          </button>
        </div>
      ) : (
        <button
          onClick={() => setConfirm(true)}
          className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--color-muted-raw)] transition-colors hover:text-red-500"
          title="Remove friend"
        >
          <Trash2 className="h-4 w-4" />
        </button>
      )}
    </div>
  )
}
