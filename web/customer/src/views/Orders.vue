<script setup lang="ts">
import { ref, reactive } from 'vue'

interface Order {
  id: string
  productName: string
  quantity: number
  amount: string
  status: string
  createdAt: string
}

const searchForm = reactive({
  orderId: '',
  status: '',
  dateRange: [] as string[],
})

const orders = ref<Order[]>([
  {
    id: 'ORD202401150001',
    productName: 'Steam充值卡 50元',
    quantity: 2,
    amount: '98.00',
    status: 'completed',
    createdAt: '2024-01-15 14:30:00',
  },
  {
    id: 'ORD202401150002',
    productName: 'Netflix会员月卡',
    quantity: 1,
    amount: '45.00',
    status: 'processing',
    createdAt: '2024-01-15 15:20:00',
  },
  {
    id: 'ORD202401150003',
    productName: 'iTunes礼品卡 100元',
    quantity: 3,
    amount: '285.00',
    status: 'failed',
    createdAt: '2024-01-15 16:10:00',
  },
])

const loading = ref(false)

const statusOptions = [
  { label: '全部', value: '' },
  { label: '处理中', value: 'processing' },
  { label: '已完成', value: 'completed' },
  { label: '失败', value: 'failed' },
]

const getStatusType = (status: string) => {
  const map: Record<string, string> = {
    processing: 'warning',
    completed: 'success',
    failed: 'danger',
  }
  return map[status] || 'info'
}

const getStatusLabel = (status: string) => {
  const map: Record<string, string> = {
    processing: '处理中',
    completed: '已完成',
    failed: '失败',
  }
  return map[status] || status
}

const handleSearch = () => {
  loading.value = true
  setTimeout(() => {
    loading.value = false
  }, 500)
}

const handleReset = () => {
  searchForm.orderId = ''
  searchForm.status = ''
  searchForm.dateRange = []
}
</script>

<template>
  <div class="orders-page">
    <h2>订单查询</h2>

    <el-form :model="searchForm" inline class="search-form">
      <el-form-item label="订单号">
        <el-input
          v-model="searchForm.orderId"
          placeholder="请输入订单号"
          clearable
          style="width: 200px"
        />
      </el-form-item>
      <el-form-item label="状态">
        <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 120px">
          <el-option
            v-for="opt in statusOptions"
            :key="opt.value"
            :label="opt.label"
            :value="opt.value"
          />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="handleSearch">搜索</el-button>
        <el-button @click="handleReset">重置</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="orders" stripe :loading="loading" style="width: 100%">
      <el-table-column prop="id" label="订单号" min-width="180" />
      <el-table-column prop="productName" label="商品名称" min-width="160" />
      <el-table-column prop="quantity" label="数量" width="80" align="center" />
      <el-table-column prop="amount" label="金额" width="100" align="right">
        <template #default="{ row }">
          ¥{{ row.amount }}
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ getStatusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="创建时间" width="180" />
    </el-table>
  </div>
</template>

<style scoped>
h2 {
  margin: 0 0 20px;
  font-size: 22px;
  color: #303133;
}

.search-form {
  margin-bottom: 20px;
  padding: 16px;
  background: #fff;
  border-radius: 4px;
}
</style>
