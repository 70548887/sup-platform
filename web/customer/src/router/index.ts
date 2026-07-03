import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import CustomerLayout from '@/layouts/CustomerLayout.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: CustomerLayout,
    redirect: '/orders',
    children: [
      {
        path: 'api-keys',
        name: 'ApiKeys',
        component: () => import('@/views/ApiKeys.vue'),
        meta: { title: 'API密钥管理' },
      },
      {
        path: 'orders',
        name: 'Orders',
        component: () => import('@/views/Orders.vue'),
        meta: { title: '订单查询' },
      },
      {
        path: 'account',
        name: 'Account',
        component: () => import('@/views/Account.vue'),
        meta: { title: '账户余额' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
