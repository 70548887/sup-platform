package order

const (
	StatusPaid         int8 = 1 // 已付款
	StatusPending      int8 = 2 // 待处理
	StatusProcessing   int8 = 3 // 处理中
	StatusReplenishing int8 = 4 // 补单中
	StatusRefunding    int8 = 5 // 退单中
	StatusCompleted    int8 = 6 // 已完成（终态）
	StatusReturned     int8 = 7 // 已退单（终态）
	StatusRefunded     int8 = 8 // 已退款（终态）
	StatusAbnormal     int8 = 9 // 有异常
)

// ValidTransitions 合法状态转移表
var ValidTransitions = map[int8][]int8{
	StatusPaid:         {StatusPending, StatusProcessing},
	StatusPending:      {StatusProcessing},
	StatusProcessing:   {StatusCompleted, StatusReplenishing, StatusRefunding, StatusAbnormal},
	StatusReplenishing: {StatusProcessing, StatusCompleted},
	StatusRefunding:    {StatusReturned},
	StatusReturned:     {StatusRefunded},
	StatusAbnormal:     {StatusProcessing, StatusCompleted, StatusRefunding},
}

// IsValidTransition 检查状态转移是否合法
func IsValidTransition(from, to int8) bool {
	targets, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// IsTerminalStatus 判断是否是终态
func IsTerminalStatus(status int8) bool {
	return status == StatusCompleted || status == StatusReturned || status == StatusRefunded
}
