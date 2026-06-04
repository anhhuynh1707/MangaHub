package tcp

import (
	"testing"
)

func TestEncodeMessage_EndsWithNewline(t *testing.T) {
	msg := TCPMessage{Type: "ping"}
	data, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("EncodeMessage failed: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("encoded message must end with newline")
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	original := TCPMessage{
		Type:     "progress",
		UserID:   "user-alice",
		MangaID:  "one-piece",
		Chapter:  1095,
		DeviceID: "cli-laptop",
	}

	encoded, err := EncodeMessage(original)
	if err != nil {
		t.Fatalf("EncodeMessage failed: %v", err)
	}

	// DecodeMessage receives the line without the trailing newline
	decoded, err := DecodeMessage(encoded[:len(encoded)-1])
	if err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type: want %q, got %q", original.Type, decoded.Type)
	}
	if decoded.UserID != original.UserID {
		t.Errorf("UserID: want %q, got %q", original.UserID, decoded.UserID)
	}
	if decoded.MangaID != original.MangaID {
		t.Errorf("MangaID: want %q, got %q", original.MangaID, decoded.MangaID)
	}
	if decoded.Chapter != original.Chapter {
		t.Errorf("Chapter: want %d, got %d", original.Chapter, decoded.Chapter)
	}
	if decoded.DeviceID != original.DeviceID {
		t.Errorf("DeviceID: want %q, got %q", original.DeviceID, decoded.DeviceID)
	}
}

func TestDecodeMessage_InvalidJSON(t *testing.T) {
	_, err := DecodeMessage([]byte("not valid json {{"))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestDecodeMessage_EmptyObject(t *testing.T) {
	msg, err := DecodeMessage([]byte("{}"))
	if err != nil {
		t.Fatalf("DecodeMessage failed on empty object: %v", err)
	}
	if msg.Type != "" {
		t.Errorf("expected empty Type for {}, got %q", msg.Type)
	}
}

func TestNewWelcomeMessage(t *testing.T) {
	msg := NewWelcomeMessage()
	if msg.Type != "welcome" {
		t.Errorf("Type: want %q, got %q", "welcome", msg.Type)
	}
	if msg.Message == "" {
		t.Error("Message should be non-empty")
	}
	if msg.Timestamp == 0 {
		t.Error("Timestamp should be non-zero")
	}
}

func TestNewErrorMessage(t *testing.T) {
	msg := NewErrorMessage("something failed")
	if msg.Type != "error" {
		t.Errorf("Type: want %q, got %q", "error", msg.Type)
	}
	if msg.Message != "something failed" {
		t.Errorf("Message: want %q, got %q", "something failed", msg.Message)
	}
	if msg.Timestamp == 0 {
		t.Error("Timestamp should be non-zero")
	}
}

func TestNewBroadcastMessage(t *testing.T) {
	msg := NewBroadcastMessage("user-alice", "naruto", 700)
	if msg.Type != "broadcast" {
		t.Errorf("Type: want %q, got %q", "broadcast", msg.Type)
	}
	if msg.UserID != "user-alice" {
		t.Errorf("UserID: want %q, got %q", "user-alice", msg.UserID)
	}
	if msg.MangaID != "naruto" {
		t.Errorf("MangaID: want %q, got %q", "naruto", msg.MangaID)
	}
	if msg.Chapter != 700 {
		t.Errorf("Chapter: want %d, got %d", 700, msg.Chapter)
	}
	if msg.Message == "" {
		t.Error("broadcast Message should be non-empty")
	}
	if msg.Timestamp == 0 {
		t.Error("Timestamp should be non-zero")
	}
}

func TestNewStatusMessage(t *testing.T) {
	msg := NewStatusMessage(7, "server healthy")
	if msg.Type != "status" {
		t.Errorf("Type: want %q, got %q", "status", msg.Type)
	}
	if msg.ConnectedUsers != 7 {
		t.Errorf("ConnectedUsers: want %d, got %d", 7, msg.ConnectedUsers)
	}
	if msg.Message != "server healthy" {
		t.Errorf("Message: want %q, got %q", "server healthy", msg.Message)
	}
}

func TestEncodeMessage_OmitsEmptyFields(t *testing.T) {
	// omitempty fields should not appear in JSON when zero/empty
	msg := TCPMessage{Type: "ping"}
	data, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("EncodeMessage failed: %v", err)
	}
	json := string(data)
	if contains(json, `"user_id"`) {
		t.Error("empty user_id should be omitted from JSON")
	}
	if contains(json, `"manga_id"`) {
		t.Error("empty manga_id should be omitted from JSON")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
