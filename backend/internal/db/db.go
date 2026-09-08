package db

import (
	"fenx/backend/internal/config"
	"fenx/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
}

func Migrate(database *gorm.DB) error {
	return database.AutoMigrate(&models.PlatformUser{}, &models.Agent{}, &models.MonitorTask{}, &models.AuditEvent{}, &models.IPClaim{}, &models.SyncRun{}, &models.KKUDSnapshot{}, &models.UserMatch{}, &models.IPConflict{}, &models.ConflictMember{}, &models.ActionJob{}, &models.Allowlist{}, &models.SysConfig{})
}
