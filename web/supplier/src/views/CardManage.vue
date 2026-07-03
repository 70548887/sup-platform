<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

interface CardBatch {
  id: number
  productName: string
  batchNo: string
  totalCount: number
  usedCount: number
  createdAt: string
}

const batches = ref<CardBatch[]>([
  {
    id: 1,
    productName: 'Steam充值卡 50元',
    batchNo: 'BATCH-20240115-001',
    totalCount: 100,
    usedCount: 35,
    createdAt: '2024-01-15 10:00:00',
  },
  {
    id: 2,
    productName: 'Netflix会员月卡',
    batchNo: 'BATCH-20240115-002',
    totalCount: 50,
    usedCount: 12,
    createdAt: '2024-01-15 11:30:00',
  },
])

const totalStock = ref(103)

const importDialogVisible = ref(false)
const importForm = ref({
  productName: '',
  content: '',
})

const handleImport = () => {
  importDialogVisible.value = true
}

const submitImport = () => {
  if (!importForm.value.productName || !importForm.value.content) {
    ElMessage.error('请填写完整信息')
    return
  }
  const lines = importForm.value.content.split('\n').filter(l => l.trim())
  ElMessage.success(`成功导入 ${lines.length} 张卡密`)
  importDialogVisible.value = false
  importForm.value = { productName: '', content: '' }
}
</script>

<template>
  <div class="card-manage-page">
    <div class="page-header">
      <div class="stock-info">
        <span class="stock-label">总库存：</span>
        <span class="stock-count">{{ totalStock }}</span>
      </div>
      <el-button type="primary" @click="handleImport">导入卡密</el-button>
    </div>

    <el-table :data="batches" stripe style="width: 100%">
      <el-table-column prop="batchNo" label="批次号" min-width="180" />
      <el-table-column prop="productName" label="关联商品" min-width="160" />
      <el-table-column prop="totalCount" label="总数" width="80" align="center" />
      <el-table-column prop="usedCount" label="已使用" width="80" align="center" />
      <el-table-column label="剩余" width="80" align="center">
        <template #default="{ row }">
          <el-tag :type="row.totalCount - row.usedCount > 0 ? 'success' : 'danger'" size="small">
            {{ row.totalCount - row.usedCount }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="导入时间" width="180" />
    </el-table>

    <!-- 导入弹窗 -->
    <el-dialog v-model="importDialogVisible" title="导入卡密" width="550px">
      <el-form :model="importForm" label-width="80px">
        <el-form-item label="关联商品">
          <el-select v-model="importForm.productName" placeholder="请选择商品" style="width: 100%">
            <el-option label="Steam充值卡 50元" value="Steam充值卡 50元" />
            <el-option label="Netflix会员月卡" value="Netflix会员月卡" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡密内容">
          <el-input
            v-model="importForm.content"
            type="textarea"
            :rows="8"
            placeholder="每行一个卡密，支持批量导入"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitImport">确认导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.stock-info {
  font-size: 16px;
}

.stock-label {
  color: #606266;
}

.stock-count {
  font-size: 24px;
  font-weight: 600;
  color: #4A7EFF;
}
</style>
