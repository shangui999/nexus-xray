<template>
  <el-card shadow="hover" class="node-status-card">
    <div class="status-header">
      <span class="node-name">{{ node.name }}</span>
      <el-tag :type="statusType" size="small">{{ node.status }}</el-tag>
    </div>
    <div class="status-info">
      <div class="info-row">
        <span class="label">地址</span>
        <span class="value">{{ node.address }}</span>
      </div>
      <div class="info-row">
        <span class="label">最近心跳</span>
        <span class="value">{{ node.last_heartbeat ? formatTime(node.last_heartbeat) : '无' }}</span>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Node } from '../types'

const props = defineProps<{
  node: Node
}>()

const statusType = computed(() => {
  switch (props.node.status) {
    case 'online': return 'success'
    case 'offline': return 'danger'
    case 'error': return 'warning'
    default: return 'info'
  }
})

function formatTime(timeStr: string) {
  const date = new Date(timeStr)
  return date.toLocaleString('zh-CN')
}
</script>

<style scoped>
.node-status-card {
  margin-bottom: 12px;
}
.status-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.node-name {
  font-weight: 600;
  font-size: 15px;
}
.status-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.info-row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
}
.label {
  color: #909399;
}
.value {
  color: #303133;
}
</style>
