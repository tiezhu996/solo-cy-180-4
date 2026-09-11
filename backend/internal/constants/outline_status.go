// 访谈提纲版本审核状态机枚举。
package constants

// 提纲版本状态定义。
const (
	OutlineStatusDraft     = "draft"     // 草稿：采访员保存改动后停留在此状态
	OutlineStatusSubmitted = "submitted" // 待审核：采访员已提交，等待档案员/管理员审核
	OutlineStatusApproved  = "approved"  // 已通过：审核通过并锁定，录音只能引用该状态版本
	OutlineStatusRejected  = "rejected"  // 已退回：审核退回，必须填写退回原因
)

// ValidOutlineStatus 校验提纲版本状态是否合法。
func ValidOutlineStatus(status string) bool {
	switch status {
	case OutlineStatusDraft, OutlineStatusSubmitted, OutlineStatusApproved, OutlineStatusRejected:
		return true
	default:
		return false
	}
}

// CanSubmitOutline 仅草稿/被退回的版本允许提交审核；待审核/已通过版本重复提交必须拒绝。
func CanSubmitOutline(status string) bool {
	return status == OutlineStatusDraft || status == OutlineStatusRejected
}

// CanReviewOutline 仅待审核版本允许通过/退回；重复审核（已通过/已退回/草稿）必须拒绝。
func CanReviewOutline(status string) bool {
	return status == OutlineStatusSubmitted
}

// CanEditOutline 仅草稿/被退回版本允许编辑问题；已提交待审与已通过锁定版本禁止改动。
func CanEditOutline(status string) bool {
	return status == OutlineStatusDraft || status == OutlineStatusRejected
}
