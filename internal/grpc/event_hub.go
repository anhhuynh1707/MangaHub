package grpc

import (
	"fmt"
	"log"
	"sync"
	"time"

	pb "mangahub/internal/grpc/pb"
)

// MangaEventHub fans out real-time manga events to all active WatchMangaUpdates streams.
// The HTTP layer calls Publish() whenever a progress update or manga change occurs;
// each connected gRPC stream client receives the event via its own buffered channel.
type MangaEventHub struct {
	mu          sync.RWMutex
	subscribers map[string]chan *pb.MangaEvent // subscriber_id -> channel
}

// NewMangaEventHub creates an event hub.
func NewMangaEventHub() *MangaEventHub {
	return &MangaEventHub{
		subscribers: make(map[string]chan *pb.MangaEvent),
	}
}

// Subscribe registers a new streaming client and returns its event channel.
// The caller must call Unsubscribe when the stream ends.
func (h *MangaEventHub) Subscribe(id string) chan *pb.MangaEvent {
	ch := make(chan *pb.MangaEvent, 32) // buffered to avoid blocking Publish
	h.mu.Lock()
	h.subscribers[id] = ch
	h.mu.Unlock()
	log.Printf("gRPC EventHub: subscriber %s joined (%d total)", id, h.SubscriberCount())
	return ch
}

// Unsubscribe removes a streaming client and closes its channel.
func (h *MangaEventHub) Unsubscribe(id string) {
	h.mu.Lock()
	if ch, ok := h.subscribers[id]; ok {
		close(ch)
		delete(h.subscribers, id)
	}
	h.mu.Unlock()
	log.Printf("gRPC EventHub: subscriber %s left (%d total)", id, h.SubscriberCount())
}

// Publish sends an event to all active subscribers, skipping any whose buffers are full.
func (h *MangaEventHub) Publish(event *pb.MangaEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, ch := range h.subscribers {
		select {
		case ch <- event:
		default:
			log.Printf("gRPC EventHub: dropped event for slow subscriber %s", id)
		}
	}
}

// SubscriberCount returns how many clients are currently watching.
func (h *MangaEventHub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

// PublishProgressUpdate is a convenience wrapper called when a user updates reading progress.
func (h *MangaEventHub) PublishProgressUpdate(userID, mangaID string, chapter int32) {
	h.Publish(&pb.MangaEvent{
		EventType: "progress_updated",
		MangaId:   mangaID,
		UserId:    userID,
		Chapter:   chapter,
		Message:   fmt.Sprintf("User %s reached chapter %d of %s", userID, chapter, mangaID),
	})
}

// PublishMangaUpdated is a convenience wrapper called when manga metadata changes.
func (h *MangaEventHub) PublishMangaUpdated(mangaID, title string) {
	h.Publish(&pb.MangaEvent{
		EventType: "manga_updated",
		MangaId:   mangaID,
		Message:   fmt.Sprintf("Manga '%s' was updated", title),
	})
}
