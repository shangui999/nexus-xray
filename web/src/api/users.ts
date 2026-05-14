import http from './index'
import type { ApiResponse, User, CreateUserRequest, UpdateUserRequest, UserListResponse, UserTrafficResponse } from '../types'

export async function listUsers(params: { page?: number; size?: number; state?: string }) {
  const res = await http.get<ApiResponse<UserListResponse>>('/api/users', { params })
  return res.data
}

export async function createUser(data: CreateUserRequest) {
  const res = await http.post<ApiResponse<User>>('/api/users', data)
  return res.data
}

export async function updateUser(id: string, data: UpdateUserRequest) {
  const res = await http.put<ApiResponse<User>>(`/api/users/${id}`, data)
  return res.data
}

export async function deleteUser(id: string) {
  const res = await http.delete<ApiResponse<null>>(`/api/users/${id}`)
  return res.data
}

export async function getUserTraffic(id: string, period: string = 'day') {
  const res = await http.get<ApiResponse<UserTrafficResponse>>(`/api/users/${id}/traffic`, { params: { period } })
  return res.data
}
