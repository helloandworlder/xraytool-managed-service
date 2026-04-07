import axios from 'axios'

export const http = axios.create({
  baseURL: '/',
  timeout: 20000
})

type HttpActivityEvent = {
  phase: 'start' | 'finish'
  requestId: string
  method: string
  url: string
  startedAt: number
  status?: number
  ok?: boolean
  durationMs?: number
  error?: string
}

type HttpConfigMeta = {
  requestId?: string
  startedAt?: number
}

const httpActivityListeners = new Set<(event: HttpActivityEvent) => void>()

function emitHttpActivity(event: HttpActivityEvent) {
  for (const listener of httpActivityListeners) {
    try {
      listener(event)
    } catch {
      // ignore listener errors
    }
  }
}

export function subscribeHttpActivity(listener: (event: HttpActivityEvent) => void) {
  httpActivityListeners.add(listener)
  return () => {
    httpActivityListeners.delete(listener)
  }
}

function requestLabel(method: string | undefined, url: string | undefined) {
  return `${String(method || 'GET').toUpperCase()} ${String(url || '/')}`
}

export function setAuthToken(token: string) {
  if (!token) {
    delete http.defaults.headers.common.Authorization
    return
  }
  http.defaults.headers.common.Authorization = `Bearer ${token}`
}

export function normalizeApiError(err: unknown): string {
  const fallback = 'Request failed'
  if (typeof err === 'string') return err
  if (!err || typeof err !== 'object') return fallback
  const anyErr = err as Record<string, any>
  if (anyErr.response?.data?.error) return String(anyErr.response.data.error)
  if (anyErr.message) return String(anyErr.message)
  return fallback
}

export function isAuthError(err: unknown): boolean {
  if (!err || typeof err !== 'object') return false
  const anyErr = err as Record<string, any>
  return Number(anyErr.response?.status || 0) === 401
}

http.interceptors.request.use((config) => {
  const meta = ((config as any).__meta ||= {}) as HttpConfigMeta
  meta.requestId = meta.requestId || `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  meta.startedAt = Date.now()
  emitHttpActivity({
    phase: 'start',
    requestId: meta.requestId,
    method: String(config.method || 'get').toUpperCase(),
    url: requestLabel(config.method, config.url),
    startedAt: meta.startedAt
  })
  return config
})

http.interceptors.response.use(
  (response) => {
    const meta = (((response.config as any).__meta || {}) as HttpConfigMeta)
    const startedAt = Number(meta.startedAt || Date.now())
    emitHttpActivity({
      phase: 'finish',
      requestId: String(meta.requestId || `${Date.now()}`),
      method: String(response.config.method || 'get').toUpperCase(),
      url: requestLabel(response.config.method, response.config.url),
      startedAt,
      status: Number(response.status || 0),
      ok: true,
      durationMs: Date.now() - startedAt
    })
    return response
  },
  (error) => {
    const config = error?.config || {}
    const meta = (((config as any).__meta || {}) as HttpConfigMeta)
    const startedAt = Number(meta.startedAt || Date.now())
    emitHttpActivity({
      phase: 'finish',
      requestId: String(meta.requestId || `${Date.now()}`),
      method: String(config.method || 'get').toUpperCase(),
      url: requestLabel(config.method, config.url),
      startedAt,
      status: Number(error?.response?.status || 0),
      ok: false,
      durationMs: Date.now() - startedAt,
      error: normalizeApiError(error)
    })
    return Promise.reject(error)
  }
)
