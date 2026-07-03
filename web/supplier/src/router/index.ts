import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import SupplierLayout from '@/layouts/SupplierLayout.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: SupplierLayout,
    redirect: '/goods',
    children: [
      {
        path: 'goods',
        name: 'GoodsManage',
        component: () => import('@/views/GoodsManage.vue'),
        meta: { title: '我的商品' },
      },
      {
        path: 'cards',
        name: 'CardManage',
        component: () => import('@/views/CardManage.vue'),
        meta: { title: '卡密管理' },
      },
      {
        path: 'orders',
        name: 'OrderProcess',
        component: () => import('@/views/OrderProcess.vue'),
        meta: { title: '订单处理' },
      },
      {
        path: 'account',
        name: 'Account',
        component: () => import('@/views/Account.vue'),
        meta: { title: '账户' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
