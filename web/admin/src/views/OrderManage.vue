<template>
  <div class="order-manage">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>订单管理</span>
          <el-select v-model="statusFilter" placeholder="状态筛选" clearable style="width: 150px">
            <el-option label="全部" value="" />
            <el-option label="待付款" value="待付款" />
            <el-option label="待发货" value="待发货" />
            <el-option label="已完成" value="已完成" />
            <el-option label="已退款" value="已退款" />
          </el-select>
        </div>
      </template>
      <el-table :data="filteredOrders" stripe>
        <el-table-column prop="orderNo" label="订单号" />
        <el-table-column prop="product" label="商品" />
        <el-table-column prop="buyer" label="买家" width="120" />
        <el-table-column prop="amount" label="金额" width="100" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="下单时间" width="160" />
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="showDetail(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="detailVisible" title="订单详情" width="500px">
      <el-descriptions :column="1" border v-if="currentOrder">
        <el-descriptions-item label="订单号">{{ currentOrder.orderNo }}</el-descriptions-item>
        <el-descriptions-item label="商品">{{ currentOrder.product }}</el-descriptions-item>
        <el-descriptions-item label="买家">{{ currentOrder.buyer }}</el-descriptions-item>
        <el-descriptions-item label="金额">{{ currentOrder.amount }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ currentOrder.status }}</el-descriptions-item>
        <el-descriptions-item label="下单时间">{{ currentOrder.createdAt }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

interface Order {
  orderNo: string
  product: string
  buyer: string
  amount: string
  status: string
  createdAt: string
}

const statusFilter = ref('')
const detailVisible = ref(false)
const currentOrder = ref<Order | null>(null)

const orderList = ref<Order[]>([
  { orderNo: 'ORD20240101001', product: 'Steam充值卡50元', buyer: '用户A', amount: '¥48.00', status: '已完成', createdAt: '2024-01-01 12:00' },
  { orderNo: 'ORD20240101002', product: 'Netflix月卡', buyer: '用户B', amount: '¥35.00', status: '待发货', createdAt: '2024-01-01 12:30' },
  { orderNo: 'ORD20240101003', product: 'App Store礼品卡', buyer: '用户C', amount: '¥100.00', status: '已完成', createdAt: '2024-01-01 13:00' },
  { orderNo: 'ORD20240101004', product: 'Spotify会员', buyer: '用户D', amount: '¥15.00', status: '已退款', createdAt: '2024-01-01 14:00' },
  { orderNo: 'ORD20240101005', product: 'Google Play卡', buyer: '用户E', amount: '¥200.00', status: '待付款', createdAt: '2024-01-01 15:00' },
])

const filteredOrders = computed(() => {
  if (!statusFilter.value) return orderList.value
  return orderList.value.filter((o) => o.status === statusFilter.value)
})

const getStatusType = (status: string) => {
  const map: Record<string, string> = {
    '已完成': 'success',
    '待发货': 'warning',
    '已退款': 'danger',
    '待付款': 'info',
  }
  return map[status] || ''
}

const showDetail = (order: Order) => {
  currentOrder.value = order
  detailVisible.value = true
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
