// Package database 负责 GORM 迁移与旧版本数据升级。
package database

import (
	"fmt"

	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/model"
	"gorm.io/gorm"
)

// migrateModels 版本化后的全部表结构。GORM 配置已关闭外键约束创建（见 database.go），
// 因此对旧库 ADD COLUMN version_id（默认 0）不会因引用不存在的版本而触发 errno 1452。
func migrateModels(db *gorm.DB) error {
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
	return nil
}

// Migrate 执行表结构迁移并把版本化改造前的历史问题/录音挂载到首版提纲。
func Migrate(db *gorm.DB) error {
	if err := migrateModels(db); err != nil {
		return err
	}
	if err := backfillOutlineVersions(db); err != nil {
		return fmt.Errorf("backfill outline versions: %w", err)
	}
	if err := backfillQuestionSnapshots(db); err != nil {
		return fmt.Errorf("backfill question snapshots: %w", err)
	}
	return nil
}

// backfillOutlineVersions 为版本化改造前的项目补齐首版提纲。
//
// 升级规则（幂等，可重复执行）：
//   - 已有提纲版本的项目跳过，不动任何历史数据；
//   - 没有任何旧问题/旧录音的项目补一个 draft v1（空提纲，符合新流程）；
//   - 含历史问题或历史录音的项目补一个 approved v1，并把 version_id=0 的旧问题、
//     旧录音整体挂到 v1。approved 保证“录音只能引用已通过版本”的不变量对历史数据成立；
//   - 旧问题 content、sort_order 原样保留，不覆盖、不删除；
//   - 每个项目一个事务，失败回滚，不影响其他项目。
func backfillOutlineVersions(db *gorm.DB) error {
	var projectIDs []uint
	if err := db.Model(&model.Project{}).Pluck("id", &projectIDs).Error; err != nil {
		return fmt.Errorf("list project ids: %w", err)
	}
	for _, projectID := range projectIDs {
		if err := backfillOneProject(db, projectID); err != nil {
			return err
		}
	}
	return nil
}

func backfillOneProject(db *gorm.DB, projectID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var versionCount int64
		if err := tx.Model(&model.OutlineVersion{}).
			Where("project_id = ?", projectID).Count(&versionCount).Error; err != nil {
			return fmt.Errorf("count outline versions of project %d: %w", projectID, err)
		}
		if versionCount > 0 {
			// 已升级过，幂等跳过。
			return nil
		}

		var legacyQuestionCount int64
		if err := tx.Model(&model.Question{}).
			Where("project_id = ? AND version_id = 0", projectID).Count(&legacyQuestionCount).Error; err != nil {
			return fmt.Errorf("count legacy questions of project %d: %w", projectID, err)
		}
		var legacyRecordingCount int64
		if err := tx.Model(&model.Recording{}).
			Where("project_id = ? AND version_id = 0", projectID).Count(&legacyRecordingCount).Error; err != nil {
			return fmt.Errorf("count legacy recordings of project %d: %w", projectID, err)
		}

		// 含历史问题或录音：首版直接置为已通过并锁定，旧录音保持可引用；否则为待编辑草稿。
		status := constants.OutlineStatusDraft
		if legacyQuestionCount > 0 || legacyRecordingCount > 0 {
			status = constants.OutlineStatusApproved
		}
		version := &model.OutlineVersion{
			ProjectID:     projectID,
			VersionNumber: 1,
			Status:        status,
		}
		if err := tx.Create(version).Error; err != nil {
			return fmt.Errorf("create backfill outline version for project %d: %w", projectID, err)
		}

		if legacyQuestionCount > 0 {
			if err := tx.Model(&model.Question{}).
				Where("project_id = ? AND version_id = 0", projectID).
				Update("version_id", version.ID).Error; err != nil {
				return fmt.Errorf("attach legacy questions of project %d: %w", projectID, err)
			}
		}
		if legacyRecordingCount > 0 {
			if err := tx.Model(&model.Recording{}).
				Where("project_id = ? AND version_id = 0", projectID).
				Update("version_id", version.ID).Error; err != nil {
				return fmt.Errorf("attach legacy recordings of project %d: %w", projectID, err)
			}
		}
		return nil
	})
}

// backfillQuestionSnapshots 为历史录音补齐“当时问题内容”快照：
// 以同项目的问题当前内容为准（升级前问题不可编辑历史，内容即录音时内容）。
// 仅填充空快照；已有快照与找不到对应问题的录音保持原样。幂等可重复执行。
func backfillQuestionSnapshots(db *gorm.DB) error {
	if err := db.Exec(`
UPDATE recordings r
JOIN questions q ON q.id = r.question_id AND q.project_id = r.project_id
SET r.question_snapshot = q.content
WHERE r.question_snapshot = '' AND q.content <> ''`).Error; err != nil {
		return fmt.Errorf("fill recording question snapshots: %w", err)
	}
	return nil
}
