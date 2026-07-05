package audit

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrAuditLogNotFound = errors.New("audit: log not found")
)

// AuditRepository 审计日志数据访问层
type AuditRepository struct {
	db *gorm.DB
}

// NewAuditRepository 创建AuditRepository
func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Create 创建审计日志记录
func (r *AuditRepository) Create(ctx context.Context, log *AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// List 分页查询审计日志
func (r *AuditRepository) List(ctx context.Context, filter AuditFilter, page, size int) ([]*AuditLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&AuditLog{})

	// 应用过滤条件
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Username != "" {
		query = query.Where("username = ?", filter.Username)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Resource != "" {
		query = query.Where("resource = ?", filter.Resource)
	}
	if filter.ResourceID > 0 {
		query = query.Where("resource_id = ?", filter.ResourceID)
	}
	if filter.StartTime > 0 {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_at <= ?", filter.EndTime)
	}

	// 查询总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	var logs []*AuditLog
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetByID 根据ID查询审计日志
func (r *AuditRepository) GetByID(ctx context.Context, id uint) (*AuditLog, error) {
	var log AuditLog
	err := r.db.WithContext(ctx).First(&log, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuditLogNotFound
		}
		return nil, err
	}
	return &log, nil
}

// Stats 获取审计统计信息（总数、按操作类型/资源类型分组计数）
func (r *AuditRepository) Stats(ctx context.Context) (*AuditStats, error) {
	stats := &AuditStats{
		ByAction:   make(map[string]int64),
		ByResource: make(map[string]int64),
	}

	// 总数
	if err := r.db.WithContext(ctx).Model(&AuditLog{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	// 按操作类型统计
	type groupCount struct {
		Key   string
		Count int64
	}
	var actionCounts []groupCount
	if err := r.db.WithContext(ctx).Model(&AuditLog{}).
		Select("action as key, count(*) as count").
		Group("action").
		Scan(&actionCounts).Error; err != nil {
		return nil, err
	}
	for _, ac := range actionCounts {
		stats.ByAction[ac.Key] = ac.Count
	}

	// 按资源类型统计
	var resourceCounts []groupCount
	if err := r.db.WithContext(ctx).Model(&AuditLog{}).
		Select("resource as key, count(*) as count").
		Group("resource").
		Scan(&resourceCounts).Error; err != nil {
		return nil, err
	}
	for _, rc := range resourceCounts {
		stats.ByResource[rc.Key] = rc.Count
	}

	return stats, nil
}
