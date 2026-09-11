package service

import (
	"log/slog"
	"testing"

	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/dto"
	"github.com/oralhistory/oralhistory/internal/model"
	"github.com/oralhistory/oralhistory/internal/repository"
)

// fakeQuestionRepo 问题仓储内存实现。
type fakeQuestionRepo struct {
	questions map[uint]*model.Question
}

func (f *fakeQuestionRepo) FindByID(id uint) (*model.Question, error) {
	if q, ok := f.questions[id]; ok {
		cp := *q
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeQuestionRepo) ListByProject(projectID uint) ([]model.Question, error) {
	var out []model.Question
	for _, q := range f.questions {
		if q.ProjectID == projectID {
			out = append(out, *q)
		}
	}
	return out, nil
}

func (f *fakeQuestionRepo) ListByVersion(versionID uint) ([]model.Question, error) {
	var out []model.Question
	for _, q := range f.questions {
		if q.VersionID == versionID {
			out = append(out, *q)
		}
	}
	return out, nil
}

func (f *fakeQuestionRepo) CountByVersion(versionID uint) (int64, error) {
	var total int64
	for _, q := range f.questions {
		if q.VersionID == versionID {
			total++
		}
	}
	return total, nil
}

// fakeRecordingRepo 录音仓储内存实现。
type fakeRecordingRepo struct {
	recordings map[uint]*model.Recording
	nextID     uint
}

func newFakeRecordingRepo() *fakeRecordingRepo {
	return &fakeRecordingRepo{recordings: map[uint]*model.Recording{}, nextID: 1}
}

func (f *fakeRecordingRepo) Create(recording *model.Recording) error {
	if recording.ID == 0 {
		recording.ID = f.nextID
		f.nextID++
	}
	f.recordings[recording.ID] = recording
	return nil
}
func (f *fakeRecordingRepo) FindByID(id uint) (*model.Recording, error) {
	if r, ok := f.recordings[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeRecordingRepo) ListByProject(projectID uint) ([]model.Recording, error) {
	return nil, nil
}
func (f *fakeRecordingRepo) ListByQuestion(questionID uint) ([]model.Recording, error) {
	return nil, nil
}
func (f *fakeRecordingRepo) FindByIDForUpdate(id uint) (*model.Recording, error) {
	return f.FindByID(id)
}
func (f *fakeRecordingRepo) Update(recording *model.Recording) error {
	f.recordings[recording.ID] = recording
	return nil
}
func (f *fakeRecordingRepo) UpdateStatus(recording *model.Recording) error {
	return f.Update(recording)
}
func (f *fakeRecordingRepo) Delete(id uint) error {
	delete(f.recordings, id)
	return nil
}
func (f *fakeRecordingRepo) CountByProject(projectID uint) (int64, error) {
	return 0, nil
}

func newRecordingServiceWith(projectStatus string, outline *model.OutlineVersion, question *model.Question) RecordingService {
	projectRepo := &fakeProjectRepo{projects: map[uint]*model.Project{
		1: {ID: 1, Title: "测试项目", Status: projectStatus},
	}}
	outlineRepo := newFakeOutlineRepo()
	if outline != nil {
		outlineRepo.versions[outline.ID] = outline
	}
	questionRepo := &fakeQuestionRepo{questions: map[uint]*model.Question{}}
	if question != nil {
		questionRepo.questions[question.ID] = question
	}
	return NewRecordingService(newFakeRecordingRepo(), projectRepo, questionRepo, outlineRepo, slog.Default())
}

func TestRecordingCreateVersionRules(t *testing.T) {
	interviewer := &model.User{ID: 7, Username: "intv", Role: constants.RoleInterviewer}
	baseReq := func() *dto.CreateRecordingRequest {
		return &dto.CreateRecordingRequest{ProjectID: 1, QuestionID: 100, VersionID: 10, DurationSeconds: 12}
	}

	t.Run("已通过版本：录音创建成功并保留问题快照", func(t *testing.T) {
		svc := newRecordingServiceWith(constants.ProjectStatusInProgress,
			&model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusApproved},
			&model.Question{ID: 100, ProjectID: 1, VersionID: 10, Content: "您小时候住在哪里？"})
		rec, err := svc.Create(interviewer, baseReq())
		if err != nil {
			t.Fatalf("create recording failed: %v", err)
		}
		if rec.VersionID != 10 || rec.QuestionSnapshot != "您小时候住在哪里？" {
			t.Fatalf("recording = %+v, want version 10 with question snapshot", rec)
		}
	})

	for _, status := range []string{
		constants.OutlineStatusDraft, constants.OutlineStatusSubmitted, constants.OutlineStatusRejected,
	} {
		st := status
		t.Run("非已通过版本("+st+")禁止录音", func(t *testing.T) {
			svc := newRecordingServiceWith(constants.ProjectStatusInProgress,
				&model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: st},
				&model.Question{ID: 100, ProjectID: 1, VersionID: 10, Content: "问题"})
			_, err := svc.Create(interviewer, baseReq())
			assertAppErrorCode(t, err, constants.CodeNotApproved)
		})
	}

	t.Run("跨项目引用提纲版本被拒绝", func(t *testing.T) {
		svc := newRecordingServiceWith(constants.ProjectStatusInProgress,
			&model.OutlineVersion{ID: 10, ProjectID: 2, VersionNumber: 1, Status: constants.OutlineStatusApproved},
			&model.Question{ID: 100, ProjectID: 2, VersionID: 10, Content: "问题"})
		_, err := svc.Create(interviewer, baseReq())
		assertAppErrorCode(t, err, constants.CodeCrossProjectRef)
	})

	t.Run("问题属于别的版本被拒绝", func(t *testing.T) {
		svc := newRecordingServiceWith(constants.ProjectStatusInProgress,
			&model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusApproved},
			&model.Question{ID: 100, ProjectID: 1, VersionID: 99, Content: "别的版本问题"})
		_, err := svc.Create(interviewer, baseReq())
		assertAppErrorCode(t, err, constants.CodeCrossProjectRef)
	})

	t.Run("归档项目禁止录音", func(t *testing.T) {
		svc := newRecordingServiceWith(constants.ProjectStatusArchived,
			&model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusApproved},
			&model.Question{ID: 100, ProjectID: 1, VersionID: 10, Content: "问题"})
		_, err := svc.Create(interviewer, baseReq())
		assertAppErrorCode(t, err, constants.CodeProjectArchived)
	})
}
