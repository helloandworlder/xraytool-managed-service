import axios from 'axios'

export const http = axios.create({
  baseURL: '/',
  timeout: 20000
})

export function setAuthToken(token: string) {
  if (!token) {
    delete http.defaults.headers.common.Authorization
    return
  }
  http.defaults.headers.common.Authorization = `Bearer ${token}`
}

export function normalizeApiError(err: unknown): string {
  const fallback = 'Request failed'
  if (!err || typeof err !== 'object') return fallback
  const anyErr = err as Record<string, any>
  if (anyErr.response?.data?.error) return String(anyErr.response.data.error)
  if (anyErr.message) return String(anyErr.message)
  return fallback
}
