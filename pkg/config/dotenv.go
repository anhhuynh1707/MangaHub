// Package config holds small shared startup helpers used by every server binary.
package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// LoadDotEnv reads a .env file and sets environment variables from it. Variables
// already present in the environment are NOT overwritten, so real env vars (e.g.
// those injected by docker-compose) always take precedence. The file is optional:
// if it does not exist, this is a no-op.
func LoadDotEnv(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return // .env is optional
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	log.Println("Loaded configuration from .env file")
}
