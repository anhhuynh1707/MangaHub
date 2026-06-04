package sanitize

import (
	"strings"
	"testing"
)

// ── Text ────────────────────────────────────────────────────────

func TestText_AcceptsNormalInput(t *testing.T) {
	got, err := Text("One Piece", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "One Piece" {
		t.Errorf("want %q, got %q", "One Piece", got)
	}
}

func TestText_TrimsWhitespace(t *testing.T) {
	got, err := Text("  Naruto  ", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Naruto" {
		t.Errorf("want %q, got %q", "Naruto", got)
	}
}

func TestText_RejectsOpeningTag(t *testing.T) {
	_, err := Text("<script>alert(1)</script>", 200)
	if err == nil {
		t.Error("expected error for input containing <, got nil")
	}
}

func TestText_RejectsClosingAngle(t *testing.T) {
	_, err := Text("title > other", 200)
	if err == nil {
		t.Error("expected error for input containing >, got nil")
	}
}

func TestText_RejectsInputOverMaxLen(t *testing.T) {
	long := strings.Repeat("a", 201)
	_, err := Text(long, 200)
	if err == nil {
		t.Error("expected error for input exceeding max length, got nil")
	}
}

func TestText_AcceptsInputAtExactMaxLen(t *testing.T) {
	exact := strings.Repeat("a", 200)
	got, err := Text(exact, 200)
	if err != nil {
		t.Fatalf("unexpected error at exact max length: %v", err)
	}
	if len(got) != 200 {
		t.Errorf("expected length 200, got %d", len(got))
	}
}

func TestText_ZeroMaxLen_SkipsLengthCheck(t *testing.T) {
	long := strings.Repeat("a", 10000)
	_, err := Text(long, 0)
	if err != nil {
		t.Errorf("maxLen=0 should skip length check, got error: %v", err)
	}
}

func TestText_AllowsAmpersand(t *testing.T) {
	// Ampersands are valid in titles e.g. "Fullmetal Alchemist & Brotherhood"
	got, err := Text("Fullmetal Alchemist & Brotherhood", 200)
	if err != nil {
		t.Fatalf("unexpected error for & character: %v", err)
	}
	if got != "Fullmetal Alchemist & Brotherhood" {
		t.Errorf("want original string, got %q", got)
	}
}

// ── ID ──────────────────────────────────────────────────────────

func TestID_AcceptsValidSlug(t *testing.T) {
	got, err := ID("one-piece")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "one-piece" {
		t.Errorf("want %q, got %q", "one-piece", got)
	}
}

func TestID_AcceptsUnderscores(t *testing.T) {
	_, err := ID("attack_on_titan")
	if err != nil {
		t.Fatalf("unexpected error for underscores: %v", err)
	}
}

func TestID_AcceptsAlphanumeric(t *testing.T) {
	_, err := ID("manga123")
	if err != nil {
		t.Fatalf("unexpected error for alphanumeric: %v", err)
	}
}

func TestID_RejectsEmptyString(t *testing.T) {
	_, err := ID("")
	if err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

func TestID_RejectsSpecialChars(t *testing.T) {
	cases := []string{"one piece", "one<piece>", "id=1; DROP TABLE", "../etc/passwd"}
	for _, c := range cases {
		_, err := ID(c)
		if err == nil {
			t.Errorf("expected error for ID %q, got nil", c)
		}
	}
}

func TestID_RejectsTooLong(t *testing.T) {
	long := strings.Repeat("a", MaxMangaIDLen+1)
	_, err := ID(long)
	if err == nil {
		t.Error("expected error for ID exceeding max length, got nil")
	}
}

func TestID_TrimsWhitespace(t *testing.T) {
	got, err := ID("  bleach  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "bleach" {
		t.Errorf("want %q, got %q", "bleach", got)
	}
}

// ── Username ────────────────────────────────────────────────────

func TestUsername_AcceptsValidName(t *testing.T) {
	got, err := Username("alice_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alice_123" {
		t.Errorf("want %q, got %q", "alice_123", got)
	}
}

func TestUsername_RejectsHTMLChars(t *testing.T) {
	_, err := Username("<admin>")
	if err == nil {
		t.Error("expected error for HTML chars in username, got nil")
	}
}

func TestUsername_RejectsSpaces(t *testing.T) {
	_, err := Username("alice bob")
	if err == nil {
		t.Error("expected error for username with spaces, got nil")
	}
}

func TestUsername_RejectsEmpty(t *testing.T) {
	_, err := Username("")
	if err == nil {
		t.Error("expected error for empty username, got nil")
	}
}

func TestUsername_RejectsTooLong(t *testing.T) {
	_, err := Username(strings.Repeat("a", MaxUsernameLen+1))
	if err == nil {
		t.Error("expected error for username exceeding max length, got nil")
	}
}

// ── ChatMessage ─────────────────────────────────────────────────

func TestChatMessage_AcceptsNormalMessage(t *testing.T) {
	got, err := ChatMessage("Hello everyone!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Hello everyone!" {
		t.Errorf("want %q, got %q", "Hello everyone!", got)
	}
}

func TestChatMessage_TrimsWhitespace(t *testing.T) {
	got, err := ChatMessage("  hi  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hi" {
		t.Errorf("want %q, got %q", "hi", got)
	}
}

func TestChatMessage_RejectsEmpty(t *testing.T) {
	_, err := ChatMessage("")
	if err == nil {
		t.Error("expected error for empty chat message, got nil")
	}
}

func TestChatMessage_RejectsWhitespaceOnly(t *testing.T) {
	_, err := ChatMessage("   ")
	if err == nil {
		t.Error("expected error for whitespace-only message, got nil")
	}
}

func TestChatMessage_RejectsTooLong(t *testing.T) {
	_, err := ChatMessage(strings.Repeat("a", MaxChatMessageLen+1))
	if err == nil {
		t.Error("expected error for message exceeding max length, got nil")
	}
}

func TestChatMessage_AllowsHTMLChars(t *testing.T) {
	// Chat allows < > because the React frontend escapes by default
	_, err := ChatMessage("<b>bold</b>")
	if err != nil {
		t.Errorf("chat messages may contain < >; unexpected error: %v", err)
	}
}
