package dto

import "time"

// SaveOutlineQuestionRequest 保存（新建/更新）提纲草稿中的单个问题。
type SaveOutlineQuestionRequest struct {
	QuestionID uint   `json:"question_id" binding:"omitempty"` // 0 表示新增；非 0 表示更新版本内已有问题
	Content    string `json:"content" binding:"required,min=1,max=512"`
	SortOrder  int    `json:"sort_order" binding:"omitempty,min=0"`
}

// SaveOutlineDraftRequest 采访员保存提纲改动（草稿），仅在草稿/被退回版本上允许。
type SaveOutlineDraftRequest struct {
	Questions []SaveOutlineQuestionRequest `json:"questions" binding:"required,min=1,dive"`
}

// CreateOutlineVersionRequest 基于项目当前已通过版本（或空提纲）创建新的草稿版本。
// BaseVersionID 为已通过提纲版本的主键 ID（不是版本号），传 0 表示从空提纲开始。
type CreateOutlineVersionRequest struct {
	BaseVersionID uint `json:"base_version_id" binding:"omitempty,min=0"`
}

// ReviewOutlineRequest 档案员/管理员审核提纲；action=reject 时 reason 必填。
type ReviewOutlineRequest struct {
	Action string `json:"action" binding:"required,oneof=approve reject"`
	Reason string `json:"reason" binding:"omitempty,max=512"`
}

// OutlineQuestionResponse 提纲问题响应。
type OutlineQuestionResponse struct {
	ID        uint   `json:"id"`
	Content   string `json:"content"`
	SortOrder int    `json:"sort_order"`
}

// OutlineVersionResponse 提纲版本响应。
type OutlineVersionResponse struct {
	ID             uint                      `json:"id"`
	ProjectID      uint                      `json:"project_id"`
	VersionNumber  int                       `json:"version_number"`
	Status         string                    `json:"status"`
	BasedOnVersion int                       `json:"based_on_version"`
	CreatedBy      uint                      `json:"created_by"`
	SubmittedBy    uint                      `json:"submitted_by"`
	SubmittedAt    *time.Time                `json:"submitted_at"`
	ReviewedBy     uint                      `json:"reviewed_by"`
	ReviewerName   string                    `json:"reviewer_name"`
	ReviewedAt     *time.Time                `json:"reviewed_at"`
	RejectReason   string                    `json:"reject_reason"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
	Questions      []OutlineQuestionResponse `json:"questions,omitempty"`
}
