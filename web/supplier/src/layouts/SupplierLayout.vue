<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

const menuItems = [
  { path: '/goods', icon: 'Goods', label: '我的商品' },
  { path: '/cards', icon: 'Ticket', label: '卡密管理' },
  { path: '/orders', icon: 'List', label: '订单处理' },
  { path: '/account', icon: 'Wallet', label: '账户' },
]
</script>

<template>
  <div class="supplier-layout">
    <aside class="layout-sidebar">
      <div class="sidebar-header">
        <h1 class="logo">SUP供货商</h1>
      </div>
      <el-menu
        :default-active="router.currentRoute.value.path"
        :router="true"
        background-color="#1d1e2c"
        text-color="#a3a6b4"
        active-text-color="#4A7EFF"
      >
        <el-menu-item
          v-for="item in menuItems"
          :key="item.path"
          :index="item.path"
        >
          <span>{{ item.label }}</span>
        </el-menu-item>
      </el-menu>
    </aside>
    <main class="layout-main">
      <header class="main-header">
        <span class="page-title">{{ router.currentRoute.value.meta.title }}</span>
      </header>
      <div class="main-content">
        <router-view />
      </div>
    </main>
  </div>
</template>

<style scoped>
.supplier-layout {
  display: flex;
  min-height: 100vh;
}

.layout-sidebar {
  width: 220px;
  background-color: #1d1e2c;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.sidebar-header {
  padding: 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.logo {
  color: #4A7EFF;
  font-size: 18px;
  margin: 0;
  text-align: center;
}

.layout-sidebar .el-menu {
  border-right: none;
}

.layout-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  background-color: #f5f7fa;
}

.main-header {
  background: #fff;
  padding: 16px 24px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.05);
}

.page-title {
  font-size: 18px;
  font-weight: 500;
  color: #303133;
}

.main-content {
  padding: 24px;
  flex: 1;
}
</style>
