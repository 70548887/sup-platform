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

const balance = ref('1,280.50')

const flowRecords = ref<FlowRecord[]>([
  {
    id: 1,
    type: 'recharge',
    amount: '+500.00',
    balance: '1,280.50',
    remark: '在线充值',
    createdAt: '2024-01-15 10:00:00',
  },
  {
    id: 2,
    type: 'consume',
    amount: '-98.00',
    balance: '780.50',
    remark: '订单消费 ORD202401150001',
    createdAt: '2024-01-15 14:30:00',
  },
  {
    id: 3,
    type: 'recharge',
    amount: '+1000.00',
    balance: '1,780.50',
    remark: '在线充值',
    createdAt: '2024-01-14 09:15:00',
  },
])

const rechargeDialogVisible = ref(false)
const rechargeAmount = ref('')

const handleRecharge = () => {
  rechargeDialogVisible.value = true
}

const submitRecharge = () => {
  if (!rechargeAmount.value || Number(rechargeAmount.value) <= 0) {
    ElMessage.error('请输入有效金额')
    return
  }
  ElMessage.success(`充值申请已提交，金额：¥${rechargeAmount.value}`)
  rechargeDialogVisible.value = false
  rechargeAmount.value = ''
}

const getFlowTypeLabel = (type: string) => {
  return type === 'recharge' ? '充值' : '消费'
}

const getFlowTypeTag = (type: string) => {
  return type === 'recharge' ? 'success' : 'warning'
}
</script>

<template>
  <div class="account-page">
    <h2>账户余额</h2>

    <div class="balance-card">
      <div class="balance-info">
        <span class="balance-label">可用余额</span>
        <span class="balance-amount">¥ {{ balance }}</span>
      </div>
      <el-button type="primary" size="large" @click="handleRecharge">
        充值
      </el-button>
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
            <span :style="{ color: row.type === 'recharge' ? '#67c23a' : '#e6a23c' }">
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

    <!-- 充值弹窗 -->
    <el-dialog v-model="rechargeDialogVisible" title="账户充值" width="400px">
      <el-form>
        <el-form-item label="充值金额">
          <el-input
            v-model="rechargeAmount"
            placeholder="请输入充值金额"
            type="number"
            min="1"
          >
            <template #prepend>¥</template>
          </el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rechargeDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitRecharge">确认充值</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
h2 {
  margin: 0 0 20px;
  font-size: 22px;
  color: #303133;
}

.balance-card {
  background: linear-gradient(135deg, #4A7EFF 0%, #6C9AFF 100%);
  border-radius: 12px;
  padding: 32px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}

.balance-info {
  display: flex;
  flex-direction: column;
}

.balance-label {
  color: rgba(255, 255, 255, 0.8);
  font-size: 14px;
  margin-bottom: 8px;
}

.balance-amount {
  color: #fff;
  font-size: 36px;
  font-weight: 600;
}

.balance-card .el-button {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.4);
  color: #fff;
}

.balance-card .el-button:hover {
  background: rgba(255, 255, 255, 0.3);
}

.flow-section h3 {
  font-size: 16px;
  color: #303133;
  margin-bottom: 16px;
}
</style>
