import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as loginApi } from '../api/auth'
import type { LoginRequest } from '../types'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref(localStorage.getItem('access_token') || '')
  const refreshToken = ref(localStorage.getItem('refresh_token') || '')

  const isLoggedIn = computed(() => !!accessToken.value)

  async function login(data: LoginRequest) {
    const res = await loginApi(data)
    accessToken.value = res.data.access_token
    refreshToken.value = res.data.refresh_token
    localStorage.setItem('access_token', res.data.access_token)
    localStorage.setItem('refresh_token', res.data.refresh_token)
  }

  function logout() {
    accessToken.value = ''
    refreshToken.value = ''
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
  }

  return {
    accessToken,
    refreshToken,
    isLoggedIn,
    login,
    logout,
  }
})
