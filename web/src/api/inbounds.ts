import http from './index'
import type { ApiResponse, Inbound, CreateInboundRequest, UpdateInboundRequest } from '../types'

export async function listInbounds(params?: { node_id?: string }) {
  const res = await http.get<ApiResponse<Inbound[]>>('/api/inbounds', { params })
  return res.data
}

export async function createInbound(data: CreateInboundRequest) {
  const res = await http.post<ApiResponse<Inbound>>('/api/inbounds', data)
  return res.data
}

export async function updateInbound(id: string, data: UpdateInboundRequest) {
  const res = await http.put<ApiResponse<Inbound>>(`/api/inbounds/${id}`, data)
  return res.data
}

export async function deleteInbound(id: string) {
  const res = await http.delete<ApiResponse<null>>(`/api/inbounds/${id}`)
  return res.data
}
