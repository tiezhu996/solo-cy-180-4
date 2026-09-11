package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/dto"
	"github.com/oralhistory/oralhistory/internal/model"
	"github.com/oralhistory/oralhistory/internal/repository"
	"github.com/oralhistory/oralhistory/internal/util"
)

// OutlineService 访谈提纲版本与审核业务接口。
type OutlineService interface {
	// CreateDraft 采访员保存改动前创建（或复用）项目的草稿版本；基于已通过版本调整时复制其问题。
	CreateDraft(actor *model.User, projectID uint, basedOnVersion uint) (*model.OutlineVersion, error)
	ListByProject(actor *model.User, projectID uint) ([]model.OutlineVersion, error)
	GetByProjectAndID(actor *model.User, projectID, versionID uint) (*model.OutlineVersion, error)
	// SaveQuestions 保存草稿改动（新增/更新问题），仅草稿/被退回版本可编辑。
	SaveQuestions(actor *model.User, projectID, versionID uint, req *dto.SaveOutlineDraftRequest) (*model.OutlineVersion, error)
	DeleteQuestion(actor *model.User, projectID, versionID, questionID uint) error
	// Submit 提交审核；重复提交（待审核/已通过）会被拒绝。
	Submit(actor *model.User, projectID, versionID uint) (*model.OutlineVersion, error)
	// ListPending 档案员/管理员待审核列表，可按项目过滤。
	ListPending(actor *model.User, page, pageSize int, projectID uint) ([]model.OutlineVersion, int64, error)
	// Review 审核通过/退回；退回必须填写原因；重复审核与越权角色会被拒绝。
	Review(actor *model.User, versionID uint, req *dto.ReviewOutlineRequest) (*model.OutlineVersion, error)
	// GetByID 供录音等模块校验提纲版本。
	GetByID(versionID uint) (*model.OutlineVersion, error)
}

type outlineService struct {
	outlineRepo repository.OutlineVersionRepository
	projectRepo repository.ProjectRepository
	logger      *slog.Logger
}

// NewOutlineService 构造提纲版本服务。
func NewOutlineService(outlineRepo repository.OutlineVersionRepository, projectRepo repository.ProjectRepository, logger *slog.Logger) OutlineService {
	return &outlineService{outlineRepo: outlineRepo, projectRepo: projectRepo, logger: logger}
}

// loadMutableProject 加载项目并统一拦截归档后操作。
func (s *outlineService) loadMutableProject(projectID uint) (*model.Project, error) {
	project, err := s.projectRepo.FindByID(projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("项目 %d 不存在", projectID), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询项目 %d 失败", projectID), err)
	}
	if project.Status == constants.ProjectStatusArchived {
		return nil, util.NewAppError(constants.CodeProjectArchived,
			fmt.Sprintf("项目 %d 已归档，归档后禁止修改提纲", projectID), nil)
	}
	return project, nil
}

func requireEditor(actor *model.User) error {
	if actor.Role != constants.RoleInterviewer && actor.Role != constants.RoleAdmin {
		return util.NewAppError(constants.CodeForbidden,
			fmt.Sprintf("角色 %s 无权编辑访谈提纲，仅采访员可以维护提纲", actor.Role), nil)
	}
	return nil
}

func requireReviewer(actor *model.User) error {
	if actor.Role != constants.RoleArchivist && actor.Role != constants.RoleAdmin {
		return util.NewAppError(constants.CodeForbidden,
			fmt.Sprintf("角色 %s 无权审核访谈提纲，仅档案员或管理员可以审核", actor.Role), nil)
	}
	return nil
}

func mapOutlineRepoError(err error, versionID uint) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return util.NewAppError(constants.CodeNotFound, fmt.Sprintf("提纲版本 %d 不存在", versionID), err)
	case errors.Is(err, repository.ErrConflict):
		return util.NewAppError(constants.CodeOutlineStatus,
			fmt.Sprintf("提纲版本 %d 当前状态不允许该操作（重复提交、重复审核或编辑已锁定版本）", versionID), err)
	case errors.Is(err, repository.ErrCrossProject):
		return util.NewAppError(constants.CodeCrossProjectRef,
			fmt.Sprintf("提纲版本 %d 跨项目引用被拒绝：基线版本必须属于同一采访项目且已审核通过", versionID), err)
	case errors.Is(err, repository.ErrValidation):
		return util.NewAppError(constants.CodeValidation,
			fmt.Sprintf("提纲版本 %d 中还没有任何问题，无法提交审核", versionID), err)
	default:
		return util.NewAppError(constants.CodeInternal, fmt.Sprintf("提纲版本 %d 操作失败", versionID), err)
	}
}

func (s *outlineService) CreateDraft(actor *model.User, projectID uint, basedOnVersion uint) (*model.OutlineVersion, error) {
	if err := requireEditor(actor); err != nil {
		return nil, err
	}
	if _, err := s.loadMutableProject(projectID); err != nil {
		return nil, err
	}
	version, err := s.outlineRepo.CreateDraft(projectID, basedOnVersion, actor.ID)
	if err != nil {
		return nil, mapOutlineRepoError(err, basedOnVersion)
	}
	s.logger.Info(fmt.Sprintf(constants.LogOutlineDraftCreate, actor.Username, projectID, version.VersionNumber, basedOnVersion))
	return version, nil
}

func (s *outlineService) ListByProject(actor *model.User, projectID uint) ([]model.OutlineVersion, error) {
	if _, err := s.projectRepo.FindByID(projectID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("项目 %d 不存在", projectID), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询项目 %d 失败", projectID), err)
	}
	versions, err := s.outlineRepo.ListByProject(projectID)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询项目 %d 提纲版本列表失败", projectID), err)
	}
	return versions, nil
}

func (s *outlineService) GetByProjectAndID(actor *model.User, projectID, versionID uint) (*model.OutlineVersion, error) {
	version, err := s.outlineRepo.FindByID(versionID)
	if err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	if version.ProjectID != projectID {
		return nil, util.NewAppError(constants.CodeCrossProjectRef,
			fmt.Sprintf("提纲版本 %d 不属于项目 %d，跨项目访问被拒绝", versionID, projectID), nil)
	}
	return version, nil
}

func (s *outlineService) GetByID(versionID uint) (*model.OutlineVersion, error) {
	version, err := s.outlineRepo.FindByID(versionID)
	if err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	return version, nil
}

func (s *outlineService) SaveQuestions(actor *model.User, projectID, versionID uint, req *dto.SaveOutlineDraftRequest) (*model.OutlineVersion, error) {
	if err := requireEditor(actor); err != nil {
		return nil, err
	}
	if _, err := s.loadMutableProject(projectID); err != nil {
		return nil, err
	}
	existing, err := s.outlineRepo.FindByID(versionID)
	if err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	if existing.ProjectID != projectID {
		return nil, util.NewAppError(constants.CodeCrossProjectRef,
			fmt.Sprintf("提纲版本 %d 不属于项目 %d，跨项目保存问题被拒绝", versionID, projectID), nil)
	}
	items := make([]repository.QuestionUpsert, 0, len(req.Questions))
	for _, q := range req.Questions {
		content := strings.TrimSpace(q.Content)
		if content == "" {
			return nil, util.NewAppError(constants.CodeValidation,
				fmt.Sprintf("提纲版本 %d 存在空白问题，问题内容不能为空", versionID), nil)
		}
		items = append(items, repository.QuestionUpsert{
			QuestionID: q.QuestionID,
			Content:    content,
			SortOrder:  q.SortOrder,
		})
	}
	version, err := s.outlineRepo.SaveQuestions(versionID, items)
	if err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	for _, item := range items {
		s.logger.Info(fmt.Sprintf(constants.LogOutlineQuestionSave, actor.Username, projectID, version.VersionNumber, item.QuestionID))
	}
	return version, nil
}

func (s *outlineService) DeleteQuestion(actor *model.User, projectID, versionID, questionID uint) error {
	if err := requireEditor(actor); err != nil {
		return err
	}
	if _, err := s.loadMutableProject(projectID); err != nil {
		return err
	}
	existing, err := s.outlineRepo.FindByID(versionID)
	if err != nil {
		return mapOutlineRepoError(err, versionID)
	}
	if existing.ProjectID != projectID {
		return util.NewAppError(constants.CodeCrossProjectRef,
			fmt.Sprintf("提纲版本 %d 不属于项目 %d，跨项目删除问题被拒绝", versionID, projectID), nil)
	}
	if err := s.outlineRepo.DeleteQuestion(versionID, questionID); err != nil {
		return mapOutlineRepoError(err, versionID)
	}
	s.logger.Info(fmt.Sprintf(constants.LogOutlineQuestionDel, actor.Username, projectID, existing.VersionNumber, questionID))
	return nil
}

func (s *outlineService) Submit(actor *model.User, projectID, versionID uint) (*model.OutlineVersion, error) {
	if err := requireEditor(actor); err != nil {
		return nil, err
	}
	if _, err := s.loadMutableProject(projectID); err != nil {
		return nil, err
	}
	existing, err := s.outlineRepo.FindByID(versionID)
	if err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	if existing.ProjectID != projectID {
		return nil, util.NewAppError(constants.CodeCrossProjectRef,
			fmt.Sprintf("提纲版本 %d 不属于项目 %d，跨项目提交被拒绝", versionID, projectID), nil)
	}
	from := existing.Status
	if !constants.CanSubmitOutline(from) {
		return nil, util.NewAppError(constants.CodeOutlineStatus,
			fmt.Sprintf("提纲版本 %d 当前为 %s 状态，重复提交被拒绝（仅草稿或被退回版本可以提交）", versionID,
				util.OutlineStatusText(from)), nil)
	}
	if err := s.outlineRepo.Submit(versionID, actor.ID); err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	version, err := s.outlineRepo.FindByID(versionID)
	if err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	s.logger.Info(fmt.Sprintf(constants.LogOutlineSubmit, actor.Username, projectID, version.VersionNumber, from))
	return version, nil
}

func (s *outlineService) ListPending(actor *model.User, page, pageSize int, projectID uint) ([]model.OutlineVersion, int64, error) {
	if err := requireReviewer(actor); err != nil {
		return nil, 0, err
	}
	versions, total, err := s.outlineRepo.ListPending(page, pageSize, projectID)
	if err != nil {
		return nil, 0, util.NewAppError(constants.CodeInternal, "待审核提纲列表查询失败", err)
	}
	return versions, total, nil
}

func (s *outlineService) Review(actor *model.User, versionID uint, req *dto.ReviewOutlineRequest) (*model.OutlineVersion, error) {
	if err := requireReviewer(actor); err != nil {
		// 越权审核：采访员或其他角色尝试审核，一律拒绝并审计。
		return nil, err
	}
	version, err := s.outlineRepo.FindByID(versionID)
	if err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	project, err := s.projectRepo.FindByID(version.ProjectID)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询项目 %d 失败", version.ProjectID), err)
	}
	if project.Status == constants.ProjectStatusArchived {
		return nil, util.NewAppError(constants.CodeProjectArchived,
			fmt.Sprintf("项目 %d 已归档，归档后禁止审核提纲", project.ID), nil)
	}
	status := constants.OutlineStatusApproved
	reason := ""
	if req.Action == "reject" {
		reason = strings.TrimSpace(req.Reason)
		if reason == "" {
			return nil, util.NewAppError(constants.CodeValidation,
				"退回提纲必须填写退回原因（reason 不能为空）", nil)
		}
		status = constants.OutlineStatusRejected
	}
	if !constants.CanReviewOutline(version.Status) {
		return nil, util.NewAppError(constants.CodeOutlineStatus,
			fmt.Sprintf("提纲版本 %d 当前为 %s 状态，重复审核被拒绝（仅待审核版本可以审核）", versionID,
				util.OutlineStatusText(version.Status)), nil)
	}
	if err := s.outlineRepo.Review(versionID, actor.ID, actor.Username, status, reason); err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	updated, err := s.outlineRepo.FindByID(versionID)
	if err != nil {
		return nil, mapOutlineRepoError(err, versionID)
	}
	if status == constants.OutlineStatusApproved {
		s.logger.Info(fmt.Sprintf(constants.LogOutlineApprove, actor.Username, version.ProjectID, version.VersionNumber, actor.Username))
	} else {
		s.logger.Info(fmt.Sprintf(constants.LogOutlineReject, actor.Username, version.ProjectID, version.VersionNumber, actor.Username, reason))
	}
	return updated, nil
}
