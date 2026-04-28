package database

import (
	"log"

	"github.com/clawcoin-com/clawlink/internal/core/config"
	"github.com/clawcoin-com/clawlink/internal/core/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Connect initializes the PostgreSQL connection and runs auto-migrations.
func Connect() {
	cfg := config.App

	logLevel := logger.Silent
	if cfg.Env == "development" {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := migrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	DB = db
	log.Println("database connected and migrations applied")
}

// migrate runs GORM AutoMigrate for all core models.
// It also handles column type changes that AutoMigrate cannot do automatically.
func migrate(db *gorm.DB) error {
	// Make wallet_address nullable if it isn't already (idempotent ALTER).
	// AutoMigrate won't change NOT NULL → NULL on its own.
	db.Exec("ALTER TABLE users ALTER COLUMN wallet_address DROP NOT NULL")
	db.Exec("ALTER TABLE users ALTER COLUMN api_key_hash DROP NOT NULL")
	db.Exec("ALTER TABLE users ALTER COLUMN oauth_provider DROP NOT NULL")
	db.Exec("ALTER TABLE users ALTER COLUMN oauth_id DROP NOT NULL")
	db.Exec("UPDATE users SET api_key_hash = NULL WHERE api_key_hash = ''")
	db.Exec("UPDATE users SET oauth_provider = NULL WHERE oauth_provider = ''")
	db.Exec("UPDATE users SET oauth_id = NULL WHERE oauth_id = ''")

	return db.AutoMigrate(
		&models.User{},
		&models.Post{},
		&models.Reply{},
		&models.SubMolt{},
		&models.SubMoltMember{},
		&models.Tag{},
		&models.PostTag{},
		&models.Follow{},
		&models.Like{},
		&models.Notification{},
		&models.EventLog{},
		&models.RewardRule{},
	)
}
