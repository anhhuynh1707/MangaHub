package auth

import (
	"testing"
	"time"
)

func TestGenerateToken_ReturnsNonEmptyString(t *testing.T) {
	token, err := GenerateToken("user-alice", "alice")
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken returned empty token")
	}
}

func TestValidateToken_ValidToken(t *testing.T) {
	token, err := GenerateToken("user-alice", "alice")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken returned error for valid token: %v", err)
	}
	if claims.UserID != "user-alice" {
		t.Errorf("UserID: want %q, got %q", "user-alice", claims.UserID)
	}
	if claims.Username != "alice" {
		t.Errorf("Username: want %q, got %q", "alice", claims.Username)
	}
	if claims.Issuer != "mangahub" {
		t.Errorf("Issuer: want %q, got %q", "mangahub", claims.Issuer)
	}
}

func TestValidateToken_InvalidString(t *testing.T) {
	_, err := ValidateToken("not.a.valid.token")
	if err == nil {
		t.Error("expected error for invalid token string, got nil")
	}
}

func TestValidateToken_EmptyString(t *testing.T) {
	_, err := ValidateToken("")
	if err == nil {
		t.Error("expected error for empty token string, got nil")
	}
}

func TestValidateToken_TamperedSignature(t *testing.T) {
	token, err := GenerateToken("user-bob", "bob")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	tampered := token[:len(token)-4] + "XXXX"
	_, err = ValidateToken(tampered)
	if err == nil {
		t.Error("expected error for tampered signature, got nil")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	original := JWTSecret
	defer func() { JWTSecret = original }()

	JWTSecret = []byte("secret-A")
	token, err := GenerateToken("user-carol", "carol")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	JWTSecret = []byte("secret-B")
	_, err = ValidateToken(token)
	if err == nil {
		t.Error("expected error when validating with wrong secret, got nil")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	original := TokenExpiry
	defer func() { TokenExpiry = original }()

	TokenExpiry = -time.Second // generate a token already in the past
	token, err := GenerateToken("user-dave", "dave")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestGenerateToken_DifferentUsersProduceDifferentTokens(t *testing.T) {
	t1, _ := GenerateToken("user-a", "alice")
	t2, _ := GenerateToken("user-b", "bob")
	if t1 == t2 {
		t.Error("tokens for different users should not be equal")
	}
}
