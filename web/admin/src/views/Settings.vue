<template>
  <div class="settings">
    <el-card shadow="never">
      <template #header>
        <span>系统配置</span>
      </template>
      <el-form :model="settingsForm" label-width="120px" style="max-width: 600px">
        <el-form-item label="站点名称">
          <el-input v-model="settingsForm.siteName" />
        </el-form-item>
        <el-form-item label="站点描述">
          <el-input v-model="settingsForm.siteDescription" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="联系邮箱">
          <el-input v-model="settingsForm.contactEmail" />
        </el-form-item>
        <el-form-item label="订单超时(分钟)">
          <el-input-number v-model="settingsForm.orderTimeout" :min="5" :max="1440" />
        </el-form-item>
        <el-form-item label="注册开关">
          <el-switch v-model="settingsForm.enableRegistration" />
        </el-form-item>
        <el-form-item label="维护模式">
          <el-switch v-model="settingsForm.maintenanceMode" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSave">保存配置</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive } from 'vue'
import { ElMessage } from 'element-plus'

const settingsForm = reactive({
  siteName: 'SUP平台',
  siteDescription: '数字商品供应链管理平台',
  contactEmail: 'admin@example.com',
  orderTimeout: 30,
  enableRegistration: true,
  maintenanceMode: false,
})

const handleSave = () => {
  ElMessage.success('配置保存成功')
}
</script>

<style scoped>
</style>
