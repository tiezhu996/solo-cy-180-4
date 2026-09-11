package service

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/dto"
	"github.com/oralhistory/oralhistory/internal/model"
	"github.com/oralhistory/oralhistory/internal/repository"
	"github.com/oralhistory/oralhistory/internal/util"
)

// fakeOutlineRepo 提纲版本仓储的内存实现，状态机规则与真实仓储保持一致。
type fakeOutlineRepo struct {
	versions  map[uint]*model.OutlineVersion
	questions map[uint]*model.Question
	nextVID   uint
	nextQID   uint
}

func newFakeOutlineRepo() *fakeOutlineRepo {
	return &fakeOutlineRepo{
		versions:  map[uint]*model.OutlineVersion{},
		questions: map[uint]*model.Question{},
		nextVID:   1,
		nextQID:   1,
	}
}

func (f *fakeOutlineRepo) Create(version *model.OutlineVersion) error {
	if version.ID == 0 {
		version.ID = f.nextVID
		f.nextVID++
	}
	f.versions[version.ID] = version
	return nil
}

func (f *fakeOutlineRepo) FindByID(id uint) (*model.OutlineVersion, error) {
	v, ok := f.versions[id]
	if !ok {
		return nil, errors.Join(repository.ErrNotFound)
	}
	cp := *v
	return &cp, nil
}

func (f *fakeOutlineRepo) ListByProject(projectID uint) ([]model.OutlineVersion, error) {
	var out []model.OutlineVersion
	for _, v := range f.versions {
		if v.ProjectID == projectID {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (f *fakeOutlineRepo) LatestByProject(projectID uint) (*model.OutlineVersion, error) {
	return nil, repository.ErrNotFound
}

func (f *fakeOutlineRepo) LatestApprovedByProject(projectID uint) (*model.OutlineVersion, error) {
	return nil, repository.ErrNotFound
}

func (f *fakeOutlineRepo) ListPending(page, pageSize int, projectID uint) ([]model.OutlineVersion, int64, error) {
	var out []model.OutlineVersion
	var total int64
	for _, v := range f.versions {
		if v.Status == constants.OutlineStatusSubmitted && (projectID == 0 || v.ProjectID == projectID) {
			out = append(out, *v)
			total++
		}
	}
	return out, total, nil
}

func (f *fakeOutlineRepo) CreateDraft(projectID, baseVersionID, actorID uint) (*model.OutlineVersion, error) {
	for _, v := range f.versions {
		if v.ProjectID == projectID && (v.Status == constants.OutlineStatusDraft || v.Status == constants.OutlineStatusRejected) {
			cp := *v
			return &cp, nil
		}
		if v.ProjectID == projectID && v.Status == constants.OutlineStatusSubmitted {
			return nil, repository.ErrConflict
		}
	}
	num := 1
	for _, v := range f.versions {
		if v.ProjectID == projectID && v.VersionNumber >= num {
			num = v.VersionNumber + 1
		}
	}
	v := &model.OutlineVersion{ProjectID: projectID, VersionNumber: num, Status: constants.OutlineStatusDraft, CreatedBy: actorID}
	if baseVersionID > 0 {
		base, ok := f.versions[baseVersionID]
		if !ok {
			return nil, repository.ErrNotFound
		}
		if base.ProjectID != projectID || base.Status != constants.OutlineStatusApproved {
			return nil, repository.ErrCrossProject
		}
		v.BasedOnVersion = base.VersionNumber
	}
	if err := f.Create(v); err != nil {
		return nil, err
	}
	return f.FindByID(v.ID)
}

func (f *fakeOutlineRepo) SaveQuestions(versionID uint, items []repository.QuestionUpsert) (*model.OutlineVersion, error) {
	v, ok := f.versions[versionID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if !constants.CanEditOutline(v.Status) {
		return nil, repository.ErrConflict
	}
	for _, item := range items {
		if item.QuestionID == 0 {
			q := &model.Question{ID: f.nextQID, ProjectID: v.ProjectID, VersionID: v.ID, Content: item.Content, SortOrder: item.SortOrder}
			f.nextQID++
			f.questions[q.ID] = q
			v.Questions = append(v.Questions, *q)
			continue
		}
		q, ok := f.questions[item.QuestionID]
		if !ok {
			return nil, repository.ErrNotFound
		}
		if q.VersionID != v.ID || q.ProjectID != v.ProjectID {
			return nil, repository.ErrCrossProject
		}
		q.Content = item.Content
		q.SortOrder = item.SortOrder
	}
	return f.FindByID(versionID)
}

func (f *fakeOutlineRepo) DeleteQuestion(versionID, questionID uint) error {
	v, ok := f.versions[versionID]
	if !ok {
		return repository.ErrNotFound
	}
	if !constants.CanEditOutline(v.Status) {
		return repository.ErrConflict
	}
	q, ok := f.questions[questionID]
	if !ok || q.VersionID != v.ID {
		return repository.ErrNotFound
	}
	delete(f.questions, questionID)
	return nil
}

func (f *fakeOutlineRepo) Submit(versionID, actorID uint) error {
	v, ok := f.versions[versionID]
	if !ok {
		return repository.ErrNotFound
	}
	if !constants.CanSubmitOutline(v.Status) {
		return repository.ErrConflict
	}
	var count int
	for _, q := range f.questions {
		if q.VersionID == v.ID {
			count++
		}
	}
	if count == 0 {
		return repository.ErrValidation
	}
	v.Status = constants.OutlineStatusSubmitted
	v.SubmittedBy = actorID
	return nil
}

func (f *fakeOutlineRepo) Review(versionID, reviewerID uint, reviewerName, status, reason string) error {
	v, ok := f.versions[versionID]
	if !ok {
		return repository.ErrNotFound
	}
	if !constants.CanReviewOutline(v.Status) {
		return repository.ErrConflict
	}
	v.Status = status
	v.ReviewedBy = reviewerID
	v.ReviewerName = reviewerName
	if status == constants.OutlineStatusRejected {
		v.RejectReason = reason
	}
	return nil
}

func newOutlineServiceWith(repo *fakeOutlineRepo, projects ...*model.Project) OutlineService {
	pm := map[uint]*model.Project{}
	for _, p := range projects {
		pm[p.ID] = p
	}
	return NewOutlineService(repo, &fakeProjectRepo{projects: pm}, slog.Default())
}

func TestOutlineReviewAuthorizationAndDuplicate(t *testing.T) {
	interviewer := &model.User{ID: 1, Username: "intv", Role: constants.RoleInterviewer}
	archivist := &model.User{ID: 2, Username: "arch", Role: constants.RoleArchivist}
	admin := &model.User{ID: 3, Username: "admin", Role: constants.RoleAdmin}

	t.Run("越权审核：采访员审核被拒绝", func(t *testing.T) {
		repo := newFakeOutlineRepo()
		svc := newOutlineServiceWith(repo, &model.Project{ID: 1, Status: constants.ProjectStatusInProgress})
		repo.versions[10] = &model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusSubmitted}
		_, err := svc.Review(interviewer, 10, &dto.ReviewOutlineRequest{Action: "approve"})
		assertAppErrorCode(t, err, constants.CodeForbidden)
	})

	t.Run("通过后重复审核被拒绝", func(t *testing.T) {
		repo := newFakeOutlineRepo()
		svc := newOutlineServiceWith(repo, &model.Project{ID: 1, Status: constants.ProjectStatusInProgress})
		repo.versions[10] = &model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusSubmitted}
		if _, err := svc.Review(archivist, 10, &dto.ReviewOutlineRequest{Action: "approve"}); err != nil {
			t.Fatalf("first approve failed: %v", err)
		}
		_, err := svc.Review(admin, 10, &dto.ReviewOutlineRequest{Action: "approve"})
		assertAppErrorCode(t, err, constants.CodeOutlineStatus)
		if repo.versions[10].Status != constants.OutlineStatusApproved {
			t.Fatalf("status = %s, want approved", repo.versions[10].Status)
		}
	})

	t.Run("退回必须填写原因", func(t *testing.T) {
		repo := newFakeOutlineRepo()
		svc := newOutlineServiceWith(repo, &model.Project{ID: 1, Status: constants.ProjectStatusInProgress})
		repo.versions[10] = &model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusSubmitted}
		_, err := svc.Review(archivist, 10, &dto.ReviewOutlineRequest{Action: "reject", Reason: "   "})
		assertAppErrorCode(t, err, constants.CodeValidation)
		if repo.versions[10].Status != constants.OutlineStatusSubmitted {
			t.Fatalf("status changed to %s despite missing reason", repo.versions[10].Status)
		}
	})

	t.Run("退回写入原因后可再次提交", func(t *testing.T) {
		repo := newFakeOutlineRepo()
		svc := newOutlineServiceWith(repo, &model.Project{ID: 1, Status: constants.ProjectStatusInProgress})
		repo.versions[10] = &model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusSubmitted}
		repo.questions[1] = &model.Question{ID: 1, ProjectID: 1, VersionID: 10, Content: "童年记忆？"}
		v, err := svc.Review(archivist, 10, &dto.ReviewOutlineRequest{Action: "reject", Reason: "问题太宽泛"})
		if err != nil {
			t.Fatalf("reject failed: %v", err)
		}
		if v.RejectReason != "问题太宽泛" {
			t.Fatalf("reject reason = %q", v.RejectReason)
		}
		if _, err := svc.Submit(interviewer, 1, 10); err != nil {
			t.Fatalf("resubmit rejected version failed: %v", err)
		}
		if repo.versions[10].Status != constants.OutlineStatusSubmitted {
			t.Fatalf("status = %s, want submitted", repo.versions[10].Status)
		}
	})
}

func TestOutlineDuplicateSubmit(t *testing.T) {
	interviewer := &model.User{ID: 1, Username: "intv", Role: constants.RoleInterviewer}
	repo := newFakeOutlineRepo()
	svc := newOutlineServiceWith(repo, &model.Project{ID: 1, Status: constants.ProjectStatusInProgress})
	draft, err := svc.CreateDraft(interviewer, 1, 0)
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}
	if _, err := svc.SaveQuestions(interviewer, 1, draft.ID, &dto.SaveOutlineDraftRequest{
		Questions: []dto.SaveOutlineQuestionRequest{{Content: "请讲讲您的童年"}},
	}); err != nil {
		t.Fatalf("save question failed: %v", err)
	}
	if _, err := svc.Submit(interviewer, 1, draft.ID); err != nil {
		t.Fatalf("first submit failed: %v", err)
	}
	// 重复提交待审核版本必须拒绝。
	_, err = svc.Submit(interviewer, 1, draft.ID)
	assertAppErrorCode(t, err, constants.CodeOutlineStatus)

	// 已通过锁定版本不允许再编辑。
	archivist := &model.User{ID: 2, Username: "arch", Role: constants.RoleArchivist}
	if _, err := svc.Review(archivist, draft.ID, &dto.ReviewOutlineRequest{Action: "approve"}); err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	_, err = svc.SaveQuestions(interviewer, 1, draft.ID, &dto.SaveOutlineDraftRequest{
		Questions: []dto.SaveOutlineQuestionRequest{{Content: "锁定后尝试改"}},
	})
	assertAppErrorCode(t, err, constants.CodeOutlineStatus)
}

func TestOutlineCrossProjectReference(t *testing.T) {
	interviewer := &model.User{ID: 1, Username: "intv", Role: constants.RoleInterviewer}
	repo := newFakeOutlineRepo()
	svc := newOutlineServiceWith(repo,
		&model.Project{ID: 1, Status: constants.ProjectStatusInProgress},
		&model.Project{ID: 2, Status: constants.ProjectStatusInProgress})

	// 项目 2 的已通过版本不能作为项目 1 的新草稿基线。
	repo.versions[20] = &model.OutlineVersion{ID: 20, ProjectID: 2, VersionNumber: 1, Status: constants.OutlineStatusApproved}
	_, err := svc.CreateDraft(interviewer, 1, 20)
	assertAppErrorCode(t, err, constants.CodeCrossProjectRef)

	// 项目 2 的版本不允许通过项目 1 的路径查看/提交。
	_, err = svc.GetByProjectAndID(interviewer, 1, 20)
	assertAppErrorCode(t, err, constants.CodeCrossProjectRef)

	repo.versions[20].Status = constants.OutlineStatusSubmitted
	_, err = svc.Submit(interviewer, 1, 20)
	assertAppErrorCode(t, err, constants.CodeCrossProjectRef)
}

func TestOutlineArchivedProjectRejected(t *testing.T) {
	interviewer := &model.User{ID: 1, Username: "intv", Role: constants.RoleInterviewer}
	archivist := &model.User{ID: 2, Username: "arch", Role: constants.RoleArchivist}
	repo := newFakeOutlineRepo()
	svc := newOutlineServiceWith(repo, &model.Project{ID: 1, Status: constants.ProjectStatusArchived})

	_, err := svc.CreateDraft(interviewer, 1, 0)
	assertAppErrorCode(t, err, constants.CodeProjectArchived)

	repo.versions[10] = &model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusSubmitted}
	_, err = svc.Review(archivist, 10, &dto.ReviewOutlineRequest{Action: "approve"})
	assertAppErrorCode(t, err, constants.CodeProjectArchived)
}

func TestOutlineAdjustApprovedCreatesNewVersion(t *testing.T) {
	interviewer := &model.User{ID: 1, Username: "intv", Role: constants.RoleInterviewer}
	repo := newFakeOutlineRepo()
	svc := newOutlineServiceWith(repo, &model.Project{ID: 1, Status: constants.ProjectStatusInProgress})
	v1 := &model.OutlineVersion{ID: 10, ProjectID: 1, VersionNumber: 1, Status: constants.OutlineStatusApproved}
	repo.versions[10] = v1
	repo.questions[1] = &model.Question{ID: 1, ProjectID: 1, VersionID: 10, Content: "旧问题", SortOrder: 0}

	draft, err := svc.CreateDraft(interviewer, 1, 10)
	if err != nil {
		t.Fatalf("create draft from approved version failed: %v", err)
	}
	if draft.VersionNumber != 2 || draft.Status != constants.OutlineStatusDraft {
		t.Fatalf("new version = v%d %s, want v2 draft", draft.VersionNumber, draft.Status)
	}
	// 历史版本保持已通过锁定状态，不被覆盖。
	if repo.versions[10].Status != constants.OutlineStatusApproved {
		t.Fatalf("historical version overwritten: %s", repo.versions[10].Status)
	}
}

func assertAppErrorCode(t *testing.T, err error, wantCode int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %d, got nil", wantCode)
	}
	var appErr *util.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *util.AppError, got %T: %v", err, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("error code = %d (%s), want %d", appErr.Code, appErr.Message, wantCode)
	}
}
