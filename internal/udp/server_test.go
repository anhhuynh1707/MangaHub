package udp

import (
	"testing"
	"time"
)

func TestNotificationServerStopReturnsFromStart(t *testing.T) {
	server := NewNotificationServer("0")
	result := make(chan error, 1)
	go func() {
		result <- server.Start()
	}()

	deadline := time.After(2 * time.Second)
	for server.connection() == nil {
		select {
		case err := <-result:
			t.Fatalf("Start returned before the server was stopped: %v", err)
		case <-deadline:
			t.Fatal("timed out waiting for UDP server to start")
		case <-time.After(5 * time.Millisecond):
		}
	}

	server.Stop()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Start returned an error after a graceful stop: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop closed the UDP socket")
	}
}
