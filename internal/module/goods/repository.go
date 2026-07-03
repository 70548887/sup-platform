package goods

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// GoodsFilter 商品列表筛选条件
type GoodsFilter struct {
	CategoryID   *uint
	SupplierID   *uint
	Name         string // 模糊搜索
	SerialNumber string // 精确匹配
	Status       *int8
	Page         int
	PageSize     int
}

// GoodsRepository 商品数据访问层
type GoodsRepository struct {
	db *gorm.DB
}

// NewGoodsRepository 创建商品仓储实例
func NewGoodsRepository(db *gorm.DB) *GoodsRepository {
	return &GoodsRepository{db: db}
}

// Create 创建商品
func (r *GoodsRepository) Create(ctx context.Context, goods *Goods) error {
	return r.db.WithContext(ctx).Create(goods).Error
}

// Update 更新商品
func (r *GoodsRepository) Update(ctx context.Context, goods *Goods) error {
	return r.db.WithContext(ctx).Save(goods).Error
}

// FindByID 根据ID查询商品
func (r *GoodsRepository) FindByID(ctx context.Context, id uint) (*Goods, error) {
	var goods Goods
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&goods).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("商品不存在")
		}
		return nil, fmt.Errorf("查询商品失败: %w", err)
	}
	return &goods, nil
}

// FindBySN 根据序列号查询商品
func (r *GoodsRepository) FindBySN(ctx context.Context, sn string) (*Goods, error) {
	var goods Goods
	err := r.db.WithContext(ctx).Where("serial_number = ?", sn).First(&goods).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("商品不存在")
		}
		return nil, fmt.Errorf("查询商品失败: %w", err)
	}
	return &goods, nil
}

// List 商品列表查询
func (r *GoodsRepository) List(ctx context.Context, filter GoodsFilter) ([]*Goods, int64, error) {
	query := r.db.WithContext(ctx).Model(&Goods{})

	if filter.CategoryID != nil {
		query = query.Where("category_id = ?", *filter.CategoryID)
	}
	if filter.SupplierID != nil {
		query = query.Where("supplier_id = ?", *filter.SupplierID)
	}
	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	if filter.SerialNumber != "" {
		query = query.Where("serial_number = ?", filter.SerialNumber)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询商品总数失败: %w", err)
	}

	// 分页
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var list []*Goods
	err := query.Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error
	if err != nil {
		return nil, 0, fmt.Errorf("查询商品列表失败: %w", err)
	}

	return list, total, nil
}

// DeductStock 扣减库存（事务内调用）
// 使用 stock>=quantity 条件保证并发安全，RowsAffected=0 表示库存不足
func (r *GoodsRepository) DeductStock(ctx context.Context, tx *gorm.DB, goodsID uint, quantity int) error {
	result := tx.WithContext(ctx).
		Model(&Goods{}).
		Where("id = ? AND stock >= ?", goodsID, quantity).
		Update("stock", gorm.Expr("stock - ?", quantity))

	if result.Error != nil {
		return fmt.Errorf("扣减库存失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("库存不足")
	}
	return nil
}

// AddStock 增加库存
func (r *GoodsRepository) AddStock(ctx context.Context, goodsID uint, quantity int) error {
	result := r.db.WithContext(ctx).
		Model(&Goods{}).
		Where("id = ?", goodsID).
		Update("stock", gorm.Expr("stock + ?", quantity))

	if result.Error != nil {
		return fmt.Errorf("增加库存失败: %w", result.Error)
	}
	return nil
}

// GetCategories 获取分类列表
func (r *GoodsRepository) GetCategories(ctx context.Context, parentID *uint) ([]*GoodsCategory, error) {
	query := r.db.WithContext(ctx).Where("status = 1")
	if parentID != nil {
		query = query.Where("parent_id = ?", *parentID)
	} else {
		query = query.Where("parent_id = 0")
	}

	var categories []*GoodsCategory
	err := query.Order("sort_order ASC, id ASC").Find(&categories).Error
	if err != nil {
		return nil, fmt.Errorf("查询分类失败: %w", err)
	}
	return categories, nil
}

// CreateCategory 创建分类
func (r *GoodsRepository) CreateCategory(ctx context.Context, cat *GoodsCategory) error {
	return r.db.WithContext(ctx).Create(cat).Error
}

// GetBuyParams 获取商品购买参数定义
func (r *GoodsRepository) GetBuyParams(ctx context.Context, goodsID uint) ([]*GoodsBuyParam, error) {
	var params []*GoodsBuyParam
	err := r.db.WithContext(ctx).
		Where("goods_id = ?", goodsID).
		Order("sort_order ASC, id ASC").
		Find(&params).Error
	if err != nil {
		return nil, fmt.Errorf("查询购买参数失败: %w", err)
	}
	return params, nil
}
