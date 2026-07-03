<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

const navItems = [
  { path: '/orders', label: '订单查询' },
  { path: '/api-keys', label: 'API密钥' },
  { path: '/account', label: '账户余额' },
]
</script>

<template>
  <div class="customer-layout">
    <header class="layout-header">
      <div class="header-content">
        <h1 class="logo">SUP平台</h1>
        <nav class="nav-menu">
          <el-menu
            mode="horizontal"
            :default-active="router.currentRoute.value.path"
            :router="true"
            background-color="transparent"
            text-color="#333"
            active-text-color="#4A7EFF"
          >
            <el-menu-item
              v-for="item in navItems"
              :key="item.path"
              :index="item.path"
            >
              {{ item.label }}
            </el-menu-item>
          </el-menu>
        </nav>
      </div>
    </header>
    <main class="layout-main">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.customer-layout {
  min-height: 100vh;
  background-color: #f5f7fa;
}

.layout-header {
  background: #fff;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-content {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  padding: 0 20px;
  height: 60px;
}

.logo {
  font-size: 20px;
  color: #4A7EFF;
  margin: 0;
  margin-right: 40px;
  white-space: nowrap;
}

.nav-menu {
  flex: 1;
}

.nav-menu .el-menu {
  border-bottom: none;
}

.layout-main {
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px 20px;
}
</style>
