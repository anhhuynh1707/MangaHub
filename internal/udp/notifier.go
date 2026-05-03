package udp

import (
	"fmt"
	"log"
	"net"
	"time"
)

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
