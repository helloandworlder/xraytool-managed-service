import { defineStore } from 'pinia'
import { http, normalizeApiError, setAuthToken } from '../lib/http'

type LoginPayload = {
  username: string
  password: string
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('xtool_token') || '',
    username: localStorage.getItem('xtool_user') || '',
    loading: false,
    error: ''
  }),
  getters: {
    isAuthed: (state) => Boolean(state.token)
  },
  actions: {
    init() {
      setAuthToken(this.token)
    },
    async login(payload: LoginPayload) {
      this.loading = true
      this.error = ''
      try {
        const res = await http.post('/api/auth/login', payload)
        this.token = res.data.token
        this.username = res.data.username
        localStorage.setItem('xtool_token', this.token)
        localStorage.setItem('xtool_user', this.username)
        setAuthToken(this.token)
      } catch (err) {
        this.error = normalizeApiError(err)
        throw new Error(this.error)
      } finally {
        this.loading = false
      }
    },
    logout() {
      this.token = ''
      this.username = ''
      this.error = ''
      localStorage.removeItem('xtool_token')
      localStorage.removeItem('xtool_user')
      setAuthToken('')
    }
  }
})
