// Package database 负责 GORM 初始化与迁移。
package database

import (
	"fmt"

	"github.com/oralhistory/oralhistory/internal/model"
	"gorm.io/gorm"
)

// Migrate 执行表结构迁移。
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.User{},
		&model.Project{},
		&model.OutlineVersion{},
		&model.Question{},
		&model.Recording{},
		&model.TimelineMarker{},
		&model.AuditLog{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	if err := backfillOutlineVersions(db); err != nil {
		return fmt.Errorf("backfill outline versions: %w", err)
	}
	return nil
}

// backfillOutlineVersions 为版本化改造前的历史数据补齐提纲版本：
// 已有问题和录音照常保留；含历史问题的项目补一个「已通过」v1 并把问题/录音挂上去，
// 使旧录音仍满足「录音引用已通过版本」的不变量；空项目补一个草稿 v1。
func backfillOutlineVersions(db *gorm.DB) error {
	var projectIDs []uint
	if err := db.Model(&model.Project{}).Pluck("id", &projectIDs).Error; err != nil {
		return fmt.Errorf("list project ids: %w", err)
	}
	for _, projectID := range projectIDs {
		var count int64
		if err := db.Model(&model.OutlineVersion{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
			return fmt.Errorf("count outline versions of project %d: %w", projectID, err)
		}
		if count > 0 {
			continue
		}
		var questionCount int64
		if err := db.Model(&model.Question{}).Where("project_id = ? AND version_id = 0", projectID).Count(&questionCount).Error; err != nil {
			return fmt.Errorf("count legacy questions of project %d: %w", projectID, err)
		}
		status := "draft"
		if questionCount > 0 {
			status = "approved"
		}
		version := &model.OutlineVersion{
			ProjectID:      projectID,
			VersionNumber:  1,
			Status:         status,
			BasedOnVersion: 0,
		}
		if err := db.Create(version).Error; err != nil {
			return fmt.Errorf("create backfill outline version for project %d: %w", projectID, err)
		}
		if questionCount > 0 {
			if err := db.Model(&model.Question{}).
				Where("project_id = ? AND version_id = 0", projectID).
				Update("version_id", version.ID).Error; err != nil {
				return fmt.Errorf("attach legacy questions of project %d: %w", projectID, err)
			}
			if err := db.Model(&model.Recording{}).
				Where("project_id = ? AND version_id = 0", projectID).
				Updates(map[string]interface{}{"version_id": version.ID}).Error; err != nil {
				return fmt.Errorf("attach legacy recordings of project %d: %w", projectID, err)
			}
		}
	}

	// 录音问题内容快照兜底：历史录音按问题当前内容回填。
	var legacyRecordings []model.Recording
	if err := db.Where("question_snapshot = ''").Find(&legacyRecordings).Error; err != nil {
		return fmt.Errorf("list recordings without snapshot: %w", err)
	}
	for i := range legacyRecordings {
		r := legacyRecordings[i]
		var q model.Question
		if err := db.First(&q, r.QuestionID).Error; err != nil {
			continue
		}
		if err := db.Model(&model.Recording{}).Where("id = ?", r.ID).
			Update("question_snapshot", q.Content).Error; err != nil {
			return fmt.Errorf("backfill snapshot for recording %d: %w", r.ID, err)
		}
	}
	return nil
}
