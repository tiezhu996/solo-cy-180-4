// Package database 负责 GORM 初始化与迁移。
package database

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/oralhistory/oralhistory/internal/config"
	"github.com/oralhistory/oralhistory/internal/model"
	"github.com/oralhistory/oralhistory/internal/util"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// New 建立 MySQL 连接并自动迁移表结构。
func New(cfg *config.Config, logger *slog.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		// 不在数据库层创建外键约束：基线 SQL 仅使用索引（见 migrations/001_init.sql），
		// 且旧库新增 version_id 列默认值 0、回填提纲版本发生在 AutoMigrate 之后，
		// 若启用外键，ALTER TABLE 添加约束会因历史行引用不存在的版本而失败（errno 1452），导致服务启动退出。
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := Migrate(db); err != nil {
		return nil, err
	}
	logger.Info("database migrated", "host", cfg.DBHost, "db", cfg.DBName)
	return db, nil
}

// SeedAdmin 初始化默认管理员账号。
func SeedAdmin(db *gorm.DB, logger *slog.Logger) error {
	var count int64
	if err := db.Model(&model.User{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		return fmt.Errorf("count admin: %w", err)
	}
	if count > 0 {
		return nil
	}
	hash, err := util.HashPassword("admin123456")
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	admin := model.User{
		Username:     "admin",
		PasswordHash: hash,
		DisplayName:  "系统管理员",
		Email:        "admin@oralhistory.local",
		Role:         "admin",
	}
	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	logger.Info("admin seeded", "username", admin.Username)
	return nil
}
