package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/dto"
	"github.com/oralhistory/oralhistory/internal/middleware"
	"github.com/oralhistory/oralhistory/internal/service"
	"github.com/oralhistory/oralhistory/internal/util"
)

// OutlineHandler 访谈提纲版本与审核接口处理器。
type OutlineHandler struct {
	outlineSvc service.OutlineService
	auditSvc   service.AuditService
	logger     *slog.Logger
}

// NewOutlineHandler 构造提纲版本处理器。
func NewOutlineHandler(outlineSvc service.OutlineService, auditSvc service.AuditService, logger *slog.Logger) *OutlineHandler {
	return &OutlineHandler{outlineSvc: outlineSvc, auditSvc: auditSvc, logger: logger}
}

// CreateDraft 创建（或复用）项目的草稿提纲版本。
func (h *OutlineHandler) CreateDraft(c *gin.Context) {
	actor, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(err)
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req dto.CreateOutlineVersionRequest
	if c.Request.ContentLength > 0 {
		if !bindJSON(c, &req) {
			return
		}
	}
	version, err := h.outlineSvc.CreateDraft(actor, projectID, req.BaseVersionID)
	if err != nil {
		c.Error(err)
		return
	}
	h.auditSvc.Record(actor.ID, actor.Username, actor.Role, "outline.draft", "outline_version", version.ID,
		"创建提纲草稿版本 v"+strconv.Itoa(version.VersionNumber), c.ClientIP(), middleware.RequestID(c))
	util.OKMessage(c, constants.MsgOutlineDraftSaved, version)
}

// ListByProject 项目提纲版本列表（版本查看）。
func (h *OutlineHandler) ListByProject(c *gin.Context) {
	actor, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(err)
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	versions, err := h.outlineSvc.ListByProject(actor, projectID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"list": versions})
}

// GetByProject 查看指定提纲版本详情（含问题内容）。
func (h *OutlineHandler) GetByProject(c *gin.Context) {
	actor, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(err)
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	versionID, ok := parseID(c, "vid")
	if !ok {
		return
	}
	version, err := h.outlineSvc.GetByProjectAndID(actor, projectID, versionID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, version)
}

// SaveQuestions 采访员保存提纲改动（草稿）。
func (h *OutlineHandler) SaveQuestions(c *gin.Context) {
	actor, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(err)
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	versionID, ok := parseID(c, "vid")
	if !ok {
		return
	}
	var req dto.SaveOutlineDraftRequest
	if !bindJSON(c, &req) {
		return
	}
	version, err := h.outlineSvc.SaveQuestions(actor, projectID, versionID, &req)
	if err != nil {
		c.Error(err)
		return
	}
	h.auditSvc.Record(actor.ID, actor.Username, actor.Role, "outline.save", "outline_version", version.ID,
		"保存提纲草稿改动", c.ClientIP(), middleware.RequestID(c))
	util.OKMessage(c, constants.MsgOutlineDraftSaved, version)
}

// DeleteQuestion 删除草稿版本中的问题。
func (h *OutlineHandler) DeleteQuestion(c *gin.Context) {
	actor, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(err)
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	versionID, ok := parseID(c, "vid")
	if !ok {
		return
	}
	questionID, ok := parseID(c, "qid")
	if !ok {
		return
	}
	if err := h.outlineSvc.DeleteQuestion(actor, projectID, versionID, questionID); err != nil {
		c.Error(err)
		return
	}
	h.auditSvc.Record(actor.ID, actor.Username, actor.Role, "outline.question.delete", "question", questionID,
		"删除草稿提纲问题", c.ClientIP(), middleware.RequestID(c))
	util.OKMessage(c, constants.MsgQuestionDeleted, nil)
}

// Submit 采访员提交提纲版本审核。
func (h *OutlineHandler) Submit(c *gin.Context) {
	actor, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(err)
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	versionID, ok := parseID(c, "vid")
	if !ok {
		return
	}
	version, err := h.outlineSvc.Submit(actor, projectID, versionID)
	if err != nil {
		c.Error(err)
		return
	}
	h.auditSvc.Record(actor.ID, actor.Username, actor.Role, "outline.submit", "outline_version", version.ID,
		"提交提纲版本 v"+strconv.Itoa(version.VersionNumber)+" 审核", c.ClientIP(), middleware.RequestID(c))
	util.OKMessage(c, constants.MsgOutlineSubmitted, version)
}

// ListPending 档案员/管理员查看待审核提纲列表。
func (h *OutlineHandler) ListPending(c *gin.Context) {
	actor, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(err)
		return
	}
	var p dto.PageParams
	if !bindQuery(c, &p) {
		return
	}
	p.Normalize()
	var projectID uint
	if raw := c.Query("project_id"); raw != "" {
		if v, err := strconv.ParseUint(raw, 10, 64); err == nil {
			projectID = uint(v)
		}
	}
	versions, total, err := h.outlineSvc.ListPending(actor, p.Page, p.PageSize, projectID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"list": versions, "total": total, "page": p.Page, "page_size": p.PageSize})
}

// Review 档案员/管理员审核提纲：通过或退回（退回必须填写原因）。
func (h *OutlineHandler) Review(c *gin.Context) {
	actor, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(err)
		return
	}
	versionID, ok := parseID(c, "vid")
	if !ok {
		return
	}
	var req dto.ReviewOutlineRequest
	if !bindJSON(c, &req) {
		return
	}
	version, err := h.outlineSvc.Review(actor, versionID, &req)
	if err != nil {
		c.Error(err)
		return
	}
	action := "outline.approve"
	detail := "审核通过提纲版本 v" + strconv.Itoa(version.VersionNumber)
	message := constants.MsgOutlineApproved
	if req.Action == "reject" {
		action = "outline.reject"
		detail = "退回提纲版本 v" + strconv.Itoa(version.VersionNumber) + "，原因：" + req.Reason
		message = constants.MsgOutlineRejected
	}
	h.auditSvc.Record(actor.ID, actor.Username, actor.Role, action, "outline_version", version.ID,
		detail, c.ClientIP(), middleware.RequestID(c))
	util.OKMessage(c, message, version)
}
