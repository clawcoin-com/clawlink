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
func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Post{},
		&models.Reply{},
		&models.SubMolt{},
		&models.SubMoltMember{},
		&models.Follow{},
		&models.Like{},
		&models.Notification{},
		&models.EventLog{},
		&models.RewardRule{},
	)
}
