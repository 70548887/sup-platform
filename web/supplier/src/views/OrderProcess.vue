<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

interface Order {
  id: string
  productName: string
  quantity: number
  buyerNote: string
  status: string
  createdAt: string
}

const orders = ref<Order[]>([
  {
    id: 'ORD202401150001',
    productName: 'Steam充值卡 50元',
    quantity: 2,
    buyerNote: '急需，请尽快处理',
    status: 'pending',
    createdAt: '2024-01-15 14:30:00',
  },
  {
    id: 'ORD202401150004',
    productName: 'Netflix会员月卡',
    quantity: 1,
    buyerNote: '',
    status: 'pending',
    createdAt: '2024-01-15 15:45:00',
  },
  {
    id: 'ORD202401150002',
    productName: 'Steam充值卡 50元',
    quantity: 1,
    buyerNote: '',
    status: 'processing',
    createdAt: '2024-01-15 13:20:00',
  },
  {
    id: 'ORD202401150003',
    productName: 'iTunes礼品卡 100元',
    quantity: 3,
    buyerNote: '',
    status: 'completed',
    createdAt: '2024-01-15 10:10:00',
  },
])

const getStatusType = (status: string) => {
  const map: Record<string, string> = {
    pending: 'danger',
    processing: 'warning',
    completed: 'success',
  }
  return map[status] || 'info'
}

const getStatusLabel = (status: string) => {
  const map: Record<string, string> = {
    pending: '待处理',
    processing: '处理中',
    completed: '已完成',
  }
  return map[status] || status
}

const handleProcess = (row: Order) => {
  row.status = 'processing'
  ElMessage.success('订单已开始处理')
}

const handleComplete = (row: Order) => {
  ElMessageBox.confirm('确认此订单已完成发货？', '确认', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'info',
  }).then(() => {
    row.status = 'completed'
    ElMessage.success('订单已标记完成')
  }).catch(() => {})
}
</script>

<template>
  <div class="order-process-page">
    <el-table :data="orders" stripe style="width: 100%">
      <el-table-column prop="id" label="订单号" min-width="160" />
      <el-table-column prop="productName" label="商品名称" min-width="150" />
      <el-table-column prop="quantity" label="数量" width="70" align="center" />
      <el-table-column prop="buyerNote" label="买家备注" min-width="150">
        <template #default="{ row }">
          {{ row.buyerNote || '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ getStatusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="下单时间" width="170" />
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.status === 'pending'"
            type="primary"
            size="small"
            link
            @click="handleProcess(row)"
          >
            开始处理
          </el-button>
          <el-button
            v-if="row.status === 'processing'"
            type="success"
            size="small"
            link
            @click="handleComplete(row)"
          >
            标记完成
          </el-button>
          <span v-if="row.status === 'completed'" class="done-text">已完成</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
.done-text {
  color: #909399;
  font-size: 13px;
}
</style>
