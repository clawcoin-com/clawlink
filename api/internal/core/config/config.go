package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	Port            string
	Env             string
	APIBaseURL      string
	DatabaseURL     string
	JWTSecret       string
	JWTExpiryHours  int
	ClawCoinRPC     string
	ClawCoinChainID int64
	TreasuryWallet  string
	TipContractAddr string
	RateLimitRead   int
	RateLimitWrite  int
	UploadDir       string
	MaxUploadSizeMB int64

	// Frontend base URL (used for OAuth callback redirects)
	FrontendURL string

	// Google OAuth2
	GoogleClientID     string
	GoogleClientSecret string

	// Discord OAuth2
	DiscordClientID     string
	DiscordClientSecret string

	// SMTP — empty SMTP_HOST = dev mode (verification URLs are logged, not emailed)
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

var App *Config

// Load reads configuration from environment variables (after loading .env).
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	App = &Config{
		Port:            getEnv("PORT", "8080"),
		Env:             getEnv("ENV", "development"),
		APIBaseURL:      getEnv("API_BASE_URL", defaultAPIBaseURL(getEnv("ENV", "development"))),
		DatabaseURL:     mustEnv("DATABASE_URL"),
		JWTSecret:       mustEnv("JWT_SECRET"),
		JWTExpiryHours:  getEnvInt("JWT_EXPIRY_HOURS", 360),
		ClawCoinRPC:     getEnv("CLAWCOIN_RPC", "https://evm-testnet.clawcoin.com"),
		ClawCoinChainID: int64(getEnvInt("CLAWCOIN_CHAIN_ID", 11111110)),
		TreasuryWallet:  getEnv("TREASURY_WALLET", ""),
		TipContractAddr: getEnv("TIP_CONTRACT_ADDRESS", ""),
		RateLimitRead:   getEnvInt("RATE_LIMIT_READ", 60),
		RateLimitWrite:  getEnvInt("RATE_LIMIT_WRITE", 30),
		UploadDir:       getEnv("UPLOAD_DIR", "./uploads"),
		MaxUploadSizeMB: int64(getEnvInt("MAX_UPLOAD_SIZE_MB", 5)),

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),

		DiscordClientID:     getEnv("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret: getEnv("DISCORD_CLIENT_SECRET", ""),

		SMTPHost: getEnv("SMTP_HOST", ""),
		SMTPPort: getEnvInt("SMTP_PORT", 587),
		SMTPUser: getEnv("SMTP_USER", ""),
		SMTPPass: getEnv("SMTP_PASS", ""),
		SMTPFrom: getEnv("SMTP_FROM", "noreply@clawlink.app"),
	}

	return App
}

func defaultAPIBaseURL(env string) string {
	if env == "production" {
		return "https://api.clawlink.app"
	}
	return "http://localhost:8080"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
