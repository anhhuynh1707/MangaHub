package sanitize

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// MaxLengths defines the character limits for each field type.
const (
	MaxUsernameLen    = 50
	MaxPasswordLen    = 100
	MaxMangaTitleLen  = 200
	MaxMangaAuthorLen = 100
	MaxMangaDescLen   = 2000
	MaxMangaIDLen     = 100
	MaxReviewTextLen  = 2000
	MaxChatMessageLen = 500
)

// Text trims surrounding whitespace, rejects HTML tag characters, and enforces
// a maximum byte length. Returns the cleaned string or an error.
//
// Use for free-form text fields: manga title, author, description, review body.
// Pass maxLen=0 to skip the length check.
func Text(s string, maxLen int) (string, error) {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, "<>") {
		return "", errors.New("input must not contain < or > characters")
	}
	if maxLen > 0 && len(s) > maxLen {
		return "", fmt.Errorf("input exceeds maximum length of %d characters", maxLen)
	}
	return s, nil
}

// ID trims whitespace and rejects characters that are not alphanumeric,
// hyphens, or underscores — safe for use as a URL slug / database primary key.
func ID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return "", errors.New("ID must not be empty")
	}
	if len(s) > MaxMangaIDLen {
		return "", fmt.Errorf("ID exceeds maximum length of %d characters", MaxMangaIDLen)
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return "", fmt.Errorf("ID contains invalid character %q (only letters, digits, - and _ allowed)", r)
		}
	}
	return s, nil
}

// Username trims whitespace and rejects characters outside letters, digits,
// hyphens, and underscores. Length limits are already enforced by binding tags
// on RegisterRequest, so this just strips and validates the character set.
func Username(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return "", errors.New("username must not be empty")
	}
	if len(s) > MaxUsernameLen {
		return "", fmt.Errorf("username exceeds maximum length of %d characters", MaxUsernameLen)
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return "", fmt.Errorf("username contains invalid character %q", r)
		}
	}
	return s, nil
}

// ChatMessage trims whitespace and enforces the chat message length limit.
// It does not reject HTML characters because the React frontend renders via
// JSX (which escapes by default), preventing XSS in the browser.
func ChatMessage(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return "", errors.New("message must not be empty")
	}
	if len(s) > MaxChatMessageLen {
		return "", fmt.Errorf("message exceeds maximum length of %d characters", MaxChatMessageLen)
	}
	return s, nil
}
