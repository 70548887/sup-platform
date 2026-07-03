<script setup lang="ts">
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'

interface Product {
  id: number
  name: string
  category: string
  price: string
  stock: number
  status: string
}

const products = ref<Product[]>([
  { id: 1, name: 'Steam充值卡 50元', category: '游戏充值', price: '47.50', stock: 120, status: 'online' },
  { id: 2, name: 'Netflix会员月卡', category: '影音娱乐', price: '42.00', stock: 85, status: 'online' },
  { id: 3, name: 'iTunes礼品卡 100元', category: '数字礼品', price: '93.00', stock: 0, status: 'offline' },
])

const editDialogVisible = ref(false)
const editForm = reactive({
  id: 0,
  name: '',
  category: '',
  price: '',
  status: '',
})

const handleEdit = (row: Product) => {
  editForm.id = row.id
  editForm.name = row.name
  editForm.category = row.category
  editForm.price = row.price
  editForm.status = row.status
  editDialogVisible.value = true
}

const submitEdit = () => {
  const idx = products.value.findIndex(p => p.id === editForm.id)
  if (idx !== -1) {
    products.value[idx].name = editForm.name
    products.value[idx].price = editForm.price
    products.value[idx].status = editForm.status
  }
  editDialogVisible.value = false
  ElMessage.success('商品信息已更新')
}

const handleToggleStatus = (row: Product) => {
  row.status = row.status === 'online' ? 'offline' : 'online'
  ElMessage.success(`商品已${row.status === 'online' ? '上架' : '下架'}`)
}
</script>

<template>
  <div class="goods-manage-page">
    <div class="page-header">
      <el-button type="primary">添加商品</el-button>
    </div>

    <el-table :data="products" stripe style="width: 100%">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="name" label="商品名称" min-width="180" />
      <el-table-column prop="category" label="分类" width="120" />
      <el-table-column prop="price" label="供货价" width="100" align="right">
        <template #default="{ row }">
          ¥{{ row.price }}
        </template>
      </el-table-column>
      <el-table-column prop="stock" label="库存" width="80" align="center">
        <template #default="{ row }">
          <el-tag :type="row.stock > 0 ? 'success' : 'danger'" size="small">
            {{ row.stock }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="80" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 'online' ? 'success' : 'info'" size="small">
            {{ row.status === 'online' ? '上架' : '下架' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" size="small" link @click="handleEdit(row)">编辑</el-button>
          <el-button
            :type="row.status === 'online' ? 'warning' : 'success'"
            size="small"
            link
            @click="handleToggleStatus(row)"
          >
            {{ row.status === 'online' ? '下架' : '上架' }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="editDialogVisible" title="编辑商品" width="500px">
      <el-form :model="editForm" label-width="80px">
        <el-form-item label="商品名称">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="供货价">
          <el-input v-model="editForm.price" type="number">
            <template #prepend>¥</template>
          </el-input>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="editForm.status">
            <el-option label="上架" value="online" />
            <el-option label="下架" value="offline" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page-header {
  margin-bottom: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
