package admin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	pkgcrypto "github.com/70548887/sup-platform/internal/pkg/crypto"
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

// CreateApiApp POST /admin/api-apps — 为指定用户创建应用
func (h *Handler) CreateApiApp(c *gin.Context) {
	var req struct {
		UserID      uint   `json:"user_id" binding:"required"`
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Environment string `json:"environment"`
		RateLimit   int    `json:"rate_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "user_id和name不能为空")
		return
	}

	if req.Environment == "" {
		req.Environment = "production"
	}
	if req.RateLimit <= 0 {
		req.RateLimit = 60
	}

	// 生成AppId + AppSecret
	appId, err := pkgcrypto.GenerateAppId()
	if err != nil {
		response.Error(c, "生成应用ID失败")
		return
	}
	appSecret, err := pkgcrypto.GenerateAppSecret()
	if err != nil {
		response.Error(c, "生成应用密钥失败")
		return
	}

	now := time.Now()
	result := h.DB.Table("api_apps").Create(map[string]interface{}{
		"user_id":     req.UserID,
		"app_name":    req.Name,
		"app_id":      appId,
		"app_secret":  appSecret,
		"environment": req.Environment,
		"description": req.Description,
		"rate_limit":  req.RateLimit,
		"status":      1,
		"created_at":  now,
		"updated_at":  now,
	})
	if result.Error != nil {
		response.Error(c, "创建应用失败")
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "api_app.create", "api_app", 0,
		fmt.Sprintf("为用户%d创建应用%s", req.UserID, req.Name),
	))

	response.Success(c, gin.H{
		"app_id":      appId,
		"app_secret":  appSecret,
		"name":        req.Name,
		"environment": req.Environment,
		"rate_limit":  req.RateLimit,
	})
}

// UpdateApiAppStatus PATCH /admin/api-apps/:id/status — 更新应用状态
func (h *Handler) UpdateApiAppStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Status int `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "status", "请求参数格式错误")
		return
	}
	if req.Status != 0 && req.Status != 1 {
		response.ParamError(c, "status", "status仅支持0(禁用)或1(启用)")
		return
	}

	// 检查应用是否存在
	var app apiAppDBItem
	if err := h.DB.Table("api_apps").Where("id = ?", id).First(&app).Error; err != nil {
		response.Error(c, "应用不存在")
		return
	}

	if err := h.DB.Table("api_apps").Where("id = ?", id).Updates(map[string]interface{}{
		"status":     req.Status,
		"updated_at": time.Now(),
	}).Error; err != nil {
		response.Error(c, "更新状态失败")
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	statusDesc := "禁用"
	if req.Status == 1 {
		statusDesc = "启用"
	}
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "api_app.status_update", "api_app", id,
		fmt.Sprintf("%s应用%s", statusDesc, app.AppID),
	))

	response.Success(c, gin.H{
		"id":     id,
		"app_id": app.AppID,
		"status": req.Status,
	})
}

// DeleteApiApp DELETE /admin/api-apps/:id — 删除应用
func (h *Handler) DeleteApiApp(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	// 检查应用是否存在
	var app apiAppDBItem
	if err := h.DB.Table("api_apps").Where("id = ?", id).First(&app).Error; err != nil {
		response.Error(c, "应用不存在")
		return
	}

	if err := h.DB.Table("api_apps").Where("id = ?", id).Delete(nil).Error; err != nil {
		response.Error(c, "删除应用失败")
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "api_app.delete", "api_app", id,
		fmt.Sprintf("删除应用%s", app.AppID),
	))

	response.Success(c, gin.H{
		"id":      id,
		"app_id":  app.AppID,
		"deleted": true,
	})
}

// ResetApiAppSecret POST /admin/api-apps/:id/reset — 重新生成Secret
func (h *Handler) ResetApiAppSecret(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	// 检查应用是否存在
	var app apiAppDBItem
	if err := h.DB.Table("api_apps").Where("id = ?", id).First(&app).Error; err != nil {
		response.Error(c, "应用不存在")
		return
	}

	// 生成新Secret
	newSecret, err := pkgcrypto.GenerateAppSecret()
	if err != nil {
		response.Error(c, "生成新密钥失败")
		return
	}

	now := time.Now()
	if err := h.DB.Table("api_apps").Where("id = ?", id).Updates(map[string]interface{}{
		"app_secret":    newSecret,
		"key_rotated_at": now.Unix(),
		"updated_at":    now,
	}).Error; err != nil {
		response.Error(c, "重置密钥失败")
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "api_app.secret_reset", "api_app", id,
		fmt.Sprintf("重置应用%s的密钥", app.AppID),
	))

	response.Success(c, gin.H{
		"id":             id,
		"app_id":         app.AppID,
		"app_secret":     newSecret,
		"key_rotated_at": now.Unix(),
	})
}
