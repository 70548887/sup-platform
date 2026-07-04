package admin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
)

// apiAppListItem API应用列表返回项
type apiAppListItem struct {
	ID        uint   `json:"id"`
	AppID     string `json:"app_id"`
	AppName   string `json:"app_name"`
	RateLimit int    `json:"rate_limit"`
	Status    int    `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

// apiAppDBItem 查询api_apps表的中间结构（不依赖auth.ApiApp模型）
type apiAppDBItem struct {
	ID        uint
	AppID     string    `gorm:"column:app_id"`
	AppName   string    `gorm:"column:app_name"`
	RateLimit int       `gorm:"column:rate_limit"`
	Status    int
	CreatedAt time.Time
}

// ListApiApps GET /admin/api-apps — API应用分页列表
func (h *Handler) ListApiApps(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	query := h.DB.Table("api_apps")

	// 可选status过滤
	if statusStr := c.Query("status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			query = query.Where("status = ?", s)
		}
	}

	// 查询总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.Error(c, "查询应用总数失败")
		return
	}

	// 分页查询
	var apps []apiAppDBItem
	if err := query.Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&apps).Error; err != nil {
		response.Error(c, "查询应用列表失败")
		return
	}

	list := make([]apiAppListItem, 0, len(apps))
	for _, app := range apps {
		list = append(list, apiAppListItem{
			ID:        app.ID,
			AppID:     app.AppID,
			AppName:   app.AppName,
			RateLimit: app.RateLimit,
			Status:    app.Status,
			CreatedAt: app.CreatedAt.Unix(),
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// UpdateRateLimit PATCH /admin/api-apps/:id/rate-limit — 更新应用限流配额
func (h *Handler) UpdateRateLimit(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		RateLimit int `json:"rate_limit" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "rate_limit", "请求参数格式错误")
		return
	}
	if req.RateLimit <= 0 {
		response.ParamError(c, "rate_limit", "rate_limit必须大于0")
		return
	}

	// 检查应用是否存在
	var app apiAppDBItem
	if err := h.DB.Table("api_apps").Where("id = ?", id).First(&app).Error; err != nil {
		response.Error(c, "应用不存在")
		return
	}

	// 更新限流配额
	if err := h.DB.Table("api_apps").Where("id = ?", id).Update("rate_limit", req.RateLimit).Error; err != nil {
		response.Error(c, "更新限流配额失败")
		return
	}

	app.RateLimit = req.RateLimit

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "api_app.rate_limit_update", "api_app", id,
		fmt.Sprintf("更新应用%s限流配额为%d", app.AppID, req.RateLimit),
	))

	response.Success(c, apiAppListItem{
		ID:        app.ID,
		AppID:     app.AppID,
		AppName:   app.AppName,
		RateLimit: app.RateLimit,
		Status:    app.Status,
		CreatedAt: app.CreatedAt.Unix(),
	})
}

// GetAppUsage GET /admin/api-apps/:id/usage — 获取应用当前用量
// 当前Handler未持有RedisClient，无法读取实时token数，返回usage=-1表示不可用
func (h *Handler) GetAppUsage(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var app apiAppDBItem
	if err := h.DB.Table("api_apps").Where("id = ?", id).First(&app).Error; err != nil {
		response.Error(c, "应用不存在")
		return
	}

	response.Success(c, gin.H{
		"app_id":        app.AppID,
		"rate_limit":    app.RateLimit,
		"current_usage": -1,
		"remaining":     -1,
	})
}
