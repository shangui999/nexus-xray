<template>
  <div class="dashboard">
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-info">
              <div class="stat-label">总用户数</div>
              <div class="stat-value">{{ overview.total_users }}</div>
            </div>
            <el-icon :size="40" class="stat-icon" style="color: #409eff"><User /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-info">
              <div class="stat-label">在线节点</div>
              <div class="stat-value">{{ overview.online_nodes }} / {{ overview.total_nodes }}</div>
            </div>
            <el-icon :size="40" class="stat-icon" style="color: #67c23a"><Monitor /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-info">
              <div class="stat-label">今日流量</div>
              <div class="stat-value">{{ formatBytes(overview.total_traffic_today) }}</div>
            </div>
            <el-icon :size="40" class="stat-icon" style="color: #e6a23c"><TrendCharts /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-info">
              <div class="stat-label">活跃用户</div>
              <div class="stat-value">{{ overview.active_users }}</div>
            </div>
            <el-icon :size="40" class="stat-icon" style="color: #f56c6c"><Avatar /></el-icon>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="16">
        <el-card shadow="hover">
          <TrafficChart title="24小时流量趋势" :data="trafficData" />
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header>
            <span style="font-weight: 600">节点状态</span>
          </template>
          <div v-loading="nodesLoading">
            <NodeStatus
              v-for="node in nodes"
              :key="node.id"
              :node="node"
            />
            <el-empty v-if="nodes.length === 0 && !nodesLoading" description="暂无节点" />
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { User, Monitor, TrendCharts, Avatar } from '@element-plus/icons-vue'
import { getStatsOverview, getTrafficStats } from '../api/stats'
import { listNodes } from '../api/nodes'
import TrafficChart from '../components/TrafficChart.vue'
import NodeStatus from '../components/NodeStatus.vue'
import type { StatsOverview, TrafficEntry, Node } from '../types'

const overview = ref<StatsOverview>({
  total_users: 0,
  active_users: 0,
  total_nodes: 0,
  online_nodes: 0,
  total_traffic_today: 0,
})
const trafficData = ref<TrafficEntry[]>([])
const nodes = ref<Node[]>([])
const nodesLoading = ref(false)

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function fetchData() {
  nodesLoading.value = true
  try {
    const [overviewRes, trafficRes, nodesRes] = await Promise.all([
      getStatsOverview(),
      getTrafficStats('24h'),
      listNodes(),
    ])
    overview.value = overviewRes.data
    trafficData.value = trafficRes.data.data || []
    nodes.value = nodesRes.data || []
  } finally {
    nodesLoading.value = false
  }
}

onMounted(fetchData)
</script>

<style scoped>
.stat-card {
  height: 100%;
}
.stat-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.stat-info {
  display: flex;
  flex-direction: column;
}
.stat-label {
  font-size: 14px;
  color: #909399;
  margin-bottom: 8px;
}
.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: #303133;
}
.stat-icon {
  opacity: 0.8;
}
</style>
