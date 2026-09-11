package repository

import (
	"errors"
	"fmt"

	"github.com/oralhistory/oralhistory/internal/model"
	"gorm.io/gorm"
)

// QuestionRepository 采访问题数据访问接口。
type QuestionRepository interface {
	FindByID(id uint) (*model.Question, error)
	ListByProject(projectID uint) ([]model.Question, error)
	ListByVersion(versionID uint) ([]model.Question, error)
	// CountByVersion 统计提纲版本内的问题数量。
	CountByVersion(versionID uint) (int64, error)
}

type questionRepository struct {
	db *gorm.DB
}

// NewQuestionRepository 构造问题仓储。
func NewQuestionRepository(db *gorm.DB) QuestionRepository {
	return &questionRepository{db: db}
}

func (r *questionRepository) FindByID(id uint) (*model.Question, error) {
	var question model.Question
	if err := r.db.First(&question, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find question by id %d: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("find question by id: %w", err)
	}
	return &question, nil
}

func (r *questionRepository) ListByProject(projectID uint) ([]model.Question, error) {
	var questions []model.Question
	if err := r.db.Where("project_id = ?", projectID).Order("version_id ASC, sort_order ASC, id ASC").Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("list questions of project %d: %w", projectID, err)
	}
	return questions, nil
}

func (r *questionRepository) ListByVersion(versionID uint) ([]model.Question, error) {
	var questions []model.Question
	if err := r.db.Where("version_id = ?", versionID).Order("sort_order ASC, id ASC").Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("list questions of outline version %d: %w", versionID, err)
	}
	return questions, nil
}

func (r *questionRepository) CountByVersion(versionID uint) (int64, error) {
	var total int64
	if err := r.db.Model(&model.Question{}).Where("version_id = ?", versionID).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count questions of outline version %d: %w", versionID, err)
	}
	return total, nil
}
