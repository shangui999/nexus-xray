import axios from 'axios'
import type { AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'
import type { ApiResponse } from '../types'

const http = axios.create({
  baseURL: '',
  timeout: 15000,
})

// 请求拦截器：自动添加 Authorization
http.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('access_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error),
)

// 响应拦截器：统一处理 {code, message, data} 格式
http.interceptors.response.use(
  (response: AxiosResponse<ApiResponse<unknown>>) => {
    const res = response.data
    if (res.code !== 0) {
      ElMessage.error(res.message || '请求失败')
      return Promise.reject(new Error(res.message || '请求失败'))
    }
    return response
  },
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
      router.push('/login')
      ElMessage.error('登录已过期，请重新登录')
    } else {
      const msg = error.response?.data?.message || error.message || '网络错误'
      ElMessage.error(msg)
    }
    return Promise.reject(error)
  },
)

export default http
