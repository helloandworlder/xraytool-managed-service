import { defineStore } from 'pinia'
import { http, normalizeApiError } from '../lib/http'
import type {
  AllocationPreview,
  BackupInfo,
  Customer,
  CustomerRuntimeStat,
	DedicatedInbound,
	DedicatedIngress,
	DedicatedEntry,
  ForwardOutbound,
  HostIP,
  ImportPreviewRow,
  Order,
  OversellRow,
  SingboxScanResult,
  SocksMigrationPreviewResult,
  TaskLog,
  XrayNode
} from '../lib/types'

type BatchResult = { id: number; success: boolean; error?: string }

type OrderSubmitResult = {
  order?: Order
  warnings: string[]
}

export const usePanelStore = defineStore('panel', {
  state: () => ({
    loading: false,
    notice: '',
    error: '',
    activeTab: 'dashboard',

    customers: [] as Customer[],
    hostIPs: [] as HostIP[],
    oversell: [] as OversellRow[],
    oversellCustomerID: 0,
    orders: [] as Order[],
    selectedOrder: null as Order | null,
    orderSelection: [] as number[],
    allocationPreview: null as AllocationPreview | null,

    settings: {} as Record<string, string>,
    taskLogs: [] as TaskLog[],
    backups: [] as BackupInfo[],
    runtimeStats: [] as CustomerRuntimeStat[],

    importPreview: [] as ImportPreviewRow[],
    singboxScan: null as SingboxScanResult | null,
    nodes: [] as XrayNode[],
		migrationPreview: null as SocksMigrationPreviewResult | null,
		forwardOutbounds: [] as ForwardOutbound[],
		dedicatedEntries: [] as DedicatedEntry[],
		dedicatedInbounds: [] as DedicatedInbound[],
		dedicatedIngresses: [] as DedicatedIngress[]
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
        this.loadOversell(this.oversellCustomerID),
        this.loadOrders(),
        this.loadNodes(),
        this.loadForwardOutbounds(),
			this.loadDedicatedEntries(),
			this.loadDedicatedInbounds(),
			this.loadDedicatedIngresses(),
        this.loadSettings(),
        this.loadTaskLogs(),
        this.loadBackups(),
        this.loadRuntimeStats().catch(() => {
          this.runtimeStats = []
        })
      ])
    },
    async loadCustomers() {
      const res = await http.get('/api/customers')
      this.customers = res.data
    },
    async createCustomer(payload: { name: string; code?: string; contact: string; notes: string }) {
      await http.post('/api/customers', payload)
      await this.loadCustomers()
      this.setNotice('客户已创建')
    },
    async updateCustomer(id: number, payload: { name: string; code?: string; contact: string; notes: string; status: string }) {
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
    async loadNodes() {
      const res = await http.get('/api/nodes')
      this.nodes = res.data
    },
    async createNode(payload: { name: string; base_url: string; username: string; password: string; enabled: boolean; is_local: boolean }) {
      await http.post('/api/nodes', payload)
      await this.loadNodes()
      this.setNotice('节点已创建')
    },
    async updateNode(id: number, payload: { name: string; base_url: string; username: string; password: string; enabled: boolean; is_local: boolean }) {
      await http.put(`/api/nodes/${id}`, payload)
      await this.loadNodes()
      this.setNotice('节点已更新')
    },
    async deleteNode(id: number) {
      await http.delete(`/api/nodes/${id}`)
      await this.loadNodes()
      this.setNotice('节点已删除')
    },
    async loadForwardOutbounds() {
      const res = await http.get('/api/orders/forward-outbounds')
      this.forwardOutbounds = res.data || []
    },
		async loadDedicatedEntries() {
			const res = await http.get('/api/orders/dedicated-entries')
			this.dedicatedEntries = res.data || []
		},
		async loadDedicatedInbounds() {
			const res = await http.get('/api/orders/dedicated-inbounds')
			this.dedicatedInbounds = res.data || []
		},
		async loadDedicatedIngresses() {
			const res = await http.get('/api/orders/dedicated-ingresses')
			this.dedicatedIngresses = res.data || []
		},
		async createDedicatedInbound(payload: {
			name: string
			protocol: string
			listen_port: number
			priority: number
			enabled: boolean
			notes?: string
		}) {
			await http.post('/api/orders/dedicated-inbounds', payload)
			await this.loadDedicatedInbounds()
			this.setNotice('Inbound已创建')
		},
		async updateDedicatedInbound(id: number, payload: {
			name: string
			protocol: string
			listen_port: number
			priority: number
			enabled: boolean
			notes?: string
		}) {
			await http.put(`/api/orders/dedicated-inbounds/${id}`, payload)
			await this.loadDedicatedInbounds()
			this.setNotice('Inbound已更新')
		},
		async toggleDedicatedInbound(id: number, enabled: boolean) {
			await http.post(`/api/orders/dedicated-inbounds/${id}/toggle`, { enabled })
			await this.loadDedicatedInbounds()
		},
		async deleteDedicatedInbound(id: number) {
			await http.delete(`/api/orders/dedicated-inbounds/${id}`)
			await this.loadDedicatedInbounds()
			this.setNotice('Inbound已删除')
		},
		async createDedicatedIngress(payload: {
			dedicated_inbound_id: number
			name: string
			domain: string
			ingress_port: number
			country_code?: string
			region?: string
			priority: number
			enabled: boolean
			notes?: string
		}) {
			await http.post('/api/orders/dedicated-ingresses', payload)
			await this.loadDedicatedIngresses()
			this.setNotice('Ingress已创建')
		},
		async updateDedicatedIngress(id: number, payload: {
			dedicated_inbound_id: number
			name: string
			domain: string
			ingress_port: number
			country_code?: string
			region?: string
			priority: number
			enabled: boolean
			notes?: string
		}) {
			await http.put(`/api/orders/dedicated-ingresses/${id}`, payload)
			await this.loadDedicatedIngresses()
			this.setNotice('Ingress已更新')
		},
		async toggleDedicatedIngress(id: number, enabled: boolean) {
			await http.post(`/api/orders/dedicated-ingresses/${id}/toggle`, { enabled })
			await this.loadDedicatedIngresses()
		},
		async deleteDedicatedIngress(id: number) {
			await http.delete(`/api/orders/dedicated-ingresses/${id}`)
			await this.loadDedicatedIngresses()
			this.setNotice('Ingress已删除')
		},
		async createDedicatedEntry(payload: {
			name: string
			domain: string
			mixed_port: number
			vmess_port: number
			vless_port: number
			shadowsocks_port: number
			priority: number
			features: string[]
			enabled: boolean
			notes?: string
		}) {
			await http.post('/api/orders/dedicated-entries', payload)
			await this.loadDedicatedEntries()
			this.setNotice('专线入口已创建')
		},
		async updateDedicatedEntry(id: number, payload: {
			name: string
			domain: string
			mixed_port: number
			vmess_port: number
			vless_port: number
			shadowsocks_port: number
			priority: number
			features: string[]
			enabled: boolean
			notes?: string
		}) {
			await http.put(`/api/orders/dedicated-entries/${id}`, payload)
			await this.loadDedicatedEntries()
			this.setNotice('专线入口已更新')
		},
		async toggleDedicatedEntry(id: number, enabled: boolean) {
			await http.post(`/api/orders/dedicated-entries/${id}/toggle`, { enabled })
			await this.loadDedicatedEntries()
		},
		async deleteDedicatedEntry(id: number) {
			await http.delete(`/api/orders/dedicated-entries/${id}`)
			await this.loadDedicatedEntries()
			this.setNotice('专线入口已删除')
		},
    async createForwardOutbound(payload: {
      name: string
      address: string
      port: number
      username: string
      password: string
      route_user?: string
      enabled: boolean
    }) {
      await http.post('/api/orders/forward-outbounds', payload)
      await this.loadForwardOutbounds()
      this.setNotice('转发出口已创建')
    },
    async updateForwardOutbound(id: number, payload: {
      name: string
      address: string
      port: number
      username: string
      password: string
      route_user?: string
      enabled: boolean
    }) {
      await http.put(`/api/orders/forward-outbounds/${id}`, payload)
      await this.loadForwardOutbounds()
      this.setNotice('转发出口已更新')
    },
    async toggleForwardOutbound(id: number, enabled: boolean) {
      await http.post(`/api/orders/forward-outbounds/${id}/toggle`, { enabled })
      await this.loadForwardOutbounds()
    },
    async deleteForwardOutbound(id: number) {
      await http.delete(`/api/orders/forward-outbounds/${id}`)
      await this.loadForwardOutbounds()
      this.setNotice('转发出口已删除')
    },
    async importForwardOutbounds(lines: string) {
      const res = await http.post('/api/orders/forward-outbounds/import', { lines })
      await this.loadForwardOutbounds()
      return res.data as Array<Record<string, any>>
    },
    async probeForwardOutbound(id: number) {
      const res = await http.post(`/api/orders/forward-outbounds/${id}/probe`, {})
      await this.loadForwardOutbounds()
      return res.data
    },
    async probeAllForwardOutbounds(enabledOnly = true) {
      const res = await http.post('/api/orders/forward-outbounds/probe-all', { enabled_only: enabledOnly })
      this.forwardOutbounds = res.data || []
      return this.forwardOutbounds
    },
    async previewForwardReuseWarnings(payload: { customer_id: number; forward_outbound_ids: number[]; exclude_order_id?: number }) {
      if (!payload.customer_id || !payload.forward_outbound_ids || payload.forward_outbound_ids.length === 0) {
        return [] as string[]
      }
      const res = await http.post('/api/orders/forward/reuse-warnings', payload)
      const warnings = Array.isArray(res.data?.warnings) ? res.data.warnings : []
      return warnings.map((v: unknown) => String(v))
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
    async loadOversell(customerID = 0) {
      this.oversellCustomerID = customerID
      const params = new URLSearchParams()
      if (customerID > 0) params.set('customer_id', String(customerID))
      const query = params.toString()
      const res = await http.get(`/api/oversell${query ? `?${query}` : ''}`)
      this.oversell = res.data.rows || []
    },
    async loadAllocationPreview(customerID: number, excludeOrderID = 0) {
      if (!customerID) {
        this.allocationPreview = null
        return null
      }
      const params = new URLSearchParams({ customer_id: String(customerID) })
      if (excludeOrderID > 0) params.set('exclude_order_id', String(excludeOrderID))
      const res = await http.get(`/api/orders/allocation/preview?${params.toString()}`)
      this.allocationPreview = res.data
      return this.allocationPreview
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
		const res = await http.post('/api/orders', payload)
      await this.loadOrders()
      await this.loadOversell(this.oversellCustomerID)
      const customerID = Number(payload.customer_id || 0)
      if (customerID > 0) {
        await this.loadAllocationPreview(customerID)
      }
      this.setNotice('订单创建完成并已同步')
		const body = res.data || {}
		const warnings = Array.isArray(body.warnings) ? body.warnings.map((v: unknown) => String(v)) : []
		const order = body.order || body
		return { order, warnings } as OrderSubmitResult
    },
    async updateOrder(orderID: number, payload: Record<string, unknown>) {
      const res = await http.put(`/api/orders/${orderID}`, payload)
      await this.loadOrders()
      await this.loadOversell(this.oversellCustomerID)
      const customerID = this.orders.find((o) => o.id === orderID)?.customer_id
      if (customerID) {
        await this.loadAllocationPreview(customerID, orderID)
      }
      this.setNotice('订单已更新')
		const body = res.data || {}
		const warnings = Array.isArray(body.warnings) ? body.warnings.map((v: unknown) => String(v)) : []
		const order = (body.order || body) as Order
		return { order, warnings } as OrderSubmitResult
    },
		async splitOrder(orderID: number) {
			const res = await http.post(`/api/orders/${orderID}/split`, {})
			await this.loadOrders()
			return res.data?.children || []
		},
		async updateOrderGroupSocks5(orderID: number, lines: string) {
			await http.post(`/api/orders/${orderID}/group/update-socks5`, { lines })
			await this.loadOrders()
		},
		async updateOrderGroupSocks5Selected(orderID: number, childOrderIDs: number[], lines: string) {
			await http.post(`/api/orders/${orderID}/group/update-socks5-selected`, {
				child_order_ids: childOrderIDs,
				lines
			})
			await this.loadOrders()
		},
		async updateOrderGroupSocks5XLSX(orderID: number, file: File) {
			const form = new FormData()
			form.append('file', file)
			await http.post(`/api/orders/${orderID}/group/update-socks5/xlsx`, form, {
				headers: { 'Content-Type': 'multipart/form-data' }
			})
			await this.loadOrders()
		},
		async updateOrderGroupCredentials(orderID: number, payload: { lines?: string; regenerate?: boolean }) {
			await http.post(`/api/orders/${orderID}/group/update-credentials`, payload)
			await this.loadOrders()
		},
		async updateOrderGroupCredentialsSelected(orderID: number, childOrderIDs: number[], payload: { lines?: string; regenerate?: boolean }) {
			await http.post(`/api/orders/${orderID}/group/update-credentials-selected`, {
				child_order_ids: childOrderIDs,
				lines: payload.lines,
				regenerate: payload.regenerate
			})
			await this.loadOrders()
		},
		async updateOrderGroupEgressGeo(orderID: number, childOrderIDs: number[], countryCode: string, region = '') {
			await http.post(`/api/orders/${orderID}/group/update-egress-geo`, {
				child_order_ids: childOrderIDs,
				country_code: countryCode,
				region: region
			})
			await this.loadOrders()
		},
		async updateOrderGroupEgressGeoByMapping(orderID: number, lines: string, defaultCountryCode = '', defaultRegion = '') {
			await http.post(`/api/orders/${orderID}/group/update-egress-geo/mapping`, {
				lines,
				default_country_code: defaultCountryCode,
				default_region: defaultRegion
			})
			await this.loadOrders()
		},
		async updateOrderGroupCredentialsXLSX(orderID: number, file: File) {
			const form = new FormData()
			form.append('file', file)
			await http.post(`/api/orders/${orderID}/group/update-credentials/xlsx`, form, {
				headers: { 'Content-Type': 'multipart/form-data' }
			})
			await this.loadOrders()
		},
		async renewOrderGroupSelected(orderID: number, childOrderIDs: number[], moreDays: number, expiresAt = '') {
			await http.post(`/api/orders/${orderID}/group/renew-selected`, {
				child_order_ids: childOrderIDs,
				more_days: moreDays,
				expires_at: expiresAt
			})
			await this.loadOrders()
			this.setNotice('组内选中子订单续期完成')
		},
		async downloadOrderGroupSocks5Template(orderID: number) {
			return http.get(`/api/orders/${orderID}/group/template/socks5.xlsx`, { responseType: 'blob' })
		},
		async downloadOrderGroupCredentialsTemplate(orderID: number) {
			return http.get(`/api/orders/${orderID}/group/template/credentials.xlsx`, { responseType: 'blob' })
		},
    async deactivateOrder(orderID: number) {
      await http.post(`/api/orders/${orderID}/deactivate`, {})
      await this.loadOrders()
      this.setNotice('订单已停用')
    },
    async renewOrder(orderID: number, moreDays: number, expiresAt = '') {
      await http.post(`/api/orders/${orderID}/renew`, { more_days: moreDays, expires_at: expiresAt })
      await this.loadOrders()
      this.setNotice('订单续期完成')
    },
    async deleteOrder(orderID: number) {
      await http.delete(`/api/orders/${orderID}`)
      await this.loadOrders()
      this.setNotice('订单已删除')
    },
    async resetOrderCredentials(orderID: number) {
      await http.post(`/api/orders/${orderID}/credentials/reset`, {})
      await this.loadOrders()
      this.setNotice('家宽凭据已刷新')
    },
    async testOrder(orderID: number, samplePercent = 100) {
      const res = await http.post(`/api/orders/${orderID}/test`, { sample_percent: samplePercent })
      return res.data as Record<string, string>
    },
    async streamTestOrder(orderID: number, samplePercent: number, onEvent: (event: Record<string, any>) => void) {
      const token = localStorage.getItem('xtool_token') || ''
      const resp = await fetch(`/api/orders/${orderID}/test/stream`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({ sample_percent: samplePercent })
      })
      if (!resp.ok || !resp.body) {
        throw new Error(`stream test failed: ${resp.status}`)
      }
      const reader = resp.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''
        for (const line of lines) {
          const text = line.trim()
          if (!text) continue
          try {
            onEvent(JSON.parse(text))
          } catch {
            // ignore malformed line
          }
        }
      }
      const tail = buffer.trim()
      if (tail) {
        try {
          onEvent(JSON.parse(tail))
        } catch {
          // ignore malformed tail
        }
      }
    },
    async batchRenew(orderIDs: number[], moreDays: number, expiresAt = '') {
      const res = await http.post('/api/orders/batch/renew', {
        order_ids: orderIDs,
        more_days: moreDays,
        expires_at: expiresAt
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
    async batchExport(orderIDs: number[], format: 'txt' | 'xlsx' = 'xlsx', includeRawSocks5 = false) {
      const res = await http.post('/api/orders/batch/export', {
			order_ids: orderIDs,
			format,
			include_raw_socks5: includeRawSocks5
      }, {
			responseType: 'blob'
      })
		return res
    },
    async loadSettings() {
      const res = await http.get('/api/settings')
      this.settings = res.data
    },
    async saveSettings(payload: Record<string, string>) {
      await http.put('/api/settings', payload)
      await this.loadSettings()
      this.setNotice('设置已保存')
    },
    async testBark() {
      await http.post('/api/settings/bark/test', {})
      this.setNotice('Bark 测试通知已发送')
    },
    async loadRuntimeStats() {
      const res = await http.get('/api/runtime/customers')
      this.runtimeStats = res.data
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
    async scanSingboxConfigs() {
      const res = await http.post('/api/migrations/singbox/scan', {})
      this.singboxScan = res.data
      return this.singboxScan
    },
    async previewSingboxImport(files: string[]) {
      const res = await http.post('/api/migrations/singbox/preview', { files })
      this.importPreview = res.data
      return this.importPreview
    },
    async previewSocksMigration(lines: string) {
      const res = await http.post('/api/migrations/socks5/preview', { lines })
      this.migrationPreview = res.data
      return this.migrationPreview
    },
    async confirmImport(payload: {
      customer_id: number
      order_name: string
      expires_at: string
      rows: ImportPreviewRow[]
    }) {
      await http.post('/api/orders/import/confirm', payload)
      await this.loadOrders()
      await this.loadOversell(this.oversellCustomerID)
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
    async downloadBackup(name: string) {
      return http.get(`/api/db/backups/${encodeURIComponent(name)}/download`, { responseType: 'blob' })
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
