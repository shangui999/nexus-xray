import http from './index'
import type { ApiResponse, Node, CreateNodeRequest, CreateNodeResponse, NodeStatus } from '../types'

export async function listNodes() {
  const res = await http.get<ApiResponse<Node[]>>('/api/nodes')
  return res.data
}

export async function createNode(data: CreateNodeRequest) {
  const res = await http.post<ApiResponse<CreateNodeResponse>>('/api/nodes', data)
  return res.data
}

export async function deleteNode(id: string) {
  const res = await http.delete<ApiResponse<null>>(`/api/nodes/${id}`)
  return res.data
}

export async function getNodeStatus(id: string) {
  const res = await http.get<ApiResponse<NodeStatus>>(`/api/nodes/${id}/status`)
  return res.data
}
