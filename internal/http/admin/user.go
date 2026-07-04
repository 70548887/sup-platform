package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/account"
	"github.com/70548887/sup-platform/internal/module/audit"
)

// CreateUser POST /admin/users — 创建用户
func (h *Handler) CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	// 校验角色
	if req.Role != "admin" && req.Role != "supplier" && req.Role != "customer" {
		response.ParamError(c, "role", "角色只能是 admin/supplier/customer")
		return
	}

	// 校验用户名长度
	if len(req.Username) < 3 || len(req.Username) > 64 {
		response.ParamError(c, "username", "用户名长度为3-64个字符")
		return
	}

	// 校验密码长度
	if len(req.Password) < 6 {
		response.ParamError(c, "password", "密码长度不能少于6个字符")
		return
	}

	// 检查用户名是否已存在
	var existing account.User
	if err := h.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		response.Error(c, "用户名已存在")
		return
	}

	// bcrypt加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Error(c, "密码加密失败")
		return
	}

	user := account.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Nickname: req.Nickname,
		Email:    req.Email,
		Phone:    req.Phone,
		Role:     req.Role,
		Status:   1,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		response.Error(c, "创建用户失败")
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "user.create", "user", user.ID,
		fmt.Sprintf("创建用户: %s, 角色: %s", user.Username, user.Role),
	))

	// 返回用户信息（不含password）
	response.Success(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"nickname":   user.Nickname,
		"email":      user.Email,
		"phone":      user.Phone,
		"role":       user.Role,
		"status":     user.Status,
		"created_at": user.CreatedAt,
	})
}

// ListUsers GET /admin/users — 用户列表
func (h *Handler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	role := c.Query("role")

	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	query := h.DB.Model(&account.User{})

	// 可选角色过滤
	if role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.Error(c, "查询用户总数失败")
		return
	}

	var users []account.User
	if err := query.Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&users).Error; err != nil {
		response.Error(c, "查询用户列表失败")
		return
	}

	// 构造返回列表（不含password）
	type userItem struct {
		ID        uint   `json:"id"`
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
		Role      string `json:"role"`
		Status    int8   `json:"status"`
		CreatedAt int64  `json:"created_at"`
		UpdatedAt int64  `json:"updated_at"`
	}

	list := make([]userItem, 0, len(users))
	for _, u := range users {
		list = append(list, userItem{
			ID:        u.ID,
			Username:  u.Username,
			Nickname:  u.Nickname,
			Email:     u.Email,
			Phone:     u.Phone,
			Role:      u.Role,
			Status:    u.Status,
			CreatedAt: u.CreatedAt,
			UpdatedAt: u.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetUser GET /admin/users/:id — 用户详情
func (h *Handler) GetUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var user account.User
	if err := h.DB.Where("id = ?", id).First(&user).Error; err != nil {
		response.Error(c, "用户不存在")
		return
	}

	// 返回用户信息（不含password）
	response.Success(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"nickname":   user.Nickname,
		"email":      user.Email,
		"phone":      user.Phone,
		"role":       user.Role,
		"status":     user.Status,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	})
}

// UpdateUserStatus PATCH /admin/users/:id/status — 启用/禁用用户
func (h *Handler) UpdateUserStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	// 校验status值
	if req.Status != 0 && req.Status != 1 {
		response.ParamError(c, "status", "status只能是0(禁用)或1(启用)")
		return
	}

	// 检查用户是否存在
	var user account.User
	if err := h.DB.Where("id = ?", id).First(&user).Error; err != nil {
		response.Error(c, "用户不存在")
		return
	}

	// 更新状态
	if err := h.DB.Model(&account.User{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		response.Error(c, "更新用户状态失败")
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	statusText := "启用"
	if req.Status == 0 {
		statusText = "禁用"
	}
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "user.status_change", "user", id,
		fmt.Sprintf("%s用户: %s", statusText, user.Username),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": req.Status,
	})
}
