import http from './index'
import type { ApiResponse, Plan, CreatePlanRequest, UpdatePlanRequest } from '../types'

export async function listPlans() {
  const res = await http.get<ApiResponse<Plan[]>>('/api/plans')
  return res.data
}

export async function createPlan(data: CreatePlanRequest) {
  const res = await http.post<ApiResponse<Plan>>('/api/plans', data)
  return res.data
}

export async function updatePlan(id: string, data: UpdatePlanRequest) {
  const res = await http.put<ApiResponse<Plan>>(`/api/plans/${id}`, data)
  return res.data
}

export async function deletePlan(id: string) {
  const res = await http.delete<ApiResponse<null>>(`/api/plans/${id}`)
  return res.data
}
