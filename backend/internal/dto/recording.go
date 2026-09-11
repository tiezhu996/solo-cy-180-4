package dto

// CreateRecordingRequest 创建录音记录请求。
// VersionID 必填：录音只能选择已审核通过的提纲版本；QuestionID 必须属于该版本与同一项目。
type CreateRecordingRequest struct {
	ProjectID       uint   `json:"project_id" binding:"required"`
	QuestionID      uint   `json:"question_id" binding:"required"`
	VersionID       uint   `json:"version_id" binding:"required"`
	DurationSeconds int    `json:"duration_seconds" binding:"omitempty,min=0"`
	Summary         string `json:"summary" binding:"omitempty,max=512"`
}

// UpdateRecordingRequest 更新录音信息请求。
type UpdateRecordingRequest struct {
	DurationSeconds int    `json:"duration_seconds" binding:"omitempty,min=0"`
	Summary         string `json:"summary" binding:"omitempty,max=512"`
	Status          string `json:"status" binding:"omitempty,oneof=recording processing ready failed"`
}

// RecordingResponse 录音响应。
type RecordingResponse struct {
	ID               uint   `json:"id"`
	ProjectID        uint   `json:"project_id"`
	QuestionID       uint   `json:"question_id"`
	VersionID        uint   `json:"version_id"`
	QuestionSnapshot string `json:"question_snapshot"`
	AudioKey         string `json:"audio_key"`
	DurationSeconds  int    `json:"duration_seconds"`
	Summary          string `json:"summary"`
	Status           string `json:"status"`
	CreatedBy        uint   `json:"created_by"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}
