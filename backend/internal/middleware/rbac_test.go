package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oralhistory/oralhistory/internal/constants"
)

func TestRBACRejectsUnauthorizedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name      string
		role      string
		allowed   []string
		wantCode  int
		wantBiz   int
		allowPass bool
	}{
		{name: "采访员访问审核入口被拒绝", role: constants.RoleInterviewer, allowed: []string{constants.RoleArchivist, constants.RoleAdmin}, wantCode: http.StatusForbidden, wantBiz: constants.CodeForbidden},
		{name: "档案员允许通过", role: constants.RoleArchivist, allowed: []string{constants.RoleArchivist, constants.RoleAdmin}, wantCode: http.StatusOK, allowPass: true},
		{name: "管理员允许通过", role: constants.RoleAdmin, allowed: []string{constants.RoleArchivist, constants.RoleAdmin}, wantCode: http.StatusOK, allowPass: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(ContextKeyRole, tc.role)
				c.Set(ContextKeyUsername, "tester")
				c.Next()
			}, RBAC(slog.Default(), tc.allowed...))
			r.GET("/outlines/:vid/review", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0})
			})
			req := httptest.NewRequest(http.MethodGet, "/outlines/1/review", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Fatalf("http status = %d, want %d", w.Code, tc.wantCode)
			}
			if !tc.allowPass {
				var body struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("invalid json body: %v", err)
				}
				if body.Code != tc.wantBiz {
					t.Fatalf("biz code = %d, want %d", body.Code, tc.wantBiz)
				}
			}
		})
	}
}
