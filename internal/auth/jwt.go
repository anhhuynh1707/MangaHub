package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// JWTSecret is the secret key used for signing and validating JWT tokens.
// It has no default: call InitSecret at startup to load it from the JWT_SECRET
// environment variable. This prevents the app from ever running with a weak,
// publicly-known default key.
var JWTSecret []byte

// TokenExpiry is how long a JWT token remains valid.
var TokenExpiry = 24 * time.Hour

// InitSecret loads the JWT signing key from the JWT_SECRET environment variable.
// It returns an error if the secret is unset or shorter than 16 characters, so
// each server binary can fail fast instead of silently falling back to a default.
// Every service that signs or validates tokens must call this with the same secret.
func InitSecret() error {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 16 {
		return fmt.Errorf("JWT_SECRET must be set to at least 16 characters; refusing to start with a default key")
	}
	JWTSecret = []byte(secret)
	return nil
}

// Claims represents the custom JWT claims.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT token for a user.
func GenerateToken(userID, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mangahub",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ValidateToken parses and validates a JWT token string.
func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// AuthMiddleware is a Gin middleware that validates JWT tokens.
// It extracts the token from the Authorization header (Bearer <token>),
// validates it, and sets user info in the context.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.UnauthorizedResponse(c, "Authorization header is required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			utils.UnauthorizedResponse(c, "Authorization header must be in the format: Bearer <token>")
			c.Abort()
			return
		}

		claims, err := ValidateToken(parts[1])
		if err != nil {
			utils.UnauthorizedResponse(c, "Invalid or expired token")
			c.Abort()
			return
		}

		// Store user info in context for downstream handlers
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// GetUserIDFromContext extracts the user ID from the Gin context.
func GetUserIDFromContext(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", errors.New("user ID not found in context")
	}
	return userID.(string), nil
}

// GetUsernameFromContext extracts the username from the Gin context.
func GetUsernameFromContext(c *gin.Context) (string, error) {
	username, exists := c.Get("username")
	if !exists {
		return "", errors.New("username not found in context")
	}
	return username.(string), nil
}
