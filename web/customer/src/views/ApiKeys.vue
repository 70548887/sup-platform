<script setup lang="ts">
import { ref, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

interface ApiKey {
  id: number
  appId: string
  appSecret: string
  createdAt: string
  status: string
}

const apiKeys = ref<ApiKey[]>([
  {
    id: 1,
    appId: 'app_demo_001',
    appSecret: '••••••••••••••••',
    createdAt: '2024-01-15 10:30:00',
    status: 'active',
  },
])

const loading = ref(false)

const handleGenerate = async () => {
  try {
    await ElMessageBox.confirm('确定要生成新的API密钥吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    loading.value = true
    // 模拟API调用
    setTimeout(() => {
      const newKey: ApiKey = {
        id: apiKeys.value.length + 1,
        appId: `app_${Date.now()}`,
        appSecret: '••••••••••••••••',
        createdAt: new Date().toLocaleString(),
        status: 'active',
      }
      apiKeys.value.push(newKey)
      loading.value = false
      ElMessage.success('API密钥生成成功')
    }, 1000)
  } catch {
    // 取消操作
  }
}

const handleRevoke = (row: ApiKey) => {
  ElMessageBox.confirm('确定要吊销此密钥吗？此操作不可恢复。', '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'error',
  }).then(() => {
    row.status = 'revoked'
    ElMessage.success('密钥已吊销')
  }).catch(() => {})
}
</script>

<template>
  <div class="api-keys-page">
    <div class="page-header">
      <h2>API密钥管理</h2>
      <el-button type="primary" @click="handleGenerate" :loading="loading">
        生成新密钥
      </el-button>
    </div>

    <el-table :data="apiKeys" stripe style="width: 100%">
      <el-table-column prop="appId" label="AppId" min-width="180" />
      <el-table-column prop="appSecret" label="AppSecret" min-width="200">
        <template #default="{ row }">
          <code>{{ row.appSecret }}</code>
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="创建时间" width="180" />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'danger'">
            {{ row.status === 'active' ? '有效' : '已吊销' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.status === 'active'"
            type="danger"
            size="small"
            link
            @click="handleRevoke(row)"
          >
            吊销
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
  font-size: 22px;
  color: #303133;
}
</style>
