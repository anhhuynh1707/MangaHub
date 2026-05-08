package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config stores CLI configuration (server URL, JWT token, etc.)
type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	Username  string `json:"username"`
	UserID    string `json:"user_id"`
	LoginAt   string `json:"login_at"`
}

// APIResponse is the standard API response format.
type APIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

var configDir string

func init() {
	home, _ := os.UserHomeDir()
	configDir = filepath.Join(home, ".mangahub")
}

// getProfileName returns the active profile name.
// Each terminal can set MANGAHUB_PROFILE=alice to use a different profile.
// If not set, it defaults to session_<ppid> for terminal isolation.
func getProfileName() string {
	if p := os.Getenv("MANGAHUB_PROFILE"); p != "" {
		return p
	}
	return "session_" + strconv.Itoa(os.Getppid())
}

// getConfigPath returns the config file path for the active profile.
func getConfigPath() string {
	return filepath.Join(configDir, "profiles", getProfileName()+".json")
}

// loadConfig reads the stored configuration for the current profile.
// Priority: MANGAHUB_TOKEN env var > profile config file
func loadConfig() *Config {
	cfg := &Config{ServerURL: "http://localhost:8080"}

	// Read from profile config file
	data, err := os.ReadFile(getConfigPath())
	if err == nil {
		json.Unmarshal(data, cfg)
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://localhost:8080"
	}

	// MANGAHUB_TOKEN env var overrides the stored token (per-terminal)
	if envToken := os.Getenv("MANGAHUB_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}

	return cfg
}

// saveConfig writes configuration to the profile-specific config file.
func saveConfig(cfg *Config) error {
	profileDir := filepath.Join(configDir, "profiles")
	os.MkdirAll(profileDir, 0755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(getConfigPath(), data, 0644)
}

// clearToken removes the stored token for the current profile.
func clearToken() {
	cfg := loadConfig()
	cfg.Token = ""
	cfg.Username = ""
	cfg.UserID = ""
	cfg.LoginAt = ""
	saveConfig(cfg)
}

// requireAuth checks if the user is logged in and returns the config.
func requireAuth() *Config {
	cfg := loadConfig()
	if cfg.Token == "" {
		fmt.Println("✗ Not logged in. Please login first:")
		fmt.Println("  mangahub auth login --username <username>")
		os.Exit(1)
	}
	return cfg
}

// apiRequest makes an HTTP request to the API server.
func apiRequest(method, path string, body interface{}, token string) (*APIResponse, error) {
	cfg := loadConfig()
	urlStr := cfg.ServerURL + path

	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("server connection error: %w", err)
	}
	defer resp.Body.Close()

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("invalid server response: %w", err)
	}
	return &apiResp, nil
}

// apiGet is a shortcut for authenticated GET requests.
func apiGet(path string) (*APIResponse, error) {
	cfg := requireAuth()
	return apiRequest("GET", path, nil, cfg.Token)
}

// apiPost is a shortcut for authenticated POST requests.
func apiPost(path string, body interface{}) (*APIResponse, error) {
	cfg := requireAuth()
	return apiRequest("POST", path, body, cfg.Token)
}

// apiPut is a shortcut for authenticated PUT requests.
func apiPut(path string, body interface{}) (*APIResponse, error) {
	cfg := requireAuth()
	return apiRequest("PUT", path, body, cfg.Token)
}

// apiDelete is a shortcut for authenticated DELETE requests.
func apiDelete(path string) (*APIResponse, error) {
	cfg := requireAuth()
	return apiRequest("DELETE", path, nil, cfg.Token)
}

// parseFlag extracts a flag value from args: --flag value
func parseFlag(args []string, flag string) string {
	for i, arg := range args {
		if arg == "--"+flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// hasFlag checks if a flag exists in args.
func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == "--"+flag || arg == "-"+flag {
			return true
		}
	}
	return false
}

// getPositionalArg gets a non-flag argument.
func getPositionalArg(args []string) string {
	for i, arg := range args {
		if arg[0] != '-' {
			// Make sure it's not a value for a flag
			if i > 0 && args[i-1][0] == '-' {
				continue
			}
			return arg
		}
	}
	return ""
}

// urlEncode encodes a string for use in URLs.
func urlEncode(s string) string {
	return url.QueryEscape(s)
}

// printTable prints a simple table with headers and rows.
func printTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print separator
	printSep := func() {
		fmt.Print("├")
		for i, w := range widths {
			fmt.Print(repeat("─", w+2))
			if i < len(widths)-1 {
				fmt.Print("┼")
			}
		}
		fmt.Println("┤")
	}

	printTopSep := func() {
		fmt.Print("┌")
		for i, w := range widths {
			fmt.Print(repeat("─", w+2))
			if i < len(widths)-1 {
				fmt.Print("┬")
			}
		}
		fmt.Println("┐")
	}

	printBottomSep := func() {
		fmt.Print("└")
		for i, w := range widths {
			fmt.Print(repeat("─", w+2))
			if i < len(widths)-1 {
				fmt.Print("┴")
			}
		}
		fmt.Println("┘")
	}

	printRow := func(cells []string) {
		fmt.Print("│")
		for i, w := range widths {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			fmt.Printf(" %-*s │", w, cell)
		}
		fmt.Println()
	}

	printTopSep()
	printRow(headers)
	printSep()
	for _, row := range rows {
		printRow(row)
	}
	printBottomSep()
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
