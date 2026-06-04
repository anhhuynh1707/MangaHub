package udp

import (
	"fmt"
	"sync"
	"time"
)

// DeliveryRecord captures the result of a single broadcast with ACK tracking.
type DeliveryRecord struct {
	NotifID   string    `json:"notif_id"`
	Message   string    `json:"message"`
	SentAt    time.Time `json:"sent_at"`
	SentTo    []string  `json:"sent_to"`    // all addresses the notification was sent to
	AckedBy   []string  `json:"acked_by"`   // addresses that replied with ACK in time
	Unacked   []string  `json:"unacked"`    // addresses that did not ACK in time
	AckRate   float64   `json:"ack_rate"`   // 0.0 – 1.0
	TimedOut  bool      `json:"timed_out"`  // true if at least one client did not ACK
}

// pendingDelivery is the in-flight state for one broadcast.
type pendingDelivery struct {
	record *DeliveryRecord
	ackCh  chan string // receives ACK-ing client addresses
}

// AckTracker manages delivery confirmation for UDP broadcasts.
type AckTracker struct {
	mu      sync.Mutex
	pending map[string]*pendingDelivery // notif_id -> in-flight
	history []*DeliveryRecord           // last 50 completed records
}

// newAckTracker creates an AckTracker.
func newAckTracker() *AckTracker {
	return &AckTracker{
		pending: make(map[string]*pendingDelivery),
	}
}

// generateID returns a unique notification ID based on current time.
func generateID() string {
	return fmt.Sprintf("notif-%d", time.Now().UnixNano())
}

// track registers a new in-flight broadcast and returns its ID.
func (t *AckTracker) track(msg string, recipients []string) (string, *pendingDelivery) {
	id := generateID()
	addrs := make([]string, len(recipients))
	copy(addrs, recipients)

	pd := &pendingDelivery{
		record: &DeliveryRecord{
			NotifID: id,
			Message: msg,
			SentAt:  time.Now(),
			SentTo:  addrs,
			AckedBy: []string{},
			Unacked: []string{},
		},
		ackCh: make(chan string, len(recipients)+1),
	}

	t.mu.Lock()
	t.pending[id] = pd
	t.mu.Unlock()
	return id, pd
}

// RecordACK marks a client as having acknowledged a notification.
// Called from the server's message handler when it receives type="ack".
func (t *AckTracker) RecordACK(notifID, clientAddr string) {
	t.mu.Lock()
	pd, ok := t.pending[notifID]
	t.mu.Unlock()
	if !ok {
		return // already finalised or unknown ID
	}
	select {
	case pd.ackCh <- clientAddr:
	default:
	}
}

// finalise closes an in-flight delivery after its timeout, builds the report,
// and stores it in history.
func (t *AckTracker) finalise(id string, pd *pendingDelivery, timeout time.Duration) *DeliveryRecord {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	acked := make(map[string]bool)

	// Drain ACKs until timeout
	for {
		select {
		case addr := <-pd.ackCh:
			acked[addr] = true
		case <-timer.C:
			goto done
		}
	}

done:
	for _, addr := range pd.record.SentTo {
		if acked[addr] {
			pd.record.AckedBy = append(pd.record.AckedBy, addr)
		} else {
			pd.record.Unacked = append(pd.record.Unacked, addr)
			pd.record.TimedOut = true
		}
	}
	if len(pd.record.SentTo) > 0 {
		pd.record.AckRate = float64(len(pd.record.AckedBy)) / float64(len(pd.record.SentTo))
	} else {
		pd.record.AckRate = 1.0
	}

	t.mu.Lock()
	delete(t.pending, id)
	t.history = append(t.history, pd.record)
	// keep only last 50 records
	if len(t.history) > 50 {
		t.history = t.history[len(t.history)-50:]
	}
	t.mu.Unlock()

	return pd.record
}

// GetHistory returns a copy of recent delivery records.
func (t *AckTracker) GetHistory() []*DeliveryRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*DeliveryRecord, len(t.history))
	copy(out, t.history)
	return out
}
