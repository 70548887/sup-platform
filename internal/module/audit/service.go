package audit

import (
	"context"
	"log"

	"gorm.io/gorm"
)

// AuditService 审计日志服务
type AuditService struct {
	repo *AuditRepository
	db   *gorm.DB
}

// NewAuditService 创建AuditService
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{
		repo: NewAuditRepository(db),
		db:   db,
	}
}

// Log 异步写入审计日志，不阻塞业务流程
// 如果写入失败用log.Printf记录错误
func (s *AuditService) Log(ctx context.Context, entry *AuditLog) error {
	go func() {
		if err := s.repo.Create(context.Background(), entry); err != nil {
			log.Printf("audit: async write failed, action=%s resource=%s resourceID=%d err=%v",
				entry.Action, entry.Resource, entry.ResourceID, err)
		}
	}()
	return nil
}

// LogSync 同步写入审计日志，用于关键操作
func (s *AuditService) LogSync(ctx context.Context, entry *AuditLog) error {
	if err := s.repo.Create(ctx, entry); err != nil {
		log.Printf("audit: sync write failed, action=%s resource=%s resourceID=%d err=%v",
			entry.Action, entry.Resource, entry.ResourceID, err)
		return err
	}
	return nil
}

// Query 分页查询审计日志
func (s *AuditService) Query(ctx context.Context, filter AuditFilter, page, size int) ([]*AuditLog, int64, error) {
	return s.repo.List(ctx, filter, page, size)
}

// GetByID 根据ID获取审计日志
func (s *AuditService) GetByID(ctx context.Context, id uint) (*AuditLog, error) {
	return s.repo.GetByID(ctx, id)
}

// GetStats 获取审计统计信息
func (s *AuditService) GetStats(ctx context.Context) (*AuditStats, error) {
	return s.repo.Stats(ctx)
}

// NewEntry 辅助构造方法，快速创建审计日志条目
func NewEntry(userID uint, username, action, resource string, resourceID uint, detail string) *AuditLog {
	return &AuditLog{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
	}
}
