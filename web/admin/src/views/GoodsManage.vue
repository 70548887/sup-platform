<template>
  <div class="goods-manage">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>商品管理</span>
          <div class="header-actions">
            <el-input
              v-model="searchQuery"
              placeholder="搜索商品"
              style="width: 200px; margin-right: 12px"
              clearable
            />
            <el-button type="primary" @click="handleAdd">新增商品</el-button>
          </div>
        </div>
      </template>
      <el-table :data="filteredGoods" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="商品名称" />
        <el-table-column prop="category" label="分类" width="120" />
        <el-table-column prop="price" label="价格" width="100" />
        <el-table-column prop="stock" label="库存" width="100" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === '上架' ? 'success' : 'info'">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180">
          <template #default>
            <el-button size="small" type="primary" link>编辑</el-button>
            <el-button size="small" type="danger" link>删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'

const searchQuery = ref('')

const goodsList = ref([
  { id: 1, name: 'Steam充值卡50元', category: '游戏充值', price: '¥48.00', stock: 200, status: '上架' },
  { id: 2, name: 'Netflix月卡', category: '影音娱乐', price: '¥35.00', stock: 150, status: '上架' },
  { id: 3, name: 'App Store礼品卡100元', category: '应用商店', price: '¥95.00', stock: 0, status: '下架' },
  { id: 4, name: 'Spotify会员月卡', category: '影音娱乐', price: '¥15.00', stock: 300, status: '上架' },
  { id: 5, name: 'Google Play卡200元', category: '应用商店', price: '¥190.00', stock: 80, status: '上架' },
])

const filteredGoods = computed(() => {
  if (!searchQuery.value) return goodsList.value
  return goodsList.value.filter((g) =>
    g.name.includes(searchQuery.value)
  )
})

const handleAdd = () => {
  ElMessage.info('新增商品功能开发中')
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  align-items: center;
}
</style>
