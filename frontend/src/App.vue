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
	forward_outbound_ids: [] as number[],
	dedicated_entry_id: 0,
	dedicated_inbound_id: 0,
	dedicated_ingress_id: 0,
	dedicated_protocol: 'mixed',
	dedicated_egress_lines: ''
})
const importForm = reactive({
	customer_id: 0,
	order_name: '',
	expires_at: '',
	lines: ''
})
const singboxSelectedFiles = ref<string[]>([])
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
const forwardManagerOpen = ref(false)
const forwardEditOpen = ref(false)
const dedicatedManagerOpen = ref(false)
const dedicatedEditOpen = ref(false)
const dedicatedInboundEditOpen = ref(false)
const dedicatedIngressEditOpen = ref(false)
const dedicatedForm = reactive({
	id: 0,
	name: '',
	domain: '',
	mixed_port: 1080,
	vmess_port: 10086,
	vless_port: 10087,
	shadowsocks_port: 10088,
	priority: 100,
	features: ['mixed', 'vmess', 'vless', 'shadowsocks'] as string[],
	enabled: true,
	notes: ''
})
const dedicatedInboundForm = reactive({
	name: '',
	protocol: 'mixed',
	listen_port: 1080,
	priority: 100,
	enabled: true,
	notes: ''
})
const dedicatedInboundEditForm = reactive({
	id: 0,
	name: '',
	protocol: 'mixed',
	listen_port: 1080,
	priority: 100,
	enabled: true,
	notes: ''
})
const dedicatedIngressForm = reactive({
	dedicated_inbound_id: 0,
	name: '',
	domain: '',
	ingress_port: 0,
	country_code: '',
	region: '',
	priority: 100,
	enabled: true,
	notes: ''
})
const dedicatedIngressEditForm = reactive({
	id: 0,
	dedicated_inbound_id: 0,
	name: '',
	domain: '',
	ingress_port: 0,
	country_code: '',
	region: '',
	priority: 100,
	enabled: true,
	notes: ''
})
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
	forward_outbound_ids: [] as number[],
	dedicated_entry_id: 0,
	dedicated_inbound_id: 0,
	dedicated_ingress_id: 0,
	dedicated_protocol: 'mixed',
	dedicated_egress_lines: '',
	dedicated_credential_lines: '',
	regenerate_dedicated_credentials: false
})
const oversellCustomerID = ref<number>(0)
const testSamplePercent = ref<number>(100)
const streamTestOpen = ref(false)
const streamTestOrderID = ref<number>(0)
const streamMeta = reactive({ total: 0, sampled: 0, sample_percent: 100, success: 0, failed: 0 })
const streamRows = ref<Array<{ item_id: number; status: string; detail: string }>>([])
const exportingOrderID = ref<number | null>(null)
const exportCount = ref<number>(0)
const exportFormat = ref<'txt' | 'xlsx'>('xlsx')
const exportIncludeRawSocks5 = ref(false)
const groupSocksModalOpen = ref(false)
const groupCredModalOpen = ref(false)
const groupGeoModalOpen = ref(false)
const groupEditorOpen = ref(false)
const groupEditorHeadOrderID = ref<number>(0)
const groupEditorChildOrderIDs = ref<number[]>([])
const groupTargetOrderID = ref<number>(0)
const groupBatchChildOrderIDs = ref<number[]>([])
const groupSocksLines = ref('')
const groupCredLines = ref('')
const groupCredRegenerate = ref(false)
const groupGeoCountryCode = ref('')
const groupGeoRegion = ref('')
const groupGeoMappingLines = ref('')
const groupRenewModalOpen = ref(false)
const groupRenewHeadOrderID = ref<number>(0)
const groupRenewDays = ref<number>(30)
const groupRenewExpiresAt = ref('')
const groupRenewChildOrderIDs = ref<number[]>([])
const creatingOrder = ref(false)
const savingOrderEdit = ref(false)
const confirmingImport = ref(false)
const previewingImport = ref(false)
const previewingSingboxImport = ref(false)
const groupSocksSaving = ref(false)
const groupCredSaving = ref(false)
const groupRenewSaving = ref(false)
const importPreviewFingerprint = ref('')
const importPreviewSource = ref<'lines' | 'singbox' | ''>('')
const dedicatedProbeRunning = ref(false)
const dedicatedProbeRows = ref<Array<{ index: number; raw: string; available: boolean; exit_ip: string; country_code: string; region: string; error?: string }>>([])
const dedicatedProbeMeta = reactive({ total: 0, success: 0, failed: 0 })

const siderCollapsed = ref(false)
const probeResult = ref('')
const testingOrderID = ref<number | null>(null)
const testResult = ref<Record<string, string> | null>(null)
const batchTestResult = ref<Array<{ id: number; success: boolean; result?: Record<string, string>; error?: string }> | null>(null)
const orderDetailOpen = ref(false)
const orderDetailLoading = ref(false)
const orderSearchKeyword = ref('')
const orderModeFilter = ref<'all' | 'home' | 'dedicated'>('all')
const orderStatusFilter = ref<'all' | 'active' | 'expired' | 'disabled'>('all')
const deliverySearchKeyword = ref('')
const orderPagination = reactive({ current: 1, pageSize: 12 })
const deliveryPagination = reactive({ current: 1, pageSize: 12 })
const orderEditCurrentInboundLines = ref('')
const orderEditCurrentEgressLines = ref('')
const batchMoreDays = ref(30)
const batchRenewExpiresAt = ref('')
const createForwardReuseHints = ref<string[]>([])
const editForwardReuseHints = ref<string[]>([])

const menuItems = [
  { key: 'dashboard', icon: () => h(DashboardOutlined), label: '总览' },
  { key: 'customers', icon: () => h(TeamOutlined), label: '客户' },
  { key: 'ips', icon: () => h(DatabaseOutlined), label: 'IP池' },
  { key: 'orders', icon: () => h(UnorderedListOutlined), label: '订单' },
  { key: 'delivery', icon: () => h(ApiOutlined), label: '发货' },
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

function isResidentialMode(mode: string): boolean {
	const v = String(mode || '').toLowerCase()
	return v === 'auto' || v === 'manual' || v === 'import'
}

const orderRows = computed(() => {
	const rows = panel.orders.map((o) => ({ ...o, key: o.id, children: [] as any[] }))
	const map = new Map<number, any>()
	const roots: any[] = []
	for (const row of rows) {
		map.set(Number(row.id), row)
	}
	for (const row of rows) {
		const parentID = Number((row as any).parent_order_id || 0)
		if (parentID > 0 && map.has(parentID)) {
			map.get(parentID)?.children?.push(row)
			continue
		}
		roots.push(row)
	}
	for (const row of rows) {
		if (Array.isArray(row.children) && row.children.length > 0) {
			row.children.sort((a: any, b: any) => Number(a.sequence_no || 0) - Number(b.sequence_no || 0) || Number(a.id) - Number(b.id))
		}
	}
	return roots
})

function matchesOrderModeFilter(mode: string, filterValue: 'all' | 'home' | 'dedicated'): boolean {
	if (filterValue === 'all') return true
	if (filterValue === 'home') return isResidentialMode(mode)
	return String(mode || '') === 'dedicated'
}

function orderSearchContent(order: any): string {
	const parts: string[] = []
	parts.push(String(order.id || ''))
	parts.push(String(order.order_no || ''))
	parts.push(String(order.name || ''))
	parts.push(String(order.mode || ''))
	parts.push(String(order.status || ''))
	parts.push(String(order.customer?.name || ''))
	parts.push(String(order.customer?.code || ''))
	parts.push(String(order.dedicated_ingress?.domain || ''))
	parts.push(String(order.dedicated_inbound?.protocol || ''))
	parts.push(String(order.dedicated_protocol || ''))
	for (const item of order.items || []) {
		parts.push(String(item.ip || ''))
		parts.push(String(item.port || ''))
		parts.push(String(item.username || ''))
		parts.push(String(item.forward_address || ''))
		parts.push(String(item.forward_port || ''))
	}
	return parts.join(' ').toLowerCase()
}

function matchesOrderFilters(order: any): boolean {
	if (!matchesOrderModeFilter(String(order.mode || ''), orderModeFilter.value)) return false
	if (orderStatusFilter.value !== 'all' && String(order.status || '') !== orderStatusFilter.value) return false
	const keyword = String(orderSearchKeyword.value || '').trim().toLowerCase()
	if (!keyword) return true
	return orderSearchContent(order).includes(keyword)
}

const filteredOrderRows = computed(() => {
	const rows: any[] = []
	for (const root of orderRows.value) {
		const children = Array.isArray(root.children) ? root.children : []
		const rootMatched = matchesOrderFilters(root)
		if (children.length === 0) {
			if (rootMatched) rows.push(root)
			continue
		}
		if (rootMatched) {
			rows.push(root)
			continue
		}
		const childMatched = children.filter((child: any) => matchesOrderFilters(child))
		if (childMatched.length > 0) {
			rows.push({ ...root, children: childMatched })
		}
	}
	return rows
})

const deliveryCustomerID = ref<number>(0)
const deliveryMode = ref<'all' | 'home' | 'dedicated'>('all')
const deliveryRows = computed(() =>
	panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === 0)
		.filter((row) => {
			if (deliveryCustomerID.value > 0 && Number(row.customer_id) !== Number(deliveryCustomerID.value)) return false
			if (deliveryMode.value === 'home') return isResidentialMode(String(row.mode || ''))
			if (deliveryMode.value === 'dedicated') return String(row.mode || '') === 'dedicated'
			return true
		})
		.filter((row) => {
			const keyword = String(deliverySearchKeyword.value || '').trim().toLowerCase()
			if (!keyword) return true
			return orderSearchContent(row).includes(keyword)
		})
		.map((row) => ({ ...row, key: row.id }))
)

const isForwardDeprecatedOrderEdit = computed(() => orderEditOpen.value && String(orderEditForm.mode || '') === 'forward')
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
const enabledDedicatedEntries = computed(() => panel.dedicatedEntries.filter((row) => row.enabled))
const enabledDedicatedInbounds = computed(() => panel.dedicatedInbounds.filter((row) => row.enabled))
const enabledDedicatedIngresses = computed(() => panel.dedicatedIngresses.filter((row) => row.enabled))
const dedicatedProtocolOptions = [
	{ label: 'Socks5(Mixed)', value: 'mixed' },
	{ label: 'Vmess', value: 'vmess' },
	{ label: 'Vless', value: 'vless' },
	{ label: 'Shadowsocks', value: 'shadowsocks' }
]

const filteredDedicatedInboundsForCreate = computed(() =>
	enabledDedicatedInbounds.value.filter((row) => String(row.protocol || '') === String(orderForm.dedicated_protocol || 'mixed'))
)

const filteredDedicatedIngressesForCreate = computed(() =>
	enabledDedicatedIngresses.value.filter((row) => Number(row.dedicated_inbound_id) === Number(orderForm.dedicated_inbound_id || 0))
)

const filteredDedicatedInboundsForEdit = computed(() =>
	panel.dedicatedInbounds.filter((row) => {
		if (String(row.protocol || '') !== String(orderEditForm.dedicated_protocol || 'mixed')) return false
		if (row.enabled) return true
		return Number(row.id) === Number(orderEditForm.dedicated_inbound_id || 0)
	})
)

const filteredDedicatedIngressesForEdit = computed(() =>
	panel.dedicatedIngresses.filter((row) => {
		if (Number(row.dedicated_inbound_id) !== Number(orderEditForm.dedicated_inbound_id || 0)) return false
		if (row.enabled) return true
		return Number(row.id) === Number(orderEditForm.dedicated_ingress_id || 0)
	})
)

const groupRenewCandidates = computed(() =>
	panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(groupRenewHeadOrderID.value || 0))
		.sort((a, b) => Number((a as any).sequence_no || 0) - Number((b as any).sequence_no || 0))
)

const groupBatchCandidates = computed(() =>
	panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(groupTargetOrderID.value || 0))
		.sort((a, b) => Number((a as any).sequence_no || 0) - Number((b as any).sequence_no || 0))
)

const groupEditorCandidates = computed(() =>
	panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(groupEditorHeadOrderID.value || 0))
		.sort((a, b) => Number((a as any).sequence_no || 0) - Number((b as any).sequence_no || 0))
)

const importPreviewValid = computed(() => {
	if ((panel.importPreview || []).length === 0) return false
	return importPreviewFingerprint.value !== '' && importPreviewFingerprint.value === currentImportPreviewFingerprint()
})

const selectableSingboxFiles = computed(() => (panel.singboxScan?.files || []).filter((file) => file.selectable).map((file) => file.path))
const allSingboxSelected = computed(
	() =>
		selectableSingboxFiles.value.length > 0 &&
		selectableSingboxFiles.value.every((path) => singboxSelectedFiles.value.includes(path))
)

const ordersColumns = [
	{ title: '模式', dataIndex: 'mode', width: 110 },
	{ title: 'ID', dataIndex: 'id', width: 80 },
	{ title: '订单号', key: 'order_no', width: 170 },
	{ title: '客户', key: 'customer', width: 180 },
	{ title: '订单', key: 'order_name', width: 220 },
	{ title: '状态', dataIndex: 'status', width: 110 },
	{ title: '数量', dataIndex: 'quantity', width: 90 },
	{ title: '线路信息', key: 'forward_summary', width: 280 },
	{ title: '端口', dataIndex: 'port', width: 100 },
	{ title: '到期', key: 'expires', width: 210 },
	{ title: '动作', key: 'action', fixed: 'right', width: 360 }
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
	{ title: '来源文件', dataIndex: 'source_file', key: 'source_file', width: 280 },
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

watch(
	() => [orderForm.mode, Number(orderForm.customer_id), orderForm.forward_outbound_ids.slice().sort((a, b) => a - b).join(',')],
	async () => {
		createForwardReuseHints.value = []
	}
)

watch(
	() => [orderEditOpen.value, orderEditForm.mode, Number(orderEditForm.customer_id), Number(orderEditForm.id), orderEditForm.forward_outbound_ids.slice().sort((a, b) => a - b).join(',')],
	async () => {
		editForwardReuseHints.value = []
	}
)

watch(
	() => orderForm.dedicated_protocol,
	() => {
		if (!filteredDedicatedInboundsForCreate.value.some((row: any) => Number(row.id) === Number(orderForm.dedicated_inbound_id))) {
			orderForm.dedicated_inbound_id = 0
			orderForm.dedicated_ingress_id = 0
		}
	}
)

watch(
	() => orderEditForm.dedicated_protocol,
	() => {
		if (!orderEditOpen.value) return
		if (!filteredDedicatedInboundsForEdit.value.some((row: any) => Number(row.id) === Number(orderEditForm.dedicated_inbound_id))) {
			orderEditForm.dedicated_inbound_id = 0
			orderEditForm.dedicated_ingress_id = 0
		}
	}
)

watch(
	() => orderForm.dedicated_inbound_id,
	() => {
		if (!filteredDedicatedIngressesForCreate.value.some((row: any) => Number(row.id) === Number(orderForm.dedicated_ingress_id))) {
			orderForm.dedicated_ingress_id = 0
		}
	}
)

watch(
	() => orderEditForm.dedicated_inbound_id,
	() => {
		if (!orderEditOpen.value) return
		if (!filteredDedicatedIngressesForEdit.value.some((row: any) => Number(row.id) === Number(orderEditForm.dedicated_ingress_id))) {
			orderEditForm.dedicated_ingress_id = 0
		}
	}
)

watch(
	() => importForm.lines,
	() => {
		if (importPreviewFingerprint.value && importPreviewSource.value === 'lines') {
			importPreviewFingerprint.value = ''
		}
	}
)

watch(
	() => singboxSelectedFiles.value.slice().sort().join('|'),
	() => {
		if (importPreviewFingerprint.value && importPreviewSource.value === 'singbox') {
			importPreviewFingerprint.value = ''
		}
	}
)

watch(
	() => [orderSearchKeyword.value, orderModeFilter.value, orderStatusFilter.value],
	() => {
		orderPagination.current = 1
	}
)

watch(
	() => [deliverySearchKeyword.value, deliveryMode.value, deliveryCustomerID.value],
	() => {
		deliveryPagination.current = 1
	}
)

function seedDefaultsFromSettings() {
  const p = Number(panel.settings.default_inbound_port || '23457')
  if (Number.isFinite(p) && p > 0) {
    orderForm.port = p
    probeForm.port = p
  }
	if (!importForm.expires_at) {
		setImportExpiryDays(15)
	}
}

function setImportExpiryDays(days: number) {
	const value = new Date(Date.now() + days * 24 * 3600 * 1000).toISOString().slice(0, 19)
	importForm.expires_at = value
}

function currentImportPreviewFingerprint(): string {
	if (importPreviewSource.value === 'singbox') {
		const files = [...singboxSelectedFiles.value].map((v) => String(v)).sort().join('|')
		return `singbox@@${files}`
	}
	const lines = String(importForm.lines || '').trim()
	return `lines@@${lines}`
}

function toggleSingboxSelectAll(checked: boolean) {
	singboxSelectedFiles.value = checked ? [...selectableSingboxFiles.value] : []
}

function statusColor(status: string) {
  if (status === 'active') return 'green'
  if (status === 'expired') return 'gold'
  if (status === 'disabled') return 'red'
  return 'default'
}

function modeColor(mode: string) {
  if (mode === 'import') return 'blue'
  if (mode === 'manual') return 'geekblue'
  if (mode === 'auto') return 'cyan'
  if (mode === 'dedicated') return 'magenta'
  return 'default'
}

function modeLabel(mode: string) {
	if (String(mode || '') === 'dedicated') return '专线'
	if (isResidentialMode(String(mode || ''))) return '家宽'
	if (String(mode || '') === 'forward') return '家宽(废弃旧模式)'
	return String(mode || '-')
}

function forwardSummary(order: any) {
	if (order.mode !== 'forward' && order.mode !== 'auto' && order.mode !== 'manual' && order.mode !== 'import') return '-'
	if (order.mode === 'auto' || order.mode === 'manual' || order.mode === 'import') {
		const count = Number(order.quantity || (order.items || []).length || 0)
		return `家宽 / Socks5 / ${count} 条`
	}
	const rows = (order.items || []).filter((item: any) => String(item.outbound_type || '') === 'socks5')
	if (rows.length === 0) return '0 条'
	const names = rows.map((item: any) => `${item.forward_address || '-'}:${item.forward_port || '-'}`)
	const preview = names.slice(0, 2).join(' | ')
	if (names.length <= 2) return `${rows.length} 条 / ${preview}`
	return `${rows.length} 条 / ${preview} +${names.length - 2}`
}

function dedicatedLinesCount(lines: string): number {
	if (!lines) return 0
	return lines
		.split('\n')
		.map((row) => row.trim())
		.filter((row) => row.length > 0).length
}

function dedicatedSummary(order: any) {
	if (order.mode !== 'dedicated') return '-'
	const ingress = order.dedicated_ingress
	const inbound = order.dedicated_inbound
	if (!ingress || !inbound) return 'Inbound/Ingress 未绑定'
	const protocol = String(order.dedicated_protocol || 'mixed')
	const entryText = ingress.name || `${ingress.domain}`
	return `${entryText}:${ingress.ingress_port} / ${protocol.toUpperCase()}@:${inbound.listen_port}`
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
	if (creatingOrder.value) return
	try {
		if (!Number(orderForm.customer_id || 0)) {
			message.warning('请选择客户')
			return
		}
		creatingOrder.value = true
		if (orderForm.mode === 'dedicated') {
			if (!orderForm.dedicated_protocol) {
				message.warning('请选择协议')
				return
			}
			if (!orderForm.dedicated_inbound_id) {
				message.warning('请选择Inbound')
				return
			}
			if (!orderForm.dedicated_ingress_id) {
				message.warning('请选择Ingress入口')
				return
			}
			if (dedicatedLinesCount(orderForm.dedicated_egress_lines) <= 0) {
				message.warning('请粘贴至少 1 行 Socks5 上游')
				return
			}
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
		if (orderForm.mode === 'dedicated') {
			payload.dedicated_entry_id = Number(orderForm.dedicated_entry_id)
			payload.dedicated_inbound_id = Number(orderForm.dedicated_inbound_id)
			payload.dedicated_ingress_id = Number(orderForm.dedicated_ingress_id)
			payload.dedicated_protocol = String(orderForm.dedicated_protocol || 'mixed')
			payload.dedicated_egress_lines = String(orderForm.dedicated_egress_lines || '')
		} else {
			payload.quantity = Number(orderForm.quantity)
		}
		const result = await panel.createOrder(payload)
		orderForm.name = ''
		orderForm.expires_at = ''
		if (orderForm.mode === 'dedicated') {
			orderForm.dedicated_entry_id = 0
			orderForm.dedicated_inbound_id = 0
			orderForm.dedicated_ingress_id = 0
			orderForm.dedicated_protocol = 'mixed'
			orderForm.dedicated_egress_lines = ''
		}
    panel.orderSelection = []
    message.success('订单创建成功')
    showForwardWarnings(result?.warnings || [])
  } catch (err) {
    panel.setError(err)
		message.error(panel.error || '创建订单失败')
  } finally {
		creatingOrder.value = false
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
	if (groupEditorOpen.value) {
		groupEditorOpen.value = false
	}
	orderEditForm.id = row.id
	orderEditForm.customer_id = row.customer_id
	orderEditForm.mode = row.mode
	orderEditForm.name = row.name
	orderEditForm.quantity = row.quantity
	orderEditForm.port = row.port
	orderEditForm.expires_at = row.expires_at ? new Date(row.expires_at).toISOString().slice(0, 19) : ''
	orderEditForm.manual_ip_ids = Array.from(new Set((row.items || []).map((item: any) => Number(item.host_ip_id || 0)).filter((v) => v > 0)))
	orderEditForm.forward_outbound_ids = Array.from(new Set((row.items || []).map((item: any) => Number(item.socks_outbound_id || 0)).filter((v) => v > 0)))
	orderEditForm.dedicated_entry_id = Number((row as any).dedicated_entry_id || 0)
	orderEditForm.dedicated_inbound_id = Number((row as any).dedicated_inbound_id || 0)
	orderEditForm.dedicated_ingress_id = Number((row as any).dedicated_ingress_id || 0)
	orderEditForm.dedicated_protocol = String((row as any).dedicated_protocol || 'mixed')
	orderEditForm.dedicated_egress_lines = ''
	orderEditForm.dedicated_credential_lines = ''
	orderEditForm.regenerate_dedicated_credentials = false
	orderEditCurrentInboundLines.value = (row.items || [])
		.map((item) => {
			if (String(row.mode || '') === 'dedicated') {
				const host = String(row.dedicated_ingress?.domain || row.dedicated_entry?.domain || item.ip)
				const port = dedicatedCopyPort(row)
				return `${host}:${port}:${item.username}:${item.password}`
			}
			return `${item.ip}:${item.port}:${item.username}:${item.password}`
		})
		.join('\n')
	orderEditCurrentEgressLines.value = (row.items || [])
		.filter((item) => String(item.forward_address || '').trim() !== '' && Number(item.forward_port || 0) > 0)
		.map((item) => `${item.forward_address}:${item.forward_port}:${item.forward_username || ''}:${item.forward_password || ''}`)
		.join('\n')
	editForwardReuseHints.value = []
	orderEditOpen.value = true
	void panel.loadAllocationPreview(row.customer_id, row.id)
}

async function copyOrderEditInboundLines() {
	if (!String(orderEditCurrentInboundLines.value || '').trim()) {
		message.warning('暂无可复制的当前入站凭据')
		return
	}
	await navigator.clipboard.writeText(orderEditCurrentInboundLines.value)
	message.success('当前入站凭据已复制')
}

async function copyOrderEditEgressLines() {
	if (!String(orderEditCurrentEgressLines.value || '').trim()) {
		message.warning('暂无可复制的当前出口Socks5')
		return
	}
	await navigator.clipboard.writeText(orderEditCurrentEgressLines.value)
	message.success('当前出口Socks5已复制')
}

function openOrderEditSmart(row: Order) {
	if ((row as any).is_group_head) {
		openGroupEditor(Number(row.id))
		return
	}
	openOrderEdit(row)
}

async function saveOrderEdit() {
	if (savingOrderEdit.value) return
	try {
		savingOrderEdit.value = true
		if (orderEditForm.mode === 'forward') {
			message.warning('forward 模式已废弃，历史订单仅支持只读查看')
			return
		}
		if (orderEditForm.mode === 'dedicated' && !orderEditForm.dedicated_inbound_id) {
			message.warning('请选择Inbound')
			return
		}
		if (orderEditForm.mode === 'dedicated' && !orderEditForm.dedicated_ingress_id) {
			message.warning('请选择Ingress入口')
			return
		}
		if (orderEditForm.mode === 'dedicated' && !orderEditForm.dedicated_protocol) {
			message.warning('请选择协议')
			return
		}
		const payload: Record<string, unknown> = {
			name: orderEditForm.name,
			port: Number(orderEditForm.port),
			expires_at: orderEditForm.expires_at ? new Date(orderEditForm.expires_at).toISOString() : ''
		}
		if (orderEditForm.mode === 'manual') {
			payload.manual_ip_ids = orderEditForm.manual_ip_ids.map((v) => Number(v))
		}
		if (orderEditForm.mode === 'dedicated') {
			payload.dedicated_entry_id = Number(orderEditForm.dedicated_entry_id)
			payload.dedicated_inbound_id = Number(orderEditForm.dedicated_inbound_id)
			payload.dedicated_ingress_id = Number(orderEditForm.dedicated_ingress_id)
			payload.dedicated_protocol = String(orderEditForm.dedicated_protocol || 'mixed')
			if (String(orderEditForm.dedicated_egress_lines || '').trim()) {
				payload.dedicated_egress_lines = String(orderEditForm.dedicated_egress_lines || '')
			}
			if (String(orderEditForm.dedicated_credential_lines || '').trim()) {
				payload.dedicated_credential_lines = String(orderEditForm.dedicated_credential_lines || '')
			}
			if (orderEditForm.regenerate_dedicated_credentials) {
				payload.regenerate_dedicated_credentials = true
			}
		} else {
			payload.quantity = Number(orderEditForm.quantity)
		}
		const result = await panel.updateOrder(orderEditForm.id, payload)
		orderEditOpen.value = false
		message.success('订单已更新')
		showForwardWarnings(result?.warnings || [])
	} catch (err) {
		panel.setError(err)
		message.error(panel.error || '更新订单失败')
	} finally {
		savingOrderEdit.value = false
	}
}

async function renewOrder(orderID: number, moreDays?: number) {
  const days = moreDays || Number(batchMoreDays.value)
  const expiresAt = String(batchRenewExpiresAt.value || '').trim()
  if (!days && !expiresAt) return
  try {
    await panel.renewOrder(orderID, days, expiresAt ? new Date(expiresAt).toISOString() : '')
    message.success('续期成功')
  } catch (err) {
    panel.setError(err)
  }
}

async function deactivateOrder(orderID: number) {
	Modal.confirm({
		title: '停用订单',
		content: `确认停用订单 #${orderID} 吗？`,
		okText: '停用',
		okType: 'danger',
		onOk: async () => {
			try {
				await panel.deactivateOrder(orderID)
				message.success('订单已停用')
			} catch (err) {
				panel.setError(err)
				message.error(panel.error || '停用失败')
			}
		}
	})
}

async function resetOrderCredentials(orderID: number) {
	Modal.confirm({
		title: '刷新家宽凭据',
		content: `确认刷新订单 #${orderID} 的家宽凭据吗？`,
		okText: '刷新',
		onOk: async () => {
			try {
				await panel.resetOrderCredentials(orderID)
				message.success('家宽凭据已刷新')
			} catch (err) {
				panel.setError(err)
				message.error(panel.error || '刷新凭据失败')
			}
		}
	})
}

async function removeOrder(orderID: number) {
	Modal.confirm({
		title: '删除订单',
		content: `确认删除订单 #${orderID} 吗？组头订单会连同子订单一起删除。`,
		okText: '删除',
		okType: 'danger',
		onOk: async () => {
			try {
				await panel.deleteOrder(orderID)
				message.success('订单已删除')
			} catch (err) {
				panel.setError(err)
				message.error(panel.error || '删除订单失败')
			}
		}
	})
}

async function doBatchRenew() {
  if (panel.orderSelection.length === 0) return
  try {
    const expiresAt = String(batchRenewExpiresAt.value || '').trim()
    const results = await panel.batchRenew(panel.orderSelection, Number(batchMoreDays.value), expiresAt ? new Date(expiresAt).toISOString() : '')
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
	const dedicatedSelected = panel.orders.filter((row) => panel.orderSelection.includes(Number(row.id)) && String(row.mode || '') === 'dedicated')
	if (dedicatedSelected.length > 0) {
		message.warning('已自动跳过专线订单测活')
	}
	const targetIDs = panel.orderSelection.filter((id) => {
		const row = panel.orders.find((item) => Number(item.id) === Number(id))
		return String(row?.mode || '') !== 'dedicated'
	})
	if (targetIDs.length === 0) return
	try {
		batchTestResult.value = await panel.batchTest(targetIDs)
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
		const res = await panel.batchExport(panel.orderSelection, exportFormat.value, exportIncludeRawSocks5.value)
		const header = String(res.headers?.['content-disposition'] || '')
		const filename = parseContentDispositionFilename(header, `orders-batch-${Date.now()}.${exportFormat.value === 'txt' ? 'txt' : 'zip'}`)
		downloadBlobFile(res.data, filename)
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
		params.set('format', exportFormat.value)
		params.set('include_raw_socks5', exportIncludeRawSocks5.value ? 'true' : 'false')
		params.set('shuffle', 'false')
		const query = params.toString()
		const res = await http.get(`/api/orders/${orderID}/export${query ? `?${query}` : ''}`, { responseType: 'blob' })
		const header = String(res.headers['content-disposition'] || '')
		const ext = exportFormat.value === 'txt' ? 'txt' : 'xlsx'
		const filename = parseContentDispositionFilename(header, `order-${orderID}-export.${ext}`)
		downloadBlobFile(res.data, filename)
	} catch (err) {
		panel.setError(err)
	} finally {
		exportingOrderID.value = null
	}
}

async function testOrder(orderID: number) {
	const order = panel.orders.find((row) => Number(row.id) === Number(orderID))
	if (order && String(order.mode || '') === 'dedicated') {
		message.warning('专线暂不支持测活')
		return
	}
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
	const order = panel.orders.find((row) => Number(row.id) === Number(orderID))
	if (order && String(order.mode || '') === 'dedicated') {
		message.warning('专线暂不支持流式测活')
		return
	}
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

function dedicatedCopyPort(order: Order): number {
	const ingress = order.dedicated_ingress
	if (ingress && Number(ingress.ingress_port) > 0) return Number(ingress.ingress_port)
	return Number(order.port || 0)
}

async function copyOrderLines(order: Order) {
	const lines = order.items.map((item) => {
		if (order.mode === 'dedicated') {
			const host = String(order.dedicated_ingress?.domain || order.dedicated_entry?.domain || item.ip)
			const port = dedicatedCopyPort(order)
			return `${host}:${port}:${item.username}:${item.password}`
		}
		return `${item.ip}:${item.port}:${item.username}:${item.password}`
	}).join('\n')
	await navigator.clipboard.writeText(lines)
	message.success('发货内容已复制')
}

async function previewImport() {
	if (previewingImport.value) return
  try {
		if (!String(importForm.lines || '').trim()) {
			message.warning('请先粘贴导入内容')
			return
		}
		previewingImport.value = true
    await panel.previewImport(importForm.lines)
		importPreviewSource.value = 'lines'
		importPreviewFingerprint.value = currentImportPreviewFingerprint()
    message.success('预检完成')
  } catch (err) {
    panel.setError(err)
		message.error(panel.error || '预检失败')
  } finally {
		previewingImport.value = false
  }
}

async function scanSingboxConfigs() {
	try {
		const result = await panel.scanSingboxConfigs()
		singboxSelectedFiles.value = (result?.files || []).filter((file) => file.selectable).map((file) => file.path)
		importPreviewSource.value = ''
		importPreviewFingerprint.value = ''
		if (!importForm.expires_at) {
			setImportExpiryDays(15)
		}
		message.success(`扫描完成，共 ${result?.total_files || 0} 个文件，提取 ${result?.total_entries || 0} 条`) 
	} catch (err) {
		panel.setError(err)
	}
}

async function previewSelectedSingboxFiles() {
	if (singboxSelectedFiles.value.length === 0) {
		message.warning('请先选择至少 1 个配置文件')
		return
	}
	if (previewingSingboxImport.value) return
	try {
		previewingSingboxImport.value = true
		await panel.previewSingboxImport(singboxSelectedFiles.value)
		importPreviewSource.value = 'singbox'
		importPreviewFingerprint.value = currentImportPreviewFingerprint()
		message.success('sing-box 预检完成')
	} catch (err) {
		panel.setError(err)
		message.error(panel.error || 'sing-box 预检失败')
	} finally {
		previewingSingboxImport.value = false
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

function openForwardManager() {
	forwardManagerOpen.value = true
	void panel.loadForwardOutbounds()
}

function resetDedicatedForm() {
	dedicatedForm.id = 0
	dedicatedForm.name = ''
	dedicatedForm.domain = ''
	dedicatedForm.mixed_port = 1080
	dedicatedForm.vmess_port = 10086
	dedicatedForm.vless_port = 10087
	dedicatedForm.shadowsocks_port = 10088
	dedicatedForm.priority = 100
	dedicatedForm.features = ['mixed', 'vmess', 'vless', 'shadowsocks']
	dedicatedForm.enabled = true
	dedicatedForm.notes = ''
}

function resetDedicatedInboundForm() {
	dedicatedInboundForm.name = ''
	dedicatedInboundForm.protocol = 'mixed'
	dedicatedInboundForm.listen_port = 1080
	dedicatedInboundForm.priority = 100
	dedicatedInboundForm.enabled = true
	dedicatedInboundForm.notes = ''
}

function resetDedicatedIngressForm() {
	if (!dedicatedIngressForm.dedicated_inbound_id && enabledDedicatedInbounds.value.length > 0) {
		dedicatedIngressForm.dedicated_inbound_id = Number(enabledDedicatedInbounds.value[0]?.id || 0)
	}
	dedicatedIngressForm.name = ''
	dedicatedIngressForm.domain = ''
	dedicatedIngressForm.ingress_port = 0
	dedicatedIngressForm.country_code = ''
	dedicatedIngressForm.region = ''
	dedicatedIngressForm.priority = 100
	dedicatedIngressForm.enabled = true
	dedicatedIngressForm.notes = ''
}

function openDedicatedManager() {
	dedicatedManagerOpen.value = true
	void panel.loadDedicatedEntries()
	void panel.loadDedicatedInbounds()
	void panel.loadDedicatedIngresses()
	resetDedicatedInboundForm()
	resetDedicatedIngressForm()
}

async function createDedicatedEntry() {
	try {
		await panel.createDedicatedEntry({
			name: dedicatedForm.name,
			domain: dedicatedForm.domain,
			mixed_port: Number(dedicatedForm.mixed_port),
			vmess_port: Number(dedicatedForm.vmess_port),
			vless_port: Number(dedicatedForm.vless_port),
			shadowsocks_port: Number(dedicatedForm.shadowsocks_port),
			priority: Number(dedicatedForm.priority),
			features: dedicatedForm.features,
			enabled: dedicatedForm.enabled,
			notes: dedicatedForm.notes
		})
		message.success('专线入口已创建')
		resetDedicatedForm()
	} catch (err) {
		panel.setError(err)
	}
}

async function createDedicatedInbound() {
	try {
		await panel.createDedicatedInbound({
			name: dedicatedInboundForm.name,
			protocol: dedicatedInboundForm.protocol,
			listen_port: Number(dedicatedInboundForm.listen_port),
			priority: Number(dedicatedInboundForm.priority),
			enabled: dedicatedInboundForm.enabled,
			notes: dedicatedInboundForm.notes
		})
		if (!dedicatedIngressForm.dedicated_inbound_id && enabledDedicatedInbounds.value.length > 0) {
			dedicatedIngressForm.dedicated_inbound_id = Number(enabledDedicatedInbounds.value[0]?.id || 0)
		}
		resetDedicatedInboundForm()
		message.success('Inbound已创建')
	} catch (err) {
		panel.setError(err)
	}
}

async function createDedicatedIngress() {
	try {
		if (!Number(dedicatedIngressForm.dedicated_inbound_id || 0)) {
			message.warning('请先选择绑定的Inbound')
			return
		}
		await panel.createDedicatedIngress({
			dedicated_inbound_id: Number(dedicatedIngressForm.dedicated_inbound_id),
			name: dedicatedIngressForm.name,
			domain: dedicatedIngressForm.domain,
			ingress_port: Number(dedicatedIngressForm.ingress_port),
			country_code: dedicatedIngressForm.country_code,
			region: dedicatedIngressForm.region,
			priority: Number(dedicatedIngressForm.priority),
			enabled: dedicatedIngressForm.enabled,
			notes: dedicatedIngressForm.notes
		})
		resetDedicatedIngressForm()
		message.success('Ingress已创建')
	} catch (err) {
		panel.setError(err)
	}
}

function openDedicatedInboundEdit(row: any) {
	dedicatedInboundEditForm.id = Number(row.id)
	dedicatedInboundEditForm.name = String(row.name || '')
	dedicatedInboundEditForm.protocol = String(row.protocol || 'mixed')
	dedicatedInboundEditForm.listen_port = Number(row.listen_port || 0)
	dedicatedInboundEditForm.priority = Number(row.priority || 100)
	dedicatedInboundEditForm.enabled = Boolean(row.enabled)
	dedicatedInboundEditForm.notes = String(row.notes || '')
	dedicatedInboundEditOpen.value = true
}

async function saveDedicatedInboundEdit() {
	try {
		await panel.updateDedicatedInbound(Number(dedicatedInboundEditForm.id), {
			name: dedicatedInboundEditForm.name,
			protocol: dedicatedInboundEditForm.protocol,
			listen_port: Number(dedicatedInboundEditForm.listen_port),
			priority: Number(dedicatedInboundEditForm.priority),
			enabled: dedicatedInboundEditForm.enabled,
			notes: dedicatedInboundEditForm.notes
		})
		dedicatedInboundEditOpen.value = false
		message.success('Inbound已更新')
	} catch (err) {
		panel.setError(err)
	}
}

function openDedicatedIngressEdit(row: any) {
	dedicatedIngressEditForm.id = Number(row.id)
	dedicatedIngressEditForm.dedicated_inbound_id = Number(row.dedicated_inbound_id || 0)
	dedicatedIngressEditForm.name = String(row.name || '')
	dedicatedIngressEditForm.domain = String(row.domain || '')
	dedicatedIngressEditForm.ingress_port = Number(row.ingress_port || 0)
	dedicatedIngressEditForm.country_code = String(row.country_code || '')
	dedicatedIngressEditForm.region = String(row.region || '')
	dedicatedIngressEditForm.priority = Number(row.priority || 100)
	dedicatedIngressEditForm.enabled = Boolean(row.enabled)
	dedicatedIngressEditForm.notes = String(row.notes || '')
	dedicatedIngressEditOpen.value = true
}

async function saveDedicatedIngressEdit() {
	try {
		await panel.updateDedicatedIngress(Number(dedicatedIngressEditForm.id), {
			dedicated_inbound_id: Number(dedicatedIngressEditForm.dedicated_inbound_id),
			name: dedicatedIngressEditForm.name,
			domain: dedicatedIngressEditForm.domain,
			ingress_port: Number(dedicatedIngressEditForm.ingress_port),
			country_code: dedicatedIngressEditForm.country_code,
			region: dedicatedIngressEditForm.region,
			priority: Number(dedicatedIngressEditForm.priority),
			enabled: dedicatedIngressEditForm.enabled,
			notes: dedicatedIngressEditForm.notes
		})
		dedicatedIngressEditOpen.value = false
		message.success('Ingress已更新')
	} catch (err) {
		panel.setError(err)
	}
}

function removeDedicatedInbound(id: number) {
	Modal.confirm({
		title: '删除Inbound',
		content: '确认删除这个Inbound吗？',
		okText: '删除',
		okType: 'danger',
		onOk: async () => {
			try {
				await panel.deleteDedicatedInbound(id)
			} catch (err) {
				panel.setError(err)
			}
		}
	})
}

function removeDedicatedIngress(id: number) {
	Modal.confirm({
		title: '删除Ingress',
		content: '确认删除这个Ingress吗？',
		okText: '删除',
		okType: 'danger',
		onOk: async () => {
			try {
				await panel.deleteDedicatedIngress(id)
			} catch (err) {
				panel.setError(err)
			}
		}
	})
}

function openDedicatedEdit(row: any) {
	dedicatedForm.id = Number(row.id)
	dedicatedForm.name = String(row.name || '')
	dedicatedForm.domain = String(row.domain || '')
	dedicatedForm.mixed_port = Number(row.mixed_port || 1080)
	dedicatedForm.vmess_port = Number(row.vmess_port || 10086)
	dedicatedForm.vless_port = Number(row.vless_port || 10087)
	dedicatedForm.shadowsocks_port = Number(row.shadowsocks_port || 10088)
	dedicatedForm.priority = Number(row.priority || 100)
	dedicatedForm.features = String(row.features || '')
		.split(',')
		.map((v) => v.trim())
		.filter((v) => v)
	dedicatedForm.enabled = Boolean(row.enabled)
	dedicatedForm.notes = String(row.notes || '')
	dedicatedEditOpen.value = true
}

async function saveDedicatedEdit() {
	try {
		await panel.updateDedicatedEntry(Number(dedicatedForm.id), {
			name: dedicatedForm.name,
			domain: dedicatedForm.domain,
			mixed_port: Number(dedicatedForm.mixed_port),
			vmess_port: Number(dedicatedForm.vmess_port),
			vless_port: Number(dedicatedForm.vless_port),
			shadowsocks_port: Number(dedicatedForm.shadowsocks_port),
			priority: Number(dedicatedForm.priority),
			features: dedicatedForm.features,
			enabled: dedicatedForm.enabled,
			notes: dedicatedForm.notes
		})
		dedicatedEditOpen.value = false
		message.success('专线入口已更新')
	} catch (err) {
		panel.setError(err)
	}
}

async function removeDedicatedEntry(id: number) {
	Modal.confirm({
		title: '删除专线入口',
		content: '确认删除这个专线入口吗？',
		okText: '删除',
		okType: 'danger',
		onOk: async () => {
			try {
				await panel.deleteDedicatedEntry(id)
			} catch (err) {
				panel.setError(err)
			}
		}
	})
}

async function splitOrderHead(orderID: number) {
	try {
		const rows = await panel.splitOrder(orderID)
		message.success(`拆分完成，子订单 ${rows.length} 个`)
	} catch (err) {
		panel.setError(err)
	}
}

function openGroupSocksModal(orderID: number) {
	groupTargetOrderID.value = orderID
	groupBatchChildOrderIDs.value = panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(orderID))
		.map((row) => Number(row.id))
	groupSocksLines.value = ''
	groupSocksModalOpen.value = true
}

function openGroupEditor(orderID: number) {
	groupEditorHeadOrderID.value = orderID
	groupEditorChildOrderIDs.value = panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(orderID))
		.map((row) => Number(row.id))
	groupEditorOpen.value = true
}

function openGroupSocksModalFromEditor() {
	groupTargetOrderID.value = Number(groupEditorHeadOrderID.value || 0)
	groupBatchChildOrderIDs.value = groupEditorChildOrderIDs.value.map((v) => Number(v))
	groupSocksLines.value = ''
	groupSocksModalOpen.value = true
}

function openGroupCredModalFromEditor() {
	groupTargetOrderID.value = Number(groupEditorHeadOrderID.value || 0)
	groupBatchChildOrderIDs.value = groupEditorChildOrderIDs.value.map((v) => Number(v))
	groupCredLines.value = ''
	groupCredRegenerate.value = false
	groupCredModalOpen.value = true
}

function openGroupRenewModalFromEditor() {
	groupRenewHeadOrderID.value = Number(groupEditorHeadOrderID.value || 0)
	groupRenewDays.value = Number(batchMoreDays.value || 30)
	groupRenewExpiresAt.value = String(batchRenewExpiresAt.value || '')
	groupRenewChildOrderIDs.value = groupEditorChildOrderIDs.value.map((v) => Number(v))
	groupRenewModalOpen.value = true
}

function openGroupGeoModal(orderID: number) {
	groupTargetOrderID.value = orderID
	groupBatchChildOrderIDs.value = panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(orderID))
		.map((row) => Number(row.id))
	groupGeoCountryCode.value = ''
	groupGeoRegion.value = ''
	groupGeoMappingLines.value = ''
	groupGeoModalOpen.value = true
}

function openGroupGeoModalFromEditor() {
	groupTargetOrderID.value = Number(groupEditorHeadOrderID.value || 0)
	groupBatchChildOrderIDs.value = groupEditorChildOrderIDs.value.map((v) => Number(v))
	groupGeoCountryCode.value = ''
	groupGeoRegion.value = ''
	groupGeoMappingLines.value = ''
	groupGeoModalOpen.value = true
}

function openGroupHeadOrderEditFromEditor() {
	const head = panel.orders.find((row) => Number(row.id) === Number(groupEditorHeadOrderID.value || 0))
	if (!head) {
		message.warning('组头订单不存在')
		return
	}
	openOrderEdit(head)
}

async function submitGroupSocksUpdate() {
	if (groupSocksSaving.value) return
	if (!groupTargetOrderID.value) return
	if (!groupSocksLines.value.trim()) {
		message.warning('请粘贴 Socks5 列表')
		return
	}
	if (groupBatchChildOrderIDs.value.length === 0) {
		message.warning('请至少选择 1 个子订单')
		return
	}
	try {
		groupSocksSaving.value = true
		await panel.updateOrderGroupSocks5Selected(groupTargetOrderID.value, groupBatchChildOrderIDs.value.map((v) => Number(v)), groupSocksLines.value)
		groupSocksModalOpen.value = false
		groupSocksLines.value = ''
		message.success(`已更新 ${groupBatchChildOrderIDs.value.length} 个子订单的 Socks5`) 
	} catch (err) {
		panel.setError(err)
		message.error(panel.error || '组内 Socks5 更新失败')
	} finally {
		groupSocksSaving.value = false
	}
}

async function downloadGroupSocksTemplate() {
	if (!groupTargetOrderID.value) {
		message.warning('请先选择组头订单')
		return
	}
	try {
		const res = await panel.downloadOrderGroupSocks5Template(groupTargetOrderID.value)
		const header = String(res.headers?.['content-disposition'] || '')
		downloadBlobFile(res.data, parseContentDispositionFilename(header, `group-${groupTargetOrderID.value}-socks5-template.xlsx`))
	} catch (err) {
		panel.setError(err)
	}
}

function downloadDedicatedCreateSample() {
	downloadTextFile('1.1.1.1:1080:user001:pass001', `dedicated-socks5-sample-${Date.now()}.txt`)
}

async function probeDedicatedCreateLines() {
	const lines = String(orderForm.dedicated_egress_lines || '').trim()
	if (!lines) {
		message.warning('请先粘贴 Socks5 出口列表')
		return
	}
	if (dedicatedProbeRunning.value) return
	dedicatedProbeRunning.value = true
	dedicatedProbeRows.value = []
	dedicatedProbeMeta.total = 0
	dedicatedProbeMeta.success = 0
	dedicatedProbeMeta.failed = 0
	try {
		const token = localStorage.getItem('xtool_token') || ''
		const resp = await fetch('/api/orders/dedicated/egress/probe-stream', {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Authorization: `Bearer ${token}`
			},
			body: JSON.stringify({ lines })
		})
		if (!resp.ok || !resp.body) {
			throw new Error(`探测失败: ${resp.status}`)
		}
		const reader = resp.body.getReader()
		const decoder = new TextDecoder()
		let buffer = ''
		while (true) {
			const { done, value } = await reader.read()
			if (done) break
			buffer += decoder.decode(value, { stream: true })
			const chunks = buffer.split('\n')
			buffer = chunks.pop() || ''
			for (const chunk of chunks) {
				const text = chunk.trim()
				if (!text) continue
				const event = JSON.parse(text)
				if (event.type === 'start') {
					dedicatedProbeMeta.total = Number(event.total || 0)
					continue
				}
				if (event.type === 'result') {
					dedicatedProbeRows.value.push({
						index: Number(event.index || 0),
						raw: String(event.raw || ''),
						available: Boolean(event.available),
						exit_ip: String(event.exit_ip || ''),
						country_code: String(event.country_code || ''),
						region: String(event.region || ''),
						error: String(event.error || '') || undefined
					})
					if (event.available) dedicatedProbeMeta.success += 1
					else dedicatedProbeMeta.failed += 1
					continue
				}
				if (event.type === 'done') {
					dedicatedProbeMeta.total = Number(event.total || dedicatedProbeMeta.total)
					dedicatedProbeMeta.success = Number(event.success || dedicatedProbeMeta.success)
					dedicatedProbeMeta.failed = Number(event.failed || dedicatedProbeMeta.failed)
				}
			}
		}
		message.success('专线出口探测完成')
	} catch (err) {
		panel.setError(err)
		message.error(panel.error || '专线出口探测失败')
	} finally {
		dedicatedProbeRunning.value = false
	}
}

function openGroupCredModal(orderID: number) {
	groupTargetOrderID.value = orderID
	groupBatchChildOrderIDs.value = panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(orderID))
		.map((row) => Number(row.id))
	groupCredLines.value = ''
	groupCredRegenerate.value = false
	groupCredModalOpen.value = true
}

function openGroupRenewModal(orderID: number) {
	groupRenewHeadOrderID.value = orderID
	groupRenewDays.value = Number(batchMoreDays.value || 30)
	groupRenewExpiresAt.value = String(batchRenewExpiresAt.value || '')
	groupRenewChildOrderIDs.value = panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(orderID))
		.map((row) => Number(row.id))
	groupRenewModalOpen.value = true
}

async function submitGroupSelectedRenew() {
	if (groupRenewSaving.value) return
	if (!groupRenewHeadOrderID.value) return
	if (groupRenewChildOrderIDs.value.length === 0) {
		message.warning('请至少选择 1 个子订单')
		return
	}
	if (!Number(groupRenewDays.value) && !String(groupRenewExpiresAt.value || '').trim()) {
		message.warning('请输入续期天数')
		return
	}
	try {
		groupRenewSaving.value = true
		const expiresAt = String(groupRenewExpiresAt.value || '').trim()
		await panel.renewOrderGroupSelected(
			Number(groupRenewHeadOrderID.value),
			groupRenewChildOrderIDs.value.map((v) => Number(v)),
			Number(groupRenewDays.value),
			expiresAt ? new Date(expiresAt).toISOString() : ''
		)
		groupRenewModalOpen.value = false
		message.success(`已续期 ${groupRenewChildOrderIDs.value.length} 个子订单`)
	} catch (err) {
		panel.setError(err)
		message.error(panel.error || '部分续期失败')
	} finally {
		groupRenewSaving.value = false
	}
}

async function submitGroupCredentialUpdate() {
	if (groupCredSaving.value) return
	if (!groupTargetOrderID.value) return
	if (!groupCredRegenerate.value && !groupCredLines.value.trim()) {
		message.warning('请粘贴凭据列表或启用随机重置')
		return
	}
	if (groupBatchChildOrderIDs.value.length === 0) {
		message.warning('请至少选择 1 个子订单')
		return
	}
	const regenerate = groupCredRegenerate.value
	if (regenerate) {
		Modal.confirm({
			title: '确认随机重置组内凭据',
			content: '将随机重置当前组全部子订单入站凭据，原凭据将立即失效，是否继续？',
			okText: '确认重置',
			okType: 'danger',
			onOk: async () => {
				await applyGroupCredentialUpdate(regenerate)
			}
		})
		return
	}
	await applyGroupCredentialUpdate(regenerate)
}

async function applyGroupCredentialUpdate(regenerate: boolean) {
	try {
		groupCredSaving.value = true
		await panel.updateOrderGroupCredentialsSelected(groupTargetOrderID.value, groupBatchChildOrderIDs.value.map((v) => Number(v)), {
			lines: groupCredLines.value,
			regenerate
		})
		groupCredModalOpen.value = false
		groupCredLines.value = ''
		groupCredRegenerate.value = false
		message.success(regenerate ? `已随机重置 ${groupBatchChildOrderIDs.value.length} 个子订单凭据` : `已顺序更新 ${groupBatchChildOrderIDs.value.length} 个子订单凭据`)
	} catch (err) {
		panel.setError(err)
		message.error(panel.error || '组内凭据更新失败')
	} finally {
		groupCredSaving.value = false
	}
}

async function submitGroupGeoUpdate() {
	if (!groupTargetOrderID.value) return
	if (groupBatchChildOrderIDs.value.length === 0) {
		message.warning('请至少选择 1 个子订单')
		return
	}
	try {
		if (String(groupGeoMappingLines.value || '').trim()) {
			await panel.updateOrderGroupEgressGeoByMapping(
				groupTargetOrderID.value,
				groupGeoMappingLines.value,
				String(groupGeoCountryCode.value || '').trim(),
				String(groupGeoRegion.value || '').trim()
			)
			groupGeoModalOpen.value = false
			message.success('已按映射批量写入国家地区')
			return
		}
		if (!String(groupGeoCountryCode.value || '').trim()) {
			message.warning('请输入国家代码或提供映射行')
			return
		}
		await panel.updateOrderGroupEgressGeo(
			groupTargetOrderID.value,
			groupBatchChildOrderIDs.value.map((v) => Number(v)),
			String(groupGeoCountryCode.value || '').trim(),
			String(groupGeoRegion.value || '').trim()
		)
		groupGeoModalOpen.value = false
		message.success(`已更新 ${groupBatchChildOrderIDs.value.length} 个子订单国家地区`)
	} catch (err) {
		panel.setError(err)
		message.error(panel.error || '批量写入国家地区失败')
	}
}

async function downloadGroupCredentialTemplate() {
	if (!groupTargetOrderID.value) {
		message.warning('请先选择组头订单')
		return
	}
	try {
		const res = await panel.downloadOrderGroupCredentialsTemplate(groupTargetOrderID.value)
		const header = String(res.headers?.['content-disposition'] || '')
		downloadBlobFile(res.data, parseContentDispositionFilename(header, `group-${groupTargetOrderID.value}-credentials-template.xlsx`))
	} catch (err) {
		panel.setError(err)
	}
}

async function beforeUploadGroupSocksXLSX(file: File) {
	if (!groupTargetOrderID.value) {
		message.warning('请先选择组头订单')
		return false
	}
	try {
		await panel.updateOrderGroupSocks5XLSX(groupTargetOrderID.value, file)
		groupSocksModalOpen.value = false
		groupSocksLines.value = ''
		message.success('已通过 xlsx 回填并更新组内 Socks5')
	} catch (err) {
		panel.setError(err)
	}
	return false
}

async function beforeUploadGroupCredXLSX(file: File) {
	if (!groupTargetOrderID.value) {
		message.warning('请先选择组头订单')
		return false
	}
	try {
		await panel.updateOrderGroupCredentialsXLSX(groupTargetOrderID.value, file)
		groupCredModalOpen.value = false
		groupCredLines.value = ''
		groupCredRegenerate.value = false
		message.success('已通过 xlsx 回填并更新组内凭据')
	} catch (err) {
		panel.setError(err)
	}
	return false
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
	if (confirmingImport.value) return
  try {
		if (!importPreviewValid.value) {
			message.warning('预检结果已失效，请重新预检后再导入')
			return
		}
		confirmingImport.value = true
    await panel.confirmImport({
      customer_id: Number(importForm.customer_id),
      order_name: importForm.order_name,
      expires_at: importForm.expires_at ? new Date(importForm.expires_at).toISOString() : '',
      rows: panel.importPreview as ImportPreviewRow[]
    })
		importForm.customer_id = 0
		importForm.order_name = ''
		setImportExpiryDays(15)
		importForm.lines = ''
    panel.importPreview = []
		importPreviewSource.value = ''
		importPreviewFingerprint.value = ''
    message.success('导入成功')
  } catch (err) {
    panel.setError(err)
		message.error(panel.error || '导入失败')
  } finally {
		confirmingImport.value = false
  }
}

async function saveSettings() {
	try {
		await panel.saveSettings({
			default_inbound_port: panel.settings.default_inbound_port || '23457',
			bark_enabled: panel.settings.bark_enabled === 'true' ? 'true' : 'false',
			bark_base_url: panel.settings.bark_base_url || '',
			bark_device_key: panel.settings.bark_device_key || '',
			bark_group: panel.settings.bark_group || 'xraytool',
			dedicated_vless_security: panel.settings.dedicated_vless_security || 'tls',
			dedicated_vless_sni: panel.settings.dedicated_vless_sni || '',
			dedicated_vless_type: panel.settings.dedicated_vless_type || 'tcp',
			dedicated_vless_path: panel.settings.dedicated_vless_path || '',
			dedicated_vless_host: panel.settings.dedicated_vless_host || '',
			residential_name_prefix: panel.settings.residential_name_prefix || '家宽-Socks5'
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
		const name = parseContentDispositionFilename(header, `xraytool-backup-${Date.now()}.db`)
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
		const saveName = parseContentDispositionFilename(header, name)
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
	downloadBlobFile(blob, filename)
}

function parseContentDispositionFilename(header: string, fallback: string): string {
	const raw = String(header || '')
	const encodedMatch = raw.match(/filename\*=UTF-8''([^;]+)/i)
	if (encodedMatch?.[1]) {
		try {
			return decodeURIComponent(String(encodedMatch[1]).replace(/\+/g, '%20'))
		} catch {
			// ignore decode errors and fallback to filename
		}
	}
	const filenameMatch = raw.match(/filename="?([^";]+)"?/i)
	if (filenameMatch?.[1]) return filenameMatch[1]
	return fallback
}

function downloadBlobFile(data: Blob, filename: string) {
	const blob = data instanceof Blob ? data : new Blob([data])
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
                          <a-tooltip title="编辑客户">
                            <a-button size="small" :icon="h(EditOutlined)" aria-label="编辑客户" @click="openCustomerEdit(record)" />
                          </a-tooltip>
                          <a-tooltip title="删除客户">
                            <a-button size="small" danger :icon="h(DeleteOutlined)" aria-label="删除客户" @click="deleteCustomer(record.id)" />
                          </a-tooltip>
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
				  <a-select-option value="auto">家宽-自动</a-select-option>
				  <a-select-option value="manual">家宽-手动</a-select-option>
				  <a-select-option value="dedicated">专线分发</a-select-option>
                </a-select></a-col>
              </a-row>
              <a-row :gutter="8" class="mt-2">
				<a-col v-if="orderForm.mode !== 'dedicated'" :xs="24" :md="8"><a-input-number v-model:value="orderForm.quantity" :min="1" style="width: 100%" placeholder="数量" /></a-col>
				<a-col v-else :xs="24" :md="8"><a-input :value="`专线数: ${dedicatedLinesCount(orderForm.dedicated_egress_lines)}`" disabled /></a-col>
                <a-col :xs="24" :md="8"><a-input-number v-model:value="orderForm.duration_day" :min="1" style="width: 100%" placeholder="有效天数" /></a-col>
                <a-col :xs="24" :md="8"><a-input-number v-model:value="orderForm.port" :min="1" :max="65535" :disabled="orderForm.mode === 'dedicated'" style="width: 100%" placeholder="端口" /></a-col>
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
			  <div v-if="orderForm.mode === 'dedicated'" class="mt-2">
				<a-space class="mb-2" wrap>
				  <span class="text-xs text-slate-500">专线 Inbound/Ingress 请到 设置-专线 管理；先选协议再选入口</span>
				</a-space>
				<a-select v-model:value="orderForm.dedicated_protocol" style="width:100%" placeholder="选择协议">
				  <a-select-option v-for="opt in dedicatedProtocolOptions" :key="opt.value" :value="opt.value">
					{{ opt.label }}
				  </a-select-option>
				</a-select>
				<a-select v-model:value="orderForm.dedicated_inbound_id" class="mt-2" style="width:100%" placeholder="选择Inbound(协议+本机端口)">
				  <a-select-option v-for="row in filteredDedicatedInboundsForCreate" :key="row.id" :value="row.id">
					{{ row.name }} / {{ row.protocol }} / :{{ row.listen_port }}
				  </a-select-option>
				</a-select>
				<a-select v-model:value="orderForm.dedicated_ingress_id" class="mt-2" style="width:100%" placeholder="选择Ingress(入口域名:端口)">
				  <a-select-option v-for="row in filteredDedicatedIngressesForCreate" :key="row.id" :value="row.id">
					{{ row.name || row.domain }} / {{ row.domain }}:{{ row.ingress_port }} / {{ (row.country_code || '--').toUpperCase() }} {{ row.region || '' }}
				  </a-select-option>
				</a-select>
				<a-space class="mt-2" wrap>
				  <a-button size="small" @click="downloadDedicatedCreateSample">下载示例</a-button>
				  <a-button size="small" :loading="dedicatedProbeRunning" @click="probeDedicatedCreateLines">探测出口可用性</a-button>
				  <span class="text-xs text-slate-500">示例格式: ip:port:user:pass，可直接复制多行</span>
				</a-space>
				<a-textarea v-model:value="orderForm.dedicated_egress_lines" class="mt-2" :rows="6" placeholder="每行: ip:port:user:pass（按顺序创建子订单）" />
				<a-alert class="mt-2" type="info" show-icon :message="`专线数量 = ${dedicatedLinesCount(orderForm.dedicated_egress_lines)}，订单将自动拆分为子订单 1:1`" />
				<a-alert v-if="dedicatedProbeMeta.total > 0" class="mt-2" type="info" show-icon :message="`探测进度 ${dedicatedProbeMeta.success + dedicatedProbeMeta.failed}/${dedicatedProbeMeta.total}，可用 ${dedicatedProbeMeta.success}，失败 ${dedicatedProbeMeta.failed}`" />
				<div v-if="dedicatedProbeRows.length" class="mt-2 max-h-40 overflow-auto rounded border border-slate-200 p-2">
				  <div v-for="row in dedicatedProbeRows" :key="`${row.index}-${row.raw}`" class="text-xs leading-6">
					<span class="font-mono">#{{ row.index }} {{ row.raw }}</span>
					<span v-if="row.available" class="ml-2 text-emerald-600">可用 / {{ row.exit_ip }} / {{ (row.country_code || '--').toUpperCase() }} {{ row.region || '' }}</span>
					<span v-else class="ml-2 text-rose-600">失败 / {{ row.error || 'unknown' }}</span>
				  </div>
				</div>
              </div>
              <div class="mt-3 flex justify-end">
                <a-button type="primary" :loading="creatingOrder" @click="createOrder">下单并同步Xray</a-button>
              </div>
            </a-card>

            <a-card :bordered="false" title="订单列表">
              <template #extra>
                <a-space>
				  <a-input v-model:value="orderSearchKeyword" size="small" style="width: 220px" placeholder="搜索: 订单号/客户/域名/IP/账号" allow-clear />
				  <a-select v-model:value="orderModeFilter" size="small" style="width: 120px">
					<a-select-option value="all">全部模式</a-select-option>
					<a-select-option value="home">家宽</a-select-option>
					<a-select-option value="dedicated">专线</a-select-option>
				  </a-select>
				  <a-select v-model:value="orderStatusFilter" size="small" style="width: 120px">
					<a-select-option value="all">全部状态</a-select-option>
					<a-select-option value="active">active</a-select-option>
					<a-select-option value="expired">expired</a-select-option>
					<a-select-option value="disabled">disabled</a-select-option>
				  </a-select>
				  <a-select v-model:value="exportFormat" size="small" style="width: 110px">
					<a-select-option value="xlsx">导出XLSX</a-select-option>
					<a-select-option value="txt">导出TXT</a-select-option>
				  </a-select>
                  <span class="text-xs text-slate-500">已选 {{ panel.orderSelection.length }} 个</span>
                  <a-select v-model:value="testSamplePercent" size="small" style="width: 110px">
                    <a-select-option :value="100">测活100%</a-select-option>
                    <a-select-option :value="10">测活10%</a-select-option>
                    <a-select-option :value="5">测活5%</a-select-option>
                  </a-select>
				  <a-input-number v-model:value="exportCount" :min="0" size="small" :placeholder="'导出条数(0=全部)'" />
				  <a-checkbox v-model:checked="exportIncludeRawSocks5" class="text-xs">附带原始Socks5</a-checkbox>
				  <a-input-number v-model:value="batchMoreDays" :min="1" :max="365" size="small" />
				  <a-button size="small" @click="batchMoreDays = 30">30天</a-button>
				  <a-button size="small" @click="batchMoreDays = 60">60天</a-button>
				  <a-button size="small" @click="batchMoreDays = 90">90天</a-button>
				  <a-date-picker v-model:value="batchRenewExpiresAt" size="small" show-time value-format="YYYY-MM-DDTHH:mm:ss" placeholder="续期到期时间(可选)" />
                  <a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchRenew">批量续期</a-button>
                  <a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchResync">批量重同步</a-button>
                  <a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchTest">批量测活</a-button>
                  <a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchExport">批量导出</a-button>
                  <a-button size="small" danger :disabled="panel.orderSelection.length===0" @click="doBatchDeactivate">批量停用</a-button>
                </a-space>
              </template>

              <a-table
                :columns="ordersColumns"
				:data-source="filteredOrderRows"
                :row-selection="rowSelection"
                :scroll="{ x: 1300 }"
				:pagination="{ current: orderPagination.current, pageSize: orderPagination.pageSize, showSizeChanger: false, onChange: (page:number, pageSize:number) => { orderPagination.current = Number(page || 1); orderPagination.pageSize = Number(pageSize || 12) } }"
                size="small"
              >
				<template #bodyCell="{ column, record }">
				  <template v-if="column.key === 'order_no'">
					<span class="font-mono text-xs">{{ record.order_no || '-' }}</span>
				  </template>
				  <template v-if="column.key === 'customer'">
					{{ record.customer?.name || record.customer_id }}
				  </template>
				  <template v-else-if="column.key === 'order_name'">
					  <div class="text-xs">
						<div class="font-medium text-slate-700">{{ record.name || '-' }}</div>
						<div class="text-slate-500">
						  <span class="font-mono">{{ record.order_no || `OD-${record.id}` }}</span>
							<span v-if="record.is_group_head">组头单</span>
							<span v-else-if="record.parent_order_id">子单 #{{ record.sequence_no || '-' }}</span>
							<span v-else>普通单</span>
						<span class="ml-2" v-if="record.group_id">G{{ record.group_id }}</span>
					  </div>
					</div>
				  </template>
                  <template v-else-if="column.dataIndex === 'status'">
                    <a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag>
                  </template>
                  <template v-else-if="column.dataIndex === 'mode'">
                    <a-tag :color="modeColor(record.mode)">{{ modeLabel(record.mode) }}</a-tag>
                  </template>
                  <template v-else-if="column.key === 'forward_summary'">
					<span class="text-xs" :class="record.mode === 'dedicated' ? 'font-mono text-slate-700' : 'text-slate-500'">
						{{ record.mode === 'dedicated' ? dedicatedSummary(record) : forwardSummary(record) }}
					</span>
				  </template>
                  <template v-else-if="column.key === 'expires'">
                    <div>{{ expiresHint(record.expires_at) }}</div>
                    <div class="text-xs text-slate-500">{{ formatTime(record.expires_at) }}</div>
                  </template>
                  <template v-else-if="column.key === 'action'">
					<a-space :size="4" wrap>
					  <a-button size="small" @click="openOrderDetail(record)">详情</a-button>
					  <a-button size="small" @click="openOrderEditSmart(record)">{{ record.is_group_head ? '组编辑' : '编辑' }}</a-button>
					  <a-button size="small" :loading="exportingOrderID===record.id" @click="exportOrder(record.id)">导出</a-button>
					  <a-button size="small" :disabled="record.mode === 'dedicated'" :loading="testingOrderID===record.id" @click="testOrder(record.id)">测活</a-button>
					  <a-dropdown>
						<a-button size="small">更多</a-button>
						<template #overlay>
						  <a-menu>
							<a-menu-item @click="renewOrder(record.id)">续期</a-menu-item>
							<a-menu-item :disabled="record.mode === 'dedicated'" @click="streamTestOrder(record.id)">流式测活</a-menu-item>
							<a-menu-item v-if="isResidentialMode(record.mode)" @click="resetOrderCredentials(record.id)">刷新家宽凭据</a-menu-item>
							<a-menu-item v-if="!record.parent_order_id && record.quantity > 1" @click="splitOrderHead(record.id)">拆分为子订单</a-menu-item>
							<a-menu-item v-if="record.is_group_head" @click="openGroupEditor(record.id)">组编辑工作台</a-menu-item>
							<a-menu-item v-if="record.is_group_head" @click="openGroupGeoModal(record.id)">批量设置国家地区</a-menu-item>
							<a-menu-item v-if="record.is_group_head" @click="openGroupSocksModal(record.id)">组内顺序改 Socks5</a-menu-item>
							<a-menu-item v-if="record.is_group_head" @click="openGroupCredModal(record.id)">组内批量改凭据</a-menu-item>
							<a-menu-item v-if="record.is_group_head" @click="openGroupRenewModal(record.id)">组内部分续期</a-menu-item>
							<a-menu-divider />
							<a-menu-item danger @click="removeOrder(record.id)">删除订单</a-menu-item>
							<a-menu-item danger @click="deactivateOrder(record.id)">停用订单</a-menu-item>
						  </a-menu>
						</template>
					  </a-dropdown>
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

		  <template v-else-if="panel.activeTab === 'delivery'">
			<a-card :bordered="false" title="发货控制台">
			  <template #extra>
				<a-space>
				  <a-input v-model:value="deliverySearchKeyword" style="width:220px" placeholder="搜索: 订单号/客户/域名/IP/账号" allow-clear />
				  <a-select v-model:value="deliveryCustomerID" style="width:180px" placeholder="客户筛选">
					<a-select-option :value="0">全部客户</a-select-option>
					<a-select-option v-for="c in panel.customers" :key="c.id" :value="c.id">{{ c.name }}</a-select-option>
				  </a-select>
				  <a-select v-model:value="deliveryMode" style="width:140px">
					<a-select-option value="all">全部类型</a-select-option>
					<a-select-option value="home">家宽</a-select-option>
					<a-select-option value="dedicated">专线</a-select-option>
				  </a-select>
				  <a-select v-model:value="exportFormat" size="small" style="width: 110px">
					<a-select-option value="xlsx">导出XLSX</a-select-option>
					<a-select-option value="txt">导出TXT</a-select-option>
				  </a-select>
				</a-space>
			  </template>
			  <a-alert class="mb-3" type="info" show-icon message="订单管理与发货已分离：本页用于导出、复制发货内容、刷新家宽凭据。" />
			  <a-table :data-source="deliveryRows" :row-key="(row:any)=>row.id" size="small" :pagination="{ current: deliveryPagination.current, pageSize: deliveryPagination.pageSize, showSizeChanger: false, onChange: (page:number, pageSize:number) => { deliveryPagination.current = Number(page || 1); deliveryPagination.pageSize = Number(pageSize || 12) } }">
				<a-table-column title="类型" key="mode" width="120">
				  <template #default="{ record }">
					<a-tag :color="record.mode === 'dedicated' ? 'magenta' : 'cyan'">{{ record.mode === 'dedicated' ? '专线' : '家宽' }}</a-tag>
				  </template>
				</a-table-column>
				<a-table-column title="订单" key="name" width="280">
				  <template #default="{ record }">#{{ record.id }} / {{ record.name || '-' }}</template>
				</a-table-column>
				<a-table-column title="客户" key="customer" width="160">
				  <template #default="{ record }">{{ record.customer?.name || record.customer_id }}</template>
				</a-table-column>
				<a-table-column title="订单号" key="order_no" width="170">
				  <template #default="{ record }"><span class="font-mono text-xs">{{ record.order_no || '-' }}</span></template>
				</a-table-column>
				<a-table-column title="到期" key="expires" width="180">
				  <template #default="{ record }">{{ formatTime(record.expires_at) }}</template>
				</a-table-column>
				<a-table-column title="动作" key="action" width="360">
				  <template #default="{ record }">
					<a-space :size="4" wrap>
					  <a-button size="small" :loading="exportingOrderID===record.id" @click="exportOrder(record.id)">导出</a-button>
					  <a-button size="small" @click="copyOrderLines(record)">复制发货</a-button>
					  <a-button v-if="isResidentialMode(record.mode)" size="small" @click="resetOrderCredentials(record.id)">刷新凭据</a-button>
					  <a-button size="small" danger @click="removeOrder(record.id)">删除</a-button>
					</a-space>
				  </template>
				</a-table-column>
			  </a-table>
			</a-card>
		  </template>

          <template v-else-if="panel.activeTab === 'import'">
            <a-row :gutter="12">
              <a-col :xs="24" :lg="10">
                <a-card :bordered="false" title="批量导入已有 Socks5">
                  <a-space direction="vertical" style="width:100%">
                    <a-alert
                      type="info"
                      show-icon
                      message="一键导入 sing-box"
                      description="扫描 /etc/sing-box/conf/*.json + /etc/sing-box/*.json，默认导入有效期预设 15 天。"
                    />
                    <a-space>
                      <a-button @click="scanSingboxConfigs">扫描宿主机配置</a-button>
                      <a-button type="primary" ghost :loading="previewingSingboxImport" :disabled="singboxSelectedFiles.length === 0" @click="previewSelectedSingboxFiles">预检已选文件</a-button>
                    </a-space>
                    <a-checkbox :checked="allSingboxSelected" @change="(e:any)=>toggleSingboxSelectAll(Boolean(e.target?.checked))">全选可导入文件</a-checkbox>
                    <div class="max-h-52 overflow-auto rounded border border-slate-200 p-2">
                      <a-checkbox-group v-model:value="singboxSelectedFiles" style="width:100%">
                        <a-space direction="vertical" style="width:100%">
                          <a-checkbox v-for="file in panel.singboxScan?.files || []" :key="file.path" :value="file.path" :disabled="!file.selectable">
                            <span class="font-mono text-xs">{{ file.path }}</span>
                            <span class="text-xs text-slate-500"> ({{ file.entry_count }} 条)</span>
                            <span v-if="file.error" class="text-xs text-rose-500"> - {{ file.error }}</span>
                          </a-checkbox>
                        </a-space>
                      </a-checkbox-group>
                      <div v-if="!(panel.singboxScan?.files || []).length" class="text-xs text-slate-500">尚未扫描配置文件</div>
                    </div>
                    <a-select v-model:value="importForm.customer_id" style="width:100%" placeholder="选择客户">
                      <a-select-option :value="0">暂不配置客户（自动归入未分配客户）</a-select-option>
                      <a-select-option v-for="c in panel.customers" :key="c.id" :value="c.id">{{ c.name }}</a-select-option>
                    </a-select>
                    <a-input v-model:value="importForm.order_name" placeholder="导入订单名" />
                    <a-date-picker v-model:value="importForm.expires_at" show-time style="width:100%" value-format="YYYY-MM-DDTHH:mm:ss" />
                    <a-space>
                      <a-button size="small" @click="setImportExpiryDays(15)">15天(预设)</a-button>
                      <a-button size="small" @click="setImportExpiryDays(30)">30天</a-button>
                      <a-button size="small" @click="setImportExpiryDays(2)">2天</a-button>
                    </a-space>
                    <a-textarea v-model:value="importForm.lines" :rows="14" placeholder="每行: ip:port:user:pass" />
                    <a-space>
                      <a-button :loading="previewingImport" @click="previewImport">预检</a-button>
                      <a-button danger ghost @click="previewCrossNodeMigration">跨节点预检</a-button>
                      <a-button type="primary" :loading="confirmingImport" :disabled="!importPreviewValid" @click="confirmImport">确认导入</a-button>
                    </a-space>
                    <a-alert v-if="(panel.importPreview || []).length > 0 && !importPreviewValid" type="warning" show-icon message="预检结果已失效，请重新预检后导入" />
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

				<a-alert class="mt-3" type="info" show-icon message="转发出口仅保留历史兼容能力，家宽订单请使用 auto/manual。" />
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

			<a-card :bordered="false" title="设置-专线" class="max-w-4xl mb-3">
			  <a-row :gutter="12">
				<a-col :xs="24" :md="12"><a-form-item label="VLESS Security"><a-input v-model:value="panel.settings.dedicated_vless_security" placeholder="tls / none" /></a-form-item></a-col>
				<a-col :xs="24" :md="12"><a-form-item label="VLESS SNI"><a-input v-model:value="panel.settings.dedicated_vless_sni" placeholder="如 edge.example.com" /></a-form-item></a-col>
				<a-col :xs="24" :md="12"><a-form-item label="VLESS Type"><a-input v-model:value="panel.settings.dedicated_vless_type" placeholder="tcp/ws/grpc" /></a-form-item></a-col>
				<a-col :xs="24" :md="12"><a-form-item label="VLESS Host"><a-input v-model:value="panel.settings.dedicated_vless_host" placeholder="可选 Host" /></a-form-item></a-col>
				<a-col :xs="24"><a-form-item label="VLESS Path"><a-input v-model:value="panel.settings.dedicated_vless_path" placeholder="可选 /path" /></a-form-item></a-col>
			  </a-row>
			  <a-space>
				<a-button @click="openDedicatedManager">管理 Inbound / Ingress</a-button>
				<span class="text-xs text-slate-500">专线入口管理已迁移到设置中心</span>
			  </a-space>
			</a-card>

			<a-card :bordered="false" title="设置-家宽" class="max-w-4xl mb-3">
			  <a-row :gutter="12">
				<a-col :xs="24" :md="12"><a-form-item label="家宽订单名称前缀"><a-input v-model:value="panel.settings.residential_name_prefix" placeholder="家宽-Socks5" /></a-form-item></a-col>
			  </a-row>
			  <a-alert type="info" show-icon message="forward 模式已废弃，家宽订单统一使用 家宽-自动 / 家宽-手动。" />
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

    <a-drawer v-model:open="forwardManagerOpen" title="SOCKS5 转发出口管理" width="980" :destroy-on-close="false">
      <a-row :gutter="8" class="mb-2">
        <a-col :span="6"><a-statistic title="出口总数" :value="forwardStats.total" /></a-col>
        <a-col :span="6"><a-statistic title="启用中" :value="forwardStats.enabled" /></a-col>
        <a-col :span="6"><a-statistic title="探测成功" :value="forwardStats.ok" /></a-col>
        <a-col :span="6"><a-statistic title="已含分流账号" :value="forwardStats.autoUser" /></a-col>
      </a-row>
      <a-row :gutter="12">
        <a-col :xs="24" :lg="10">
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
        </a-col>
        <a-col :xs="24" :lg="14">
          <a-table
            :columns="forwardOutboundColumns"
            :data-source="panel.forwardOutbounds"
            size="small"
            :pagination="{ pageSize: 8 }"
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
        </a-col>
      </a-row>
    </a-drawer>

	<a-drawer v-model:open="dedicatedManagerOpen" title="专线 Inbound / Ingress 管理" width="1180" :destroy-on-close="false">
	  <a-row :gutter="12">
		<a-col :xs="24" :lg="12">
		  <a-card size="small" title="新增 Inbound（协议 + 本机监听端口）">
			<a-space direction="vertical" style="width:100%">
			  <a-input v-model:value="dedicatedInboundForm.name" placeholder="Inbound名称，如 us-mixed-a" />
			  <a-select v-model:value="dedicatedInboundForm.protocol" style="width:100%" placeholder="协议">
				<a-select-option v-for="opt in dedicatedProtocolOptions" :key="opt.value" :value="opt.value">{{ opt.value }}</a-select-option>
			  </a-select>
			  <a-input-number v-model:value="dedicatedInboundForm.listen_port" :min="1" :max="65535" style="width:100%" placeholder="本机监听端口" />
			  <a-input-number v-model:value="dedicatedInboundForm.priority" :min="1" :max="999" style="width:100%" placeholder="优先级(越小越高)" />
			  <a-input v-model:value="dedicatedInboundForm.notes" placeholder="备注" />
			  <a-space>
				<a-switch :checked="dedicatedInboundForm.enabled" @change="(v:boolean)=>dedicatedInboundForm.enabled=v" />
				<span class="text-xs text-slate-500">启用</span>
			  </a-space>
			  <a-space>
				<a-button @click="resetDedicatedInboundForm">重置</a-button>
				<a-button type="primary" @click="createDedicatedInbound">新增 Inbound</a-button>
			  </a-space>
			</a-space>
		  </a-card>
		</a-col>
		<a-col :xs="24" :lg="12">
		  <a-card size="small" title="新增 Ingress（公网域名 + 入口端口）">
			<a-space direction="vertical" style="width:100%">
			  <a-select v-model:value="dedicatedIngressForm.dedicated_inbound_id" style="width:100%" placeholder="绑定 Inbound">
				<a-select-option v-for="row in enabledDedicatedInbounds" :key="row.id" :value="row.id">
				  {{ row.name }} / {{ row.protocol }} / :{{ row.listen_port }}
				</a-select-option>
			  </a-select>
			  <a-input v-model:value="dedicatedIngressForm.name" placeholder="Ingress名称，如 us-east-01" />
			  <a-input v-model:value="dedicatedIngressForm.domain" placeholder="入口域名或IP，如 line-us.example.com" />
			  <a-input-number v-model:value="dedicatedIngressForm.ingress_port" :min="1" :max="65535" style="width:100%" placeholder="对外入口端口" />
			  <a-space style="width:100%">
				<a-input v-model:value="dedicatedIngressForm.country_code" placeholder="国家代码，如 US" />
				<a-input v-model:value="dedicatedIngressForm.region" placeholder="区域，如 Virginia" />
			  </a-space>
			  <a-input-number v-model:value="dedicatedIngressForm.priority" :min="1" :max="999" style="width:100%" placeholder="优先级(越小越高)" />
			  <a-input v-model:value="dedicatedIngressForm.notes" placeholder="备注" />
			  <a-space>
				<a-switch :checked="dedicatedIngressForm.enabled" @change="(v:boolean)=>dedicatedIngressForm.enabled=v" />
				<span class="text-xs text-slate-500">启用</span>
			  </a-space>
			  <a-space>
				<a-button @click="resetDedicatedIngressForm">重置</a-button>
				<a-button type="primary" @click="createDedicatedIngress">新增 Ingress</a-button>
			  </a-space>
			</a-space>
		  </a-card>
		</a-col>
	  </a-row>

	  <a-row :gutter="12" class="mt-3">
		<a-col :xs="24" :lg="12">
		  <a-card size="small" title="Inbound 列表">
			<a-table :data-source="panel.dedicatedInbounds" :row-key="(row:any)=>row.id" size="small" :pagination="{ pageSize: 6 }">
			  <a-table-column title="名称" key="name" width="160">
				<template #default="{ record }">
				  <span class="font-mono text-xs">{{ record.name || '-' }}</span>
				</template>
			  </a-table-column>
			  <a-table-column title="协议" data-index="protocol" key="protocol" width="100" />
			  <a-table-column title="监听" key="listen_port" width="100">
				<template #default="{ record }">:{{ record.listen_port }}</template>
			  </a-table-column>
			  <a-table-column title="优先级" data-index="priority" key="priority" width="88" />
			  <a-table-column title="启用" key="enabled" width="80">
				<template #default="{ record }">
				  <a-switch :checked="record.enabled" @change="(checked:boolean)=>panel.toggleDedicatedInbound(record.id, checked)" />
				</template>
			  </a-table-column>
			  <a-table-column title="动作" key="action" width="140">
				<template #default="{ record }">
				  <a-space :size="4">
					<a-button size="small" @click="openDedicatedInboundEdit(record)">编辑</a-button>
					<a-button size="small" danger @click="removeDedicatedInbound(record.id)">删除</a-button>
				  </a-space>
				</template>
			  </a-table-column>
			</a-table>
		  </a-card>
		</a-col>
		<a-col :xs="24" :lg="12">
		  <a-card size="small" title="Ingress 列表">
			<a-table :data-source="panel.dedicatedIngresses" :row-key="(row:any)=>row.id" size="small" :pagination="{ pageSize: 6 }">
			  <a-table-column title="入口" key="entry" width="230">
				<template #default="{ record }">
				  <div class="font-mono text-xs">{{ record.name || '-' }} / {{ record.domain }}:{{ record.ingress_port }}</div>
				</template>
			  </a-table-column>
			  <a-table-column title="Inbound" key="inbound" width="180">
				<template #default="{ record }">
				  <span class="text-xs">
					{{ record.dedicated_inbound?.name || `#${record.dedicated_inbound_id}` }}
					/ {{ record.dedicated_inbound?.protocol || '-' }}
					/ :{{ record.dedicated_inbound?.listen_port || '-' }}
				  </span>
				</template>
			  </a-table-column>
			  <a-table-column title="区域" key="region" width="140">
				<template #default="{ record }">{{ (record.country_code || '--').toUpperCase() }} {{ record.region || '' }}</template>
			  </a-table-column>
			  <a-table-column title="优先级" data-index="priority" key="priority" width="88" />
			  <a-table-column title="启用" key="enabled" width="80">
				<template #default="{ record }">
				  <a-switch :checked="record.enabled" @change="(checked:boolean)=>panel.toggleDedicatedIngress(record.id, checked)" />
				</template>
			  </a-table-column>
			  <a-table-column title="动作" key="action" width="140">
				<template #default="{ record }">
				  <a-space :size="4">
					<a-button size="small" @click="openDedicatedIngressEdit(record)">编辑</a-button>
					<a-button size="small" danger @click="removeDedicatedIngress(record.id)">删除</a-button>
				  </a-space>
				</template>
			  </a-table-column>
			</a-table>
		  </a-card>
		</a-col>
	  </a-row>

	  <a-divider class="my-3" />
	  <a-alert type="warning" show-icon message="兼容模式：老版 DedicatedEntry 仅用于历史数据映射，新建订单请优先使用 Inbound + Ingress" class="mb-2" />
	  <a-row :gutter="12">
		<a-col :xs="24" :lg="10">
		  <a-space direction="vertical" style="width:100%">
			<a-input v-model:value="dedicatedForm.name" placeholder="入口名称(可空)" />
			<a-input v-model:value="dedicatedForm.domain" placeholder="域名或IP，如 line-us.example.com" />
			<a-input-number v-model:value="dedicatedForm.priority" :min="1" :max="999" style="width:100%" placeholder="优先级(越小越高)" />
			<a-input-number v-model:value="dedicatedForm.mixed_port" :min="1" :max="65535" style="width:100%" addon-before="Mixed" />
			<a-input-number v-model:value="dedicatedForm.vmess_port" :min="1" :max="65535" style="width:100%" addon-before="Vmess" />
			<a-input-number v-model:value="dedicatedForm.vless_port" :min="1" :max="65535" style="width:100%" addon-before="Vless" />
			<a-input-number v-model:value="dedicatedForm.shadowsocks_port" :min="1" :max="65535" style="width:100%" addon-before="Shadowsocks" />
			<a-checkbox-group v-model:value="dedicatedForm.features" :options="['mixed','vmess','vless','shadowsocks']" />
			<a-input v-model:value="dedicatedForm.notes" placeholder="备注" />
			<a-space>
			  <a-switch :checked="dedicatedForm.enabled" @change="(v:boolean)=>dedicatedForm.enabled=v" />
			  <span class="text-xs text-slate-500">启用</span>
			</a-space>
			<a-space>
			  <a-button @click="resetDedicatedForm">重置</a-button>
			  <a-button type="primary" @click="createDedicatedEntry">新增Legacy入口</a-button>
			</a-space>
		  </a-space>
		</a-col>
		<a-col :xs="24" :lg="14">
		  <a-table :data-source="panel.dedicatedEntries" :row-key="(row:any)=>row.id" size="small" :pagination="{ pageSize: 6 }">
			<a-table-column title="入口" key="entry" width="220">
			  <template #default="{ record }">
				<div class="font-mono text-xs">{{ record.name || '-' }} / {{ record.domain }}</div>
			  </template>
			</a-table-column>
			<a-table-column title="协议端口" key="ports" width="220">
			  <template #default="{ record }">
				<div class="text-xs">M {{ record.mixed_port }} | VM {{ record.vmess_port }} | VL {{ record.vless_port }} | SS {{ record.shadowsocks_port }}</div>
			  </template>
			</a-table-column>
			<a-table-column title="特性" key="features" width="120">
			  <template #default="{ record }">
				<span class="text-xs">{{ record.features }}</span>
			  </template>
			</a-table-column>
			<a-table-column title="优先级" data-index="priority" key="priority" width="90" />
			<a-table-column title="启用" key="enabled" width="80">
			  <template #default="{ record }">
				<a-switch :checked="record.enabled" @change="(checked:boolean)=>panel.toggleDedicatedEntry(record.id, checked)" />
			  </template>
			</a-table-column>
			<a-table-column title="动作" key="action" width="150">
			  <template #default="{ record }">
				<a-space :size="4">
				  <a-button size="small" @click="openDedicatedEdit(record)">编辑</a-button>
				  <a-button size="small" danger @click="removeDedicatedEntry(record.id)">删除</a-button>
				</a-space>
			  </template>
			</a-table-column>
		  </a-table>
		</a-col>
	  </a-row>
	</a-drawer>

    <a-modal v-model:open="orderDetailOpen" title="订单详情" width="980px" :footer="null">
      <div v-if="orderDetailLoading" class="py-8 text-center">加载中...</div>
      <div v-else-if="panel.selectedOrder">
        <a-descriptions bordered :column="2" size="small" class="mb-3">
          <a-descriptions-item label="订单ID">#{{ panel.selectedOrder.id }}</a-descriptions-item>
          <a-descriptions-item label="订单号"><span class="font-mono">{{ panel.selectedOrder.order_no || '-' }}</span></a-descriptions-item>
          <a-descriptions-item label="客户">{{ panel.selectedOrder.customer?.name || panel.selectedOrder.customer_id }}</a-descriptions-item>
          <a-descriptions-item label="状态"><a-tag :color="statusColor(panel.selectedOrder.status)">{{ panel.selectedOrder.status }}</a-tag></a-descriptions-item>
          <a-descriptions-item label="模式"><a-tag :color="modeColor(panel.selectedOrder.mode)">{{ modeLabel(panel.selectedOrder.mode) }}</a-tag></a-descriptions-item>
          <a-descriptions-item label="开始">{{ formatTime(panel.selectedOrder.starts_at) }}</a-descriptions-item>
          <a-descriptions-item label="到期">{{ formatTime(panel.selectedOrder.expires_at) }}</a-descriptions-item>
        </a-descriptions>

        <div class="mb-2 flex items-center justify-between">
          <div class="font-semibold">订单条目 ({{ panel.selectedOrder.items.length }})</div>
          <a-space>
            <a-input-number v-model:value="exportCount" :min="0" :max="panel.selectedOrder.items.length" size="small" />
            <a-checkbox v-model:checked="exportIncludeRawSocks5" class="text-xs">附带原始Socks5</a-checkbox>
            <a-button size="small" :loading="exportingOrderID===panel.selectedOrder.id" @click="exportOrder(panel.selectedOrder.id)">提取导出</a-button>
            <a-button size="small" @click="copyOrderLines(panel.selectedOrder)">复制发货内容</a-button>
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

    <a-modal v-model:open="orderEditOpen" title="编辑订单" :confirm-loading="savingOrderEdit" :ok-button-props="isForwardDeprecatedOrderEdit ? { disabled: true } : undefined" @ok="saveOrderEdit">
      <a-form layout="vertical">
        <a-form-item label="模式"><a-tag :color="modeColor(orderEditForm.mode)">{{ modeLabel(orderEditForm.mode) }}</a-tag></a-form-item>
        <a-form-item label="订单名称"><a-input v-model:value="orderEditForm.name" :disabled="isForwardDeprecatedOrderEdit" /></a-form-item>
        <a-form-item v-if="orderEditForm.mode !== 'forward' && orderEditForm.mode !== 'dedicated'" label="数量"><a-input-number v-model:value="orderEditForm.quantity" :min="1" style="width:100%" /></a-form-item>
        <a-form-item v-if="orderEditForm.mode === 'manual'" label="手动IP绑定(可多选)">
          <a-select v-model:value="orderEditForm.manual_ip_ids" mode="multiple" style="width:100%" placeholder="选择手动IP">
            <a-select-option v-for="ip in manualHostIPOptions" :key="ip.id" :value="ip.id">{{ ip.ip }}</a-select-option>
          </a-select>
          <div class="mt-1 text-xs text-slate-500">若不调整数量，可留空保持原绑定；调整数量时建议明确选择。</div>
        </a-form-item>
        <a-form-item v-else-if="orderEditForm.mode === 'forward'" label="废弃模式（只读）">
          <a-alert type="warning" show-icon message="forward 模式已废弃：历史订单可查看，但不支持编辑。" />
          <a-input class="mt-2" :value="`历史出口数量: ${orderEditForm.forward_outbound_ids.length}`" disabled />
        </a-form-item>
        <a-form-item v-else label="专线入口">
          <a-space class="mb-2" wrap>
				<span class="text-xs text-slate-500">专线 Inbound/Ingress 请到 设置-专线 管理</span>
          </a-space>
          <a-select v-model:value="orderEditForm.dedicated_protocol" style="width:100%" placeholder="选择协议">
            <a-select-option v-for="opt in dedicatedProtocolOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </a-select-option>
          </a-select>
          <a-select v-model:value="orderEditForm.dedicated_inbound_id" class="mt-2" style="width:100%" placeholder="选择Inbound(协议+本机端口)">
            <a-select-option v-for="row in filteredDedicatedInboundsForEdit" :key="row.id" :value="row.id">
              {{ row.name }} / {{ row.protocol }} / :{{ row.listen_port }} <span v-if="!row.enabled">(停用)</span>
            </a-select-option>
          </a-select>
          <a-select v-model:value="orderEditForm.dedicated_ingress_id" class="mt-2" style="width:100%" placeholder="选择Ingress(入口域名:端口)">
            <a-select-option v-for="row in filteredDedicatedIngressesForEdit" :key="row.id" :value="row.id">
              {{ row.name || row.domain }} / {{ row.domain }}:{{ row.ingress_port }} / {{ (row.country_code || '--').toUpperCase() }} {{ row.region || '' }} <span v-if="!row.enabled">(停用)</span>
            </a-select-option>
          </a-select>
		  <div class="mt-2">
			<div class="mb-1 text-xs text-slate-500">当前入站Socks5凭据(可复制)</div>
			<a-space class="mb-1">
			  <a-button size="small" @click="copyOrderEditInboundLines">一键复制当前入站</a-button>
			</a-space>
			<a-textarea :value="orderEditCurrentInboundLines" :rows="3" readonly />
		  </div>
		  <div class="mt-2">
			<div class="mb-1 text-xs text-slate-500">当前出站Socks5(可复制)</div>
			<a-space class="mb-1">
			  <a-button size="small" @click="copyOrderEditEgressLines">一键复制当前出站</a-button>
			</a-space>
			<a-textarea :value="orderEditCurrentEgressLines" :rows="3" readonly />
		  </div>
          <a-textarea v-model:value="orderEditForm.dedicated_egress_lines" class="mt-2" :rows="4" placeholder="可选: 顺序更新上游，每行 ip:port:user:pass" />
          <a-textarea v-model:value="orderEditForm.dedicated_credential_lines" class="mt-2" :rows="3" placeholder="可选: 顺序更新入站凭据，每行 user:pass[:uuid]" />
          <a-checkbox v-model:checked="orderEditForm.regenerate_dedicated_credentials" class="mt-2">随机重置组内凭据</a-checkbox>
        </a-form-item>
        <a-form-item label="端口"><a-input-number v-model:value="orderEditForm.port" :min="1" :max="65535" :disabled="orderEditForm.mode === 'dedicated' || isForwardDeprecatedOrderEdit" style="width:100%" /></a-form-item>
        <a-form-item label="到期时间"><a-date-picker v-model:value="orderEditForm.expires_at" :disabled="isForwardDeprecatedOrderEdit" show-time style="width:100%" value-format="YYYY-MM-DDTHH:mm:ss" /></a-form-item>
        <a-space class="mb-2">
          <a-button size="small" :disabled="isForwardDeprecatedOrderEdit" @click="setQuickExpiry(7, 'edit')">7天</a-button>
          <a-button size="small" :disabled="isForwardDeprecatedOrderEdit" @click="setQuickExpiry(30, 'edit')">30天</a-button>
          <a-button size="small" :disabled="isForwardDeprecatedOrderEdit" @click="setQuickExpiry(90, 'edit')">90天</a-button>
        </a-space>
        <a-alert v-if="panel.allocationPreview" type="info" show-icon :message="`可分配IP: ${panel.allocationPreview.available} / 池总量: ${panel.allocationPreview.pool_size} / 已占用: ${panel.allocationPreview.used_by_customer}`" />
        <a-alert v-if="orderEditForm.mode === 'forward'" class="mt-2" type="info" show-icon message="该模式已冻结，仅用于历史兼容。" />
      </a-form>
    </a-modal>

	<a-modal v-model:open="dedicatedEditOpen" title="编辑专线入口" @ok="saveDedicatedEdit">
	  <a-form layout="vertical">
		<a-form-item label="入口名称"><a-input v-model:value="dedicatedForm.name" /></a-form-item>
		<a-form-item label="域名"><a-input v-model:value="dedicatedForm.domain" /></a-form-item>
		<a-form-item label="优先级"><a-input-number v-model:value="dedicatedForm.priority" :min="1" :max="999" style="width:100%" /></a-form-item>
		<a-form-item label="Mixed端口"><a-input-number v-model:value="dedicatedForm.mixed_port" :min="1" :max="65535" style="width:100%" /></a-form-item>
		<a-form-item label="Vmess端口"><a-input-number v-model:value="dedicatedForm.vmess_port" :min="1" :max="65535" style="width:100%" /></a-form-item>
		<a-form-item label="Vless端口"><a-input-number v-model:value="dedicatedForm.vless_port" :min="1" :max="65535" style="width:100%" /></a-form-item>
		<a-form-item label="Shadowsocks端口"><a-input-number v-model:value="dedicatedForm.shadowsocks_port" :min="1" :max="65535" style="width:100%" /></a-form-item>
		<a-form-item label="特性">
		  <a-checkbox-group v-model:value="dedicatedForm.features" :options="['mixed','vmess','vless','shadowsocks']" />
		</a-form-item>
		<a-form-item label="备注"><a-input v-model:value="dedicatedForm.notes" /></a-form-item>
		<a-form-item>
		  <a-switch :checked="dedicatedForm.enabled" @change="(v:boolean)=>dedicatedForm.enabled=v" />
		  <span class="ml-2 text-xs text-slate-500">启用</span>
		</a-form-item>
	  </a-form>
	</a-modal>

	<a-modal v-model:open="dedicatedInboundEditOpen" title="编辑 Inbound" @ok="saveDedicatedInboundEdit">
	  <a-form layout="vertical">
		<a-form-item label="名称"><a-input v-model:value="dedicatedInboundEditForm.name" /></a-form-item>
		<a-form-item label="协议">
		  <a-select v-model:value="dedicatedInboundEditForm.protocol" style="width:100%">
			<a-select-option v-for="opt in dedicatedProtocolOptions" :key="opt.value" :value="opt.value">{{ opt.value }}</a-select-option>
		  </a-select>
		</a-form-item>
		<a-form-item label="监听端口"><a-input-number v-model:value="dedicatedInboundEditForm.listen_port" :min="1" :max="65535" style="width:100%" /></a-form-item>
		<a-form-item label="优先级"><a-input-number v-model:value="dedicatedInboundEditForm.priority" :min="1" :max="999" style="width:100%" /></a-form-item>
		<a-form-item label="备注"><a-input v-model:value="dedicatedInboundEditForm.notes" /></a-form-item>
		<a-form-item>
		  <a-switch :checked="dedicatedInboundEditForm.enabled" @change="(v:boolean)=>dedicatedInboundEditForm.enabled=v" />
		  <span class="ml-2 text-xs text-slate-500">启用</span>
		</a-form-item>
	  </a-form>
	</a-modal>

	<a-modal v-model:open="dedicatedIngressEditOpen" title="编辑 Ingress" @ok="saveDedicatedIngressEdit">
	  <a-form layout="vertical">
		<a-form-item label="绑定 Inbound">
		  <a-select v-model:value="dedicatedIngressEditForm.dedicated_inbound_id" style="width:100%">
			<a-select-option v-for="row in panel.dedicatedInbounds" :key="row.id" :value="row.id">
			  {{ row.name }} / {{ row.protocol }} / :{{ row.listen_port }}
			</a-select-option>
		  </a-select>
		</a-form-item>
		<a-form-item label="名称"><a-input v-model:value="dedicatedIngressEditForm.name" /></a-form-item>
		<a-form-item label="域名"><a-input v-model:value="dedicatedIngressEditForm.domain" /></a-form-item>
		<a-form-item label="入口端口"><a-input-number v-model:value="dedicatedIngressEditForm.ingress_port" :min="1" :max="65535" style="width:100%" /></a-form-item>
		<a-form-item label="国家/区域">
		  <a-space style="width:100%">
			<a-input v-model:value="dedicatedIngressEditForm.country_code" placeholder="US" />
			<a-input v-model:value="dedicatedIngressEditForm.region" placeholder="Virginia" />
		  </a-space>
		</a-form-item>
		<a-form-item label="优先级"><a-input-number v-model:value="dedicatedIngressEditForm.priority" :min="1" :max="999" style="width:100%" /></a-form-item>
		<a-form-item label="备注"><a-input v-model:value="dedicatedIngressEditForm.notes" /></a-form-item>
		<a-form-item>
		  <a-switch :checked="dedicatedIngressEditForm.enabled" @change="(v:boolean)=>dedicatedIngressEditForm.enabled=v" />
		  <span class="ml-2 text-xs text-slate-500">启用</span>
		</a-form-item>
	  </a-form>
	</a-modal>

	<a-modal v-model:open="groupSocksModalOpen" title="顺序更新组内 Socks5" :confirm-loading="groupSocksSaving" @ok="submitGroupSocksUpdate">
	  <a-alert type="info" show-icon message="每行格式: ip:port:user:pass；顺序必须与子订单顺序一致" class="mb-2" />
	  <a-checkbox-group v-model:value="groupBatchChildOrderIDs" style="width:100%">
		<a-space direction="vertical" style="width:100%" class="mb-2">
		  <a-checkbox v-for="row in groupBatchCandidates" :key="row.id" :value="row.id">
			#{{ row.id }} / {{ row.name }} / {{ row.sequence_no || '-' }}
		  </a-checkbox>
		</a-space>
	  </a-checkbox-group>
	  <a-space class="mb-2">
		<a-button size="small" @click="downloadGroupSocksTemplate">下载模板</a-button>
		<a-upload :show-upload-list="false" accept=".xlsx" :before-upload="beforeUploadGroupSocksXLSX">
		  <a-button size="small">上传XLSX回填</a-button>
		</a-upload>
		<span class="text-xs text-slate-500">模板按子订单顺序生成，上传后自动回填</span>
	  </a-space>
	  <a-textarea v-model:value="groupSocksLines" :rows="10" placeholder="按顺序粘贴 Socks5 列表" />
	</a-modal>

	<a-modal v-model:open="groupCredModalOpen" title="批量更新组内入站凭据" :confirm-loading="groupCredSaving" @ok="submitGroupCredentialUpdate">
	  <a-alert type="info" show-icon message="每行格式: user:pass[:uuid]；不填 uuid 自动生成" class="mb-2" />
	  <a-checkbox-group v-model:value="groupBatchChildOrderIDs" style="width:100%">
		<a-space direction="vertical" style="width:100%" class="mb-2">
		  <a-checkbox v-for="row in groupBatchCandidates" :key="row.id" :value="row.id">
			#{{ row.id }} / {{ row.name }} / {{ row.sequence_no || '-' }}
		  </a-checkbox>
		</a-space>
	  </a-checkbox-group>
	  <a-space class="mb-2">
		<a-button size="small" @click="downloadGroupCredentialTemplate">下载模板</a-button>
		<a-upload :show-upload-list="false" accept=".xlsx" :before-upload="beforeUploadGroupCredXLSX">
		  <a-button size="small">上传XLSX回填</a-button>
		</a-upload>
		<span class="text-xs text-slate-500">第三段 uuid 留空则自动生成</span>
	  </a-space>
	  <a-checkbox v-model:checked="groupCredRegenerate" class="mb-2">忽略文本，随机重置全组凭据</a-checkbox>
	  <a-textarea v-model:value="groupCredLines" :rows="8" :disabled="groupCredRegenerate" placeholder="按顺序粘贴凭据" />
	</a-modal>

	<a-modal v-model:open="groupGeoModalOpen" title="批量设置国家地区" @ok="submitGroupGeoUpdate">
	  <a-alert class="mb-2" type="info" show-icon message="支持两种方式：A) 统一赋值；B) 映射行（ip:port:user:pass|US|Virginia）" />
	  <a-checkbox-group v-model:value="groupBatchChildOrderIDs" style="width:100%">
		<a-space direction="vertical" style="width:100%" class="mb-2">
		  <a-checkbox v-for="row in groupBatchCandidates" :key="row.id" :value="row.id">
			#{{ row.id }} / {{ row.name }} / {{ row.sequence_no || '-' }}
		  </a-checkbox>
		</a-space>
	  </a-checkbox-group>
	  <a-space class="mb-2" style="width:100%">
		<a-input v-model:value="groupGeoCountryCode" placeholder="国家代码，如 US" />
		<a-input v-model:value="groupGeoRegion" placeholder="地区，如 Virginia" />
	  </a-space>
	  <a-textarea v-model:value="groupGeoMappingLines" :rows="7" placeholder="可选映射行\nip:port:user:pass|US|Virginia\nip:port:user:pass|MX|Jalisco" />
	</a-modal>

	<a-modal v-model:open="groupEditorOpen" title="组编辑工作台" :footer="null" width="860px">
	  <a-alert class="mb-2" type="info" show-icon message="先多选子订单，再执行批量操作；也可直接点单条编辑。" />
	  <a-space class="mb-2" wrap>
		<a-button @click="openGroupHeadOrderEditFromEditor">编辑组头信息</a-button>
		<a-button type="primary" @click="openGroupSocksModalFromEditor">批量改 Socks5</a-button>
		<a-button @click="openGroupCredModalFromEditor">批量改凭据</a-button>
		<a-button @click="openGroupGeoModalFromEditor">批量设国家地区</a-button>
		<a-button @click="openGroupRenewModalFromEditor">部分续期</a-button>
	  </a-space>
	  <a-checkbox-group v-model:value="groupEditorChildOrderIDs" style="width:100%">
		<a-space direction="vertical" style="width:100%">
		  <div v-for="row in groupEditorCandidates" :key="row.id" class="rounded border border-slate-200 px-2 py-2">
			<a-space style="width:100%;justify-content:space-between" align="center">
			  <a-checkbox :value="row.id">#{{ row.id }} / {{ row.name }} / 到期 {{ formatTime(row.expires_at) }}</a-checkbox>
			  <a-button size="small" @click="openOrderEdit(row)">单独编辑</a-button>
			</a-space>
		  </div>
		</a-space>
	  </a-checkbox-group>
	</a-modal>

	<a-modal v-model:open="groupRenewModalOpen" title="部分续期子订单" :confirm-loading="groupRenewSaving" @ok="submitGroupSelectedRenew">
	  <a-space class="mb-2" align="center">
		<span class="text-xs text-slate-600">续期天数</span>
		<a-input-number v-model:value="groupRenewDays" :min="1" :max="365" />
		<span class="text-xs text-slate-500">仅对选中子订单生效</span>
	  </a-space>
	  <a-space class="mb-2" align="center">
		<span class="text-xs text-slate-600">或指定到期时间</span>
		<a-date-picker v-model:value="groupRenewExpiresAt" show-time value-format="YYYY-MM-DDTHH:mm:ss" />
	  </a-space>
	  <a-space class="mb-2">
		<a-button size="small" @click="groupRenewDays = 30">30天</a-button>
		<a-button size="small" @click="groupRenewDays = 60">60天</a-button>
		<a-button size="small" @click="groupRenewDays = 90">90天</a-button>
	  </a-space>
	  <a-checkbox-group v-model:value="groupRenewChildOrderIDs" style="width:100%">
		<a-space direction="vertical" style="width:100%">
		  <a-checkbox v-for="row in groupRenewCandidates" :key="row.id" :value="row.id">
			#{{ row.id }} / {{ row.name }} / 到期 {{ formatTime(row.expires_at) }}
		  </a-checkbox>
		</a-space>
	  </a-checkbox-group>
	  <a-alert v-if="!groupRenewCandidates.length" class="mt-2" type="warning" show-icon message="该组暂无可选择的子订单" />
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
