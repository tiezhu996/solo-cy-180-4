package model

import "time"

// Question 采访问题实体，隶属于采访项目与提纲版本。
// VersionID 为 0 表示版本化改造前的历史问题（照常保留）；提纲版本化后，问题随版本整体审核、锁定。
type Question struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"index;not null" json:"project_id"`
	VersionID uint      `gorm:"index;not null;default:0" json:"version_id"`
	Content   string    `gorm:"size:512;not null" json:"content"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (Question) TableName() string { return "questions" }
