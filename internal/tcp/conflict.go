package tcp

import (
	"fmt"
	"log"
	"sync"
	"time"

	"mangahub/pkg/models"
)

// ConflictResolver handles conflict detection and resolution for concurrent
// progress updates arriving from multiple devices/sessions.
type ConflictResolver struct {
	// Strategy is the active resolution strategy: "last_write_wins", "merge", "user_choice"
	Strategy string

	// recentUpdates tracks the most recent update per (user, manga) pair
	// so we can detect when two devices send conflicting chapters.
	recentUpdates map[string]*trackedUpdate

	// conflictLog stores resolved conflicts for audit/display.
	conflictLog []models.ProgressConflict

	mu sync.RWMutex
}

// trackedUpdate records the last accepted update for a (user, manga) key.
type trackedUpdate struct {
	UserID    string
	MangaID   string
	Chapter   int
	DeviceID  string
	Timestamp int64
}

// NewConflictResolver creates a resolver with the given strategy.
func NewConflictResolver(strategy string) *ConflictResolver {
	if strategy == "" {
		strategy = "last_write_wins"
	}
	return &ConflictResolver{
		Strategy:      strategy,
		recentUpdates: make(map[string]*trackedUpdate),
		conflictLog:   make([]models.ProgressConflict, 0),
	}
}

// conflictKey produces a unique key for a (user, manga) pair.
func conflictKey(userID, mangaID string) string {
	return userID + ":" + mangaID
}

// ResolveResult is what the resolver returns after evaluating an update.
type ResolveResult struct {
	// Accepted is true if the incoming update should be persisted/broadcast.
	Accepted bool

	// FinalChapter is the chapter number to persist (may differ from incoming
	// in merge mode where we pick the higher chapter).
	FinalChapter int

	// Conflict is non-nil when a conflict was detected.
	Conflict *models.ProgressConflict

	// Message is a human-readable explanation.
	Message string
}

// Resolve evaluates an incoming progress update against the last known update
// for the same (user, manga) pair and applies the configured strategy.
func (cr *ConflictResolver) Resolve(userID, mangaID string, chapter int, deviceID string) *ResolveResult {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	key := conflictKey(userID, mangaID)
	now := time.Now().Unix()

	existing, found := cr.recentUpdates[key]

	// No previous update → accept unconditionally.
	if !found {
		cr.recentUpdates[key] = &trackedUpdate{
			UserID:    userID,
			MangaID:   mangaID,
			Chapter:   chapter,
			DeviceID:  deviceID,
			Timestamp: now,
		}
		return &ResolveResult{
			Accepted:     true,
			FinalChapter: chapter,
			Message:      "No conflict",
		}
	}

	// Same chapter → no conflict.
	if existing.Chapter == chapter {
		existing.Timestamp = now
		existing.DeviceID = deviceID
		return &ResolveResult{
			Accepted:     true,
			FinalChapter: chapter,
			Message:      "No conflict (same chapter)",
		}
	}

	// ----- CONFLICT DETECTED -----
	conflict := models.ProgressConflict{
		UserID:          userID,
		MangaID:         mangaID,
		ExistingChapter: existing.Chapter,
		ExistingDevice:  existing.DeviceID,
		ExistingTime:    existing.Timestamp,
		IncomingChapter: chapter,
		IncomingDevice:  deviceID,
		IncomingTime:    now,
	}

	var result *ResolveResult

	switch cr.Strategy {
	case "merge":
		// Merge strategy: always pick the higher chapter (furthest progress).
		winner := chapter
		if existing.Chapter > chapter {
			winner = existing.Chapter
		}

		conflict.Resolution = models.ConflictResolution{
			Strategy:   "merge",
			Timestamp:  now,
			DeviceID:   deviceID,
			Resolution: fmt.Sprintf("Merged: kept higher chapter %d (existing ch.%d from %s vs incoming ch.%d from %s)",
				winner, existing.Chapter, existing.DeviceID, chapter, deviceID),
		}

		result = &ResolveResult{
			Accepted:     true,
			FinalChapter: winner,
			Conflict:     &conflict,
			Message:      conflict.Resolution.Resolution,
		}

	case "user_choice":
		// User-choice strategy: reject the incoming update and notify the user
		// so they can decide which version to keep.
		conflict.Resolution = models.ConflictResolution{
			Strategy:   "user_choice",
			Timestamp:  now,
			DeviceID:   deviceID,
			Resolution: fmt.Sprintf("Conflict detected: existing ch.%d from %s vs incoming ch.%d from %s — awaiting user choice",
				existing.Chapter, existing.DeviceID, chapter, deviceID),
		}

		result = &ResolveResult{
			Accepted:     false,
			FinalChapter: existing.Chapter,
			Conflict:     &conflict,
			Message:      conflict.Resolution.Resolution,
		}

	default: // "last_write_wins"
		conflict.Resolution = models.ConflictResolution{
			Strategy:   "last_write_wins",
			Timestamp:  now,
			DeviceID:   deviceID,
			Resolution: fmt.Sprintf("Last-write-wins: accepted ch.%d from %s (replaced ch.%d from %s)",
				chapter, deviceID, existing.Chapter, existing.DeviceID),
		}

		result = &ResolveResult{
			Accepted:     true,
			FinalChapter: chapter,
			Conflict:     &conflict,
			Message:      conflict.Resolution.Resolution,
		}
	}

	// Record the conflict.
	cr.conflictLog = append(cr.conflictLog, conflict)
	// Cap log at 100 entries.
	if len(cr.conflictLog) > 100 {
		cr.conflictLog = cr.conflictLog[len(cr.conflictLog)-100:]
	}

	log.Printf("TCP Conflict [%s]: %s", cr.Strategy, result.Message)

	// Update the tracked state to the accepted chapter.
	if result.Accepted {
		cr.recentUpdates[key] = &trackedUpdate{
			UserID:    userID,
			MangaID:   mangaID,
			Chapter:   result.FinalChapter,
			DeviceID:  deviceID,
			Timestamp: now,
		}
	}

	return result
}

// GetConflictLog returns the recent conflict history.
func (cr *ConflictResolver) GetConflictLog() []models.ProgressConflict {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	out := make([]models.ProgressConflict, len(cr.conflictLog))
	copy(out, cr.conflictLog)
	return out
}

// GetStrategy returns the current conflict resolution strategy.
func (cr *ConflictResolver) GetStrategy() string {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	return cr.Strategy
}

// SetStrategy changes the active strategy at runtime.
func (cr *ConflictResolver) SetStrategy(strategy string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.Strategy = strategy
	log.Printf("TCP Conflict: Strategy changed to '%s'", strategy)
}
