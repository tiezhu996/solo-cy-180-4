package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/oralhistory/oralhistory/internal/config"
	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/handler"
	"github.com/oralhistory/oralhistory/internal/middleware"
)

// RegisterOutlineRoutes 注册访谈提纲版本与审核路由。
// 采访员在项目维度创建草稿/保存/提交；档案员与管理员在 /outlines 下执行审核。
// 角色细粒度校验在 service 层二次执行，避免仅依赖路由中间件导致越权审核。
func RegisterOutlineRoutes(g *gin.RouterGroup, h *handler.OutlineHandler, cfg *config.Config, logger *slog.Logger) {
	projectGroup := g.Group("/projects/:id/outline-versions", middleware.Auth(cfg.JWTSecret, logger))
	{
		projectGroup.GET("", h.ListByProject)
		projectGroup.POST("", h.CreateDraft)
		projectGroup.GET("/:vid", h.GetByProject)
		projectGroup.PUT("/:vid/questions", h.SaveQuestions)
		projectGroup.DELETE("/:vid/questions/:qid", h.DeleteQuestion)
		projectGroup.POST("/:vid/submit", h.Submit)
	}

	reviewGroup := g.Group("/outlines",
		middleware.Auth(cfg.JWTSecret, logger),
		middleware.RBAC(logger, constants.RoleArchivist, constants.RoleAdmin))
	{
		reviewGroup.GET("/pending", h.ListPending)
		reviewGroup.POST("/:vid/review", h.Review)
	}
}
