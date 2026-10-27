package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
)

// Config holds all gateway configuration parameters.
type Config struct {
	Port           string
	MetricsPort    string
	DatabaseURL    string
	MasterKey      []byte // 32 bytes for AES-256-GCM
	SessionKey     string
	AdminEmail     string
	AdminPassword  string
	StoreTimezone  string
	OllamaBaseURL  string
	LLMProvider    string // "mock", "openai", "opencode", "ollama"
	LLMBaseURL     string
	LLMAPIKey      string
	LLMModel       string
}

// Load loads configuration from environment variables with safe defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		MetricsPort:   getEnv("METRICS_PORT", "9090"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/gateway?sslmode=disable"),
		SessionKey:    getEnv("SESSION_KEY", "urumi-session-secret-key-32bytes!"),
		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@localtest.me"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "AdminPass123!"),
		StoreTimezone: getEnv("STORE_TIMEZONE", "UTC"),
		LLMProvider:   getEnv("LLM_PROVIDER", "mock"),
		LLMBaseURL:    getEnv("LLM_BASE_URL", "https://opencode.ai/zen/go/v1"),
		LLMAPIKey:     getEnv("LLM_API_KEY", ""),
		LLMModel:      getEnv("LLM_MODEL", "deepseek-v4-flash"),
	}

	// Master key: read from file path if specified, else from env hex, else random
	masterKeyFile := os.Getenv("MASTER_KEY_FILE")
	if masterKeyFile != "" {
		if data, err := os.ReadFile(masterKeyFile); err == nil && len(data) >= 32 {
			cfg.MasterKey = data[:32]
		}
	}
	if len(cfg.MasterKey) == 0 {
		masterKeyHex := os.Getenv("MASTER_KEY_HEX")
		if masterKeyHex != "" {
			if b, err := hex.DecodeString(masterKeyHex); err == nil && len(b) >= 32 {
				cfg.MasterKey = b[:32]
			}
		}
	}
	if len(cfg.MasterKey) == 0 {
		// Fallback deterministic default or random for dev
		devKey := make([]byte, 32)
		copy(devKey, []byte("urumi-master-encryption-key-32b!"))
		cfg.MasterKey = devKey
	}

	return cfg, nil
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func getEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return def
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
