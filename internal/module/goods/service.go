package goods

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// CreateGoodsParams 创建商品参数
type CreateGoodsParams struct {
	CategoryID    uint
	SupplierID    uint
	Name          string
	Description   string
	Price         decimal.Decimal
	CostPrice     decimal.Decimal
	Unit          string
	BuyMin        int
	BuyMax        int
	BuyBase       int
	IsCardProduct bool
	Images        string
}

// UpdateGoodsParams 修改商品参数
type UpdateGoodsParams struct {
	Name        *string
	Description *string
	Price       *decimal.Decimal
	CostPrice   *decimal.Decimal
	CategoryID  *uint
	Unit        *string
	BuyMin      *int
	BuyMax      *int
	BuyBase     *int
	IsClose     *bool
	IsRepeat    *bool
	Images      *string
	Status      *int8
}

// GoodsService 商品服务层
type GoodsService struct {
	repo *GoodsRepository
	db   *gorm.DB
}

// NewGoodsService 创建商品服务实例
func NewGoodsService(db *gorm.DB) *GoodsService {
	return &GoodsService{
		repo: NewGoodsRepository(db),
		db:   db,
	}
}

// 序列号计数器（进程内序号，避免同一秒内重复）
var serialSeq uint64

// GenerateSerialNumber 生成商品编号 格式：G+年月日+4位序号
func (s *GoodsService) GenerateSerialNumber() string {
	now := time.Now()
	seq := atomic.AddUint64(&serialSeq, 1) % 10000
	return fmt.Sprintf("G%s%04d", now.Format("20060102"), seq)
}

// CreateGoods 创建商品
func (s *GoodsService) CreateGoods(ctx context.Context, params CreateGoodsParams) (*Goods, error) {
	if params.Name == "" {
		return nil, fmt.Errorf("商品名称不能为空")
	}
	if params.Price.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("商品价格必须大于0")
	}
	if params.SupplierID == 0 {
		return nil, fmt.Errorf("供货商ID不能为空")
	}

	unit := params.Unit
	if unit == "" {
		unit = "件"
	}
	buyMin := params.BuyMin
	if buyMin < 1 {
		buyMin = 1
	}
	buyMax := params.BuyMax
	if buyMax < 1 {
		buyMax = 100
	}
	buyBase := params.BuyBase
	if buyBase < 1 {
		buyBase = 1
	}

	goods := &Goods{
		SerialNumber:  s.GenerateSerialNumber(),
		CategoryID:    params.CategoryID,
		SupplierID:    params.SupplierID,
		Name:          params.Name,
		Description:   params.Description,
		Price:         params.Price,
		CostPrice:     params.CostPrice,
		Unit:          unit,
		BuyMin:        buyMin,
		BuyMax:        buyMax,
		BuyBase:       buyBase,
		IsCardProduct: params.IsCardProduct,
		Images:        params.Images,
		Status:        1,
	}

	if err := s.repo.Create(ctx, goods); err != nil {
		// 序列号冲突则重试一次
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			goods.SerialNumber = s.GenerateSerialNumber()
			if err2 := s.repo.Create(ctx, goods); err2 != nil {
				return nil, fmt.Errorf("创建商品失败: %w", err2)
			}
			return goods, nil
		}
		return nil, fmt.Errorf("创建商品失败: %w", err)
	}
	return goods, nil
}

// UpdateGoods 修改商品
func (s *GoodsService) UpdateGoods(ctx context.Context, goodsSN string, params UpdateGoodsParams) error {
	goods, err := s.repo.FindBySN(ctx, goodsSN)
	if err != nil {
		return err
	}

	if params.Name != nil {
		if *params.Name == "" {
			return fmt.Errorf("商品名称不能为空")
		}
		goods.Name = *params.Name
	}
	if params.Description != nil {
		goods.Description = *params.Description
	}
	if params.Price != nil {
		if params.Price.LessThanOrEqual(decimal.Zero) {
			return fmt.Errorf("商品价格必须大于0")
		}
		goods.Price = *params.Price
	}
	if params.CostPrice != nil {
		goods.CostPrice = *params.CostPrice
	}
	if params.CategoryID != nil {
		goods.CategoryID = *params.CategoryID
	}
	if params.Unit != nil {
		goods.Unit = *params.Unit
	}
	if params.BuyMin != nil {
		goods.BuyMin = *params.BuyMin
	}
	if params.BuyMax != nil {
		goods.BuyMax = *params.BuyMax
	}
	if params.BuyBase != nil {
		goods.BuyBase = *params.BuyBase
	}
	if params.IsClose != nil {
		goods.IsClose = *params.IsClose
	}
	if params.IsRepeat != nil {
		goods.IsRepeat = *params.IsRepeat
	}
	if params.Images != nil {
		goods.Images = *params.Images
	}
	if params.Status != nil {
		goods.Status = *params.Status
	}

	return s.repo.Update(ctx, goods)
}

// GetGoods 获取商品详情
func (s *GoodsService) GetGoods(ctx context.Context, goodsSN string) (*Goods, error) {
	return s.repo.FindBySN(ctx, goodsSN)
}

// ListGoods 商品列表
func (s *GoodsService) ListGoods(ctx context.Context, filter GoodsFilter) ([]*Goods, int64, error) {
	return s.repo.List(ctx, filter)
}

// DeductStock 扣库存（事务内调用）
func (s *GoodsService) DeductStock(ctx context.Context, tx *gorm.DB, goodsID uint, quantity int) error {
	if quantity <= 0 {
		return fmt.Errorf("扣减数量必须大于0")
	}
	return s.repo.DeductStock(ctx, tx, goodsID, quantity)
}

// ValidateBuyParams 验证购买参数
func (s *GoodsService) ValidateBuyParams(goods *Goods, params map[string]string) error {
	// 获取商品定义的购买参数
	buyParams, err := s.repo.GetBuyParams(context.Background(), goods.ID)
	if err != nil {
		return err
	}

	for _, bp := range buyParams {
		value, exists := params[bp.Name]

		// 必填校验
		if bp.Required && (!exists || strings.TrimSpace(value) == "") {
			return fmt.Errorf("参数 %s 为必填项", bp.Name)
		}

		if !exists || value == "" {
			continue
		}

		// 长度校验
		if bp.MinLength > 0 && len(value) < bp.MinLength {
			return fmt.Errorf("参数 %s 长度不能小于 %d", bp.Name, bp.MinLength)
		}
		if bp.MaxLength > 0 && len(value) > bp.MaxLength {
			return fmt.Errorf("参数 %s 长度不能大于 %d", bp.Name, bp.MaxLength)
		}

		// 类型校验
		if err := validateParamType(bp.Type, bp.Name, value); err != nil {
			return err
		}
	}

	return nil
}

// validateParamType 校验参数类型
func validateParamType(paramType, name, value string) error {
	switch paramType {
	case "email":
		if _, err := mail.ParseAddress(value); err != nil {
			return fmt.Errorf("参数 %s 必须是有效的邮箱地址", name)
		}
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("参数 %s 必须是数字", name)
		}
	case "phone":
		phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)
		if !phoneRegex.MatchString(value) {
			return fmt.Errorf("参数 %s 必须是有效的手机号", name)
		}
	case "qq":
		qqRegex := regexp.MustCompile(`^[1-9]\d{4,10}$`)
		if !qqRegex.MatchString(value) {
			return fmt.Errorf("参数 %s 必须是有效的QQ号", name)
		}
	case "text":
		// text类型无特殊校验
	}
	return nil
}

// ValidatePurchase 验证购买规则（数量限制、是否关闭、是否重复）
func (s *GoodsService) ValidatePurchase(goods *Goods, buyNumber int, customerID uint) error {
	// 检查商品是否关闭下单
	if goods.IsClose {
		return fmt.Errorf("该商品已关闭下单")
	}

	// 检查商品状态
	if goods.Status != 1 {
		return fmt.Errorf("该商品已下架")
	}

	// 检查购买数量范围
	if buyNumber < goods.BuyMin {
		return fmt.Errorf("最小购买数量为 %d", goods.BuyMin)
	}
	if buyNumber > goods.BuyMax {
		return fmt.Errorf("最大购买数量为 %d", goods.BuyMax)
	}

	// 检查购买基数（购买数量必须是基数的整数倍）
	if goods.BuyBase > 1 && buyNumber%goods.BuyBase != 0 {
		return fmt.Errorf("购买数量必须是 %d 的整数倍", goods.BuyBase)
	}

	// 检查库存
	if goods.Stock < buyNumber {
		return fmt.Errorf("库存不足，当前库存: %d", goods.Stock)
	}

	// 检查是否允许重复下单
	if !goods.IsRepeat && customerID > 0 {
		var count int64
		s.db.Table("orders").
			Where("goods_id = ? AND customer_id = ? AND status NOT IN (7, 8, 9)", goods.ID, customerID).
			Count(&count)
		if count > 0 {
			return fmt.Errorf("该商品不允许重复下单")
		}
	}

	return nil
}

// GetCategories 获取分类列表
func (s *GoodsService) GetCategories(ctx context.Context, parentID *uint) ([]*GoodsCategory, error) {
	return s.repo.GetCategories(ctx, parentID)
}

// CreateCategory 创建分类
func (s *GoodsService) CreateCategory(ctx context.Context, name string, parentID uint) (*GoodsCategory, error) {
	if name == "" {
		return nil, fmt.Errorf("分类名称不能为空")
	}

	cat := &GoodsCategory{
		ParentID: parentID,
		Name:     name,
		Status:   1,
	}

	if err := s.repo.CreateCategory(ctx, cat); err != nil {
		return nil, fmt.Errorf("创建分类失败: %w", err)
	}
	return cat, nil
}
