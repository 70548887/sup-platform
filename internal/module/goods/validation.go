package goods

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// GoodsValidationService 商品验证服务
type GoodsValidationService struct {
	repo *GoodsRepository
	db   *gorm.DB
}

// NewGoodsValidationService 创建商品验证服务实例
func NewGoodsValidationService(repo *GoodsRepository, db *gorm.DB) *GoodsValidationService {
	return &GoodsValidationService{repo: repo, db: db}
}

// ValidateBuyParams 验证购买参数
func (v *GoodsValidationService) ValidateBuyParams(goods *Goods, params map[string]string) error {
	// 获取商品定义的购买参数
	buyParams, err := v.repo.GetBuyParams(context.Background(), goods.ID)
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
func (v *GoodsValidationService) ValidatePurchase(goods *Goods, buyNumber int, customerID uint) error {
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
		v.db.Table("orders").
			Where("goods_id = ? AND customer_id = ? AND status NOT IN (7, 8, 9)", goods.ID, customerID).
			Count(&count)
		if count > 0 {
			return fmt.Errorf("该商品不允许重复下单")
		}
	}

	return nil
}
