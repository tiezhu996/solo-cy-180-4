package repository

import (
	"errors"
	"fmt"

	"github.com/oralhistory/oralhistory/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// QuestionUpsert 提纲问题保存入参：QuestionID 为 0 表示新增，否则更新版本内已有问题。
type QuestionUpsert struct {
	QuestionID uint
	Content    string
	SortOrder  int
}

// OutlineVersionRepository 访谈提纲版本数据访问接口。
type OutlineVersionRepository interface {
	Create(version *model.OutlineVersion) error
	FindByID(id uint) (*model.OutlineVersion, error)
	ListByProject(projectID uint) ([]model.OutlineVersion, error)
	LatestByProject(projectID uint) (*model.OutlineVersion, error)
	LatestApprovedByProject(projectID uint) (*model.OutlineVersion, error)
	ListPending(page, pageSize int, projectID uint) ([]model.OutlineVersion, int64, error)
	// CreateDraft 在事务内锁定项目行，基于指定版本（0 表示空提纲）复制出新草稿版本。
	CreateDraft(projectID, baseVersionID, actorID uint) (*model.OutlineVersion, error)
	// SaveQuestions 在事务内锁定版本行，校验版本可编辑后 upsert 问题；返回最新版本（含问题）。
	SaveQuestions(versionID uint, items []QuestionUpsert) (*model.OutlineVersion, error)
	// DeleteQuestion 在事务内锁定版本行，校验版本可编辑后删除版本内问题。
	DeleteQuestion(versionID, questionID uint) error
	// Submit 在事务内锁定版本行并执行提交状态机（重复提交返回 ErrConflict）。
	Submit(versionID, actorID uint) error
	// Review 在事务内锁定版本行并执行审核状态机（重复审核返回 ErrConflict）。
	Review(versionID, reviewerID uint, reviewerName, status, reason string) error
}

type outlineVersionRepository struct {
	db *gorm.DB
}

// NewOutlineVersionRepository 构造提纲版本仓储。
func NewOutlineVersionRepository(db *gorm.DB) OutlineVersionRepository {
	return &outlineVersionRepository{db: db}
}

func (r *outlineVersionRepository) Create(version *model.OutlineVersion) error {
	if err := r.db.Create(version).Error; err != nil {
		return fmt.Errorf("create outline version of project %d: %w", version.ProjectID, err)
	}
	return nil
}

func (r *outlineVersionRepository) FindByID(id uint) (*model.OutlineVersion, error) {
	var version model.OutlineVersion
	if err := r.db.Preload("Questions", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, id ASC")
	}).First(&version, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find outline version by id %d: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("find outline version by id: %w", err)
	}
	return &version, nil
}

func (r *outlineVersionRepository) ListByProject(projectID uint) ([]model.OutlineVersion, error) {
	var versions []model.OutlineVersion
	if err := r.db.Where("project_id = ?", projectID).Order("version_number DESC, id DESC").Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list outline versions of project %d: %w", projectID, err)
	}
	return versions, nil
}

func (r *outlineVersionRepository) LatestByProject(projectID uint) (*model.OutlineVersion, error) {
	var version model.OutlineVersion
	if err := r.db.Where("project_id = ?", projectID).Order("version_number DESC, id DESC").First(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find latest outline version of project %d: %w", projectID, ErrNotFound)
		}
		return nil, fmt.Errorf("find latest outline version: %w", err)
	}
	return &version, nil
}

func (r *outlineVersionRepository) LatestApprovedByProject(projectID uint) (*model.OutlineVersion, error) {
	var version model.OutlineVersion
	if err := r.db.Where("project_id = ? AND status = ?", projectID, "approved").
		Order("version_number DESC, id DESC").First(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find approved outline version of project %d: %w", projectID, ErrNotFound)
		}
		return nil, fmt.Errorf("find approved outline version: %w", err)
	}
	return &version, nil
}

func (r *outlineVersionRepository) ListPending(page, pageSize int, projectID uint) ([]model.OutlineVersion, int64, error) {
	var versions []model.OutlineVersion
	var total int64
	q := r.db.Model(&model.OutlineVersion{}).Where("status = ?", "submitted")
	if projectID > 0 {
		q = q.Where("project_id = ?", projectID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count pending outline versions: %w", err)
	}
	if err := q.Scopes(paginate(page, pageSize)).Order("submitted_at ASC, id ASC").Find(&versions).Error; err != nil {
		return nil, 0, fmt.Errorf("list pending outline versions: %w", err)
	}
	return versions, total, nil
}

func (r *outlineVersionRepository) CreateDraft(projectID, baseVersionID, actorID uint) (*model.OutlineVersion, error) {
	created := &model.OutlineVersion{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 锁定项目行，串行化同一项目的新版本创建，避免并发产生重复草稿/版本号。
		var project model.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, projectID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock project %d: %w", projectID, ErrNotFound)
			}
			return fmt.Errorf("lock project %d: %w", projectID, err)
		}
		var latest model.OutlineVersion
		findErr := tx.Where("project_id = ?", projectID).Order("version_number DESC, id DESC").First(&latest).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load latest outline version of project %d: %w", projectID, findErr)
		}
		nextNumber := 1
		if findErr == nil {
			switch latest.Status {
			case "draft", "rejected":
				// 已存在可编辑草稿，直接复用，不另建版本。
				*created = latest
				return nil
			case "submitted":
				// 上一版本仍在审核，禁止再开草稿。
				return ErrConflict
			}
			nextNumber = latest.VersionNumber + 1
		}
		draft := &model.OutlineVersion{
			ProjectID:      projectID,
			VersionNumber:  nextNumber,
			Status:         "draft",
			BasedOnVersion: 0,
			CreatedBy:      actorID,
		}
		if baseVersionID > 0 {
			var base model.OutlineVersion
			if err := tx.First(&base, baseVersionID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("load base outline version %d: %w", baseVersionID, ErrNotFound)
				}
				return fmt.Errorf("load base outline version %d: %w", baseVersionID, err)
			}
			// 跨项目引用基线版本必须拒绝。
			if base.ProjectID != projectID || base.Status != "approved" {
				return ErrCrossProject
			}
			draft.BasedOnVersion = base.VersionNumber
			if err := tx.Create(draft).Error; err != nil {
				return fmt.Errorf("create draft outline version: %w", err)
			}
			// 复制基线版本的问题到新草稿，历史版本与问题保持不变。
			if err := tx.Exec(
				"INSERT INTO questions (project_id, version_id, content, sort_order, created_at, updated_at) "+
					"SELECT project_id, ?, content, sort_order, NOW(3), NOW(3) FROM questions WHERE version_id = ?",
				draft.ID, base.ID).Error; err != nil {
				return fmt.Errorf("copy questions from version %d to %d: %w", base.ID, draft.ID, err)
			}
		} else {
			if err := tx.Create(draft).Error; err != nil {
				return fmt.Errorf("create draft outline version: %w", err)
			}
		}
		*created = *draft
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(created.ID)
}

func (r *outlineVersionRepository) SaveQuestions(versionID uint, items []QuestionUpsert) (*model.OutlineVersion, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var version model.OutlineVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&version, versionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock outline version %d: %w", versionID, ErrNotFound)
			}
			return fmt.Errorf("lock outline version %d: %w", versionID, err)
		}
		if version.Status != "draft" && version.Status != "rejected" {
			return ErrConflict
		}
		for _, item := range items {
			if item.QuestionID == 0 {
				q := model.Question{
					ProjectID: version.ProjectID,
					VersionID: version.ID,
					Content:   item.Content,
					SortOrder: item.SortOrder,
				}
				if err := tx.Create(&q).Error; err != nil {
					return fmt.Errorf("create question in outline version %d: %w", versionID, err)
				}
				continue
			}
			var q model.Question
			if err := tx.First(&q, item.QuestionID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("update question %d: %w", item.QuestionID, ErrNotFound)
				}
				return fmt.Errorf("update question %d: %w", item.QuestionID, err)
			}
			// 只能改动本版本内的问题，跨版本/跨项目引用一律拒绝。
			if q.VersionID != version.ID || q.ProjectID != version.ProjectID {
				return ErrCrossProject
			}
			q.Content = item.Content
			q.SortOrder = item.SortOrder
			if err := tx.Save(&q).Error; err != nil {
				return fmt.Errorf("save question %d: %w", item.QuestionID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(versionID)
}

func (r *outlineVersionRepository) DeleteQuestion(versionID, questionID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var version model.OutlineVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&version, versionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock outline version %d: %w", versionID, ErrNotFound)
			}
			return fmt.Errorf("lock outline version %d: %w", versionID, err)
		}
		if version.Status != "draft" && version.Status != "rejected" {
			return ErrConflict
		}
		result := tx.Where("id = ? AND version_id = ? AND project_id = ?", questionID, versionID, version.ProjectID).
			Delete(&model.Question{})
		if result.Error != nil {
			return fmt.Errorf("delete question %d of outline version %d: %w", questionID, versionID, result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("delete question %d of outline version %d: %w", questionID, versionID, ErrNotFound)
		}
		return nil
	})
}

func (r *outlineVersionRepository) Submit(versionID, actorID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var version model.OutlineVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&version, versionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock outline version %d: %w", versionID, ErrNotFound)
			}
			return fmt.Errorf("lock outline version %d: %w", versionID, err)
		}
		// 状态机：仅草稿/被退回可提交；待审核重复提交、已通过再次提交均拒绝。
		if version.Status != "draft" && version.Status != "rejected" {
			return ErrConflict
		}
		var count int64
		if err := tx.Model(&model.Question{}).Where("version_id = ?", versionID).Count(&count).Error; err != nil {
			return fmt.Errorf("count questions of outline version %d: %w", versionID, err)
		}
		if count == 0 {
			return ErrValidation
		}
		if err := tx.Model(&version).Updates(map[string]interface{}{
			"status":       "submitted",
			"submitted_by": actorID,
			"submitted_at": gorm.Expr("NOW(3)"),
		}).Error; err != nil {
			return fmt.Errorf("submit outline version %d: %w", versionID, err)
		}
		return nil
	})
}

func (r *outlineVersionRepository) Review(versionID, reviewerID uint, reviewerName, status, reason string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var version model.OutlineVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&version, versionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock outline version %d: %w", versionID, ErrNotFound)
			}
			return fmt.Errorf("lock outline version %d: %w", versionID, err)
		}
		// 状态机：仅待审核可审核，重复审核（已通过/已退回/草稿）一律拒绝。
		if version.Status != "submitted" {
			return ErrConflict
		}
		updates := map[string]interface{}{
			"status":        status,
			"reviewed_by":   reviewerID,
			"reviewer_name": reviewerName,
			"reviewed_at":   gorm.Expr("NOW(3)"),
		}
		if status == "rejected" {
			updates["reject_reason"] = reason
		}
		if err := tx.Model(&version).Updates(updates).Error; err != nil {
			return fmt.Errorf("review outline version %d: %w", versionID, err)
		}
		return nil
	})
}
