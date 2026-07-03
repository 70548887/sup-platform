<template>
  <div class="dashboard">
    <el-row :gutter="20" class="stat-cards">
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header>
            <span>订单数</span>
          </template>
          <div class="stat-value">{{ stats.orderCount }}</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header>
            <span>销售额</span>
          </template>
          <div class="stat-value">¥{{ stats.salesAmount }}</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header>
            <span>用户数</span>
          </template>
          <div class="stat-value">{{ stats.userCount }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="recent-orders" shadow="never">
      <template #header>
        <span>最近订单</span>
      </template>
      <el-table :data="recentOrders" stripe>
        <el-table-column prop="orderNo" label="订单号" />
        <el-table-column prop="product" label="商品" />
        <el-table-column prop="amount" label="金额" />
        <el-table-column prop="status" label="状态">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="时间" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive } from 'vue'

const stats = reactive({
  orderCount: 1280,
  salesAmount: '52,800.00',
  userCount: 3650,
})

const recentOrders = reactive([
  { orderNo: 'ORD20240101001', product: 'Steam充值卡50元', amount: '¥48.00', status: '已完成', createdAt: '2024-01-01 12:00' },
  { orderNo: 'ORD20240101002', product: 'Netflix月卡', amount: '¥35.00', status: '待发货', createdAt: '2024-01-01 12:30' },
  { orderNo: 'ORD20240101003', product: 'App Store礼品卡', amount: '¥100.00', status: '已完成', createdAt: '2024-01-01 13:00' },
  { orderNo: 'ORD20240101004', product: 'Spotify会员', amount: '¥15.00', status: '已退款', createdAt: '2024-01-01 14:00' },
  { orderNo: 'ORD20240101005', product: 'Google Play卡', amount: '¥200.00', status: '待付款', createdAt: '2024-01-01 15:00' },
])

const getStatusType = (status: string) => {
  const map: Record<string, string> = {
    '已完成': 'success',
    '待发货': 'warning',
    '已退款': 'danger',
    '待付款': 'info',
  }
  return map[status] || ''
}
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.stat-cards {
  margin-bottom: 20px;
}

.stat-value {
  font-size: 28px;
  font-weight: bold;
  color: #409EFF;
}

.recent-orders {
  margin-top: 0;
}
</style>
