import http from './index'
import type { ApiResponse, LoginRequest, LoginResponse, RefreshTokenResponse } from '../types'

export async function login(data: LoginRequest) {
  const res = await http.post<ApiResponse<LoginResponse>>('/api/auth/login', data)
  return res.data
}

export async function refreshToken(refresh_token: string) {
  const res = await http.post<ApiResponse<RefreshTokenResponse>>('/api/auth/refresh', { refresh_token })
  return res.data
}
