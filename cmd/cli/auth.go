package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh/terminal"
)

func handleAuth(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub auth <register|login|logout|status|change-password>")
		return
	}

	switch args[0] {
	case "register":
		authRegister(args[1:])
	case "login":
		authLogin(args[1:])
	case "logout":
		authLogout()
	case "status":
		authStatus()
	case "change-password":
		authChangePassword()
	default:
		fmt.Printf("✗ Unknown auth command: '%s'\n", args[0])
		fmt.Println("Available: register, login, logout, status, change-password")
	}
}

func authRegister(args []string) {
	username := parseFlag(args, "username")
	if username == "" {
		fmt.Println("Usage: mangahub auth register --username <username>")
		return
	}

	// Prompt for password securely
	fmt.Print("Password: ")
	passBytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("✗ Failed to read password")
		return
	}
	password := strings.TrimSpace(string(passBytes))

	fmt.Print("Confirm password: ")
	confirmBytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("✗ Failed to read password confirmation")
		return
	}
	confirm := strings.TrimSpace(string(confirmBytes))

	if password != confirm {
		fmt.Println("✗ Passwords do not match")
		return
	}

	if len(password) < 6 {
		fmt.Println("✗ Registration failed: Password too weak")
		fmt.Println("  Password must be at least 6 characters")
		return
	}

	body := map[string]string{"username": username, "password": password}
	resp, err := apiRequest("POST", "/auth/register", body, "")
	if err != nil {
		fmt.Printf("✗ Registration failed: Server connection error\n  %v\n", err)
		fmt.Println("  Check server status: mangahub server status")
		return
	}

	if !resp.Success {
		fmt.Printf("✗ Registration failed: %s\n", resp.Error)
		if strings.Contains(resp.Error, "already") {
			fmt.Printf("  Try: mangahub auth login --username %s\n", username)
		}
		return
	}

	// Parse user data
	var user struct {
		ID        string    `json:"id"`
		Username  string    `json:"username"`
		CreatedAt time.Time `json:"created_at"`
	}
	json.Unmarshal(resp.Data, &user)

	fmt.Println("✓ Account created successfully!")
	fmt.Printf("  User ID:  %s\n", user.ID)
	fmt.Printf("  Username: %s\n", user.Username)
	fmt.Printf("  Created:  %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("Please login to start using MangaHub:")
	fmt.Printf("  mangahub auth login --username %s\n", username)
}

func authLogin(args []string) {
	username := parseFlag(args, "username")
	if username == "" {
		fmt.Println("Usage: mangahub auth login --username <username>")
		return
	}

	// Prompt for password securely
	fmt.Print("Password: ")
	passBytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("✗ Failed to read password")
		return
	}
	password := strings.TrimSpace(string(passBytes))

	body := map[string]string{"username": username, "password": password}
	resp, err := apiRequest("POST", "/auth/login", body, "")
	if err != nil {
		fmt.Printf("✗ Login failed: Server connection error\n  %v\n", err)
		fmt.Println("  Check server status: mangahub server status")
		return
	}

	if !resp.Success {
		fmt.Printf("✗ Login failed: %s\n", resp.Error)
		if strings.Contains(resp.Error, "invalid") {
			fmt.Println("  Check your username and password")
		}
		return
	}

	// Parse login response
	var loginResp struct {
		Token string `json:"token"`
		User  struct {
			ID        string    `json:"id"`
			Username  string    `json:"username"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"user"`
	}
	json.Unmarshal(resp.Data, &loginResp)

	// Save to config
	cfg := loadConfig()
	cfg.Token = loginResp.Token
	cfg.Username = loginResp.User.Username
	cfg.UserID = loginResp.User.ID
	cfg.LoginAt = time.Now().Format(time.RFC3339)
	saveConfig(cfg)

	fmt.Printf("✓ Login successful!\n")
	fmt.Printf("  Welcome back, %s!\n\n", loginResp.User.Username)
	fmt.Println("Session Details:")
	fmt.Printf("  User ID:  %s\n", loginResp.User.ID)
	fmt.Printf("  Profile:  %s\n", getProfileName())
	fmt.Printf("  Token expires: %s (24 hours)\n", time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("Ready to use MangaHub! Try:")
	fmt.Println("  mangahub manga search \"your favorite manga\"")
}

func authLogout() {
	cfg := loadConfig()
	if cfg.Token == "" {
		fmt.Println("✗ Not logged in")
		return
	}

	username := cfg.Username
	clearToken()
	fmt.Printf("✓ Logged out successfully!\n")
	fmt.Printf("  Goodbye, %s!\n", username)
}

func authStatus() {
	cfg := loadConfig()
	if cfg.Token == "" {
		fmt.Println("Authentication Status: Not logged in")
		fmt.Println()
		fmt.Println("Login with: mangahub auth login --username <username>")
		return
	}

	// Verify token is still valid
	resp, err := apiRequest("GET", "/auth/status", nil, cfg.Token)
	if err != nil || !resp.Success {
		fmt.Println("Authentication Status: Token expired or invalid")
		fmt.Println("  Please login again: mangahub auth login --username " + cfg.Username)
		clearToken()
		return
	}

	fmt.Println("Authentication Status: ✓ Logged in")
	fmt.Printf("  Username: %s\n", cfg.Username)
	fmt.Printf("  User ID:  %s\n", cfg.UserID)
	fmt.Printf("  Profile:  %s\n", getProfileName())
	if cfg.LoginAt != "" {
		t, _ := time.Parse(time.RFC3339, cfg.LoginAt)
		fmt.Printf("  Logged in: %s\n", t.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("  Server:   %s\n", cfg.ServerURL)
}

func authChangePassword() {
	cfg := requireAuth()

	fmt.Print("Current password: ")
	oldBytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("✗ Failed to read password")
		return
	}

	fmt.Print("New password: ")
	newBytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("✗ Failed to read password")
		return
	}

	fmt.Print("Confirm new password: ")
	confirmBytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("✗ Failed to read password")
		return
	}

	if string(newBytes) != string(confirmBytes) {
		fmt.Println("✗ New passwords do not match")
		return
	}

	body := map[string]string{
		"old_password": strings.TrimSpace(string(oldBytes)),
		"new_password": strings.TrimSpace(string(newBytes)),
	}
	resp, err := apiRequest("PUT", "/auth/change-password", body, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to change password: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Println("✓ Password changed successfully!")
}
