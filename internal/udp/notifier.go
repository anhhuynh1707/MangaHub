package udp

import (
	"fmt"
	"log"
	"net"
	"time"
)

// AckTimeout is how long BroadcastWithACK waits for client acknowledgements.
const AckTimeout = 3 * time.Second

// BroadcastNotification sends a notification to ALL registered clients.
// This is fire-and-forget — no ACK is expected from clients.
func (s *NotificationServer) BroadcastNotification(notif Notification) int {
	if notif.Timestamp == 0 {
		notif.Timestamp = time.Now().Unix()
	}

	s.mu.RLock()
	clients := make([](*net.UDPAddr), len(s.Clients))
	for i := range s.Clients {
		addr := s.Clients[i] // copy
		clients[i] = &addr
	}
	s.mu.RUnlock()

	if len(clients) == 0 {
		log.Printf("UDP: No clients registered, notification dropped: %s", notif.Message)
		return 0
	}

	sent := 0
	for _, addr := range clients {
		s.sendTo(addr, notif)
		sent++
	}

	log.Printf("UDP: Broadcast '%s' to %d clients: %s", notif.Type, sent, notif.Message)
	return sent
}

// NotifyNewChapter broadcasts a new chapter release notification.
func (s *NotificationServer) NotifyNewChapter(mangaID, title string, chapter int) int {
	return s.BroadcastNotification(Notification{
		Type:    "new_chapter",
		MangaID: mangaID,
		Title:   title,
		Message: fmt.Sprintf("New chapter! %s — Chapter %d released!", title, chapter),
	})
}

// NotifyMangaUpdate broadcasts a manga metadata update notification.
func (s *NotificationServer) NotifyMangaUpdate(mangaID, title, updateMsg string) int {
	return s.BroadcastNotification(Notification{
		Type:    "manga_update",
		MangaID: mangaID,
		Title:   title,
		Message: updateMsg,
	})
}

// NotifySystem broadcasts a system-wide notification.
func (s *NotificationServer) NotifySystem(message string) int {
	return s.BroadcastNotification(Notification{
		Type:    "system",
		Message: message,
	})
}

// BroadcastWithACK sends a notification to all registered clients and waits up
// to AckTimeout for each client to reply with {"type":"ack","notification_id":"..."}.
// Returns a DeliveryRecord describing which clients acknowledged in time.
func (s *NotificationServer) BroadcastWithACK(notif Notification) *DeliveryRecord {
	if notif.Timestamp == 0 {
		notif.Timestamp = time.Now().Unix()
	}

	// Snapshot the current client list
	s.mu.RLock()
	addrs := make([](*net.UDPAddr), len(s.Clients))
	addrStrs := make([]string, len(s.Clients))
	for i := range s.Clients {
		addr := s.Clients[i]
		addrs[i] = &addr
		addrStrs[i] = addr.String()
	}
	s.mu.RUnlock()

	if len(addrs) == 0 {
		log.Printf("UDP ACK: No clients registered, skipping broadcast")
		return &DeliveryRecord{
			NotifID:  generateID(),
			Message:  notif.Message,
			SentAt:   time.Now(),
			AckRate:  1.0,
		}
	}

	// Register with the ACK tracker before sending
	id, pd := s.ackTracker.track(notif.Message, addrStrs)
	notif.NotificationID = id

	// Send to all clients
	for _, addr := range addrs {
		s.sendTo(addr, notif)
	}

	log.Printf("UDP ACK: Sent notif %s to %d clients, waiting %s for ACKs...", id, len(addrs), AckTimeout)

	// Wait for ACKs and build the final report
	record := s.ackTracker.finalise(id, pd, AckTimeout)
	log.Printf("UDP ACK: %s — %d/%d clients ACK'd (rate=%.0f%%)",
		id, len(record.AckedBy), len(record.SentTo), record.AckRate*100)
	return record
}
