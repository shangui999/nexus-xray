<template>
  <el-container class="layout-container">
    <el-aside :width="sidebarCollapsed ? '64px' : '220px'" class="layout-aside">
      <div class="logo-area">
        <el-icon :size="24"><Monitor /></el-icon>
        <span v-show="!sidebarCollapsed" class="logo-text">Xray Manager</span>
      </div>
      <el-menu
        :default-active="currentRoute"
        :collapse="sidebarCollapsed"
        :collapse-transition="false"
        router
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409eff"
      >
        <el-menu-item index="/dashboard">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>总览面板</template>
        </el-menu-item>
        <el-menu-item index="/nodes">
          <el-icon><Monitor /></el-icon>
          <template #title>节点管理</template>
        </el-menu-item>
        <el-menu-item index="/users">
          <el-icon><User /></el-icon>
          <template #title>用户管理</template>
        </el-menu-item>
        <el-menu-item index="/plans">
          <el-icon><Ticket /></el-icon>
          <template #title>套餐管理</template>
        </el-menu-item>
        <el-menu-item index="/inbounds">
          <el-icon><Connection /></el-icon>
          <template #title>入站配置</template>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <template #title>系统设置</template>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="layout-header">
        <div class="header-left">
          <el-icon
            :size="20"
            class="collapse-btn"
            @click="appStore.toggleSidebar()"
          >
            <Fold v-if="!sidebarCollapsed" />
            <Expand v-else />
          </el-icon>
        </div>
        <div class="header-right">
          <el-dropdown @command="handleCommand">
            <span class="el-dropdown-link">
              管理员 <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main class="layout-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../store/auth'
import { useAppStore } from '../store/app'
import {
  Monitor,
  DataAnalysis,
  User,
  Ticket,
  Connection,
  Setting,
  Fold,
  Expand,
  ArrowDown,
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const currentRoute = computed(() => route.path)

function handleCommand(command: string) {
  if (command === 'logout') {
    authStore.logout()
    router.push('/login')
  }
}
</script>

<style scoped>
.layout-container {
  height: 100vh;
}
.layout-aside {
  background-color: #304156;
  overflow: hidden;
  transition: width 0.3s;
}
.logo-area {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #fff;
  font-size: 16px;
  font-weight: 600;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}
.logo-text {
  white-space: nowrap;
}
.layout-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
}
.collapse-btn {
  cursor: pointer;
  color: #333;
}
.collapse-btn:hover {
  color: #409eff;
}
.header-right {
  display: flex;
  align-items: center;
}
.el-dropdown-link {
  cursor: pointer;
  display: flex;
  align-items: center;
  color: #333;
  font-size: 14px;
}
.layout-main {
  background: #f5f7fa;
  padding: 20px;
}
.el-menu {
  border-right: none;
}
</style>
