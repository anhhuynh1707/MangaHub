package tcp

import (
	"testing"
)

// ── helpers ────────────────────────────────────────────────────

func newResolver(strategy string) *ConflictResolver {
	return NewConflictResolver(strategy)
}

// ── first-update (no conflict) ──────────────────────────────────

func TestResolve_FirstUpdate_AlwaysAccepted(t *testing.T) {
	cr := newResolver("last_write_wins")
	result := cr.Resolve("user-alice", "one-piece", 50, "device-1")

	if !result.Accepted {
		t.Error("first update should always be accepted")
	}
	if result.FinalChapter != 50 {
		t.Errorf("FinalChapter: want 50, got %d", result.FinalChapter)
	}
	if result.Conflict != nil {
		t.Error("first update should not produce a conflict")
	}
}

func TestResolve_SameChapter_NoConflict(t *testing.T) {
	cr := newResolver("last_write_wins")
	cr.Resolve("user-alice", "one-piece", 50, "device-1")
	result := cr.Resolve("user-alice", "one-piece", 50, "device-2")

	if !result.Accepted {
		t.Error("same chapter from a different device should be accepted")
	}
	if result.Conflict != nil {
		t.Error("same chapter should not produce a conflict")
	}
}

// ── last_write_wins ─────────────────────────────────────────────

func TestResolve_LastWriteWins_AcceptsIncoming(t *testing.T) {
	cr := newResolver("last_write_wins")
	cr.Resolve("user-alice", "naruto", 300, "device-1")
	result := cr.Resolve("user-alice", "naruto", 100, "device-2") // lower chapter = conflict

	if !result.Accepted {
		t.Error("last_write_wins: incoming update should be accepted")
	}
	if result.FinalChapter != 100 {
		t.Errorf("last_write_wins: FinalChapter: want 100, got %d", result.FinalChapter)
	}
	if result.Conflict == nil {
		t.Error("last_write_wins: conflict should be recorded")
	}
}

func TestResolve_LastWriteWins_IsDefaultStrategy(t *testing.T) {
	cr := newResolver("") // empty string → default
	if cr.GetStrategy() != "last_write_wins" {
		t.Errorf("default strategy: want %q, got %q", "last_write_wins", cr.GetStrategy())
	}
}

// ── merge ───────────────────────────────────────────────────────

func TestResolve_Merge_KeepsHigherChapter_ExistingWins(t *testing.T) {
	cr := newResolver("merge")
	cr.Resolve("user-alice", "bleach", 500, "device-1")
	result := cr.Resolve("user-alice", "bleach", 200, "device-2") // existing is higher

	if !result.Accepted {
		t.Error("merge: result should be accepted")
	}
	if result.FinalChapter != 500 {
		t.Errorf("merge: FinalChapter: want 500, got %d", result.FinalChapter)
	}
}

func TestResolve_Merge_KeepsHigherChapter_IncomingWins(t *testing.T) {
	cr := newResolver("merge")
	cr.Resolve("user-alice", "bleach", 200, "device-1")
	result := cr.Resolve("user-alice", "bleach", 500, "device-2") // incoming is higher

	if !result.Accepted {
		t.Error("merge: result should be accepted")
	}
	if result.FinalChapter != 500 {
		t.Errorf("merge: FinalChapter: want 500, got %d", result.FinalChapter)
	}
}

// ── user_choice ─────────────────────────────────────────────────

func TestResolve_UserChoice_RejectsIncoming(t *testing.T) {
	cr := newResolver("user_choice")
	cr.Resolve("user-alice", "attack-on-titan", 80, "device-1")
	result := cr.Resolve("user-alice", "attack-on-titan", 60, "device-2")

	if result.Accepted {
		t.Error("user_choice: conflicting update should be rejected")
	}
	if result.FinalChapter != 80 {
		t.Errorf("user_choice: FinalChapter: want 80 (existing), got %d", result.FinalChapter)
	}
	if result.Conflict == nil {
		t.Error("user_choice: conflict should be recorded")
	}
}

// ── strategy switching ──────────────────────────────────────────

func TestSetStrategy_ChangesActiveStrategy(t *testing.T) {
	cr := newResolver("last_write_wins")
	cr.SetStrategy("merge")
	if cr.GetStrategy() != "merge" {
		t.Errorf("GetStrategy: want %q, got %q", "merge", cr.GetStrategy())
	}
	cr.SetStrategy("user_choice")
	if cr.GetStrategy() != "user_choice" {
		t.Errorf("GetStrategy: want %q, got %q", "user_choice", cr.GetStrategy())
	}
}

// ── conflict log ────────────────────────────────────────────────

func TestConflictLog_RecordsOnConflict(t *testing.T) {
	cr := newResolver("last_write_wins")
	cr.Resolve("user-alice", "one-piece", 50, "device-1")
	cr.Resolve("user-alice", "one-piece", 30, "device-2") // conflict

	log := cr.GetConflictLog()
	if len(log) != 1 {
		t.Errorf("conflict log: want 1 entry, got %d", len(log))
	}
}

func TestConflictLog_EmptyWhenNoConflict(t *testing.T) {
	cr := newResolver("last_write_wins")
	cr.Resolve("user-alice", "one-piece", 50, "device-1")
	cr.Resolve("user-alice", "one-piece", 50, "device-2") // same chapter, no conflict

	log := cr.GetConflictLog()
	if len(log) != 0 {
		t.Errorf("conflict log: want 0 entries, got %d", len(log))
	}
}

func TestConflictLog_IsolatedPerUserMangaPair(t *testing.T) {
	cr := newResolver("last_write_wins")
	// alice and bob reading the same manga don't conflict with each other
	r1 := cr.Resolve("user-alice", "one-piece", 100, "device-1")
	r2 := cr.Resolve("user-bob", "one-piece", 50, "device-2")

	if !r1.Accepted || !r2.Accepted {
		t.Error("different users should not conflict with each other")
	}
	if r1.Conflict != nil || r2.Conflict != nil {
		t.Error("different users should not produce a conflict")
	}
	if len(cr.GetConflictLog()) != 0 {
		t.Error("conflict log should be empty when different users update the same manga")
	}
}

// ── concurrent safety ───────────────────────────────────────────

func TestResolve_ConcurrentAccess(t *testing.T) {
	cr := newResolver("last_write_wins")
	done := make(chan struct{}, 20)

	for i := 0; i < 10; i++ {
		go func(ch int) {
			cr.Resolve("user-alice", "one-piece", ch, "device-A")
			done <- struct{}{}
		}(i + 1)
		go func(ch int) {
			cr.Resolve("user-bob", "naruto", ch, "device-B")
			done <- struct{}{}
		}(i + 1)
	}

	for i := 0; i < 20; i++ {
		<-done
	}
	// If we reach here without a data-race panic, the mutex is working.
}
