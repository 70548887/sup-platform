package adapter

import (
	"fmt"
	"sync"
)

// Factory 适配器工厂（根据SupplierID获取对应适配器）
type Factory struct {
	mu       sync.RWMutex
	adapters map[uint]DockingAdapter // SupplierID → Adapter
}

// NewFactory 创建适配器工厂
func NewFactory() *Factory {
	return &Factory{
		adapters: make(map[uint]DockingAdapter),
	}
}

// Register 注册适配器
func (f *Factory) Register(supplierID uint, adapter DockingAdapter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.adapters[supplierID] = adapter
}

// Get 获取适配器
func (f *Factory) Get(supplierID uint) (DockingAdapter, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	a, ok := f.adapters[supplierID]
	if !ok {
		return nil, fmt.Errorf("adapter: no adapter registered for supplier %d", supplierID)
	}
	return a, nil
}

// List 列出所有已注册的适配器
func (f *Factory) List() map[uint]string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make(map[uint]string, len(f.adapters))
	for id, a := range f.adapters {
		result[id] = a.Name()
	}
	return result
}
