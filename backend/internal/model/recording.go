package model

import "time"

// Recording 录音片段实体，audio_key 指向 MinIO 对象，status 承载状态机流转。
// VersionID 指向录音时引用的、已审核通过的提纲版本；QuestionSnapshot 保留当时的问题内容，
// 即使后续提纲调整产生新版本，历史录音展示的问题文本保持不变。
type Recording struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ProjectID        uint      `gorm:"index;not null" json:"project_id"`
	QuestionID       uint      `gorm:"index;not null" json:"question_id"`
	VersionID        uint      `gorm:"index;not null;default:0" json:"version_id"`
	QuestionSnapshot string    `gorm:"size:512;not null;default:''" json:"question_snapshot"`
	AudioKey         string    `gorm:"size:255" json:"audio_key"`
	DurationSeconds  int       `gorm:"not null;default:0" json:"duration_seconds"`
	Summary          string    `gorm:"size:512" json:"summary"`
	Status           string    `gorm:"size:32;not null;default:recording" json:"status"`
	CreatedBy        uint      `gorm:"not null" json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (Recording) TableName() string { return "recordings" }
