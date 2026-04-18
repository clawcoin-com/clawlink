package config

import (
	"log"
	"os"
	"strconv"
	"strings"

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
	CORSOrigins []string

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
//
// Precedence (highest wins):
//  1. Process environment variables (already exported / injected by docker-compose env_file)
//  2. .env file in the current working directory
//  3. Built-in defaults
//
// We use godotenv.Load (not Overload) so real env vars injected by the runtime
// take priority. If you want .env to override exported vars, use godotenv.Overload.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[config] No .env file in CWD, using process environment")
	} else {
		log.Println("[config] Loaded .env from CWD")
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
		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", defaultCORSOrigins(getEnv("ENV", "development"), getEnv("FRONTEND_URL", "http://localhost:3000")))),

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

	// Diagnostic: print the URLs the server will use for OAuth callbacks and
	// the frontend redirect. These are the values Google/Discord will receive
	// as redirect_uri. If they don't match your public deployment, fix .env
	// or docker-compose env_file before anything else.
	log.Printf("[config] ENV=%s", App.Env)
	log.Printf("[config] API_BASE_URL=%s  (used to build OAuth redirect_uri)", App.APIBaseURL)
	log.Printf("[config] FRONTEND_URL=%s  (used after OAuth for browser redirect)", App.FrontendURL)
	log.Printf("[config] CORS_ORIGINS=%v", App.CORSOrigins)
	log.Printf("[config] Google OAuth callback: %s/api/v1/auth/oauth/google/callback", App.APIBaseURL)
	log.Printf("[config] Discord OAuth callback: %s/api/v1/auth/oauth/discord/callback", App.APIBaseURL)

	return App
}

func defaultAPIBaseURL(env string) string {
	if env == "production" {
		return "https://api.clawlink.app"
	}
	return "http://localhost:8080"
}

func defaultCORSOrigins(env, frontendURL string) string {
	if env == "production" {
		return frontendURL
	}
	return "*"
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

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}
