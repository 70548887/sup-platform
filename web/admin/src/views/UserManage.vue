<template>
  <div class="user-manage">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>用户管理</span>
          <el-select v-model="roleFilter" placeholder="角色筛选" clearable style="width: 150px">
            <el-option label="全部" value="" />
            <el-option label="管理员" value="管理员" />
            <el-option label="供应商" value="供应商" />
            <el-option label="分销商" value="分销商" />
            <el-option label="普通用户" value="普通用户" />
          </el-select>
        </div>
      </template>
      <el-table :data="filteredUsers" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="username" label="用户名" />
        <el-table-column prop="email" label="邮箱" />
        <el-table-column prop="role" label="角色" width="120">
          <template #default="{ row }">
            <el-tag>{{ row.role }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === '正常' ? 'success' : 'danger'">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="注册时间" width="160" />
        <el-table-column label="操作" width="120">
          <template #default>
            <el-button size="small" type="primary" link>编辑</el-button>
            <el-button size="small" type="danger" link>禁用</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const roleFilter = ref('')

const userList = ref([
  { id: 1, username: 'admin', email: 'admin@example.com', role: '管理员', status: '正常', createdAt: '2024-01-01 08:00' },
  { id: 2, username: 'supplier01', email: 'sup01@example.com', role: '供应商', status: '正常', createdAt: '2024-01-02 10:00' },
  { id: 3, username: 'reseller01', email: 'res01@example.com', role: '分销商', status: '正常', createdAt: '2024-01-03 14:00' },
  { id: 4, username: 'user01', email: 'user01@example.com', role: '普通用户', status: '正常', createdAt: '2024-01-04 09:00' },
  { id: 5, username: 'user02', email: 'user02@example.com', role: '普通用户', status: '禁用', createdAt: '2024-01-05 11:00' },
])

const filteredUsers = computed(() => {
  if (!roleFilter.value) return userList.value
  return userList.value.filter((u) => u.role === roleFilter.value)
})
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
