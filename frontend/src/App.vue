<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import { message, Modal } from 'ant-design-vue'
import {
	ApiOutlined,
	DashboardOutlined,
	DatabaseOutlined,
	DeleteOutlined,
	EditOutlined,
	FilterOutlined,
	LogoutOutlined,
	ReloadOutlined,
	SettingOutlined,
	TeamOutlined,
	UnorderedListOutlined,
	UploadOutlined
} from '@ant-design/icons-vue'
import { useAuthStore } from './stores/auth'
import { usePanelStore } from './stores/panel'
import { http } from './lib/http'
import type { ImportPreviewRow, Order } from './lib/types'

const auth = useAuthStore()
const panel = usePanelStore()

auth.init()

const loginForm = reactive({ username: auth.username || 'admin', password: '' })
const customerForm = reactive({ name: '', code: '', contact: '', notes: '' })
const probeForm = reactive({ ip: '0.0.0.0', port: 23457 })
const orderForm = reactive({
  customer_id: 0,
  name: '',
  quantity: 1,
  duration_day: 30,
  expires_at: '',
  mode: 'auto',
  port: 23457,
  manual_ip_ids: [] as number[],
  forward_outbound_ids: [] as number[]
})
const importForm = reactive({
	customer_id: 0,
	order_name: '',
	expires_at: '',
	lines: ''
})
const nodeForm = reactive({
  name: '',
  base_url: 'http://127.0.0.1:18080',
  username: 'admin',
  password: '',
  enabled: true,
  is_local: false
})
const forwardForm = reactive({
  name: '',
  address: '',
  port: 1080,
  username: '',
  password: '',
  route_user: '',
  enabled: true
})
const forwardImportLines = ref('')
const forwardEditOpen = ref(false)
const forwardEditForm = reactive({
  id: 0,
  name: '',
  address: '',
  port: 1080,
  username: '',
  password: '',
  route_user: '',
  enabled: true
})
const logFilters = reactive({
	level: '',
	keyword: '',
	start: '',
	end: '',
	limit: 50
})
const customerEditOpen = ref(false)
const customerEditForm = reactive({
	id: 0,
	name: '',
	code: '',
	contact: '',
	notes: '',
	status: 'active'
})

const orderEditOpen = ref(false)
const orderEditForm = reactive({
	id: 0,
	customer_id: 0,
	mode: 'auto',
	name: '',
	quantity: 1,
	port: 23457,
	expires_at: '',
	manual_ip_ids: [] as number[],
	forward_outbound_ids: [] as number[]
})
const oversellCustomerID = ref<number>(0)
const testSamplePercent = ref<number>(100)
const streamTestOpen = ref(false)
const streamTestOrderID = ref<number>(0)
const streamMeta = reactive({ total: 0, sampled: 0, sample_percent: 100, success: 0, failed: 0 })
const streamRows = ref<Array<{ item_id: number; status: string; detail: string }>>([])
const exportingOrderID = ref<number | null>(null)
const exportCount = ref<number>(0)

const siderCollapsed = ref(false)
const probeResult = ref('')
const testingOrderID = ref<number | null>(null)
const testResult = ref<Record<string, string> | null>(null)
const batchTestResult = ref<Array<{ id: number; success: boolean; result?: Record<string, string>; error?: string }> | null>(null)
const orderDetailOpen = ref(false)
const orderDetailLoading = ref(false)
const batchMoreDays = ref(30)

const menuItems = [
  { key: 'dashboard', icon: () => h(DashboardOutlined), label: '总览' },
  { key: 'customers', icon: () => h(TeamOutlined), label: '客户' },
  { key: 'ips', icon: () => h(DatabaseOutlined), label: 'IP池' },
  { key: 'orders', icon: () => h(UnorderedListOutlined), label: '订单' },
  { key: 'import', icon: () => h(UploadOutlined), label: '批量导入' },
  { key: 'settings', icon: () => h(SettingOutlined), label: '设置' }
]

const healthCards = computed(() => {
  const disabled = panel.orders.filter((o) => o.status === 'disabled').length
  return [
    { title: '激活订单', value: panel.activeOrderCount, color: '#059669' },
    { title: '到期订单', value: panel.expiredOrderCount, color: '#d97706' },
    { title: '停用订单', value: disabled, color: '#dc2626' },
    { title: '可用公网IP', value: panel.activeHostPublicCount, color: '#0284c7' }
  ]
})

const forwardStats = computed(() => {
	const total = panel.forwardOutbounds.length
	const enabled = panel.forwardOutbounds.filter((row) => row.enabled).length
	const ok = panel.forwardOutbounds.filter((row) => row.probe_status === 'ok').length
	const autoUser = panel.forwardOutbounds.filter((row) => String(row.route_user || '').includes('-')).length
	return { total, enabled, ok, autoUser }
})

const orderRows = computed(() => panel.orders.map((o) => ({ ...o, key: o.id })))
const rowSelection = computed(() => ({
  selectedRowKeys: panel.orderSelection,
  onChange: (keys: Array<string | number>) => {
    panel.orderSelection = keys.map((v) => Number(v))
  }
}))

const manualHostIPOptions = computed(() => {
	const enabled = panel.hostIPs.filter((v) => v.enabled && v.ip !== '127.0.0.1')
	const publicIPs = enabled.filter((v) => v.is_public)
	return publicIPs.length > 0 ? publicIPs : enabled
})

const enabledForwardOutbounds = computed(() => panel.forwardOutbounds.filter((row) => row.enabled))

function reuseHints(customerID: number, selectedIDs: number[], excludeOrderID = 0): string[] {
	if (!customerID || selectedIDs.length === 0) return []
	const selected = new Set(selectedIDs.map((v) => Number(v)).filter((v) => Number.isFinite(v) && v > 0))
	if (selected.size === 0) return []
	const reused = new Set<number>()
	for (const order of panel.orders) {
		if (Number(order.customer_id) !== Number(customerID)) continue
		if (Number(order.id) === Number(excludeOrderID)) continue
		if (order.status !== 'active') continue
		if (new Date(order.expires_at).getTime() <= Date.now()) continue
		for (const item of order.items || []) {
			const outboundID = Number((item as any).socks_outbound_id || 0)
			if (selected.has(outboundID)) {
				reused.add(outboundID)
			}
		}
	}
	const hints: string[] = []
	for (const id of reused) {
		const row = panel.forwardOutbounds.find((v) => Number(v.id) === Number(id))
		if (!row) continue
		hints.push(`SOCKS5出口 ${row.address}:${row.port} (${row.route_user || '未设置账号'}) 已在该客户其他活动订单使用`)
	}
	return hints
}

const createForwardReuseHints = computed(() => reuseHints(Number(orderForm.customer_id), orderForm.forward_outbound_ids, 0))
const editForwardReuseHints = computed(() => reuseHints(Number(orderEditForm.customer_id), orderEditForm.forward_outbound_ids, Number(orderEditForm.id)))

const ordersColumns = [
	{ title: 'ID', dataIndex: 'id', width: 80 },
	{ title: '客户', key: 'customer', width: 180 },
	{ title: '状态', dataIndex: 'status', width: 110 },
	{ title: '模式', dataIndex: 'mode', width: 110 },
	{ title: '数量', dataIndex: 'quantity', width: 90 },
	{ title: '转发出口', key: 'forward_summary', width: 240 },
	{ title: '端口', dataIndex: 'port', width: 100 },
	{ title: '到期', key: 'expires', width: 210 },
	{ title: '动作', key: 'action', fixed: 'right', width: 420 }
]

const customerColumns = [
	{ title: 'ID', dataIndex: 'id', width: 72 },
	{ title: '名称', dataIndex: 'name', width: 180 },
	{ title: '代号', dataIndex: 'code', width: 120 },
	{ title: '联系', dataIndex: 'contact' },
	{ title: '状态', dataIndex: 'status', width: 100 },
	{ title: '备注', dataIndex: 'notes' },
	{ title: '动作', key: 'action', width: 130 }
]

const hostColumns = [
  { title: 'IP', dataIndex: 'ip', key: 'ip', width: 220 },
  { title: '公网', dataIndex: 'is_public', key: 'is_public', width: 90 },
  { title: '状态', dataIndex: 'enabled', key: 'enabled', width: 90 },
  { title: '动作', key: 'action', width: 120 }
]

const oversellColumns = [
  { title: 'IP', dataIndex: 'ip', key: 'ip', width: 220 },
  { title: '总占用', dataIndex: 'total_active_count', key: 'total_active_count', width: 90 },
  { title: '当前客户占用', dataIndex: 'customer_active_count', key: 'customer_active_count', width: 110 },
  { title: '超卖率', key: 'oversell_rate', width: 120 },
  { title: '热度', key: 'heat', width: 220 },
  { title: '可用', dataIndex: 'enabled', key: 'enabled', width: 90 }
]

const runtimeColumns = [
	{ title: '客户', key: 'customer', width: 180 },
	{ title: '在线数', dataIndex: 'online_clients', key: 'online_clients', width: 90 },
	{ title: '实时速度', key: 'speed', width: 120 },
	{ title: '1h流量', key: 't1h', width: 100 },
	{ title: '24h流量', key: 't24h', width: 100 },
	{ title: '7d流量', key: 't7d', width: 100 },
	{ title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 170 }
]

const importColumns = [
  { title: '原始', dataIndex: 'raw', key: 'raw', width: 340 },
  { title: '本机IP', dataIndex: 'is_local_ip', key: 'is_local_ip', width: 90 },
  { title: '端口占用', dataIndex: 'port_occupied', key: 'port_occupied', width: 100 },
  { title: '状态', key: 'state', width: 160 }
]

const nodeColumns = [
  { title: '节点', dataIndex: 'name', key: 'name', width: 140 },
  { title: '地址', dataIndex: 'base_url', key: 'base_url' },
  { title: '状态', dataIndex: 'enabled', key: 'enabled', width: 80 },
  { title: '动作', key: 'action', width: 90 }
]

const migrationColumns = [
  { title: '原始', dataIndex: 'raw', key: 'raw', width: 280 },
  { title: '节点', dataIndex: 'node_name', key: 'node_name', width: 140 },
  { title: '状态', dataIndex: 'state', key: 'state', width: 110 },
  { title: '原因', dataIndex: 'reason', key: 'reason' }
]

const forwardOutboundColumns = [
  { title: '出口', key: 'addr', width: 200 },
  { title: '路由用户', dataIndex: 'route_user', key: 'route_user', width: 180 },
  { title: '出口IP', dataIndex: 'exit_ip', key: 'exit_ip', width: 150 },
  { title: '国家', dataIndex: 'country_code', key: 'country_code', width: 90 },
  { title: '探测', dataIndex: 'probe_status', key: 'probe_status', width: 90 },
  { title: '启用', dataIndex: 'enabled', key: 'enabled', width: 80 },
  { title: '动作', key: 'action', width: 210 }
]

const detailColumns = [
	{ title: 'IP', dataIndex: 'ip', key: 'ip', width: 170 },
	{ title: '端口', dataIndex: 'port', key: 'port', width: 90 },
	{ title: '账号', dataIndex: 'username', key: 'username', width: 130 },
	{ title: '密码', dataIndex: 'password', key: 'password', width: 130 },
	{ title: '出口', key: 'outbound', width: 90 },
	{ title: '转发目标', key: 'forward', width: 210 },
	{ title: '资源Tag', key: 'resource' }
]

const backupColumns = [
	{ title: '文件名', dataIndex: 'name', key: 'name' },
	{ title: '大小', key: 'size', width: 120 },
	{ title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 210 },
	{ title: '动作', key: 'action', width: 220 }
]

onMounted(async () => {
  if (!auth.isAuthed) return
  try {
    await panel.bootstrap()
    seedDefaultsFromSettings()
  } catch (err) {
    panel.setError(err)
    auth.logout()
  }
})

watch(
	() => orderForm.customer_id,
	async (id) => {
		if (!id) {
			panel.allocationPreview = null
			return
		}
		try {
			await panel.loadAllocationPreview(Number(id))
		} catch (err) {
			panel.setError(err)
		}
	}
)

function seedDefaultsFromSettings() {
  const p = Number(panel.settings.default_inbound_port || '23457')
  if (Number.isFinite(p) && p > 0) {
    orderForm.port = p
    probeForm.port = p
  }
}

function statusColor(status: string) {
  if (status === 'active') return 'green'
  if (status === 'expired') return 'gold'
  if (status === 'disabled') return 'red'
  return 'default'
}

function modeColor(mode: string) {
  if (mode === 'import') return 'blue'
  if (mode === 'manual') return 'purple'
  if (mode === 'forward') return 'cyan'
  return 'default'
}

function forwardSummary(order: any) {
	if (order.mode !== 'forward') return '-'
	const rows = (order.items || []).filter((item: any) => String(item.outbound_type || '') === 'socks5')
	if (rows.length === 0) return '0 条'
	const names = rows.map((item: any) => `${item.forward_address || '-'}:${item.forward_port || '-'}`)
	const preview = names.slice(0, 2).join(' | ')
	if (names.length <= 2) return `${rows.length} 条 / ${preview}`
	return `${rows.length} 条 / ${preview} +${names.length - 2}`
}

function migrationStateColor(state: string) {
  if (state === 'ready') return 'green'
  if (state === 'blocked') return 'red'
  if (state === 'unmatched' || state === 'ambiguous' || state === 'invalid') return 'orange'
  return 'default'
}

function showForwardWarnings(warnings: string[]) {
	if (!warnings || warnings.length === 0) return
	Modal.warning({
		title: '分流复用提醒',
		okText: '知道了',
		width: 620,
		content: h('div', { class: 'text-xs leading-6' }, warnings.map((row) => h('div', row)))
	})
}

function formatTime(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function expiresHint(expiresAt: string): string {
  const ts = new Date(expiresAt).getTime()
  if (!Number.isFinite(ts)) return '-'
  const diff = Math.floor((ts - Date.now()) / 1000)
  if (diff <= 0) return '已到期'
  const day = Math.floor(diff / 86400)
  const hour = Math.floor((diff % 86400) / 3600)
  if (day > 0) return `${day}天${hour}小时`
  const minute = Math.floor((diff % 3600) / 60)
  return `${hour}小时${minute}分钟`
}

async function doLogin() {
  try {
    await auth.login(loginForm)
    await panel.bootstrap()
    seedDefaultsFromSettings()
    message.success('登录成功')
  } catch (err) {
    panel.setError(err)
    message.error(panel.error || '登录失败')
  }
}

function doLogout() {
  auth.logout()
  panel.$reset()
}

function onMenuClick(info: { key: string }) {
  panel.activeTab = String(info.key)
}

async function refreshAll() {
  try {
    await panel.bootstrap()
    message.success('数据已刷新')
  } catch (err) {
    panel.setError(err)
  }
}

async function createCustomer() {
	try {
		await panel.createCustomer(customerForm)
		customerForm.name = ''
		customerForm.code = ''
		customerForm.contact = ''
		customerForm.notes = ''
		message.success('客户已创建')
	} catch (err) {
		panel.setError(err)
	}
}

function openCustomerEdit(row: { id: number; name: string; code?: string; contact: string; notes: string; status: string }) {
	customerEditForm.id = row.id
	customerEditForm.name = row.name
	customerEditForm.code = (row as any).code || ''
	customerEditForm.contact = row.contact || ''
	customerEditForm.notes = row.notes || ''
	customerEditForm.status = row.status || 'active'
	customerEditOpen.value = true
}

async function saveCustomerEdit() {
	try {
		await panel.updateCustomer(customerEditForm.id, {
			name: customerEditForm.name,
			code: customerEditForm.code,
			contact: customerEditForm.contact,
			notes: customerEditForm.notes,
			status: customerEditForm.status
		})
		customerEditOpen.value = false
		message.success('客户已更新')
	} catch (err) {
		panel.setError(err)
	}
}

function deleteCustomer(id: number) {
	Modal.confirm({
		title: '确认删除客户',
		content: '删除后无法恢复；若该客户已有订单会被拒绝删除。',
		okText: '删除',
		okType: 'danger',
		cancelText: '取消',
		async onOk() {
			try {
				await panel.deleteCustomer(id)
				message.success('客户已删除')
			} catch (err) {
				panel.setError(err)
			}
		}
	})
}

async function probePort() {
  try {
    const res = await http.post('/api/host-ips/probe', probeForm)
    probeResult.value = res.data.occupied ? '端口已占用' : '端口空闲'
  } catch (err) {
    panel.setError(err)
  }
}

async function createOrder() {
  try {
    if (orderForm.mode === 'forward' && orderForm.forward_outbound_ids.length === 0) {
      message.warning('请至少选择 1 个 SOCKS5 出口')
      return
    }
    const payload: Record<string, unknown> = {
      customer_id: Number(orderForm.customer_id),
      name: orderForm.name,
      duration_day: Number(orderForm.duration_day),
      expires_at: orderForm.expires_at ? new Date(orderForm.expires_at).toISOString() : '',
      mode: orderForm.mode,
      port: Number(orderForm.port),
      manual_ip_ids: orderForm.manual_ip_ids.map((v) => Number(v))
    }
    if (orderForm.mode === 'forward') {
      payload.forward_outbound_ids = orderForm.forward_outbound_ids.map((v) => Number(v))
    } else {
      payload.quantity = Number(orderForm.quantity)
    }
    const result = await panel.createOrder(payload)
    orderForm.name = ''
    orderForm.expires_at = ''
    if (orderForm.mode === 'forward') {
      orderForm.forward_outbound_ids = []
    }
    panel.orderSelection = []
    message.success('订单创建成功')
    showForwardWarnings(result?.warnings || [])
  } catch (err) {
    panel.setError(err)
  }
}

function setQuickExpiry(days: number, target: 'create' | 'edit') {
	const at = new Date(Date.now() + days * 24 * 3600 * 1000)
	const text = at.toISOString().slice(0, 19)
	if (target === 'create') {
		orderForm.expires_at = text
		return
	}
	orderEditForm.expires_at = text
}

function openOrderEdit(row: Order) {
	orderEditForm.id = row.id
	orderEditForm.customer_id = row.customer_id
	orderEditForm.mode = row.mode
	orderEditForm.name = row.name
	orderEditForm.quantity = row.quantity
	orderEditForm.port = row.port
	orderEditForm.expires_at = row.expires_at ? new Date(row.expires_at).toISOString().slice(0, 19) : ''
	orderEditForm.manual_ip_ids = []
	orderEditForm.forward_outbound_ids = Array.from(new Set((row.items || []).map((item: any) => Number(item.socks_outbound_id || 0)).filter((v) => v > 0)))
	orderEditOpen.value = true
	void panel.loadAllocationPreview(row.customer_id, row.id)
}

async function saveOrderEdit() {
	try {
		if (orderEditForm.mode === 'forward' && orderEditForm.forward_outbound_ids.length === 0) {
			message.warning('请至少选择 1 个 SOCKS5 出口')
			return
		}
		const payload: Record<string, unknown> = {
			name: orderEditForm.name,
			port: Number(orderEditForm.port),
			expires_at: orderEditForm.expires_at ? new Date(orderEditForm.expires_at).toISOString() : '',
			manual_ip_ids: orderEditForm.manual_ip_ids.map((v) => Number(v))
		}
		if (orderEditForm.mode === 'forward') {
			payload.forward_outbound_ids = orderEditForm.forward_outbound_ids.map((v) => Number(v))
		} else {
			payload.quantity = Number(orderEditForm.quantity)
		}
		const result = await panel.updateOrder(orderEditForm.id, payload)
		orderEditOpen.value = false
		message.success('订单已更新')
		showForwardWarnings(result?.warnings || [])
	} catch (err) {
		panel.setError(err)
	}
}

async function renewOrder(orderID: number, moreDays?: number) {
  const days = moreDays || Number(batchMoreDays.value)
  if (!days) return
  try {
    await panel.renewOrder(orderID, days)
    message.success('续期成功')
  } catch (err) {
    panel.setError(err)
  }
}

async function deactivateOrder(orderID: number) {
  try {
    await panel.deactivateOrder(orderID)
    message.success('订单已停用')
  } catch (err) {
    panel.setError(err)
  }
}

async function doBatchRenew() {
  if (panel.orderSelection.length === 0) return
  try {
    const results = await panel.batchRenew(panel.orderSelection, Number(batchMoreDays.value))
    const ok = results.filter((r) => r.success).length
    const fail = results.length - ok
    message.success(`批量续期完成，成功 ${ok}，失败 ${fail}`)
    panel.orderSelection = []
  } catch (err) {
    panel.setError(err)
  }
}

async function doBatchResync() {
	if (panel.orderSelection.length === 0) return
	try {
		const results = await panel.batchResync(panel.orderSelection)
		const ok = results.filter((r) => r.success).length
		const fail = results.length - ok
		message.success(`批量重同步完成，成功 ${ok}，失败 ${fail}`)
	} catch (err) {
		panel.setError(err)
	}
}

async function doBatchTest() {
	if (panel.orderSelection.length === 0) return
	try {
		batchTestResult.value = await panel.batchTest(panel.orderSelection)
		const ok = batchTestResult.value.filter((r) => r.success).length
		const fail = batchTestResult.value.length - ok
		message.success(`批量测活完成，成功 ${ok}，失败 ${fail}`)
	} catch (err) {
		panel.setError(err)
	}
}

async function doBatchExport() {
	if (panel.orderSelection.length === 0) return
	try {
		const text = await panel.batchExport(panel.orderSelection)
		downloadTextFile(text, `orders-batch-${Date.now()}.txt`)
	} catch (err) {
		panel.setError(err)
	}
}

async function doBatchDeactivate() {
  if (panel.orderSelection.length === 0) return
  Modal.confirm({
    title: '确认批量停用',
    content: `将停用 ${panel.orderSelection.length} 个订单，是否继续？`,
    okText: '继续',
    cancelText: '取消',
    async onOk() {
      try {
        const results = await panel.batchDeactivate(panel.orderSelection, 'disabled')
        const ok = results.filter((r) => r.success).length
        const fail = results.length - ok
        message.success(`批量停用完成，成功 ${ok}，失败 ${fail}`)
        panel.orderSelection = []
      } catch (err) {
        panel.setError(err)
      }
    }
  })
}

async function exportOrder(orderID: number) {
	try {
		exportingOrderID.value = orderID
		const params = new URLSearchParams()
		if (Number(exportCount.value) > 0) {
			params.set('count', String(Number(exportCount.value)))
		}
		params.set('shuffle', 'true')
		const query = params.toString()
		const res = await http.get(`/api/orders/${orderID}/export${query ? `?${query}` : ''}`, { responseType: 'text' })
		const text = typeof res.data === 'string' ? res.data : String(res.data)
		const header = String(res.headers['content-disposition'] || '')
		const match = header.match(/filename="?([^";]+)"?/i)
		downloadTextFile(text, match?.[1] || `order-${orderID}-socks5.txt`)
	} catch (err) {
		panel.setError(err)
	} finally {
		exportingOrderID.value = null
	}
}

async function testOrder(orderID: number) {
  try {
    testingOrderID.value = orderID
    testResult.value = await panel.testOrder(orderID, Number(testSamplePercent.value))
    message.success('测活已完成')
  } catch (err) {
    panel.setError(err)
  } finally {
    testingOrderID.value = null
  }
}

async function streamTestOrder(orderID: number) {
	streamTestOrderID.value = orderID
	streamRows.value = []
	streamMeta.total = 0
	streamMeta.sampled = 0
	streamMeta.sample_percent = Number(testSamplePercent.value)
	streamMeta.success = 0
	streamMeta.failed = 0
	streamTestOpen.value = true
	try {
		await panel.streamTestOrder(orderID, Number(testSamplePercent.value), (event) => {
			if (event.type === 'meta') {
				streamMeta.total = Number(event.total || 0)
				streamMeta.sampled = Number(event.sampled || 0)
				streamMeta.sample_percent = Number(event.sample_percent || testSamplePercent.value)
				return
			}
			if (event.type === 'result') {
				streamRows.value.unshift({
					item_id: Number(event.item_id || 0),
					status: String(event.status || ''),
					detail: String(event.detail || '')
				})
				if (event.status === 'ok') streamMeta.success += 1
				if (event.status === 'failed') streamMeta.failed += 1
				return
			}
			if (event.type === 'done') {
				streamMeta.success = Number(event.success_count || streamMeta.success)
				streamMeta.failed = Number(event.failure_count || streamMeta.failed)
			}
		})
		message.success('流式测活完成')
	} catch (err) {
		panel.setError(err)
	}
}

async function openOrderDetail(order: Order) {
  orderDetailOpen.value = true
  orderDetailLoading.value = true
  try {
    await panel.loadOrderDetail(order.id)
  } catch (err) {
    panel.setError(err)
  } finally {
    orderDetailLoading.value = false
  }
}

async function copyOrderLines(items: Order['items']) {
  const lines = items.map((item) => `${item.ip}:${item.port}:${item.username}:${item.password}`).join('\n')
  await navigator.clipboard.writeText(lines)
  message.success('发货内容已复制')
}

async function previewImport() {
  try {
    await panel.previewImport(importForm.lines)
    message.success('预检完成')
  } catch (err) {
    panel.setError(err)
  }
}

async function previewCrossNodeMigration() {
  try {
    const result = await panel.previewSocksMigration(importForm.lines)
    if (!result) return
    if (Number(result.blocked_node_count || 0) > 0) {
      message.warning(`发现 ${result.blocked_node_count} 个节点端口占用，已标红`)
      return
    }
    message.success('跨节点预检通过')
  } catch (err) {
    panel.setError(err)
  }
}

async function createNode() {
  try {
    await panel.createNode({
      name: nodeForm.name,
      base_url: nodeForm.base_url,
      username: nodeForm.username,
      password: nodeForm.password,
      enabled: nodeForm.enabled,
      is_local: nodeForm.is_local
    })
    nodeForm.name = ''
    nodeForm.password = ''
  } catch (err) {
    panel.setError(err)
  }
}

async function removeNode(id: number) {
  Modal.confirm({
    title: '删除节点',
    content: '确认删除这个 xraytool 节点吗？',
    okText: '删除',
    okType: 'danger',
    onOk: async () => {
      try {
        await panel.deleteNode(id)
      } catch (err) {
        panel.setError(err)
      }
    }
  })
}

async function createForwardOutbound() {
	try {
		await panel.createForwardOutbound({
			name: forwardForm.name,
			address: forwardForm.address,
			port: Number(forwardForm.port),
			username: forwardForm.username,
			password: forwardForm.password,
			route_user: forwardForm.route_user,
			enabled: forwardForm.enabled
		})
		forwardForm.name = ''
		forwardForm.address = ''
		forwardForm.username = ''
		forwardForm.password = ''
		forwardForm.route_user = ''
		message.success('转发出口已创建')
	} catch (err) {
		panel.setError(err)
	}
}

async function importForwardOutbounds() {
	if (!forwardImportLines.value.trim()) return
	try {
		const rows = await panel.importForwardOutbounds(forwardImportLines.value)
		const failed = rows.filter((row) => String(row.error || '').trim() !== '').length
		if (failed > 0) {
			message.warning(`导入完成，失败 ${failed} 条，请检查格式`) 
		} else {
			message.success('转发出口导入完成')
		}
	} catch (err) {
		panel.setError(err)
	}
}

async function probeForwardOutbound(id: number) {
	try {
		await panel.probeForwardOutbound(id)
		message.success('探测完成')
	} catch (err) {
		panel.setError(err)
	}
}

async function probeAllForwardOutbounds() {
	try {
		await panel.probeAllForwardOutbounds(true)
		message.success('批量探测完成')
	} catch (err) {
		panel.setError(err)
	}
}

async function removeForwardOutbound(id: number) {
	Modal.confirm({
		title: '删除转发出口',
		content: '确认删除这个 socks5 出口吗？',
		okText: '删除',
		okType: 'danger',
		onOk: async () => {
			try {
				await panel.deleteForwardOutbound(id)
			} catch (err) {
				panel.setError(err)
			}
		}
	})
}

function openForwardEdit(row: any) {
	forwardEditForm.id = Number(row.id)
	forwardEditForm.name = String(row.name || '')
	forwardEditForm.address = String(row.address || '')
	forwardEditForm.port = Number(row.port || 1080)
	forwardEditForm.username = String(row.username || '')
	forwardEditForm.password = ''
	forwardEditForm.route_user = String(row.route_user || '')
	forwardEditForm.enabled = Boolean(row.enabled)
	forwardEditOpen.value = true
}

async function saveForwardEdit() {
	try {
		const row = panel.forwardOutbounds.find((v) => v.id === Number(forwardEditForm.id))
		await panel.updateForwardOutbound(Number(forwardEditForm.id), {
			name: forwardEditForm.name,
			address: forwardEditForm.address,
			port: Number(forwardEditForm.port),
			username: forwardEditForm.username,
			password: forwardEditForm.password ? forwardEditForm.password : String(row?.password || ''),
			route_user: forwardEditForm.route_user,
			enabled: forwardEditForm.enabled
		})
		forwardEditOpen.value = false
		message.success('转发出口已更新')
	} catch (err) {
		panel.setError(err)
	}
}

async function confirmImport() {
  try {
    await panel.confirmImport({
      customer_id: Number(importForm.customer_id),
      order_name: importForm.order_name,
      expires_at: importForm.expires_at ? new Date(importForm.expires_at).toISOString() : '',
      rows: panel.importPreview as ImportPreviewRow[]
    })
    importForm.lines = ''
    panel.importPreview = []
    message.success('导入成功')
  } catch (err) {
    panel.setError(err)
  }
}

async function saveSettings() {
	try {
		await panel.saveSettings({
			default_inbound_port: panel.settings.default_inbound_port || '23457',
			bark_enabled: panel.settings.bark_enabled === 'true' ? 'true' : 'false',
			bark_base_url: panel.settings.bark_base_url || '',
			bark_device_key: panel.settings.bark_device_key || '',
			bark_group: panel.settings.bark_group || 'xraytool'
		})
		message.success('设置已保存')
	} catch (err) {
		panel.setError(err)
	}
}

async function sendBarkTest() {
	try {
		await panel.testBark()
		message.success('Bark 测试通知已发送')
	} catch (err) {
		panel.setError(err)
	}
}

async function applyLogFilter() {
	try {
		await panel.loadTaskLogs({
			level: logFilters.level,
			keyword: logFilters.keyword,
			start: logFilters.start,
			end: logFilters.end,
			limit: logFilters.limit
		})
	} catch (err) {
		panel.setError(err)
	}
}

async function resetLogFilter() {
	logFilters.level = ''
	logFilters.keyword = ''
	logFilters.start = ''
	logFilters.end = ''
	logFilters.limit = 50
	await applyLogFilter()
}

function bytesText(size: number): string {
	if (size < 1024) return `${size} B`
	if (size < 1024*1024) return `${(size/1024).toFixed(1)} KB`
	return `${(size/1024/1024).toFixed(2)} MB`
}

function bpsText(size: number): string {
	if (!Number.isFinite(size) || size <= 0) return '0 B/s'
	if (size < 1024) return `${size.toFixed(0)} B/s`
	if (size < 1024*1024) return `${(size/1024).toFixed(1)} KB/s`
	return `${(size/1024/1024).toFixed(2)} MB/s`
}

async function changeOversellView(customerID: number) {
	oversellCustomerID.value = customerID
	try {
		await panel.loadOversell(customerID)
	} catch (err) {
		panel.setError(err)
	}
}

async function createBackup() {
	try {
		await panel.createBackup()
		message.success('备份创建成功')
	} catch (err) {
		panel.setError(err)
	}
}

async function exportBackupDirect() {
	try {
		const res = await http.get('/api/db/backup/export', { responseType: 'blob' })
		const header = String(res.headers['content-disposition'] || '')
		const match = header.match(/filename="?([^";]+)"?/i)
		const name = match?.[1] || `xraytool-backup-${Date.now()}.db`
		const url = URL.createObjectURL(res.data)
		const a = document.createElement('a')
		a.href = url
		a.download = name
		a.click()
		URL.revokeObjectURL(url)
		message.success('已导出备份到浏览器下载')
	} catch (err) {
		panel.setError(err)
	}
}

async function downloadBackup(name: string) {
	try {
		const res = await panel.downloadBackup(name)
		const header = String(res.headers['content-disposition'] || '')
		const match = header.match(/filename="?([^";]+)"?/i)
		const saveName = match?.[1] || name
		const url = URL.createObjectURL(res.data)
		const a = document.createElement('a')
		a.href = url
		a.download = saveName
		a.click()
		URL.revokeObjectURL(url)
	} catch (err) {
		panel.setError(err)
	}
}

function deleteBackup(name: string) {
	Modal.confirm({
		title: '删除备份',
		content: `确认删除备份 ${name} ?`,
		okType: 'danger',
		async onOk() {
			try {
				await panel.deleteBackup(name)
				message.success('备份已删除')
			} catch (err) {
				panel.setError(err)
			}
		}
	})
}

function restoreBackup(name: string) {
	Modal.confirm({
		title: '恢复数据库',
		content: `将从 ${name} 恢复数据库，服务会自动重启，是否继续？`,
		okType: 'danger',
		async onOk() {
			try {
				await panel.restoreBackup(name)
				message.success('恢复指令已下发，服务将重启')
			} catch (err) {
				panel.setError(err)
			}
		}
	})
}

function downloadTextFile(text: string, filename: string) {
	const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
	const url = URL.createObjectURL(blob)
	const a = document.createElement('a')
	a.href = url
	a.download = filename
	a.click()
	URL.revokeObjectURL(url)
}
</script>

<template>
  <div class="app-shell">
    <div v-if="!auth.isAuthed" class="login-wrap">
      <a-card class="login-card" :bordered="false">
        <template #title>
          <span class="text-lg font-bold">XrayTool 托管服务</span>
        </template>
        <a-form layout="vertical">
          <a-form-item label="用户名">
            <a-input v-model:value="loginForm.username" placeholder="请输入用户名" @pressEnter="doLogin" />
          </a-form-item>
          <a-form-item label="密码">
            <a-input-password v-model:value="loginForm.password" placeholder="请输入密码" @pressEnter="doLogin" />
          </a-form-item>
          <a-button type="primary" block :loading="auth.loading" @click="doLogin">登录面板</a-button>
        </a-form>
        <a-alert v-if="auth.error" class="mt-3" type="error" :message="auth.error" show-icon />
      </a-card>
    </div>

    <a-layout v-else class="layout-root">
      <a-layout-sider v-model:collapsed="siderCollapsed" collapsible theme="dark" width="228">
        <div class="logo-row">XrayTool</div>
        <a-menu
          :selected-keys="[panel.activeTab]"
          theme="dark"
          mode="inline"
          :items="menuItems"
          @click="onMenuClick"
        />
      </a-layout-sider>

      <a-layout>
        <a-layout-header class="layout-header">
          <div>
            <div class="title">XrayTool Managed Panel</div>
            <div class="subtitle">Ant Design Vue 管理面板风格 · 支持详情弹窗/批量操作/状态可视化</div>
          </div>
          <a-space>
            <a-button :icon="h(ReloadOutlined)" @click="refreshAll">刷新</a-button>
            <a-button :icon="h(LogoutOutlined)" @click="doLogout">退出</a-button>
          </a-space>
        </a-layout-header>

        <a-layout-content class="layout-content">
          <a-alert v-if="panel.notice" :message="panel.notice" class="mb-3" type="success" show-icon />
          <a-alert v-if="panel.error" :message="panel.error" class="mb-3" type="error" show-icon />

          <template v-if="panel.activeTab === 'dashboard'">
            <a-row :gutter="12" class="mb-3">
              <a-col v-for="card in healthCards" :key="card.title" :xs="24" :sm="12" :lg="6">
                <a-card :bordered="false" class="metric-card">
                  <div class="metric-title">{{ card.title }}</div>
                  <div class="metric-value" :style="{ color: card.color }">{{ card.value }}</div>
                </a-card>
              </a-col>
            </a-row>

            <a-row :gutter="12">
              <a-col :xs="24" :lg="13">
                <a-card :bordered="false" title="IP 超卖热度" class="mb-3">
                  <template #extra>
                    <a-space>
                      <span class="text-xs text-slate-500">视图</span>
                      <a-select :value="oversellCustomerID" size="small" style="width: 180px" @change="(v:number) => changeOversellView(Number(v))">
                        <a-select-option :value="0">本机全局</a-select-option>
                        <a-select-option v-for="c in panel.customers" :key="c.id" :value="c.id">{{ c.name }}{{ c.code ? ` (${c.code})` : '' }}</a-select-option>
                      </a-select>
                    </a-space>
                  </template>
                  <div class="mb-2 ip-grid">
                    <div v-for="row in panel.oversell" :key="row.ip" class="ip-cell" :style="{ backgroundColor: row.oversold_count > 0 ? 'rgba(239,68,68,0.16)' : row.total_active_count > 0 ? 'rgba(34,197,94,0.16)' : 'rgba(148,163,184,0.08)' }" :title="`${row.ip} 总占用:${row.total_active_count}`">
                      <span>{{ row.ip }}</span>
                    </div>
                  </div>
                  <a-table
                    :columns="oversellColumns"
                    :data-source="panel.oversell"
                    :pagination="false"
                    size="small"
                    :row-key="(row:any) => row.ip"
                  >
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'enabled'">
                        <a-tag :color="record.enabled ? 'green' : 'red'">{{ record.enabled ? '启用' : '禁用' }}</a-tag>
                      </template>
                      <template v-else-if="column.key === 'oversell_rate'">
                        <a-tag :color="record.oversold_count > 0 ? 'red' : 'green'">{{ Number(record.oversell_rate || 0).toFixed(1) }}%</a-tag>
                      </template>
                      <template v-else-if="column.key === 'heat'">
                        <a-progress :percent="Math.min(100, Number(record.total_active_count) * 8)" :show-info="false" status="active" />
                      </template>
                    </template>
                  </a-table>
                </a-card>
                <a-card :bordered="false" title="Socks5 客户实时状态" class="mb-3">
                  <template #extra>
                    <a-button size="small" @click="panel.loadRuntimeStats">刷新</a-button>
                  </template>
                  <a-table :columns="runtimeColumns" :data-source="panel.runtimeStats" size="small" :row-key="(row:any)=>row.customer_id" :pagination="{ pageSize: 8 }">
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'customer'">
                        {{ record.customer_name }}{{ record.customer_code ? ` (${record.customer_code})` : '' }}
                      </template>
                      <template v-else-if="column.key === 'speed'">
                        {{ bpsText(Number(record.realtime_bps || 0)) }}
                      </template>
                      <template v-else-if="column.key === 't1h'">
                        {{ bytesText(Number(record.traffic_1h || 0)) }}
                      </template>
                      <template v-else-if="column.key === 't24h'">
                        {{ bytesText(Number(record.traffic_24h || 0)) }}
                      </template>
                      <template v-else-if="column.key === 't7d'">
                        {{ bytesText(Number(record.traffic_7d || 0)) }}
                      </template>
                      <template v-else-if="column.key === 'updated_at'">
                        {{ formatTime(record.updated_at) }}
                      </template>
                    </template>
                  </a-table>
                </a-card>
              </a-col>
              <a-col :xs="24" :lg="11">
                <a-card :bordered="false" title="系统任务日志" class="mb-3">
                  <div class="mb-2 grid grid-cols-2 gap-2">
                    <a-select v-model:value="logFilters.level" allow-clear size="small" placeholder="级别">
                      <a-select-option value="info">info</a-select-option>
                      <a-select-option value="warn">warn</a-select-option>
                      <a-select-option value="error">error</a-select-option>
                    </a-select>
                    <a-input v-model:value="logFilters.keyword" size="small" placeholder="关键词" />
                    <a-input v-model:value="logFilters.start" size="small" placeholder="开始时间 RFC3339" />
                    <a-input v-model:value="logFilters.end" size="small" placeholder="结束时间 RFC3339" />
                  </div>
                  <div class="mb-2 flex gap-2">
                    <a-button size="small" :icon="h(FilterOutlined)" @click="applyLogFilter">筛选</a-button>
                    <a-button size="small" @click="resetLogFilter">重置</a-button>
                  </div>
                  <a-list :data-source="panel.taskLogs" size="small" class="max-h-[24rem] overflow-auto">
                    <template #renderItem="{ item }">
                      <a-list-item>
                        <a-list-item-meta :description="item.message">
                          <template #title>{{ formatTime(item.created_at) }} · {{ item.level }}</template>
                        </a-list-item-meta>
                      </a-list-item>
                    </template>
                  </a-list>
                </a-card>
              </a-col>
            </a-row>
          </template>

          <template v-else-if="panel.activeTab === 'customers'">
            <a-row :gutter="12">
              <a-col :xs="24" :lg="8">
                <a-card :bordered="false" title="创建客户">
                  <a-form layout="vertical">
                    <a-form-item label="客户名"><a-input v-model:value="customerForm.name" /></a-form-item>
                    <a-form-item label="客户代号"><a-input v-model:value="customerForm.code" placeholder="如 liunian" /></a-form-item>
                    <a-form-item label="联系方式"><a-input v-model:value="customerForm.contact" /></a-form-item>
                    <a-form-item label="备注"><a-textarea v-model:value="customerForm.notes" :rows="4" /></a-form-item>
                    <a-button type="primary" block @click="createCustomer">创建客户</a-button>
                  </a-form>
                </a-card>
              </a-col>
              <a-col :xs="24" :lg="16">
                <a-card :bordered="false" title="客户列表">
                  <a-table
                    :columns="customerColumns"
                    :data-source="panel.customers"
                    :pagination="{ pageSize: 10 }"
                    :row-key="(row:any) => row.id"
                    size="small"
                  >
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.dataIndex === 'status'">
                        <a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status }}</a-tag>
                      </template>
                      <template v-else-if="column.key === 'action'">
                        <a-space :size="2">
                          <a-button size="small" :icon="h(EditOutlined)" @click="openCustomerEdit(record)" />
                          <a-button size="small" danger :icon="h(DeleteOutlined)" @click="deleteCustomer(record.id)" />
                        </a-space>
                      </template>
                    </template>
                  </a-table>
                </a-card>
              </a-col>
            </a-row>
          </template>

          <template v-else-if="panel.activeTab === 'ips'">
            <a-row :gutter="12">
              <a-col :xs="24" :lg="8">
                <a-card :bordered="false" title="IP扫描与端口检测" class="mb-3">
                  <a-space direction="vertical" style="width: 100%">
                    <a-button type="primary" block @click="panel.scanHostIPs">扫描本机IP</a-button>
                    <a-divider style="margin: 8px 0" />
                    <a-input v-model:value="probeForm.ip" addon-before="IP" />
                    <a-input-number v-model:value="probeForm.port" addon-before="Port" style="width: 100%" />
                    <a-button block @click="probePort">检测端口占用</a-button>
                    <a-alert v-if="probeResult" :message="probeResult" type="info" show-icon />
                  </a-space>
                </a-card>
              </a-col>
              <a-col :xs="24" :lg="16">
                <a-card :bordered="false" title="宿主机IP池">
                  <a-table
                    :columns="hostColumns"
                    :data-source="panel.hostIPs"
                    :row-key="(row:any) => row.id"
                    size="small"
                    :pagination="{ pageSize: 12 }"
                  >
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'ip'">
                        <span class="font-mono">{{ record.ip }}</span>
                      </template>
                      <template v-else-if="column.key === 'is_public'">
                        {{ record.is_public ? '是' : '否' }}
                      </template>
                      <template v-else-if="column.key === 'enabled'">
                        <a-tag :color="record.enabled ? 'green' : 'red'">{{ record.enabled ? '启用' : '禁用' }}</a-tag>
                      </template>
                      <template v-else-if="column.key === 'action'">
                        <a-switch :checked="record.enabled" @change="(checked:boolean) => panel.toggleHostIP(record.id, checked)" />
                      </template>
                    </template>
                  </a-table>
                </a-card>
              </a-col>
            </a-row>
          </template>

          <template v-else-if="panel.activeTab === 'orders'">
            <a-card :bordered="false" title="创建订单" class="mb-3">
              <a-row :gutter="8">
                <a-col :xs="24" :md="8"><a-select v-model:value="orderForm.customer_id" style="width: 100%" placeholder="选择客户">
                  <a-select-option :value="0">请选择</a-select-option>
                  <a-select-option v-for="c in panel.customers" :key="c.id" :value="c.id">{{ c.name }}</a-select-option>
                </a-select></a-col>
                <a-col :xs="24" :md="8"><a-input v-model:value="orderForm.name" placeholder="订单名(可空)" /></a-col>
                <a-col :xs="24" :md="8"><a-select v-model:value="orderForm.mode" style="width: 100%">
                  <a-select-option value="auto">自动分配</a-select-option>
                  <a-select-option value="manual">手动分配</a-select-option>
                  <a-select-option value="forward">转发分流</a-select-option>
                </a-select></a-col>
              </a-row>
              <a-row :gutter="8" class="mt-2">
                <a-col v-if="orderForm.mode !== 'forward'" :xs="24" :md="8"><a-input-number v-model:value="orderForm.quantity" :min="1" style="width: 100%" placeholder="数量" /></a-col>
                <a-col v-else :xs="24" :md="8"><a-input :value="`规则数: ${orderForm.forward_outbound_ids.length}`" disabled /></a-col>
                <a-col :xs="24" :md="8"><a-input-number v-model:value="orderForm.duration_day" :min="1" style="width: 100%" placeholder="有效天数" /></a-col>
                <a-col :xs="24" :md="8"><a-input-number v-model:value="orderForm.port" :min="1" :max="65535" style="width: 100%" placeholder="端口" /></a-col>
              </a-row>
              <a-row :gutter="8" class="mt-2">
                <a-col :xs="24" :md="12">
                  <a-date-picker v-model:value="orderForm.expires_at" show-time style="width:100%" value-format="YYYY-MM-DDTHH:mm:ss" placeholder="指定到期时间(可选)" />
                </a-col>
                <a-col :xs="24" :md="12" class="flex items-center">
                  <a-space>
                    <a-button size="small" @click="setQuickExpiry(7, 'create')">7天</a-button>
                    <a-button size="small" @click="setQuickExpiry(15, 'create')">15天</a-button>
                    <a-button size="small" @click="setQuickExpiry(30, 'create')">30天</a-button>
                    <a-button size="small" @click="setQuickExpiry(90, 'create')">90天</a-button>
                  </a-space>
                </a-col>
              </a-row>
              <a-alert v-if="panel.allocationPreview" class="mt-2" type="info" show-icon :message="`可分配IP: ${panel.allocationPreview.available} / 池总量: ${panel.allocationPreview.pool_size} / 已被该客户占用: ${panel.allocationPreview.used_by_customer}`" />
              <div v-if="orderForm.mode === 'manual'" class="mt-2">
                <a-select v-model:value="orderForm.manual_ip_ids" mode="multiple" style="width: 100%" placeholder="选择手动IP">
                  <a-select-option v-for="ip in manualHostIPOptions" :key="ip.id" :value="ip.id">{{ ip.ip }}</a-select-option>
                </a-select>
              </div>
              <div v-else-if="orderForm.mode === 'forward'" class="mt-2">
                <a-select
                  v-model:value="orderForm.forward_outbound_ids"
                  mode="multiple"
                  style="width:100%"
                  :max-tag-count="6"
                  placeholder="选择SOCKS5出口(可多选)"
                >
                  <a-select-option v-for="row in enabledForwardOutbounds" :key="row.id" :value="row.id">
                    {{ row.address }}:{{ row.port }} / {{ row.route_user || '未设置账号' }} / {{ (row.country_code || '--').toUpperCase() }}
                  </a-select-option>
                </a-select>
                <a-alert
                  class="mt-2"
                  type="info"
                  show-icon
                  :message="`SOCKS5出口数 = 转发规则数，当前可用出口: ${enabledForwardOutbounds.length}`"
                />
                <a-alert
                  v-if="createForwardReuseHints.length"
                  class="mt-2"
                  type="warning"
                  show-icon
                  :message="`复用提醒（不阻断）: ${createForwardReuseHints.length} 条`"
                  :description="createForwardReuseHints.join('；')"
                />
              </div>
              <div class="mt-3 flex justify-end">
                <a-button type="primary" @click="createOrder">下单并同步Xray</a-button>
              </div>
            </a-card>

            <a-card :bordered="false" title="订单列表">
              <template #extra>
                <a-space>
                  <span class="text-xs text-slate-500">已选 {{ panel.orderSelection.length }} 个</span>
                  <a-select v-model:value="testSamplePercent" size="small" style="width: 110px">
                    <a-select-option :value="100">测活100%</a-select-option>
                    <a-select-option :value="10">测活10%</a-select-option>
                    <a-select-option :value="5">测活5%</a-select-option>
                  </a-select>
                  <a-input-number v-model:value="exportCount" :min="0" size="small" :placeholder="'导出条数(0=全部)'" />
                  <a-input-number v-model:value="batchMoreDays" :min="1" :max="365" size="small" />
                  <a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchRenew">批量续期</a-button>
                  <a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchResync">批量重同步</a-button>
                  <a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchTest">批量测活</a-button>
                  <a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchExport">批量导出</a-button>
                  <a-button size="small" danger :disabled="panel.orderSelection.length===0" @click="doBatchDeactivate">批量停用</a-button>
                </a-space>
              </template>

              <a-table
                :columns="ordersColumns"
                :data-source="orderRows"
                :row-selection="rowSelection"
                :scroll="{ x: 1300 }"
                :pagination="{ pageSize: 12 }"
                size="small"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'customer'">
                    {{ record.customer?.name || record.customer_id }}
                  </template>
                  <template v-else-if="column.dataIndex === 'status'">
                    <a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag>
                  </template>
                  <template v-else-if="column.dataIndex === 'mode'">
                    <a-tag :color="modeColor(record.mode)">{{ record.mode }}</a-tag>
                  </template>
                  <template v-else-if="column.key === 'forward_summary'">
                    <span class="text-xs" :class="record.mode === 'forward' ? 'font-mono text-slate-700' : 'text-slate-400'">{{ forwardSummary(record) }}</span>
                  </template>
                  <template v-else-if="column.key === 'expires'">
                    <div>{{ expiresHint(record.expires_at) }}</div>
                    <div class="text-xs text-slate-500">{{ formatTime(record.expires_at) }}</div>
                  </template>
                  <template v-else-if="column.key === 'action'">
                    <a-space :size="4" wrap>
                      <a-button size="small" @click="openOrderDetail(record)">详情</a-button>
                      <a-button size="small" :loading="exportingOrderID===record.id" @click="exportOrder(record.id)">导出</a-button>
                      <a-button size="small" :loading="testingOrderID===record.id" @click="testOrder(record.id)">测活</a-button>
                      <a-button size="small" @click="streamTestOrder(record.id)">流式测活</a-button>
                      <a-button size="small" @click="openOrderEdit(record)">编辑</a-button>
                      <a-button size="small" @click="renewOrder(record.id)">续期</a-button>
                      <a-button size="small" danger @click="deactivateOrder(record.id)">停用</a-button>
                    </a-space>
                  </template>
                </template>
              </a-table>

              <a-alert v-if="testResult" class="mt-3" type="info" show-icon message="测活结果">
                <template #description>
                  <div v-for="(value, key) in testResult" :key="key" class="font-mono text-xs">item#{{ key }} => {{ value }}</div>
                </template>
              </a-alert>

              <a-alert v-if="batchTestResult" class="mt-3" type="info" show-icon message="批量测活结果">
                <template #description>
                  <div v-for="entry in batchTestResult" :key="entry.id" class="text-xs">
                    <strong>#{{ entry.id }}</strong>
                    <span v-if="entry.success"> - success</span>
                    <span v-else class="text-rose-600"> - {{ entry.error }}</span>
                  </div>
                </template>
              </a-alert>
            </a-card>
          </template>

          <template v-else-if="panel.activeTab === 'import'">
            <a-row :gutter="12">
              <a-col :xs="24" :lg="10">
                <a-card :bordered="false" title="批量导入已有 Socks5">
                  <a-space direction="vertical" style="width:100%">
                    <a-select v-model:value="importForm.customer_id" style="width:100%" placeholder="选择客户">
                      <a-select-option :value="0">请选择</a-select-option>
                      <a-select-option v-for="c in panel.customers" :key="c.id" :value="c.id">{{ c.name }}</a-select-option>
                    </a-select>
                    <a-input v-model:value="importForm.order_name" placeholder="导入订单名" />
                    <a-date-picker v-model:value="importForm.expires_at" show-time style="width:100%" value-format="YYYY-MM-DDTHH:mm:ss" />
                    <a-textarea v-model:value="importForm.lines" :rows="14" placeholder="每行: ip:port:user:pass" />
                    <a-space>
                      <a-button @click="previewImport">预检</a-button>
                      <a-button danger ghost @click="previewCrossNodeMigration">跨节点预检</a-button>
                      <a-button type="primary" @click="confirmImport">确认导入</a-button>
                    </a-space>
                  </a-space>
                </a-card>

                <a-card :bordered="false" title="xraytool 节点管理" class="mt-3">
                  <a-space direction="vertical" style="width:100%">
                    <a-input v-model:value="nodeForm.name" placeholder="节点名，例如 香港-01" />
                    <a-input v-model:value="nodeForm.base_url" placeholder="http://node:18080" />
                    <a-input v-model:value="nodeForm.username" placeholder="管理账号" />
                    <a-input-password v-model:value="nodeForm.password" placeholder="管理密码" />
                    <a-space>
                      <a-switch :checked="nodeForm.enabled" @change="(v:boolean)=>nodeForm.enabled=v" />
                      <span class="text-xs text-slate-500">启用节点</span>
                      <a-switch :checked="nodeForm.is_local" @change="(v:boolean)=>nodeForm.is_local=v" />
                      <span class="text-xs text-slate-500">标记本机</span>
                    </a-space>
                    <a-button type="primary" @click="createNode">新增节点</a-button>
                  </a-space>
                  <a-table
                    class="mt-3"
                    :columns="nodeColumns"
                    :data-source="panel.nodes"
                    size="small"
                    :pagination="{ pageSize: 5 }"
                    :row-key="(row:any) => row.id"
                  >
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.dataIndex === 'enabled'">
                        <a-tag :color="record.enabled ? 'green' : 'red'">{{ record.enabled ? '启用' : '停用' }}</a-tag>
                      </template>
                      <template v-else-if="column.key === 'action'">
                        <a-button danger size="small" :icon="h(DeleteOutlined)" @click="removeNode(record.id)">删除</a-button>
                      </template>
                    </template>
                  </a-table>
                </a-card>

                <a-card :bordered="false" title="转发 socks5 出口池" class="mt-3">
                  <a-row :gutter="8" class="mb-2">
                    <a-col :span="6"><a-statistic title="出口总数" :value="forwardStats.total" /></a-col>
                    <a-col :span="6"><a-statistic title="启用中" :value="forwardStats.enabled" /></a-col>
                    <a-col :span="6"><a-statistic title="探测成功" :value="forwardStats.ok" /></a-col>
                    <a-col :span="6"><a-statistic title="已含分流账号" :value="forwardStats.autoUser" /></a-col>
                  </a-row>
                  <a-space direction="vertical" style="width:100%">
                    <a-input v-model:value="forwardForm.name" placeholder="备注名(可空)" />
                    <a-input v-model:value="forwardForm.address" placeholder="出口地址/IP" />
                    <a-input-number v-model:value="forwardForm.port" :min="1" :max="65535" style="width:100%" />
                    <a-input v-model:value="forwardForm.username" placeholder="出口用户" />
                    <a-input-password v-model:value="forwardForm.password" placeholder="出口密码" />
                    <a-input v-model:value="forwardForm.route_user" placeholder="分流用户(可空, 自动生成 us-xxxxxxxxxx)" />
                    <a-space>
                      <a-switch :checked="forwardForm.enabled" @change="(v:boolean)=>forwardForm.enabled=v" />
                      <span class="text-xs text-slate-500">启用</span>
                    </a-space>
                    <a-button type="primary" @click="createForwardOutbound">新增出口</a-button>
                    <a-textarea v-model:value="forwardImportLines" :rows="5" placeholder="批量导入: ip:port:user:pass[:route_user]" />
                    <a-space>
                      <a-button @click="importForwardOutbounds">批量导入</a-button>
                      <a-button @click="probeAllForwardOutbounds">批量探测出口IP</a-button>
                    </a-space>
                  </a-space>
                  <a-table
                    class="mt-3"
                    :columns="forwardOutboundColumns"
                    :data-source="panel.forwardOutbounds"
                    size="small"
                    :pagination="{ pageSize: 6 }"
                    :row-key="(row:any) => row.id"
                  >
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'addr'">
                        <span class="font-mono text-xs">{{ record.address }}:{{ record.port }}</span>
                      </template>
                      <template v-else-if="column.key === 'country_code'">
                        <a-tag>{{ (record.country_code || '--').toUpperCase() }}</a-tag>
                      </template>
                      <template v-else-if="column.key === 'probe_status'">
                        <a-tag :color="record.probe_status === 'ok' ? 'green' : 'orange'">{{ record.probe_status || 'idle' }}</a-tag>
                        <div v-if="record.probe_error" class="text-[11px] text-rose-500">{{ record.probe_error }}</div>
                      </template>
                      <template v-else-if="column.key === 'enabled'">
                        <a-switch :checked="record.enabled" @change="(checked:boolean)=>panel.toggleForwardOutbound(record.id, checked)" />
                      </template>
                      <template v-else-if="column.key === 'action'">
                        <a-space :size="4">
                          <a-button size="small" @click="openForwardEdit(record)">编辑</a-button>
                          <a-button size="small" @click="probeForwardOutbound(record.id)">探测</a-button>
                          <a-button danger size="small" @click="removeForwardOutbound(record.id)">删除</a-button>
                        </a-space>
                      </template>
                    </template>
                  </a-table>
                </a-card>
              </a-col>
              <a-col :xs="24" :lg="14">
                <a-card :bordered="false" title="导入预检结果">
                  <a-table
                    :columns="importColumns"
                    :data-source="panel.importPreview"
                    :pagination="{ pageSize: 10 }"
                    size="small"
                    :row-key="(row:any, idx:number) => `${idx}-${row.raw}`"
                  >
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'state'">
                        <a-tag :color="record.error ? 'red' : 'green'">{{ record.error || 'ok' }}</a-tag>
                      </template>
                      <template v-else-if="column.dataIndex === 'raw'">
                        <span class="font-mono text-xs">{{ record.raw }}</span>
                      </template>
                      <template v-else-if="column.dataIndex === 'is_local_ip'">
                        {{ record.is_local_ip ? '是' : '否' }}
                      </template>
                      <template v-else-if="column.dataIndex === 'port_occupied'">
                        {{ record.port_occupied ? '是' : '否' }}
                      </template>
                    </template>
                  </a-table>
                </a-card>

                <a-card :bordered="false" title="跨节点渐进迁移预检" class="mt-3">
                  <a-alert
                    v-if="panel.migrationPreview && panel.migrationPreview.blocked_node_count > 0"
                    type="error"
                    show-icon
                    :message="`发现 ${panel.migrationPreview.blocked_node_count} 个节点端口冲突，已标红`"
                    description="请到目标服务器释放占用端口后重试。"
                    class="mb-3"
                  />
                  <a-space v-if="panel.migrationPreview" direction="vertical" style="width:100%" class="mb-3">
                    <div class="text-xs text-slate-600">可迁移: {{ panel.migrationPreview.ready_rows }} | 阻塞: {{ panel.migrationPreview.blocked_rows }} | 未匹配: {{ panel.migrationPreview.unmatched_rows }} | 多重归属: {{ panel.migrationPreview.ambiguous_rows }}</div>
                    <a-row :gutter="8">
                      <a-col v-for="node in panel.migrationPreview.nodes" :key="node.node_name" :xs="24" :md="12" :lg="8">
                        <a-card size="small" :style="{ borderColor: node.highlight_color === 'red' ? '#ef4444' : '#22c55e', background: node.highlight_color === 'red' ? 'rgba(239,68,68,0.06)' : 'rgba(34,197,94,0.06)' }">
                          <div class="font-semibold">{{ node.node_name }}</div>
                          <div class="text-xs text-slate-600">分配 {{ node.assigned_count }} / 就绪 {{ node.ready_count }}</div>
                          <div v-if="node.port_conflicts?.length" class="text-xs text-rose-600">占用端口: {{ node.port_conflicts.join(', ') }}</div>
                          <div v-if="node.error" class="text-xs text-rose-600">{{ node.error }}</div>
                          <div v-if="node.action_hint" class="text-xs text-slate-500">{{ node.action_hint }}</div>
                        </a-card>
                      </a-col>
                    </a-row>
                  </a-space>
                  <a-table
                    :columns="migrationColumns"
                    :data-source="panel.migrationPreview?.rows || []"
                    size="small"
                    :pagination="{ pageSize: 8 }"
                    :row-key="(row:any, idx:number) => `${idx}-${row.raw}-${row.state}`"
                  >
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.dataIndex === 'state'">
                        <a-tag :color="migrationStateColor(record.state)">{{ record.state }}</a-tag>
                      </template>
                      <template v-else-if="column.dataIndex === 'raw'">
                        <span class="font-mono text-xs">{{ record.raw }}</span>
                      </template>
                    </template>
                  </a-table>
                </a-card>
              </a-col>
            </a-row>
          </template>

          <template v-else-if="panel.activeTab === 'settings'">
            <a-card :bordered="false" title="系统设置" class="max-w-4xl mb-3">
              <a-row :gutter="12">
                <a-col :xs="24" :md="12"><a-form-item label="默认入口端口"><a-input v-model:value="panel.settings.default_inbound_port" /></a-form-item></a-col>
                <a-col :xs="24" :md="12">
                  <a-form-item label="Bark启用">
                    <a-switch :checked="panel.settings.bark_enabled === 'true'" @change="(checked:boolean)=> panel.settings.bark_enabled = checked ? 'true' : 'false'" />
                  </a-form-item>
                </a-col>
                <a-col :xs="24" :md="12"><a-form-item label="Bark地址"><a-input v-model:value="panel.settings.bark_base_url" /></a-form-item></a-col>
                <a-col :xs="24" :md="12"><a-form-item label="Bark设备Key"><a-input v-model:value="panel.settings.bark_device_key" /></a-form-item></a-col>
                <a-col :xs="24" :md="12"><a-form-item label="Bark分组"><a-input v-model:value="panel.settings.bark_group" /></a-form-item></a-col>
              </a-row>
              <div class="text-right">
                <a-space>
                  <a-button @click="sendBarkTest">发送测试通知</a-button>
                  <a-button type="primary" :icon="h(ApiOutlined)" @click="saveSettings">保存设置</a-button>
                </a-space>
              </div>
            </a-card>

            <a-card :bordered="false" title="数据库备份恢复" class="max-w-4xl">
              <template #extra>
                <a-space>
                  <a-button type="primary" ghost @click="exportBackupDirect">一键导出到本机</a-button>
                  <a-button @click="panel.loadBackups">刷新</a-button>
                  <a-button type="primary" @click="createBackup">创建备份</a-button>
                </a-space>
              </template>
              <a-table
                :columns="backupColumns"
                :data-source="panel.backups"
                size="small"
                :row-key="(row:any) => row.name"
                :pagination="{ pageSize: 8 }"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'size'">
                    {{ bytesText(record.size_bytes) }}
                  </template>
                  <template v-else-if="column.key === 'updated_at'">
                    {{ formatTime(record.updated_at) }}
                  </template>
                  <template v-else-if="column.key === 'action'">
                    <a-space :size="4">
                      <a-button size="small" @click="downloadBackup(record.name)">下载</a-button>
                      <a-button size="small" @click="restoreBackup(record.name)">恢复</a-button>
                      <a-button size="small" danger @click="deleteBackup(record.name)">删除</a-button>
                    </a-space>
                  </template>
                </template>
              </a-table>
              <div class="mt-2 text-xs text-slate-500">恢复备份后服务会自动退出并由 systemd 拉起。</div>
            </a-card>
          </template>
        </a-layout-content>
      </a-layout>
    </a-layout>

    <a-modal v-model:open="orderDetailOpen" title="订单详情" width="980px" :footer="null">
      <div v-if="orderDetailLoading" class="py-8 text-center">加载中...</div>
      <div v-else-if="panel.selectedOrder">
        <a-descriptions bordered :column="2" size="small" class="mb-3">
          <a-descriptions-item label="订单ID">#{{ panel.selectedOrder.id }}</a-descriptions-item>
          <a-descriptions-item label="客户">{{ panel.selectedOrder.customer?.name || panel.selectedOrder.customer_id }}</a-descriptions-item>
          <a-descriptions-item label="状态"><a-tag :color="statusColor(panel.selectedOrder.status)">{{ panel.selectedOrder.status }}</a-tag></a-descriptions-item>
          <a-descriptions-item label="模式"><a-tag :color="modeColor(panel.selectedOrder.mode)">{{ panel.selectedOrder.mode }}</a-tag></a-descriptions-item>
          <a-descriptions-item label="开始">{{ formatTime(panel.selectedOrder.starts_at) }}</a-descriptions-item>
          <a-descriptions-item label="到期">{{ formatTime(panel.selectedOrder.expires_at) }}</a-descriptions-item>
        </a-descriptions>

        <div class="mb-2 flex items-center justify-between">
          <div class="font-semibold">订单条目 ({{ panel.selectedOrder.items.length }})</div>
          <a-space>
            <a-input-number v-model:value="exportCount" :min="0" :max="panel.selectedOrder.items.length" size="small" />
            <a-button size="small" :loading="exportingOrderID===panel.selectedOrder.id" @click="exportOrder(panel.selectedOrder.id)">提取导出</a-button>
            <a-button size="small" @click="copyOrderLines(panel.selectedOrder.items)">复制发货内容</a-button>
          </a-space>
        </div>

        <a-table
          :columns="detailColumns"
          :data-source="panel.selectedOrder.items.map((item)=>({ ...item, key:item.id }))"
          :pagination="false"
          size="small"
          :scroll="{ x: 900, y: 380 }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'outbound'">
              <a-tag :color="record.outbound_type === 'socks5' ? 'cyan' : 'blue'">{{ record.outbound_type || 'direct' }}</a-tag>
            </template>
            <template v-else-if="column.key === 'forward'">
              <span v-if="record.outbound_type === 'socks5'" class="font-mono text-xs">{{ record.forward_address }}:{{ record.forward_port }}</span>
              <span v-else>-</span>
            </template>
            <template v-else-if="column.key === 'resource'">
              <div v-if="record.resources?.length" class="font-mono text-xs">
                <div v-for="res in record.resources" :key="res.outbound_tag">{{ res.outbound_tag }} / {{ res.rule_tag }}</div>
              </div>
              <span v-else>-</span>
            </template>
          </template>
        </a-table>
      </div>
    </a-modal>

    <a-modal v-model:open="customerEditOpen" title="编辑客户" @ok="saveCustomerEdit">
      <a-form layout="vertical">
        <a-form-item label="客户名"><a-input v-model:value="customerEditForm.name" /></a-form-item>
        <a-form-item label="客户代号"><a-input v-model:value="customerEditForm.code" /></a-form-item>
        <a-form-item label="联系方式"><a-input v-model:value="customerEditForm.contact" /></a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="customerEditForm.status">
            <a-select-option value="active">active</a-select-option>
            <a-select-option value="disabled">disabled</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="备注"><a-textarea v-model:value="customerEditForm.notes" :rows="3" /></a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="orderEditOpen" title="编辑订单" @ok="saveOrderEdit">
      <a-form layout="vertical">
        <a-form-item label="模式"><a-tag :color="modeColor(orderEditForm.mode)">{{ orderEditForm.mode }}</a-tag></a-form-item>
        <a-form-item label="订单名称"><a-input v-model:value="orderEditForm.name" /></a-form-item>
        <a-form-item v-if="orderEditForm.mode !== 'forward'" label="数量"><a-input-number v-model:value="orderEditForm.quantity" :min="1" style="width:100%" /></a-form-item>
        <a-form-item v-else label="SOCKS5转发出口">
          <a-select v-model:value="orderEditForm.forward_outbound_ids" mode="multiple" style="width:100%" placeholder="选择SOCKS5出口">
            <a-select-option v-for="row in enabledForwardOutbounds" :key="row.id" :value="row.id">
              {{ row.address }}:{{ row.port }} / {{ row.route_user || '未设置账号' }} / {{ (row.country_code || '--').toUpperCase() }}
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="端口"><a-input-number v-model:value="orderEditForm.port" :min="1" :max="65535" style="width:100%" /></a-form-item>
        <a-form-item label="到期时间"><a-date-picker v-model:value="orderEditForm.expires_at" show-time style="width:100%" value-format="YYYY-MM-DDTHH:mm:ss" /></a-form-item>
        <a-space class="mb-2">
          <a-button size="small" @click="setQuickExpiry(7, 'edit')">7天</a-button>
          <a-button size="small" @click="setQuickExpiry(30, 'edit')">30天</a-button>
          <a-button size="small" @click="setQuickExpiry(90, 'edit')">90天</a-button>
        </a-space>
        <a-alert v-if="panel.allocationPreview" type="info" show-icon :message="`可分配IP: ${panel.allocationPreview.available} / 池总量: ${panel.allocationPreview.pool_size} / 已占用: ${panel.allocationPreview.used_by_customer}`" />
        <a-alert v-if="orderEditForm.mode === 'forward' && editForwardReuseHints.length" class="mt-2" type="warning" show-icon :message="`复用提醒（不阻断）: ${editForwardReuseHints.length} 条`" :description="editForwardReuseHints.join('；')" />
      </a-form>
    </a-modal>

    <a-modal v-model:open="forwardEditOpen" title="编辑转发出口" @ok="saveForwardEdit">
      <a-form layout="vertical">
        <a-form-item label="备注名"><a-input v-model:value="forwardEditForm.name" /></a-form-item>
        <a-form-item label="地址"><a-input v-model:value="forwardEditForm.address" /></a-form-item>
        <a-form-item label="端口"><a-input-number v-model:value="forwardEditForm.port" :min="1" :max="65535" style="width:100%" /></a-form-item>
        <a-form-item label="出口用户"><a-input v-model:value="forwardEditForm.username" /></a-form-item>
        <a-form-item label="出口密码"><a-input-password v-model:value="forwardEditForm.password" placeholder="留空则保持不变" /></a-form-item>
        <a-form-item label="分流账号"><a-input v-model:value="forwardEditForm.route_user" placeholder="例如 us-abc123xxxx" /></a-form-item>
        <a-form-item>
          <a-switch :checked="forwardEditForm.enabled" @change="(v:boolean)=>forwardEditForm.enabled=v" />
          <span class="ml-2 text-xs text-slate-500">启用</span>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="streamTestOpen" title="流式测活结果" :footer="null" width="860px">
      <a-alert type="info" show-icon :message="`总条目 ${streamMeta.total} / 抽样 ${streamMeta.sampled} (${streamMeta.sample_percent}%) / 成功 ${streamMeta.success} / 失败 ${streamMeta.failed}`" class="mb-2" />
      <a-table :data-source="streamRows.map((v, idx)=>({ ...v, key: `${v.item_id}-${idx}` }))" :pagination="{ pageSize: 20 }" size="small">
        <a-table-column title="Item" data-index="item_id" key="item_id" />
        <a-table-column title="状态" data-index="status" key="status" />
        <a-table-column title="详情" data-index="detail" key="detail" />
      </a-table>
    </a-modal>
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
}

.login-wrap {
  display: flex;
  min-height: 100vh;
  align-items: center;
  justify-content: center;
  padding: 16px;
}

.login-card {
  width: 420px;
  border-radius: 18px;
  box-shadow: 0 18px 50px rgba(15, 23, 42, 0.12);
}

.layout-root {
  min-height: 100vh;
}

.logo-row {
  margin: 16px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
  font-weight: 800;
  text-align: center;
  padding: 10px 8px;
  letter-spacing: 0.3px;
}

.layout-header {
  background: #fff;
  border-bottom: 1px solid rgba(148, 163, 184, 0.28);
  height: auto;
  min-height: 66px;
  line-height: normal;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.title {
  font-size: 20px;
  font-weight: 800;
  color: #0f172a;
}

.subtitle {
  color: #64748b;
  font-size: 12px;
}

.layout-content {
  padding: 14px;
}

.metric-card {
  border-radius: 14px;
}

.metric-title {
  color: #64748b;
  font-size: 12px;
  letter-spacing: 0.3px;
}

.metric-value {
  margin-top: 8px;
  font-size: 34px;
  font-weight: 800;
  line-height: 1;
}

.ip-grid {
	display: grid;
	grid-template-columns: repeat(4, minmax(0, 1fr));
	gap: 6px;
	max-height: 200px;
	overflow: auto;
}

.ip-cell {
	border: 1px solid rgba(148, 163, 184, 0.24);
	border-radius: 8px;
	padding: 4px 6px;
	font-size: 11px;
	line-height: 1.2;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

@media (max-width: 768px) {
	.ip-grid {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}
}
</style>
