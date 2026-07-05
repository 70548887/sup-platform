package card

import (
	"context"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/pkg/crypto"
	"github.com/70548887/sup-platform/internal/pkg/queue"
)

// CardService 卡密服务层
type CardService struct {
	repo        *CardRepository
	db          *gorm.DB
	encryptKey  []byte // AES-GCM 加密密钥（32字节）
	queueClient *queue.QueueClient
}

// NewCardService 创建卡密服务实例
func NewCardService(db *gorm.DB, encryptKey []byte) *CardService {
	return &CardService{
		repo:       NewCardRepository(db),
		db:         db,
		encryptKey: encryptKey,
	}
}

// SetQueueClient 注入队列客户端
func (s *CardService) SetQueueClient(client *queue.QueueClient) {
	s.queueClient = client
}

// ImportCardsAsync 异步入队卡密导入任务，返回任务ID
func (s *CardService) ImportCardsAsync(ctx context.Context, goodsID uint, batchName string, contents []string) error {
	if s.queueClient == nil || !s.queueClient.IsEnabled() {
		return fmt.Errorf("queue client not available")
	}
	payload := queue.CardImportPayload{
		GoodsID:   goodsID,
		BatchName: batchName,
		Contents:  contents,
	}
	return s.queueClient.Enqueue(ctx, queue.TypeCardImport, payload)
}

// ImportCards 批量导入卡密
// 1. 创建CardBatch
// 2. 批量创建Card记录
// 3. 更新商品库存
func (s *CardService) ImportCards(ctx context.Context, goodsID uint, batchName string, contents []string) (*CardBatch, error) {
	if len(contents) == 0 {
		return nil, fmt.Errorf("卡密内容不能为空")
	}
	if goodsID == 0 {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	// 过滤空行
	var validContents []string
	for _, c := range contents {
		c = strings.TrimSpace(c)
		if c != "" {
			validContents = append(validContents, c)
		}
	}
	if len(validContents) == 0 {
		return nil, fmt.Errorf("有效卡密内容为空")
	}

	var batch *CardBatch

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 创建批次
		batch = &CardBatch{
			GoodsID:    goodsID,
			Name:       batchName,
			TotalCount: len(validContents),
			Status:     1,
		}
		if err := tx.Create(batch).Error; err != nil {
			return fmt.Errorf("创建卡密批次失败: %w", err)
		}

		// 2. 批量创建卡密记录
		cards := make([]*Card, 0, len(validContents))
		for _, content := range validContents {
			storeContent := content
			if len(s.encryptKey) > 0 {
				encrypted, err := crypto.EncryptCardContent(content, s.encryptKey)
				if err != nil {
					return fmt.Errorf("加密卡密内容失败: %w", err)
				}
				storeContent = encrypted
			}
			cards = append(cards, &Card{
				BatchID: batch.ID,
				GoodsID: goodsID,
				Content: storeContent,
				Status:  1, // 可用
			})
		}

		batchSize := 500
		for i := 0; i < len(cards); i += batchSize {
			end := i + batchSize
			if end > len(cards) {
				end = len(cards)
			}
			if err := tx.CreateInBatches(cards[i:end], batchSize).Error; err != nil {
				return fmt.Errorf("批量创建卡密失败: %w", err)
			}
		}

		// 3. 更新商品库存
		result := tx.Table("goods").
			Where("id = ?", goodsID).
			Update("stock", gorm.Expr("stock + ?", len(validContents)))
		if result.Error != nil {
			return fmt.Errorf("更新商品库存失败: %w", result.Error)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return batch, nil
}

// IssueCards 发放卡密给订单（核心方法）
// 1. 查询可用卡密（status=1）
// 2. 逐张锁定并绑定订单（LockAndBind）
// 3. 如果某张被抢，重试取下一张
// 4. 全部成功返回卡密列表
// 5. 数量不足返回错误
func (s *CardService) IssueCards(ctx context.Context, tx *gorm.DB, goodsID uint, orderID uint, quantity int) ([]*Card, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("发放数量必须大于0")
	}

	issued := make([]*Card, 0, quantity)
	maxRetries := quantity * 3 // 最大重试次数（防止死循环）
	attempts := 0
	offset := 0

	for len(issued) < quantity && attempts < maxRetries {
		attempts++

		// 查询可用卡密，多取一些用于重试
		fetchSize := (quantity - len(issued)) * 2
		if fetchSize < 10 {
			fetchSize = 10
		}

		var available []*Card
		err := tx.WithContext(ctx).
			Where("goods_id = ? AND status = 1", goodsID).
			Order("id ASC").
			Offset(offset).
			Limit(fetchSize).
			Find(&available).Error
		if err != nil {
			return nil, fmt.Errorf("查询可用卡密失败: %w", err)
		}

		if len(available) == 0 {
			break // 没有更多可用卡密
		}

		for _, card := range available {
			if len(issued) >= quantity {
				break
			}

			// 尝试锁定并绑定
			err := s.repo.LockAndBind(ctx, tx, card.ID, orderID)
			if err != nil {
				// 被抢了，尝试下一张
				continue
			}

			issued = append(issued, card)
		}

		offset += fetchSize
	}

	if len(issued) < quantity {
		return nil, fmt.Errorf("可用卡密不足，需要 %d 张，实际可发放 %d 张", quantity, len(issued))
	}

	s.decryptCards(issued)
	return issued, nil
}

// GetAvailableCount 查询可用卡密数
func (s *CardService) GetAvailableCount(ctx context.Context, goodsID uint) (int64, error) {
	return s.repo.CountAvailable(ctx, goodsID)
}

// GetOrderCards 获取订单的卡密
func (s *CardService) GetOrderCards(ctx context.Context, orderID uint) ([]*Card, error) {
	cards, err := s.repo.FindByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	s.decryptCards(cards)
	return cards, nil
}

// decryptCards 解密卡密列表的 Content 字段
func (s *CardService) decryptCards(cards []*Card) {
	if len(s.encryptKey) == 0 {
		return
	}
	for _, c := range cards {
		plain, err := crypto.DecryptCardContent(c.Content, s.encryptKey)
		if err != nil {
			log.Printf("[WARN] decrypt card %d failed: %v", c.ID, err)
			continue
		}
		c.Content = plain
	}
}
