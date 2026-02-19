import { defineStore } from 'pinia'
import { http, normalizeApiError } from '../lib/http'
import type { BackupInfo, Customer, HostIP, ImportPreviewRow, Order, OversellRow, TaskLog } from '../lib/types'

type BatchResult = { id: number; success: boolean; error?: string }

export const usePanelStore = defineStore('panel', {
  state: () => ({
    loading: false,
    notice: '',
    error: '',
    activeTab: 'dashboard',

    customers: [] as Customer[],
    hostIPs: [] as HostIP[],
    oversell: [] as OversellRow[],
    orders: [] as Order[],
    selectedOrder: null as Order | null,
    orderSelection: [] as number[],

    settings: {} as Record<string, string>,
    taskLogs: [] as TaskLog[],
    backups: [] as BackupInfo[],

    importPreview: [] as ImportPreviewRow[]
  }),
  getters: {
    activeOrderCount: (state) => state.orders.filter((o) => o.status === 'active').length,
    expiredOrderCount: (state) => state.orders.filter((o) => o.status === 'expired').length,
    activeHostPublicCount: (state) => state.hostIPs.filter((v) => v.is_public && v.enabled).length,
    selectedCount: (state) => state.orderSelection.length
  },
  actions: {
    setNotice(msg: string) {
      this.notice = msg
      window.setTimeout(() => {
        if (this.notice === msg) this.notice = ''
      }, 3000)
    },
    setError(err: unknown) {
      this.error = normalizeApiError(err)
      window.setTimeout(() => {
        if (this.error) this.error = ''
      }, 3500)
    },
    async bootstrap() {
      await Promise.all([
        this.loadCustomers(),
        this.loadHostIPs(),
        this.loadOversell(),
        this.loadOrders(),
        this.loadSettings(),
        this.loadTaskLogs(),
        this.loadBackups()
      ])
    },
    async loadCustomers() {
      const res = await http.get('/api/customers')
      this.customers = res.data
    },
    async createCustomer(payload: { name: string; contact: string; notes: string }) {
      await http.post('/api/customers', payload)
      await this.loadCustomers()
      this.setNotice('客户已创建')
    },
    async updateCustomer(id: number, payload: { name: string; contact: string; notes: string; status: string }) {
      await http.put(`/api/customers/${id}`, payload)
      await this.loadCustomers()
      this.setNotice('客户已更新')
    },
    async deleteCustomer(id: number) {
      await http.delete(`/api/customers/${id}`)
      await this.loadCustomers()
      this.setNotice('客户已删除')
    },
    async loadHostIPs() {
      const res = await http.get('/api/host-ips')
      this.hostIPs = res.data
    },
    async scanHostIPs() {
      const res = await http.post('/api/host-ips/scan')
      this.hostIPs = res.data
      this.setNotice('IP扫描完成')
    },
    async toggleHostIP(id: number, enabled: boolean) {
      await http.post(`/api/host-ips/${id}/toggle`, { enabled })
      await this.loadHostIPs()
    },
    async loadOversell() {
      const res = await http.get('/api/oversell')
      this.oversell = res.data
    },
    async loadOrders() {
      const res = await http.get('/api/orders')
      this.orders = res.data
    },
    async loadOrderDetail(orderID: number) {
      const res = await http.get(`/api/orders/${orderID}`)
      this.selectedOrder = res.data
    },
    async createOrder(payload: Record<string, unknown>) {
      await http.post('/api/orders', payload)
      await this.loadOrders()
      await this.loadOversell()
      this.setNotice('订单创建完成并已同步')
    },
    async deactivateOrder(orderID: number) {
      await http.post(`/api/orders/${orderID}/deactivate`, {})
      await this.loadOrders()
      this.setNotice('订单已停用')
    },
    async renewOrder(orderID: number, moreDays: number) {
      await http.post(`/api/orders/${orderID}/renew`, { more_days: moreDays })
      await this.loadOrders()
      this.setNotice('订单续期完成')
    },
    async testOrder(orderID: number) {
      const res = await http.post(`/api/orders/${orderID}/test`, {})
      return res.data as Record<string, string>
    },
    async batchRenew(orderIDs: number[], moreDays: number) {
      const res = await http.post('/api/orders/batch/renew', {
        order_ids: orderIDs,
        more_days: moreDays
      })
      await this.loadOrders()
      return (res.data.results || []) as BatchResult[]
    },
    async batchDeactivate(orderIDs: number[], status = 'disabled') {
      const res = await http.post('/api/orders/batch/deactivate', {
        order_ids: orderIDs,
        status
      })
      await this.loadOrders()
      return (res.data.results || []) as BatchResult[]
    },
    async batchResync(orderIDs: number[]) {
      const res = await http.post('/api/orders/batch/resync', {
        order_ids: orderIDs
      })
      await this.loadOrders()
      return (res.data.results || []) as BatchResult[]
    },
    async batchTest(orderIDs: number[]) {
      const res = await http.post('/api/orders/batch/test', {
        order_ids: orderIDs
      })
      return (res.data.results || []) as Array<{ id: number; success: boolean; result?: Record<string, string>; error?: string }>
    },
    async batchExport(orderIDs: number[]) {
      const res = await http.post('/api/orders/batch/export', {
        order_ids: orderIDs
      }, {
        responseType: 'text'
      })
      return typeof res.data === 'string' ? res.data : String(res.data)
    },
    async loadSettings() {
      const res = await http.get('/api/settings')
      this.settings = res.data
    },
    async saveSettings(payload: Record<string, string>) {
      await http.put('/api/settings', payload)
      this.settings = { ...payload }
      this.setNotice('设置已保存')
    },
    async loadTaskLogs(filters?: { level?: string; keyword?: string; start?: string; end?: string; limit?: number }) {
      const params = new URLSearchParams()
      params.set('limit', String(filters?.limit || 50))
      if (filters?.level) params.set('level', filters.level)
      if (filters?.keyword) params.set('keyword', filters.keyword)
      if (filters?.start) params.set('start', filters.start)
      if (filters?.end) params.set('end', filters.end)
      const res = await http.get(`/api/task-logs?${params.toString()}`)
      this.taskLogs = res.data
    },
    async previewImport(lines: string) {
      const res = await http.post('/api/orders/import/preview', { lines })
      this.importPreview = res.data
    },
    async confirmImport(payload: {
      customer_id: number
      order_name: string
      expires_at: string
      rows: ImportPreviewRow[]
    }) {
      await http.post('/api/orders/import/confirm', payload)
      await this.loadOrders()
      await this.loadOversell()
      this.setNotice('导入成功并纳入生命周期')
    },
    async loadBackups() {
      const res = await http.get('/api/db/backups')
      this.backups = res.data
    },
    async createBackup() {
      await http.post('/api/db/backups', {})
      await this.loadBackups()
      this.setNotice('数据库备份创建成功')
    },
    backupDownloadURL(name: string) {
      return `/api/db/backups/${encodeURIComponent(name)}/download`
    },
    async deleteBackup(name: string) {
      await http.delete(`/api/db/backups/${encodeURIComponent(name)}`)
      await this.loadBackups()
      this.setNotice('备份已删除')
    },
    async restoreBackup(name: string) {
      await http.post('/api/db/restore', { name })
      this.setNotice('恢复完成，服务将自动重启')
    }
  }
})
