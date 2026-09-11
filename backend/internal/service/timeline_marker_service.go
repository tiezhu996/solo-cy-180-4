package service

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/dto"
	"github.com/oralhistory/oralhistory/internal/model"
	"github.com/oralhistory/oralhistory/internal/repository"
	"github.com/oralhistory/oralhistory/internal/util"
)

// TimelineMarkerService 时间轴节点业务接口。
type TimelineMarkerService interface {
	Create(actor *model.User, req *dto.CreateTimelineMarkerRequest) (*model.TimelineMarker, error)
	// List 同时服务「按项目」与「按录音」两个接口，复用同一 service 方法。
	List(projectID, recordingID uint) ([]model.TimelineMarker, error)
	Update(actor *model.User, id uint, req *dto.UpdateTimelineMarkerRequest) (*model.TimelineMarker, error)
	Delete(actor *model.User, id uint) error
}

type timelineMarkerService struct {
	markerRepo    repository.TimelineMarkerRepository
	projectRepo   repository.ProjectRepository
	recordingRepo repository.RecordingRepository
	logger        *slog.Logger
}

// NewTimelineMarkerService 构造时间轴节点服务。
func NewTimelineMarkerService(markerRepo repository.TimelineMarkerRepository, projectRepo repository.ProjectRepository, recordingRepo repository.RecordingRepository, logger *slog.Logger) TimelineMarkerService {
	return &timelineMarkerService{markerRepo: markerRepo, projectRepo: projectRepo, recordingRepo: recordingRepo, logger: logger}
}

func (s *timelineMarkerService) Create(actor *model.User, req *dto.CreateTimelineMarkerRequest) (*model.TimelineMarker, error) {
	project, err := s.projectRepo.FindByID(req.ProjectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("项目 %d 不存在", req.ProjectID), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询项目 %d 失败", req.ProjectID), err)
	}
	if project.Status == constants.ProjectStatusArchived {
		return nil, util.NewAppError(constants.CodeProjectArchived,
			fmt.Sprintf("项目 %d 已归档，归档后禁止标注时间轴节点", req.ProjectID), nil)
	}
	recording, err := s.recordingRepo.FindByID(req.RecordingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", req.RecordingID), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", req.RecordingID), err)
	}
	// 录音必须属于当前项目，跨项目标注节点一律拒绝。
	if recording.ProjectID != req.ProjectID {
		return nil, util.NewAppError(constants.CodeCrossProjectRef,
			fmt.Sprintf("录音 %d 属于项目 %d，不能在项目 %d 下标注节点，跨项目引用被拒绝",
				recording.ID, recording.ProjectID, req.ProjectID), nil)
	}
	marker := &model.TimelineMarker{
		ProjectID:       req.ProjectID,
		RecordingID:     req.RecordingID,
		TimestampSecond: req.TimestampSecond,
		Label:           req.Label,
		Note:            req.Note,
		CreatedBy:       actor.ID,
	}
	if err := s.markerRepo.Create(marker); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("标注录音 %d 时间轴节点失败", req.RecordingID), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogMarkerCreate, actor.Username, marker.ProjectID, marker.RecordingID, marker.TimestampSecond, marker.Label))
	return marker, nil
}

func (s *timelineMarkerService) List(projectID, recordingID uint) ([]model.TimelineMarker, error) {
	var (
		markers []model.TimelineMarker
		err     error
	)
	if projectID > 0 {
		markers, err = s.markerRepo.ListByProject(projectID)
	} else {
		markers, err = s.markerRepo.ListByRecording(recordingID)
	}
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternal, "时间轴节点查询失败", err)
	}
	return markers, nil
}

func (s *timelineMarkerService) Update(actor *model.User, id uint, req *dto.UpdateTimelineMarkerRequest) (*model.TimelineMarker, error) {
	marker, err := s.markerRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("时间轴节点 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询时间轴节点 %d 失败", id), err)
	}
	project, err := s.projectRepo.FindByID(marker.ProjectID)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询项目 %d 失败", marker.ProjectID), err)
	}
	if project.Status == constants.ProjectStatusArchived {
		return nil, util.NewAppError(constants.CodeProjectArchived,
			fmt.Sprintf("项目 %d 已归档，归档后禁止修改时间轴节点", project.ID), nil)
	}
	if req.TimestampSecond != 0 {
		marker.TimestampSecond = req.TimestampSecond
	}
	if req.Label != "" {
		marker.Label = req.Label
	}
	if req.Note != "" {
		marker.Note = req.Note
	}
	if err := s.markerRepo.Update(marker); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("更新时间轴节点 %d 失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogMarkerUpdate, actor.Username, marker.ID, marker.Label))
	return marker, nil
}

func (s *timelineMarkerService) Delete(actor *model.User, id uint) error {
	marker, err := s.markerRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(constants.CodeNotFound, fmt.Sprintf("时间轴节点 %d 不存在", id), err)
		}
		return util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询时间轴节点 %d 失败", id), err)
	}
	project, err := s.projectRepo.FindByID(marker.ProjectID)
	if err != nil {
		return util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询项目 %d 失败", marker.ProjectID), err)
	}
	if project.Status == constants.ProjectStatusArchived {
		return util.NewAppError(constants.CodeProjectArchived,
			fmt.Sprintf("项目 %d 已归档，归档后禁止删除时间轴节点", project.ID), nil)
	}
	if err := s.markerRepo.Delete(id); err != nil {
		return util.NewAppError(constants.CodeInternal, fmt.Sprintf("删除时间轴节点 %d 失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogMarkerDelete, actor.Username, id))
	return nil
}
