package supplier

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/http/middleware"
	"github.com/70548887/sup-platform/internal/http/response"
	pkgcrypto "github.com/70548887/sup-platform/internal/pkg/crypto"
)

// appResponse API应用返回结构（不含Secret）
type appResponse struct {
	ID          uint   `json:"id"`
	AppID       string `json:"app_id"`
	AppName     string `json:"app_name"`
	Environment string `json:"environment"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	CreatedAt   int64  `json:"created_at"`
}

// appDBRow 查询api_apps表的中间结构
type appDBRow struct {
	ID          uint      `gorm:"primaryKey"`
	UserID      uint      `gorm:"column:user_id"`
	AppName     string    `gorm:"column:app_name"`
	AppId       string    `gorm:"column:app_id"`
	AppSecret   string    `gorm:"column:app_secret"`
	Environment string    `gorm:"column:environment"`
	Description string    `gorm:"column:description"`
	Status      int       `gorm:"column:status"`
	KeyRotatedAt int64   `gorm:"column:key_rotated_at"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (appDBRow) TableName() string {
	return "api_apps"
}

// AppCreate POST /openapi/supplier/App/Create — 创建API应用
func (h *Handler) AppCreate(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Environment string `json:"environment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "name", "应用名称不能为空")
		return
	}

	if req.Environment == "" {
		req.Environment = "production"
	}
	if req.Environment != "sandbox" && req.Environment != "production" {
		response.ParamError(c, "environment", "environment仅支持sandbox或production")
		return
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
	app := appDBRow{
		UserID:      userID,
		AppName:     req.Name,
		AppId:       appId,
		AppSecret:   appSecret,
		Environment: req.Environment,
		Description: req.Description,
		Status:      1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.DB.Create(&app).Error; err != nil {
		response.Error(c, "创建应用失败")
		return
	}

	// AppSecret仅此次返回
	response.Success(c, gin.H{
		"id":          app.ID,
		"app_id":      appId,
		"app_secret":  appSecret,
		"name":        req.Name,
		"environment": req.Environment,
	})
}

// AppList GET /openapi/supplier/App/List — 查询当前用户的所有API应用
func (h *Handler) AppList(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var apps []appDBRow
	if err := h.DB.Where("user_id = ?", userID).Order("id DESC").Find(&apps).Error; err != nil {
		response.Error(c, "查询应用列表失败")
		return
	}

	list := make([]appResponse, 0, len(apps))
	for _, a := range apps {
		list = append(list, appResponse{
			ID:          a.ID,
			AppID:       a.AppId,
			AppName:     a.AppName,
			Environment: a.Environment,
			Description: a.Description,
			Status:      a.Status,
			CreatedAt:   a.CreatedAt.Unix(),
		})
	}

	response.Success(c, gin.H{
		"list": list,
	})
}

// AppDetail GET /openapi/supplier/App/Detail — 查看单个应用详情
func (h *Handler) AppDetail(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	appID := c.Query("app_id")
	if appID == "" {
		response.ParamError(c, "app_id", "app_id不能为空")
		return
	}

	var app appDBRow
	if err := h.DB.Where("app_id = ? AND user_id = ?", appID, userID).First(&app).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, "应用不存在")
			return
		}
		response.Error(c, "查询应用失败")
		return
	}

	response.Success(c, appResponse{
		ID:          app.ID,
		AppID:       app.AppId,
		AppName:     app.AppName,
		Environment: app.Environment,
		Description: app.Description,
		Status:      app.Status,
		CreatedAt:   app.CreatedAt.Unix(),
	})
}

// AppRotateKey POST /openapi/supplier/App/RotateKey — 轮换密钥
func (h *Handler) AppRotateKey(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		AppID string `json:"app_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "app_id", "app_id不能为空")
		return
	}

	var app appDBRow
	if err := h.DB.Where("app_id = ? AND user_id = ?", req.AppID, userID).First(&app).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, "应用不存在")
			return
		}
		response.Error(c, "查询应用失败")
		return
	}

	// 生成新Secret
	newSecret, err := pkgcrypto.GenerateAppSecret()
	if err != nil {
		response.Error(c, "生成新密钥失败")
		return
	}

	now := time.Now()
	if err := h.DB.Model(&appDBRow{}).Where("id = ?", app.ID).Updates(map[string]interface{}{
		"app_secret":    newSecret,
		"key_rotated_at": now.Unix(),
		"updated_at":    now,
	}).Error; err != nil {
		response.Error(c, "更新密钥失败")
		return
	}

	// 新Secret仅此次返回
	response.Success(c, gin.H{
		"app_id":         req.AppID,
		"app_secret":     newSecret,
		"key_rotated_at": now.Unix(),
	})
}

// AppUpdateStatus PATCH /openapi/supplier/App/Status — 启用/禁用应用
func (h *Handler) AppUpdateStatus(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		AppID  string `json:"app_id" binding:"required"`
		Status int    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "app_id", "app_id不能为空")
		return
	}

	if req.Status != 0 && req.Status != 1 {
		response.ParamError(c, "status", "status仅支持0(禁用)或1(启用)")
		return
	}

	var app appDBRow
	if err := h.DB.Where("app_id = ? AND user_id = ?", req.AppID, userID).First(&app).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, "应用不存在")
			return
		}
		response.Error(c, "查询应用失败")
		return
	}

	if err := h.DB.Model(&appDBRow{}).Where("id = ?", app.ID).Updates(map[string]interface{}{
		"status":     req.Status,
		"updated_at": time.Now(),
	}).Error; err != nil {
		response.Error(c, "更新状态失败")
		return
	}

	response.Success(c, gin.H{
		"app_id": req.AppID,
		"status": req.Status,
	})
}

// AppDelete DELETE /openapi/supplier/App/Delete — 删除应用
func (h *Handler) AppDelete(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		AppID string `json:"app_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 也支持query参数
		req.AppID = c.Query("app_id")
		if req.AppID == "" {
			response.ParamError(c, "app_id", "app_id不能为空")
			return
		}
	}

	var app appDBRow
	if err := h.DB.Where("app_id = ? AND user_id = ?", req.AppID, userID).First(&app).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, "应用不存在")
			return
		}
		response.Error(c, "查询应用失败")
		return
	}

	if err := h.DB.Delete(&appDBRow{}, app.ID).Error; err != nil {
		response.Error(c, "删除应用失败")
		return
	}

	response.Success(c, gin.H{
		"app_id":  req.AppID,
		"deleted": true,
	})
}
