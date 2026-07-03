<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

interface FlowRecord {
  id: number
  type: string
  amount: string
  balance: string
  remark: string
  createdAt: string
}

const balance = ref('8,520.00')
const frozenBalance = ref('350.00')

const flowRecords = ref<FlowRecord[]>([
  {
    id: 1,
    type: 'income',
    amount: '+98.00',
    balance: '8,520.00',
    remark: '订单收入 ORD202401150001',
    createdAt: '2024-01-15 14:30:00',
  },
  {
    id: 2,
    type: 'income',
    amount: '+45.00',
    balance: '8,422.00',
    remark: '订单收入 ORD202401150002',
    createdAt: '2024-01-15 13:20:00',
  },
  {
    id: 3,
    type: 'withdraw',
    amount: '-2000.00',
    balance: '8,377.00',
    remark: '提现到银行卡',
    createdAt: '2024-01-14 16:00:00',
  },
])

const withdrawDialogVisible = ref(false)
const withdrawAmount = ref('')

const handleWithdraw = () => {
  withdrawDialogVisible.value = true
}

const submitWithdraw = () => {
  if (!withdrawAmount.value || Number(withdrawAmount.value) <= 0) {
    ElMessage.error('请输入有效金额')
    return
  }
  ElMessage.success(`提现申请已提交，金额：¥${withdrawAmount.value}`)
  withdrawDialogVisible.value = false
  withdrawAmount.value = ''
}

const getFlowTypeLabel = (type: string) => {
  return type === 'income' ? '收入' : '提现'
}

const getFlowTypeTag = (type: string) => {
  return type === 'income' ? 'success' : 'warning'
}
</script>

<template>
  <div class="account-page">
    <div class="balance-cards">
      <div class="balance-card primary">
        <span class="label">可用余额</span>
        <span class="amount">¥ {{ balance }}</span>
      </div>
      <div class="balance-card secondary">
        <span class="label">冻结金额</span>
        <span class="amount">¥ {{ frozenBalance }}</span>
      </div>
      <div class="balance-card action">
        <el-button type="primary" size="large" @click="handleWithdraw">申请提现</el-button>
      </div>
    </div>

    <div class="flow-section">
      <h3>资金流水</h3>
      <el-table :data="flowRecords" stripe style="width: 100%">
        <el-table-column prop="type" label="类型" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="getFlowTypeTag(row.type)" size="small">
              {{ getFlowTypeLabel(row.type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="120" align="right">
          <template #default="{ row }">
            <span :style="{ color: row.type === 'income' ? '#67c23a' : '#e6a23c' }">
              {{ row.amount }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="balance" label="余额" width="120" align="right">
          <template #default="{ row }">
            ¥{{ row.balance }}
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="200" />
        <el-table-column prop="createdAt" label="时间" width="180" />
      </el-table>
    </div>

    <!-- 提现弹窗 -->
    <el-dialog v-model="withdrawDialogVisible" title="申请提现" width="400px">
      <el-form>
        <el-form-item label="提现金额">
          <el-input
            v-model="withdrawAmount"
            placeholder="请输入提现金额"
            type="number"
            min="1"
          >
            <template #prepend>¥</template>
          </el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="withdrawDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitWithdraw">确认提现</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.balance-cards {
  display: flex;
  gap: 20px;
  margin-bottom: 32px;
}

.balance-card {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  display: flex;
  flex-direction: column;
  min-width: 180px;
}

.balance-card.primary {
  background: linear-gradient(135deg, #4A7EFF 0%, #6C9AFF 100%);
}

.balance-card.primary .label {
  color: rgba(255, 255, 255, 0.8);
}

.balance-card.primary .amount {
  color: #fff;
}

.balance-card.secondary {
  border: 1px solid #e4e7ed;
}

.balance-card.secondary .label {
  color: #909399;
}

.balance-card.secondary .amount {
  color: #303133;
}

.balance-card.action {
  justify-content: center;
  align-items: center;
}

.balance-card .label {
  font-size: 14px;
  margin-bottom: 8px;
}

.balance-card .amount {
  font-size: 28px;
  font-weight: 600;
}

.flow-section h3 {
  font-size: 16px;
  color: #303133;
  margin-bottom: 16px;
}
</style>
