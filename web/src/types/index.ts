// ==================== 通用 API 响应 ====================
export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

// ==================== 认证 ====================
export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface RefreshTokenRequest {
  refresh_token: string
}

export interface RefreshTokenResponse {
  access_token: string
  expires_in: number
}

// ==================== 节点 ====================
export interface Node {
  id: string
  name: string
  address: string
  status: 'online' | 'offline' | 'error'
  last_heartbeat: string | null
  config: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export interface CreateNodeRequest {
  name: string
  address: string
}

export interface CreateNodeResponse {
  node: Node
  enrollment_token: string
  install_command: string
}

export interface NodeStatus {
  node: Node
  upload_24h: number
  download_24h: number
}

// ==================== 用户 ====================
export interface User {
  id: string
  username: string
  email: string
  vless_uuid: string
  quota_bytes: number
  used_bytes: number
  expires_at: string | null
  state: 'active' | 'suspended' | 'expired'
  traffic_rate: number
  max_connections: number
  plan_id: string | null
  plan: Plan | null
  created_at: string
  updated_at: string
}

export interface CreateUserRequest {
  username: string
  email: string
  password: string
  plan_id?: string
}

export interface UpdateUserRequest {
  email?: string
  password?: string
  quota_bytes?: number
  used_bytes?: number
  state?: string
  traffic_rate?: number
  max_connections?: number
  plan_id?: string
}

export interface UserListResponse {
  items: User[]
  total: number
  page: number
  size: number
}

// ==================== 套餐 ====================
export interface Plan {
  id: string
  name: string
  quota_bytes: number
  duration_days: number
  max_connections: number
  traffic_rate: number
  price_cents: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreatePlanRequest {
  name: string
  quota_bytes: number
  duration_days: number
  max_connections?: number
  traffic_rate?: number
  price_cents: number
}

export interface UpdatePlanRequest {
  name?: string
  quota_bytes?: number
  duration_days?: number
  max_connections?: number
  traffic_rate?: number
  price_cents?: number
  is_active?: boolean
}

// ==================== 入站 ====================
export interface Inbound {
  id: string
  node_id: string
  node: Node | null
  protocol: string
  port: number
  settings: Record<string, unknown>
  stream_settings: Record<string, unknown> | null
  tag: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateInboundRequest {
  node_id: string
  protocol: string
  port: number
  settings: Record<string, unknown>
  stream_settings?: Record<string, unknown>
  tag: string
}

export interface UpdateInboundRequest {
  protocol?: string
  port?: number
  settings?: Record<string, unknown>
  stream_settings?: Record<string, unknown>
  tag?: string
  enabled?: boolean
}

// ==================== 统计 ====================
export interface StatsOverview {
  total_users: number
  active_users: number
  total_nodes: number
  online_nodes: number
  total_traffic_today: number
}

export interface TrafficEntry {
  time: string
  upload: number
  download: number
}

export interface TrafficResponse {
  data: TrafficEntry[]
}

export interface UserTrafficEntry {
  time: string
  upload: number
  download: number
}

export interface UserTrafficResponse {
  period: string
  data: UserTrafficEntry[]
}
