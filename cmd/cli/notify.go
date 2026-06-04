package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"
)

// udpNotification matches the server's UDP message format.
type udpNotification struct {
	Type      string `json:"type"`
	MangaID   string `json:"manga_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

func handleNotify(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub notify <subscribe|unsubscribe|test|send|send-ack|ack-stats>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  subscribe     Register for UDP notifications (stays connected)")
		fmt.Println("  unsubscribe   Unregister from UDP notifications")
		fmt.Println("  test          Test the UDP notification system")
		fmt.Println("  send          Fire-and-forget broadcast via HTTP API")
		fmt.Println("  send-ack      Broadcast with delivery confirmation (waits 3s for ACKs)")
		fmt.Println("  ack-stats     View recent delivery records and ACK rates")
		return
	}

	switch args[0] {
	case "subscribe":
		notifySubscribe()
	case "unsubscribe":
		notifyUnsubscribe()
	case "test":
		notifyTest()
	case "send":
		notifySend(args[1:])
	case "send-ack":
		notifySendACK(args[1:])
	case "ack-stats":
		notifyACKStats()
	default:
		fmt.Printf("✗ Unknown notify command: '%s'\n", args[0])
		fmt.Println("Available: subscribe, unsubscribe, test, send, send-ack, ack-stats")
	}
}

// notifySubscribe registers for UDP notifications and listens for incoming messages.
func notifySubscribe() {
	serverAddr, err := net.ResolveUDPAddr("udp", "localhost:9091")
	if err != nil {
		fmt.Printf("✗ Failed to resolve server address: %v\n", err)
		return
	}

	// Use port 0 to let the OS assign a random local port
	localAddr, _ := net.ResolveUDPAddr("udp", ":0")
	conn, err := net.DialUDP("udp", localAddr, serverAddr)
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		fmt.Println("  Make sure the server is running: go run ./cmd/api-server/")
		return
	}
	defer conn.Close()

	// Send register message
	sendUDP(conn, udpNotification{Type: "register"})

	// Read the ack
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("✗ No response from UDP server. Is it running?")
		return
	}

	var ack udpNotification
	json.Unmarshal(buf[:n], &ack)

	fmt.Printf("✓ Subscribed to notifications!\n")
	fmt.Printf("  %s\n", ack.Message)
	fmt.Printf("  Listening on: %s\n", conn.LocalAddr())
	fmt.Println("\nWaiting for notifications... (Press Ctrl+C to exit)")
	fmt.Println()

	// Handle Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Listen for notifications
	go func() {
		conn.SetReadDeadline(time.Time{}) // Remove deadline for long-lived listener
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}

			var notif udpNotification
			if err := json.Unmarshal(buf[:n], &notif); err != nil {
				continue
			}

			ts := time.Unix(notif.Timestamp, 0).Format("15:04:05")
			switch notif.Type {
			case "new_chapter":
				fmt.Printf("[%s] 📖 NEW CHAPTER: %s\n", ts, notif.Message)
			case "manga_update":
				fmt.Printf("[%s] 📝 UPDATE: %s\n", ts, notif.Message)
			case "system":
				fmt.Printf("[%s] 🔔 SYSTEM: %s\n", ts, notif.Message)
			default:
				fmt.Printf("[%s] 📨 %s: %s\n", ts, notif.Type, notif.Message)
			}
		}
	}()

	<-interrupt

	// Unregister before exit
	sendUDP(conn, udpNotification{Type: "unregister"})
	fmt.Println("\n✓ Unsubscribed from notifications")
}

// notifyUnsubscribe sends an unregister message to the UDP server.
func notifyUnsubscribe() {
	serverAddr, err := net.ResolveUDPAddr("udp", "localhost:9091")
	if err != nil {
		fmt.Printf("✗ Failed to resolve server address: %v\n", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	sendUDP(conn, udpNotification{Type: "unregister"})

	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("✗ No response from server")
		return
	}

	var ack udpNotification
	json.Unmarshal(buf[:n], &ack)
	fmt.Printf("✓ %s\n", ack.Message)
}

// notifyTest sends a test message to check if the UDP server is alive.
func notifyTest() {
	serverAddr, err := net.ResolveUDPAddr("udp", "localhost:9091")
	if err != nil {
		fmt.Printf("✗ Failed to resolve: %v\n", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	sendUDP(conn, udpNotification{Type: "test"})

	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("✗ No response from UDP server. Is it running?")
		fmt.Println("  Start server: go run ./cmd/api-server/")
		return
	}

	var resp udpNotification
	json.Unmarshal(buf[:n], &resp)
	fmt.Println("UDP Notification Test:")
	fmt.Printf("  Status:  ✓ %s\n", resp.Message)
	fmt.Printf("  Server:  localhost:9091\n")
}

// notifySend triggers a notification broadcast via the HTTP API.
func notifySend(args []string) {
	cfg := requireAuth()

	notifType := parseFlag(args, "type")
	mangaID := parseFlag(args, "manga-id")
	message := parseFlag(args, "message")

	if notifType == "" {
		notifType = "system"
	}
	if message == "" {
		message = "Test notification from CLI"
	}

	body := map[string]string{
		"type":     notifType,
		"manga_id": mangaID,
		"message":  message,
	}

	resp, err := apiRequest("POST", "/notify/broadcast", body, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to send notification: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✓ Notification sent!\n")
	fmt.Printf("  Type:    %s\n", notifType)
	fmt.Printf("  Message: %s\n", message)
}

// notifySendACK broadcasts with delivery confirmation — waits 3 s for client ACKs.
func notifySendACK(args []string) {
	cfg := requireAuth()

	notifType := parseFlag(args, "type")
	mangaID := parseFlag(args, "manga-id")
	message := parseFlag(args, "message")

	if notifType == "" {
		notifType = "system"
	}
	if message == "" {
		fmt.Println("Usage: mangahub notify send-ack --message <text> [--type <type>] [--manga-id <id>]")
		fmt.Println("Example: mangahub notify send-ack --type new_chapter --manga-id one-piece --message \"Chapter 1121!\"")
		return
	}

	body := map[string]string{
		"type":     notifType,
		"manga_id": mangaID,
		"message":  message,
	}

	fmt.Printf("📡 Sending broadcast with ACK tracking (waiting up to 3s)...\n")
	fmt.Printf("   Type: %s | Message: %s\n\n", notifType, message)

	resp, err := apiRequest("POST", "/notify/broadcast-ack", body, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to send: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var record struct {
		NotifID  string   `json:"notif_id"`
		Message  string   `json:"message"`
		SentTo   []string `json:"sent_to"`
		AckedBy  []string `json:"acked_by"`
		Unacked  []string `json:"unacked"`
		AckRate  float64  `json:"ack_rate"`
		TimedOut bool     `json:"timed_out"`
	}
	json.Unmarshal(resp.Data, &record)

	fmt.Printf("📊 Delivery Report — %s\n", record.NotifID)
	fmt.Printf("   Sent to:     %d client(s)\n", len(record.SentTo))
	fmt.Printf("   ACK'd:       %d client(s)\n", len(record.AckedBy))
	fmt.Printf("   Unacked:     %d client(s)\n", len(record.Unacked))
	fmt.Printf("   ACK rate:    %.0f%%\n", record.AckRate*100)

	if len(record.AckedBy) > 0 {
		fmt.Printf("   ✓ Confirmed: %v\n", record.AckedBy)
	}
	if len(record.Unacked) > 0 {
		fmt.Printf("   ✗ No reply:  %v\n", record.Unacked)
	}
	if record.TimedOut {
		fmt.Println("   ⚠  Some clients did not ACK within 3s (fire-and-forget still delivered)")
	}
}

// notifyACKStats shows the last 50 delivery records stored on the server.
func notifyACKStats() {
	cfg := requireAuth()

	resp, err := apiRequest("GET", "/notify/ack-stats", nil, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to fetch ACK stats: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var records []struct {
		NotifID  string   `json:"notif_id"`
		Message  string   `json:"message"`
		SentTo   []string `json:"sent_to"`
		AckedBy  []string `json:"acked_by"`
		AckRate  float64  `json:"ack_rate"`
		TimedOut bool     `json:"timed_out"`
	}
	json.Unmarshal(resp.Data, &records)

	if len(records) == 0 {
		fmt.Println("No delivery records yet.")
		fmt.Println("Use 'mangahub notify send-ack' to generate tracked broadcasts.")
		return
	}

	fmt.Printf("📊 Delivery History (%d records):\n\n", len(records))
	headers := []string{"Notification ID", "Message", "Sent", "ACK'd", "Rate", "Timeout"}
	var rows [][]string
	for _, r := range records {
		msg := r.Message
		if len(msg) > 25 {
			msg = msg[:22] + "..."
		}
		timeout := "No"
		if r.TimedOut {
			timeout = "Yes"
		}
		rows = append(rows, []string{
			r.NotifID,
			msg,
			fmt.Sprintf("%d", len(r.SentTo)),
			fmt.Sprintf("%d", len(r.AckedBy)),
			fmt.Sprintf("%.0f%%", r.AckRate*100),
			timeout,
		})
	}
	printTable(headers, rows)
}

func sendUDP(conn *net.UDPConn, msg udpNotification) {
	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)
}
