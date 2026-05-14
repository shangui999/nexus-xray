import http from './index'
import type { ApiResponse, StatsOverview, TrafficResponse } from '../types'

export async function getStatsOverview() {
  const res = await http.get<ApiResponse<StatsOverview>>('/api/stats/overview')
  return res.data
}

export async function getTrafficStats(period: string = '24h') {
  const res = await http.get<ApiResponse<TrafficResponse>>('/api/stats/traffic', { params: { period } })
  return res.data
}
