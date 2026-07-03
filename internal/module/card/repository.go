package card

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// CardRepository 卡密数据访问层
type CardRepository struct {
	db *gorm.DB
}

// NewCardRepository 创建卡密仓储实例
func NewCardRepository(db *gorm.DB) *CardRepository {
	return &CardRepository{db: db}
}

// BatchCreate 批量创建卡密
func (r *CardRepository) BatchCreate(ctx context.Context, cards []*Card) error {
	if len(cards) == 0 {
		return nil
	}
	// 分批插入，每批500条
	batchSize := 500
	for i := 0; i < len(cards); i += batchSize {
		end := i + batchSize
		if end > len(cards) {
			end = len(cards)
		}
		if err := r.db.WithContext(ctx).CreateInBatches(cards[i:end], batchSize).Error; err != nil {
			return fmt.Errorf("批量创建卡密失败: %w", err)
		}
	}
	return nil
}

// FindAvailable 查询可用卡密
// WHERE goods_id=? AND status=1 ORDER BY id ASC LIMIT ?
func (r *CardRepository) FindAvailable(ctx context.Context, goodsID uint, limit int) ([]*Card, error) {
	var cards []*Card
	err := r.db.WithContext(ctx).
		Where("goods_id = ? AND status = 1", goodsID).
		Order("id ASC").
		Limit(limit).
		Find(&cards).Error
	if err != nil {
		return nil, fmt.Errorf("查询可用卡密失败: %w", err)
	}
	return cards, nil
}

// LockAndBind 锁定卡密并绑定订单（原子操作）
// UPDATE cards SET status=3, order_id=?, used_at=? WHERE id=? AND status=1
// 如果RowsAffected=0返回错误（并发被抢）
func (r *CardRepository) LockAndBind(ctx context.Context, tx *gorm.DB, cardID uint, orderID uint) error {
	now := time.Now().Unix()
	result := tx.WithContext(ctx).
		Model(&Card{}).
		Where("id = ? AND status = 1", cardID).
		Updates(map[string]interface{}{
			"status":   int8(3),
			"order_id": orderID,
			"used_at":  now,
		})

	if result.Error != nil {
		return fmt.Errorf("绑定卡密失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("卡密已被占用")
	}
	return nil
}

// CountAvailable 统计可用卡密数量
func (r *CardRepository) CountAvailable(ctx context.Context, goodsID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Card{}).
		Where("goods_id = ? AND status = 1", goodsID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("统计可用卡密失败: %w", err)
	}
	return count, nil
}

// FindByOrder 查询订单关联的卡密
func (r *CardRepository) FindByOrder(ctx context.Context, orderID uint) ([]*Card, error) {
	var cards []*Card
	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("id ASC").
		Find(&cards).Error
	if err != nil {
		return nil, fmt.Errorf("查询订单卡密失败: %w", err)
	}
	return cards, nil
}

// CreateBatch 创建卡密批次
func (r *CardRepository) CreateBatch(ctx context.Context, batch *CardBatch) error {
	return r.db.WithContext(ctx).Create(batch).Error
}
