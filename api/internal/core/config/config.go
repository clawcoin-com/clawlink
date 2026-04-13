package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	Port             string
	Env              string
	DatabaseURL      string
	JWTSecret        string
	JWTExpiryHours   int
	ClawCoinRPC      string
	ClawCoinChainID  int64
	TreasuryWallet   string
	TipContractAddr  string
	RateLimitRead    int
	RateLimitWrite   int
	UploadDir        string
	MaxUploadSizeMB  int64
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
	}

	return App
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
