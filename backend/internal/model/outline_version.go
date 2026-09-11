package model

import "time"

// OutlineVersion 访谈提纲版本实体。一个采访项目可有多个版本，审核通过后版本锁定不可修改。
type OutlineVersion struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ProjectID      uint       `gorm:"index:idx_outline_project_version,unique,priority:1;not null" json:"project_id"`
	VersionNumber  int        `gorm:"index:idx_outline_project_version,unique,priority:2;not null;default:1" json:"version_number"`
	Status         string     `gorm:"size:32;not null;default:draft;index:idx_outline_status" json:"status"`
	BasedOnVersion int        `gorm:"not null;default:0" json:"based_on_version"`
	CreatedBy      uint       `gorm:"not null" json:"created_by"`
	SubmittedBy    uint       `gorm:"not null;default:0" json:"submitted_by"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	ReviewedBy     uint       `gorm:"not null;default:0" json:"reviewed_by"`
	ReviewerName   string     `gorm:"size:64;not null;default:''" json:"reviewer_name"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	RejectReason   string     `gorm:"size:512;not null;default:''" json:"reject_reason"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Questions      []Question `gorm:"foreignKey:VersionID" json:"questions,omitempty"`
}

// TableName 指定表名。
func (OutlineVersion) TableName() string { return "outline_versions" }
