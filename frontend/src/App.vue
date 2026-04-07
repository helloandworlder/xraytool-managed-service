<script setup lang="ts">
import { computed, defineAsyncComponent, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { message, Modal } from 'ant-design-vue'
import {
	ApiOutlined,
	DashboardOutlined,
	DatabaseOutlined,
	SettingOutlined,
	TeamOutlined,
	UnorderedListOutlined,
	UploadOutlined
} from '@ant-design/icons-vue'
import { useAuthStore } from './stores/auth'
import { usePanelStore } from './stores/panel'
import { http, isAuthError, subscribeHttpActivity } from './lib/http'
import type { ImportPreviewRow, Order } from './lib/types'
import AppShell from './components/layout/AppShell.vue'
const DashboardPage = defineAsyncComponent(() => import('./pages/DashboardPage.vue'))
const CustomersPage = defineAsyncComponent(() => import('./pages/CustomersPage.vue'))
const HostIPsPage = defineAsyncComponent(() => import('./pages/HostIPsPage.vue'))
const OrdersPage = defineAsyncComponent(() => import('./pages/OrdersPage.vue'))
const DeliveryPage = defineAsyncComponent(() => import('./pages/DeliveryPage.vue'))
const DedicatedWorkbenchPage = defineAsyncComponent(() => import('./pages/DedicatedWorkbenchPage.vue'))
const ImportPage = defineAsyncComponent(() => import('./pages/ImportPage.vue'))
const SettingsPage = defineAsyncComponent(() => import('./pages/SettingsPage.vue'))

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
	residential_credential_mode: 'random',
	residential_credential_strategy: 'per_line',
	residential_credential_lines: '',
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
function createDedicatedInboundDefaults() {
	return {
		name: '',
		protocol: 'mixed',
		listen_port: 1080,
		priority: 100,
		enabled: true,
		notes: '',
		vless_security: 'none',
		vless_flow: '',
		vless_type: 'tcp',
		vless_sni: '',
		vless_host: '',
		vless_path: '',
		vless_fingerprint: 'chrome',
		vless_tls_cert_file: '',
		vless_tls_key_file: '',
		reality_show: false,
		reality_target: '',
		reality_server_names: '',
		reality_private_key: '',
		reality_public_key: '',
		reality_short_ids: '',
		reality_spider_x: '/',
		reality_xver: 0,
		reality_max_time_diff: 0,
		reality_min_client_ver: '',
		reality_max_client_ver: '',
		reality_mldsa65_seed: '',
		reality_mldsa65_verify: ''
	}
}

const dedicatedInboundForm = reactive(createDedicatedInboundDefaults())
const dedicatedInboundEditForm = reactive({
	id: 0,
	...createDedicatedInboundDefaults()
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
	residential_credential_mode: 'random',
	residential_credential_strategy: 'per_line',
	residential_credential_lines: '',
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
const copyingLinksOrderID = ref<number | null>(null)
const exportCount = ref<number>(0)
const exportDialogOpen = ref(false)
const exportDialogSubmitting = ref(false)
const exportDialogFormat = ref<'txt' | 'xlsx'>('xlsx')
const exportDialogResidentialTXTLayout = ref<'uri' | 'colon'>('uri')
const exportDialogOrderID = ref<number | null>(null)
const exportDialogOrderIDs = ref<number[]>([])
const exportDialogContainsResidential = ref(false)
const exportDialogContainsDedicated = ref(false)
const exportDialogIncludeRawSocks5 = ref(false)
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

const probeResult = ref('')
const testingOrderID = ref<number | null>(null)
const testResult = ref<Record<string, string> | null>(null)
const batchTestResult = ref<Array<{ id: number; success: boolean; result?: Record<string, string>; error?: string }> | null>(null)
const orderDetailOpen = ref(false)
const orderDetailLoading = ref(false)
const orderSearchKeyword = ref('')
const orderCustomerID = ref<number>(0)
const orderModeFilter = ref<'all' | 'home' | 'dedicated'>('all')
const orderStatusFilter = ref<'all' | 'active' | 'expired' | 'disabled'>('all')
const deliverySearchKeyword = ref('')
const orderPagination = reactive({ current: 1, pageSize: 12 })
const deliveryPagination = reactive({ current: 1, pageSize: 12 })
const dedicatedSearchKeyword = ref('')
const dedicatedCustomerID = ref<number>(0)
const dedicatedStatusFilter = ref<'all' | 'active' | 'expired' | 'disabled'>('all')
const dedicatedPagination = reactive({ current: 1, pageSize: 12 })
const activeDedicatedHeadID = ref<number>(0)
const dedicatedCheckRunning = ref(false)
const dedicatedCheckResults = ref<Record<number, Record<string, any>>>({})
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
  { key: 'dedicated', icon: () => h(DatabaseOutlined), label: '专线工作台' },
  { key: 'import', icon: () => h(UploadOutlined), label: '批量导入' },
  { key: 'settings', icon: () => h(SettingOutlined), label: '设置' }
]

const healthCards = computed(() => {
  return [
    { title: '激活订单', value: panel.activeOrderCount, color: '#059669' },
    { title: '到期订单', value: panel.expiredOrderCount, color: '#d97706' },
    { title: '停用订单', value: panel.disabledOrderCount, color: '#dc2626' },
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

function deliverySearchMatches(order: any, keyword: string): boolean {
	const normalized = String(keyword || '').trim().toLowerCase()
	if (!normalized) return true
	if (orderSearchContent(order).includes(normalized)) return true
	const children = Array.isArray(order.children) ? order.children : []
	return children.some((child: any) => orderSearchContent(child).includes(normalized))
}

function deliveryOrdersForCopy(order: Order): Order[] {
	if (String(order.mode || '') !== 'dedicated' || !order.is_group_head) {
		return [order]
	}
	return panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(order.id))
		.sort((a, b) => Number((a as any).sequence_no || 0) - Number((b as any).sequence_no || 0) || Number(a.id) - Number(b.id))
}

function deliveryLinesForOrder(order: Order): string[] {
	const orders = deliveryOrdersForCopy(order)
	const lines: string[] = []
	for (const oneOrder of orders) {
		for (const item of oneOrder.items || []) {
			if (String(oneOrder.mode || '') === 'dedicated') {
				const host = String(oneOrder.dedicated_ingress?.domain || oneOrder.dedicated_entry?.domain || item.ip || '').trim()
				const port = dedicatedCopyPort(oneOrder)
				if (host && port > 0) {
					lines.push(`${host}:${port}:${item.username}:${item.password}`)
				}
				continue
			}
			lines.push(`${item.ip}:${item.port}:${item.username}:${item.password}`)
		}
	}
	return lines
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
	orderRows.value
		.filter((row) => {
			if (deliveryCustomerID.value > 0 && Number(row.customer_id) !== Number(deliveryCustomerID.value)) return false
			if (deliveryMode.value === 'home') return isResidentialMode(String(row.mode || ''))
			if (deliveryMode.value === 'dedicated') return String(row.mode || '') === 'dedicated'
			return true
		})
		.filter((row) => {
			const keyword = String(deliverySearchKeyword.value || '').trim().toLowerCase()
			return deliverySearchMatches(row, keyword)
		})
		.map((row) => ({ ...row, key: row.id }))
)

const dedicatedGroupHeads = computed(() =>
	panel.orders
		.filter((row) => String(row.mode || '') === 'dedicated')
		.filter((row) => Boolean((row as any).is_group_head) || Number((row as any).parent_order_id || 0) === 0)
		.sort((a, b) => Number(b.id) - Number(a.id))
)

const activeDedicatedHead = computed(() =>
	dedicatedGroupHeads.value.find((row) => Number(row.id) === Number(activeDedicatedHeadID.value || 0)) || null
)

const activeDedicatedChildren = computed(() =>
	panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(activeDedicatedHeadID.value || 0))
		.sort((a, b) => Number((a as any).sequence_no || 0) - Number((b as any).sequence_no || 0) || Number(a.id) - Number(b.id))
)

const releaseVersionText = computed(() => {
	const info = panel.versionInfo
	if (!info) return 'Release dev'
	const version = String(info.version || 'dev').trim() || 'dev'
	const commit = String(info.commit || '').trim()
	return commit && commit !== 'unknown' ? `Release ${version} (${commit.slice(0, 7)})` : `Release ${version}`
})

const activeRequests = computed(() =>
	Object.entries(panel.activeRequestMap || {})
		.map(([id, row]) => ({
			id,
			method: String(row.method || 'GET').toUpperCase(),
			url: String(row.url || '/'),
			sinceText: `${Math.max(0, Math.round((Date.now() - Number(row.startedAt || Date.now())) / 1000))}s`
		}))
		.sort((a, b) => a.url.localeCompare(b.url))
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
const dedicatedVlessSecurityOptions = [
	{ label: 'None', value: 'none' },
	{ label: 'TLS', value: 'tls' },
	{ label: 'REALITY', value: 'reality' }
]

const dedicatedInboundCreateIsVless = computed(() => dedicatedInboundForm.protocol === 'vless')
const dedicatedInboundEditIsVless = computed(() => dedicatedInboundEditForm.protocol === 'vless')
const dedicatedInboundCreateUsesTLS = computed(() => dedicatedInboundCreateIsVless.value && dedicatedInboundForm.vless_security === 'tls')
const dedicatedInboundEditUsesTLS = computed(() => dedicatedInboundEditIsVless.value && dedicatedInboundEditForm.vless_security === 'tls')
const dedicatedInboundCreateUsesReality = computed(() => dedicatedInboundCreateIsVless.value && dedicatedInboundForm.vless_security === 'reality')
const dedicatedInboundEditUsesReality = computed(() => dedicatedInboundEditIsVless.value && dedicatedInboundEditForm.vless_security === 'reality')

function resetDedicatedInboundVlessFields(form: ReturnType<typeof createDedicatedInboundDefaults>) {
	form.vless_security = 'none'
	form.vless_flow = ''
	form.vless_type = 'tcp'
	form.vless_sni = ''
	form.vless_host = ''
	form.vless_path = ''
	form.vless_fingerprint = 'chrome'
	form.vless_tls_cert_file = ''
	form.vless_tls_key_file = ''
	form.reality_show = false
	form.reality_target = ''
	form.reality_server_names = ''
	form.reality_private_key = ''
	form.reality_public_key = ''
	form.reality_short_ids = ''
	form.reality_spider_x = '/'
	form.reality_xver = 0
	form.reality_max_time_diff = 0
	form.reality_min_client_ver = ''
	form.reality_max_client_ver = ''
	form.reality_mldsa65_seed = ''
	form.reality_mldsa65_verify = ''
}

function applyDedicatedInboundRow(target: typeof dedicatedInboundEditForm, row: Record<string, any>) {
	target.name = String(row.name || '')
	target.protocol = String(row.protocol || 'mixed')
	target.listen_port = Number(row.listen_port || 0)
	target.priority = Number(row.priority || 100)
	target.enabled = Boolean(row.enabled)
	target.notes = String(row.notes || '')
	target.vless_security = String(row.vless_security || 'none')
	target.vless_flow = String(row.vless_flow || '')
	target.vless_type = String(row.vless_type || 'tcp')
	target.vless_sni = String(row.vless_sni || '')
	target.vless_host = String(row.vless_host || '')
	target.vless_path = String(row.vless_path || '')
	target.vless_fingerprint = String(row.vless_fingerprint || 'chrome')
	target.vless_tls_cert_file = String(row.vless_tls_cert_file || '')
	target.vless_tls_key_file = String(row.vless_tls_key_file || '')
	target.reality_show = Boolean(row.reality_show)
	target.reality_target = String(row.reality_target || '')
	target.reality_server_names = String(row.reality_server_names || '')
	target.reality_private_key = String(row.reality_private_key || '')
	target.reality_public_key = String(row.reality_public_key || '')
	target.reality_short_ids = String(row.reality_short_ids || '')
	target.reality_spider_x = String(row.reality_spider_x || '/')
	target.reality_xver = Number(row.reality_xver || 0)
	target.reality_max_time_diff = Number(row.reality_max_time_diff || 0)
	target.reality_min_client_ver = String(row.reality_min_client_ver || '')
	target.reality_max_client_ver = String(row.reality_max_client_ver || '')
	target.reality_mldsa65_seed = String(row.reality_mldsa65_seed || '')
	target.reality_mldsa65_verify = String(row.reality_mldsa65_verify || '')
	if (target.protocol !== 'vless') {
		resetDedicatedInboundVlessFields(target as ReturnType<typeof createDedicatedInboundDefaults>)
	}
}

function buildDedicatedInboundPayload(form: typeof dedicatedInboundForm | typeof dedicatedInboundEditForm) {
	return {
		name: form.name,
		protocol: form.protocol,
		listen_port: Number(form.listen_port),
		priority: Number(form.priority),
		enabled: form.enabled,
		notes: form.notes,
		vless_security: form.protocol === 'vless' ? String(form.vless_security || 'none') : '',
		vless_flow: form.protocol === 'vless' ? form.vless_flow : '',
		vless_type: form.protocol === 'vless' ? String(form.vless_type || 'tcp') : '',
		vless_sni: form.protocol === 'vless' ? form.vless_sni : '',
		vless_host: form.protocol === 'vless' ? form.vless_host : '',
		vless_path: form.protocol === 'vless' ? form.vless_path : '',
		vless_fingerprint: form.protocol === 'vless' ? form.vless_fingerprint : '',
		vless_tls_cert_file: form.protocol === 'vless' ? form.vless_tls_cert_file : '',
		vless_tls_key_file: form.protocol === 'vless' ? form.vless_tls_key_file : '',
		reality_show: form.protocol === 'vless' ? form.reality_show : false,
		reality_target: form.protocol === 'vless' ? form.reality_target : '',
		reality_server_names: form.protocol === 'vless' ? form.reality_server_names : '',
		reality_private_key: form.protocol === 'vless' ? form.reality_private_key : '',
		reality_short_ids: form.protocol === 'vless' ? form.reality_short_ids : '',
		reality_spider_x: form.protocol === 'vless' ? form.reality_spider_x : '',
		reality_xver: form.protocol === 'vless' ? Number(form.reality_xver || 0) : 0,
		reality_max_time_diff: form.protocol === 'vless' ? Number(form.reality_max_time_diff || 0) : 0,
		reality_min_client_ver: form.protocol === 'vless' ? form.reality_min_client_ver : '',
		reality_max_client_ver: form.protocol === 'vless' ? form.reality_max_client_ver : '',
		reality_mldsa65_seed: form.protocol === 'vless' ? form.reality_mldsa65_seed : '',
		reality_mldsa65_verify: form.protocol === 'vless' ? form.reality_mldsa65_verify : ''
	}
}

async function fillRealityKeyPair(form: typeof dedicatedInboundForm | typeof dedicatedInboundEditForm) {
	try {
		const res = await panel.generateRealityKeyPair()
		form.reality_private_key = String(res.private_key || '')
		form.reality_public_key = String(res.public_key || '')
		message.success('REALITY 密钥已生成')
	} catch (err) {
		panel.setError(err)
	}
}

async function validateDedicatedInboundConfig(form: typeof dedicatedInboundForm | typeof dedicatedInboundEditForm) {
	try {
		const res = await panel.validateDedicatedInbound(buildDedicatedInboundPayload(form))
		applyDedicatedInboundRow(form as typeof dedicatedInboundEditForm, res.inbound as Record<string, any>)
		message.success('Inbound 参数校验通过')
	} catch (err) {
		panel.setError(err)
	}
}

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
  stopHttpActivitySubscription = subscribeHttpActivity((event) => {
    if (event.phase === 'start') {
      panel.noteRequestStart({
        requestId: event.requestId,
        method: event.method,
        url: event.url,
        startedAt: event.startedAt
      })
      return
    }
    panel.noteRequestFinish({
      requestId: event.requestId,
      method: event.method,
      url: event.url,
      status: event.status,
      ok: event.ok,
      durationMs: event.durationMs,
      error: event.error
    })
  })
  if (!auth.isAuthed) return
  try {
    await panel.bootstrap()
    seedDefaultsFromSettings()
  } catch (err) {
    panel.setError(err)
    if (isAuthError(err)) {
      auth.logout()
      panel.$reset()
    }
	}
})

let stopHttpActivitySubscription: null | (() => void) = null

onBeforeUnmount(() => {
  stopHttpActivitySubscription?.()
  stopHttpActivitySubscription = null
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
	() => [orderSearchKeyword.value, orderCustomerID.value, orderModeFilter.value, orderStatusFilter.value],
	() => {
		orderPagination.current = 1
		if (panel.activeTab === 'orders') {
			void loadOrdersView()
		}
	}
)

watch(
	() => [dedicatedSearchKeyword.value, dedicatedCustomerID.value, dedicatedStatusFilter.value],
	() => {
		dedicatedPagination.current = 1
		if (panel.activeTab === 'dedicated') {
			void loadDedicatedView()
		}
	}
)

watch(
	() => [deliverySearchKeyword.value, deliveryMode.value, deliveryCustomerID.value],
	() => {
		deliveryPagination.current = 1
		if (panel.activeTab === 'delivery') {
			void loadDeliveryView()
		}
	}
)

watch(
	() => panel.activeTab,
	(tab) => {
		if (tab === 'orders') {
			void loadOrdersView()
			return
		}
		if (tab === 'delivery') {
			void loadDeliveryView()
			return
		}
		if (tab === 'dedicated') {
			void loadDedicatedView()
		}
	}
)

watch(
	() => dedicatedGroupHeads.value.map((row) => Number(row.id)).join(','),
	() => {
		if (!dedicatedGroupHeads.value.length) {
			activeDedicatedHeadID.value = 0
			dedicatedCheckResults.value = {}
			return
		}
		if (!dedicatedGroupHeads.value.some((row) => Number(row.id) === Number(activeDedicatedHeadID.value || 0))) {
			activeDedicatedHeadID.value = Number(dedicatedGroupHeads.value[0].id)
			dedicatedCheckResults.value = {}
		}
	}
)

async function loadOrdersView(page = orderPagination.current, pageSize = orderPagination.pageSize) {
	try {
		await panel.loadOrders({
			page,
			page_size: pageSize,
			keyword: String(orderSearchKeyword.value || '').trim(),
			mode: orderModeFilter.value,
			status: orderStatusFilter.value,
			customer_id: Number(orderCustomerID.value || 0)
		})
		orderPagination.current = Number(panel.orderList.page || page || 1)
		orderPagination.pageSize = Number(panel.orderList.pageSize || pageSize || 12)
	} catch (err) {
		panel.setError(err)
	}
}

async function loadDeliveryView(page = deliveryPagination.current, pageSize = deliveryPagination.pageSize) {
	try {
		await panel.loadOrders({
			page,
			page_size: pageSize,
			keyword: String(deliverySearchKeyword.value || '').trim(),
			mode: deliveryMode.value,
			customer_id: Number(deliveryCustomerID.value || 0),
			status: 'all'
		})
		deliveryPagination.current = Number(panel.orderList.page || page || 1)
		deliveryPagination.pageSize = Number(panel.orderList.pageSize || pageSize || 12)
	} catch (err) {
		panel.setError(err)
	}
}

async function loadDedicatedView(page = dedicatedPagination.current, pageSize = dedicatedPagination.pageSize) {
	try {
		await panel.loadOrders({
			page,
			page_size: pageSize,
			keyword: String(dedicatedSearchKeyword.value || '').trim(),
			mode: 'dedicated',
			status: dedicatedStatusFilter.value,
			customer_id: Number(dedicatedCustomerID.value || 0)
		})
		dedicatedPagination.current = Number(panel.orderList.page || page || 1)
		dedicatedPagination.pageSize = Number(panel.orderList.pageSize || pageSize || 12)
		if (!dedicatedGroupHeads.value.some((row) => Number(row.id) === Number(activeDedicatedHeadID.value || 0))) {
			activeDedicatedHeadID.value = Number(dedicatedGroupHeads.value[0]?.id || 0)
			dedicatedCheckResults.value = {}
		}
	} catch (err) {
		panel.setError(err)
	}
}

function selectDedicatedHead(orderID: number) {
	activeDedicatedHeadID.value = Number(orderID || 0)
	dedicatedCheckResults.value = {}
}

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

function residentialCredentialLinesCount(lines: string): number {
	if (!lines) return 0
	return lines
		.split('\n')
		.map((row) => row.trim())
			.filter((row) => row.length > 0).length
}

function expectedResidentialCredentialLinesCount(strategy: string, quantity: number): number {
	return String(strategy || 'per_line') === 'shared' ? 1 : Number(quantity || 0)
}

function residentialCredentialPlaceholder(strategy: string, isEdit = false): string {
	return String(strategy || 'per_line') === 'shared'
		? '只填 1 行 user:pass；会复用到全部不同 IP'
		: `每行 user:pass；行数必须等于${isEdit ? '最终' : '家宽'}数量`
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
    if (panel.activeTab === 'orders') {
      await loadOrdersView()
    } else if (panel.activeTab === 'delivery') {
      await loadDeliveryView()
    }
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
    if (panel.activeTab === 'orders') {
      await loadOrdersView()
    } else if (panel.activeTab === 'delivery') {
      await loadDeliveryView()
    } else if (panel.activeTab === 'dedicated') {
      await loadDedicatedView()
    }
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
			} else if (
				orderForm.residential_credential_mode === 'custom' &&
				residentialCredentialLinesCount(orderForm.residential_credential_lines) !== expectedResidentialCredentialLinesCount(orderForm.residential_credential_strategy, Number(orderForm.quantity || 0))
			) {
				message.warning(
					orderForm.residential_credential_strategy === 'shared'
						? '整单复用模式只需要填写 1 行 user:pass'
						: '指定凭据行数必须等于家宽数量'
				)
				return
			}
		const payload: Record<string, unknown> = {
			customer_id: Number(orderForm.customer_id),
			name: orderForm.name,
			duration_day: Number(orderForm.duration_day),
			expires_at: orderForm.expires_at ? new Date(orderForm.expires_at).toISOString() : '',
			mode: orderForm.mode,
				port: Number(orderForm.port),
				manual_ip_ids: orderForm.manual_ip_ids.map((v) => Number(v)),
				residential_credential_mode: orderForm.residential_credential_mode,
				residential_credential_strategy: orderForm.residential_credential_strategy,
				residential_credential_lines: orderForm.residential_credential_mode === 'custom' ? String(orderForm.residential_credential_lines || '') : ''
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
		orderForm.residential_credential_mode = 'random'
		orderForm.residential_credential_strategy = 'per_line'
		orderForm.residential_credential_lines = ''
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
	orderEditForm.residential_credential_mode = 'random'
	orderEditForm.residential_credential_strategy = 'per_line'
	orderEditForm.residential_credential_lines = ''
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
		if (
			orderEditForm.mode !== 'dedicated' &&
			orderEditForm.residential_credential_mode === 'custom' &&
			residentialCredentialLinesCount(orderEditForm.residential_credential_lines) !== expectedResidentialCredentialLinesCount(orderEditForm.residential_credential_strategy, Number(orderEditForm.quantity || 0))
		) {
			message.warning(
				orderEditForm.residential_credential_strategy === 'shared'
					? '整单复用模式只需要填写 1 行 user:pass'
					: '指定凭据行数必须等于家宽数量'
			)
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
				payload.residential_credential_mode = orderEditForm.residential_credential_mode
				payload.residential_credential_strategy = orderEditForm.residential_credential_strategy
				payload.residential_credential_lines = orderEditForm.residential_credential_mode === 'custom' ? String(orderEditForm.residential_credential_lines || '') : ''
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

async function activateOrder(orderID: number) {
	Modal.confirm({
		title: '启用订单',
		content: `确认启用订单 #${orderID} 吗？`,
		okText: '启用',
		onOk: async () => {
			try {
				await panel.activateOrder(orderID)
				message.success('订单已启用')
			} catch (err) {
				panel.setError(err)
				message.error(panel.error || '启用失败')
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
	openExportDialog(panel.orderSelection)
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

async function doBatchActivate() {
	if (panel.orderSelection.length === 0) return
	const disabledIDs = panel.orderSelection.filter((id) => {
		const row = panel.orders.find((item) => Number(item.id) === Number(id))
		return String(row?.status || '') === 'disabled'
	})
	if (disabledIDs.length === 0) {
		message.warning('所选订单没有可启用项（仅 disabled 可启用）')
		return
	}
	if (disabledIDs.length < panel.orderSelection.length) {
		message.warning('已自动跳过非 disabled 订单')
	}
	Modal.confirm({
		title: '确认批量启用',
		content: `将启用 ${disabledIDs.length} 个订单，是否继续？`,
		okText: '继续',
		cancelText: '取消',
		async onOk() {
			try {
				const results = await panel.batchActivate(disabledIDs)
				const ok = results.filter((r) => r.success).length
				const fail = results.length - ok
				message.success(`批量启用完成，成功 ${ok}，失败 ${fail}`)
				panel.orderSelection = []
			} catch (err) {
				panel.setError(err)
			}
		}
	})
}

function openExportDialog(orderIDs: number[]) {
	const ids = Array.from(new Set(orderIDs.map((id) => Number(id)).filter((id) => id > 0)))
	if (ids.length === 0) return
	const rows = panel.orders.filter((row) => ids.includes(Number(row.id)))
	exportDialogOrderIDs.value = ids
	exportDialogOrderID.value = ids.length === 1 ? ids[0] : null
	exportDialogContainsResidential.value = rows.some((row) => isResidentialMode(String(row.mode || '')))
	exportDialogContainsDedicated.value = rows.some((row) => String(row.mode || '') === 'dedicated')
	exportDialogFormat.value = 'xlsx'
	exportDialogResidentialTXTLayout.value = 'uri'
	exportDialogIncludeRawSocks5.value = false
	exportDialogOpen.value = true
}

const exportDialogTitle = computed(() => {
	if (exportDialogOrderIDs.value.length > 1) {
		return `批量导出 ${exportDialogOrderIDs.value.length} 个订单`
	}
	if (exportDialogOrderID.value) {
		return `导出订单 #${exportDialogOrderID.value}`
	}
	return '导出订单'
})

function closeExportDialog() {
	if (exportDialogSubmitting.value) return
	exportDialogOpen.value = false
}

async function submitExportDialog() {
	if (exportDialogOrderIDs.value.length === 0) return
	try {
		exportDialogSubmitting.value = true
		if (exportDialogOrderID.value && exportDialogOrderIDs.value.length === 1) {
			await downloadOrderExport(
				exportDialogOrderID.value,
				exportDialogFormat.value,
				exportDialogResidentialTXTLayout.value,
				exportDialogIncludeRawSocks5.value
			)
		} else {
			const res = await panel.batchExport(
				exportDialogOrderIDs.value,
				exportDialogFormat.value,
				exportDialogResidentialTXTLayout.value,
				exportDialogIncludeRawSocks5.value
			)
			const header = String(res.headers?.['content-disposition'] || '')
			const fallbackExt = exportDialogFormat.value === 'txt' ? 'txt' : 'zip'
			const filename = parseContentDispositionFilename(header, `orders-batch-${Date.now()}.${fallbackExt}`)
			downloadBlobFile(res.data, filename)
		}
		exportDialogOpen.value = false
	} catch (err) {
		panel.setError(err)
	} finally {
		exportDialogSubmitting.value = false
	}
}

async function downloadOrderExport(orderID: number, format: 'txt' | 'xlsx', residentialTXTLayout: 'uri' | 'colon', includeRawSocks5 = false) {
	try {
		exportingOrderID.value = orderID
		const params = new URLSearchParams()
		if (Number(exportCount.value) > 0) {
			params.set('count', String(Number(exportCount.value)))
		}
		params.set('format', format)
		params.set('residential_txt_layout', residentialTXTLayout)
		if (includeRawSocks5) {
			params.set('include_raw_socks5', 'true')
		}
		params.set('shuffle', 'false')
		const query = params.toString()
		const res = await http.get(`/api/orders/${orderID}/export${query ? `?${query}` : ''}`, { responseType: 'blob' })
		const header = String(res.headers['content-disposition'] || '')
		const ext = format === 'txt' ? 'txt' : 'xlsx'
		const filename = parseContentDispositionFilename(header, `order-${orderID}-export.${ext}`)
		downloadBlobFile(res.data, filename)
	} catch (err) {
		panel.setError(err)
	} finally {
		exportingOrderID.value = null
	}
}

function exportOrder(orderID: number) {
	openExportDialog([orderID])
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
	const activityID = `stream-test:${orderID}:${Date.now()}`
	panel.startTrackedActivity(activityID, '流式测活', `订单 #${orderID} 正在流式测活`)
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
		panel.finishTrackedActivity(activityID, 'success', `订单 #${orderID} 流式测活完成，成功 ${streamMeta.success}，失败 ${streamMeta.failed}`)
		message.success('流式测活完成')
	} catch (err) {
		panel.setError(err)
		panel.finishTrackedActivity(activityID, 'error', `订单 #${orderID} 流式测活失败: ${panel.error || 'unknown error'}`)
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

function dedicatedProbeProtocol(order: Order): string {
	const raw = String(order.dedicated_protocol || 'mixed').trim().toLowerCase()
	if (raw === 'vmess') return 'VMESS'
	if (raw === 'vless') return 'VLESS'
	if (raw === 'shadowsocks') return 'SHADOWSOCKS'
	return 'SOCKS5_MIXED'
}

async function runDedicatedGroupProtocolCheck(orderID = Number(activeDedicatedHeadID.value || 0)) {
	if (dedicatedCheckRunning.value) return
	const head = panel.orders.find((row) => Number(row.id) === Number(orderID))
	if (!head) {
		message.warning('请先选择专线组头订单')
		return
	}
	const children = panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(orderID))
		.sort((a, b) => Number((a as any).sequence_no || 0) - Number((b as any).sequence_no || 0) || Number(a.id) - Number(b.id))
	if (!children.length) {
		message.warning('当前组头没有可探测的子订单')
		return
	}
	activeDedicatedHeadID.value = Number(orderID)
	dedicatedCheckRunning.value = true
	dedicatedCheckResults.value = {}
	const activityID = `dedicated-check:${orderID}:${Date.now()}`
	panel.startTrackedActivity(activityID, '专线 XrayCore 探测', `组头 #${orderID}，准备探测 ${children.length} 个子订单`)
	const queue = [...children]
	const workers = Array.from({ length: Math.min(4, queue.length) }, async () => {
		while (queue.length > 0) {
			const child = queue.shift()
			if (!child) return
			const item = child.items?.[0]
			if (!item) {
				dedicatedCheckResults.value[child.id] = { ok: false, message: '子订单缺少凭据' }
				continue
			}
			const host = String(child.dedicated_ingress?.domain || child.dedicated_entry?.domain || item.ip || '').trim()
			const port = dedicatedCopyPort(child)
			if (!host || port <= 0) {
				dedicatedCheckResults.value[child.id] = { ok: false, message: '入口域名或端口无效' }
				continue
			}
			try {
				const result = await panel.checkDedicatedRuntime({
					protocol: dedicatedProbeProtocol(child),
					ip: host,
					port,
					username: String(item.username || ''),
					password: String(item.password || ''),
					vmessUuid: String(item.vmess_uuid || '')
				})
				dedicatedCheckResults.value[child.id] = {
					ok: Boolean(result.ok || result.connectivityOk),
					exitIp: String(result.exitIp || ''),
					countryCode: String(result.countryCode || ''),
					region: String(result.region || ''),
					message: String(result.message || ''),
					errorCode: String(result.errorCode || ''),
					checkedAt: String(result.checkedAt || '')
				}
			} catch (err) {
				panel.setError(err)
				dedicatedCheckResults.value[child.id] = {
					ok: false,
					message: panel.error || '探测失败'
				}
			}
		}
	})
	try {
		await Promise.all(workers)
		const results = children.map((child) => dedicatedCheckResults.value[child.id]).filter(Boolean)
		const failed = results.filter((row) => !row.ok).length
		const succeeded = results.length - failed
		if (failed === 0) {
			panel.finishTrackedActivity(activityID, 'success', `组头 #${orderID} 已完成 ${children.length} 个子订单的 XrayCore 探测`)
			message.success(`XrayCore 探测完成，共 ${children.length} 个子订单`)
		} else if (succeeded === 0) {
			panel.finishTrackedActivity(activityID, 'error', `组头 #${orderID} XrayCore 探测全部失败，共 ${failed} 个子订单`)
			message.error(`XrayCore 探测失败，共 ${failed} 个子订单失败`)
		} else {
			panel.finishTrackedActivity(activityID, 'warning', `组头 #${orderID} XrayCore 探测部分失败，成功 ${succeeded}，失败 ${failed}`)
			message.warning(`XrayCore 探测部分失败，成功 ${succeeded}，失败 ${failed}`)
		}
	} catch (err) {
		panel.setError(err)
		panel.finishTrackedActivity(activityID, 'error', `组头 #${orderID} XrayCore 探测失败: ${panel.error || 'unknown error'}`)
	} finally {
		dedicatedCheckRunning.value = false
	}
}

async function copyOrderLines(order: Order) {
	const lines = deliveryLinesForOrder(order)
	if (lines.length === 0) {
		message.warning('当前订单没有可复制的发货内容')
		return
	}
	await navigator.clipboard.writeText(lines.join('\n'))
	message.success('发货内容已复制')
}

function copyLinksLabel(order: Order) {
	if (order.is_group_head) return '批量复制链接'
	return '复制链接'
}

async function copyOrderLinks(order: Order) {
	if (String(order.mode || '') !== 'dedicated') {
		message.warning('仅专线订单支持复制链接')
		return
	}
	try {
		copyingLinksOrderID.value = Number(order.id)
		const lines = await panel.copyOrderLinks(Number(order.id))
		if (!String(lines || '').trim()) {
			message.warning('当前订单没有可复制的链接')
			return
		}
		await navigator.clipboard.writeText(lines)
		message.success(order.is_group_head ? '整组链接已复制' : '链接已复制')
	} catch (err) {
		panel.setError(err)
	} finally {
		copyingLinksOrderID.value = null
	}
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
	applyDedicatedInboundRow(dedicatedInboundForm as typeof dedicatedInboundEditForm, createDedicatedInboundDefaults() as Record<string, any>)
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
		await panel.createDedicatedInbound(buildDedicatedInboundPayload(dedicatedInboundForm))
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
	applyDedicatedInboundRow(dedicatedInboundEditForm, row as Record<string, any>)
	dedicatedInboundEditOpen.value = true
}

async function saveDedicatedInboundEdit() {
	try {
		await panel.updateDedicatedInbound(Number(dedicatedInboundEditForm.id), buildDedicatedInboundPayload(dedicatedInboundEditForm))
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
	activeDedicatedHeadID.value = Number(orderID)
	groupEditorHeadOrderID.value = orderID
	groupEditorChildOrderIDs.value = panel.orders
		.filter((row) => Number((row as any).parent_order_id || 0) === Number(orderID))
		.map((row) => Number(row.id))
	dedicatedCheckResults.value = {}
	panel.activeTab = 'dedicated'
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
	const activityID = `dedicated-egress-probe:${Date.now()}`
	panel.startTrackedActivity(activityID, '专线出口探测', '正在并发探测专线出口可用性')
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
		panel.finishTrackedActivity(activityID, 'success', `专线出口探测完成，可用 ${dedicatedProbeMeta.success}，失败 ${dedicatedProbeMeta.failed}`)
		message.success('专线出口探测完成')
	} catch (err) {
		panel.setError(err)
		panel.finishTrackedActivity(activityID, 'error', `专线出口探测失败: ${panel.error || 'unknown error'}`)
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
			gosealight_telemetry_enabled: panel.settings.gosealight_telemetry_enabled === 'true' ? 'true' : 'false',
			gosealight_base_url: panel.settings.gosealight_base_url || '',
			gosealight_node_id: panel.settings.gosealight_node_id || '',
			gosealight_node_username: panel.settings.gosealight_node_username || '',
			gosealight_node_password: panel.settings.gosealight_node_password || '',
			gosealight_telemetry_interval_seconds: panel.settings.gosealight_telemetry_interval_seconds || '60',
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

    <AppShell
      v-else
      :active-tab="panel.activeTab"
      :menu-items="menuItems"
      :release-version-text="releaseVersionText"
      :notice="panel.notice"
      :error="panel.error"
      :pending-requests="panel.pendingRequests"
      :running-activity-count="panel.runningActivityCount"
      :active-requests="activeRequests"
      :recent-activities="panel.recentActivities"
      :task-logs="panel.taskLogs"
      @menu-click="onMenuClick"
      @refresh="refreshAll"
      @logout="doLogout"
    >
      <DashboardPage
        v-if="panel.activeTab === 'dashboard'"
        :health-cards="healthCards"
        :oversell-customer-i-d="oversellCustomerID"
        :panel="panel"
        :oversell-columns="oversellColumns"
        :log-filters="logFilters"
        :change-oversell-view="changeOversellView"
        :bps-text="bpsText"
        :bytes-text="bytesText"
        :format-time="formatTime"
        :apply-log-filter="applyLogFilter"
        :reset-log-filter="resetLogFilter"
      />

      <CustomersPage
        v-else-if="panel.activeTab === 'customers'"
        :customer-form="customerForm"
        :panel="panel"
        :customer-columns="customerColumns"
        :create-customer="createCustomer"
        :open-customer-edit="openCustomerEdit"
        :delete-customer="deleteCustomer"
      />

      <HostIPsPage
        v-else-if="panel.activeTab === 'ips'"
        :probe-form="probeForm"
        :probe-result="probeResult"
        :panel="panel"
        :host-columns="hostColumns"
        :probe-port="probePort"
      />

      <OrdersPage
        v-else-if="panel.activeTab === 'orders'"
        :panel="panel"
        :order-search-keyword="orderSearchKeyword"
        :order-customer-i-d="orderCustomerID"
        :order-mode-filter="orderModeFilter"
        :order-status-filter="orderStatusFilter"
        :test-sample-percent="testSamplePercent"
        :export-count="exportCount"
        :batch-more-days="batchMoreDays"
        :batch-renew-expires-at="batchRenewExpiresAt"
        :filtered-order-rows="filteredOrderRows"
        :row-selection="rowSelection"
        :order-pagination="orderPagination"
        :exporting-order-i-d="exportingOrderID"
        :copying-links-order-i-d="copyingLinksOrderID"
        :testing-order-i-d="testingOrderID"
        :test-result="testResult"
        :batch-test-result="batchTestResult"
        :orders-columns="ordersColumns"
        :load-orders-view="loadOrdersView"
        :open-order-detail="openOrderDetail"
        :open-order-edit-smart="openOrderEditSmart"
        :export-order="exportOrder"
        :copy-order-links="copyOrderLinks"
        :copy-links-label="copyLinksLabel"
        :test-order="testOrder"
        :renew-order="renewOrder"
        :stream-test-order="streamTestOrder"
        :reset-order-credentials="resetOrderCredentials"
        :split-order-head="splitOrderHead"
        :open-group-editor="openGroupEditor"
        :open-group-geo-modal="openGroupGeoModal"
        :open-group-socks-modal="openGroupSocksModal"
        :open-group-cred-modal="openGroupCredModal"
        :open-group-renew-modal="openGroupRenewModal"
        :remove-order="removeOrder"
        :activate-order="activateOrder"
        :deactivate-order="deactivateOrder"
        :status-color="statusColor"
        :mode-color="modeColor"
        :mode-label="modeLabel"
        :dedicated-summary="dedicatedSummary"
        :forward-summary="forwardSummary"
        :expires-hint="expiresHint"
        :format-time="formatTime"
        :do-batch-renew="doBatchRenew"
        :do-batch-resync="doBatchResync"
        :do-batch-test="doBatchTest"
        :do-batch-export="doBatchExport"
        :do-batch-activate="doBatchActivate"
        :do-batch-deactivate="doBatchDeactivate"
        @update:orderSearchKeyword="orderSearchKeyword = $event"
        @update:orderCustomerID="orderCustomerID = $event"
        @update:orderModeFilter="orderModeFilter = $event"
        @update:orderStatusFilter="orderStatusFilter = $event"
        @update:testSamplePercent="testSamplePercent = $event"
        @update:exportCount="exportCount = $event"
        @update:batchMoreDays="batchMoreDays = $event"
        @update:batchRenewExpiresAt="batchRenewExpiresAt = $event"
      />

      <DeliveryPage
        v-else-if="panel.activeTab === 'delivery'"
        :order-form="orderForm"
        :panel="panel"
        :dedicated-protocol-options="dedicatedProtocolOptions"
        :manual-host-i-p-options="manualHostIPOptions"
        :filtered-dedicated-inbounds-for-create="filteredDedicatedInboundsForCreate"
        :filtered-dedicated-ingresses-for-create="filteredDedicatedIngressesForCreate"
        :dedicated-probe-running="dedicatedProbeRunning"
        :dedicated-probe-meta="dedicatedProbeMeta"
        :dedicated-probe-rows="dedicatedProbeRows"
        :creating-order="creatingOrder"
        :delivery-search-keyword="deliverySearchKeyword"
        :delivery-customer-i-d="deliveryCustomerID"
        :delivery-mode="deliveryMode"
        :delivery-rows="deliveryRows"
        :delivery-pagination="deliveryPagination"
        :exporting-order-i-d="exportingOrderID"
        :copying-links-order-i-d="copyingLinksOrderID"
        :format-time="formatTime"
        :export-order="exportOrder"
        :set-quick-expiry="setQuickExpiry"
        :dedicated-lines-count="dedicatedLinesCount"
        :residential-credential-lines-count="residentialCredentialLinesCount"
        :residential-credential-placeholder="residentialCredentialPlaceholder"
        :download-dedicated-create-sample="downloadDedicatedCreateSample"
        :probe-dedicated-create-lines="probeDedicatedCreateLines"
        :create-order="createOrder"
        :copy-order-lines="copyOrderLines"
        :copy-order-links="copyOrderLinks"
        :copy-links-label="copyLinksLabel"
        :reset-order-credentials="resetOrderCredentials"
        :remove-order="removeOrder"
        :load-delivery-view="loadDeliveryView"
        @update:deliverySearchKeyword="deliverySearchKeyword = $event"
        @update:deliveryCustomerID="deliveryCustomerID = $event"
        @update:deliveryMode="deliveryMode = $event"
      />

      <DedicatedWorkbenchPage
        v-else-if="panel.activeTab === 'dedicated'"
        :panel="panel"
        :dedicated-search-keyword="dedicatedSearchKeyword"
        :dedicated-customer-i-d="dedicatedCustomerID"
        :dedicated-status-filter="dedicatedStatusFilter"
        :dedicated-pagination="dedicatedPagination"
        :dedicated-group-heads="dedicatedGroupHeads"
        :active-dedicated-head-i-d="activeDedicatedHeadID"
        :active-dedicated-head="activeDedicatedHead"
        :active-dedicated-children="activeDedicatedChildren"
        :dedicated-check-running="dedicatedCheckRunning"
        :dedicated-check-results="dedicatedCheckResults"
        :load-dedicated-view="loadDedicatedView"
        :select-dedicated-head="selectDedicatedHead"
        :open-order-edit="openOrderEdit"
        :open-group-socks-modal="openGroupSocksModal"
        :open-group-cred-modal="openGroupCredModal"
        :open-group-geo-modal="openGroupGeoModal"
        :open-group-renew-modal="openGroupRenewModal"
        :export-order="exportOrder"
        :copy-order-links="copyOrderLinks"
        :copy-links-label="copyLinksLabel"
        :run-dedicated-group-protocol-check="runDedicatedGroupProtocolCheck"
        :format-time="formatTime"
        :expires-hint="expiresHint"
        @update:dedicatedSearchKeyword="dedicatedSearchKeyword = $event"
        @update:dedicatedCustomerID="dedicatedCustomerID = $event"
        @update:dedicatedStatusFilter="dedicatedStatusFilter = $event"
      />

      <ImportPage
        v-else-if="panel.activeTab === 'import'"
        :panel="panel"
        :import-form="importForm"
        :singbox-selected-files="singboxSelectedFiles"
        :all-singbox-selected="allSingboxSelected"
        :selectable-singbox-files="selectableSingboxFiles"
        :previewing-singbox-import="previewingSingboxImport"
        :previewing-import="previewingImport"
        :confirming-import="confirmingImport"
        :import-preview-valid="importPreviewValid"
        :node-form="nodeForm"
        :node-columns="nodeColumns"
        :import-columns="importColumns"
        :migration-columns="migrationColumns"
        :set-import-expiry-days="setImportExpiryDays"
        :toggle-singbox-select-all="toggleSingboxSelectAll"
        :scan-singbox-configs="scanSingboxConfigs"
        :preview-selected-singbox-files="previewSelectedSingboxFiles"
        :preview-import="previewImport"
        :preview-cross-node-migration="previewCrossNodeMigration"
        :confirm-import="confirmImport"
        :create-node="createNode"
        :remove-node="removeNode"
        :migration-state-color="migrationStateColor"
        @update:singboxSelectedFiles="singboxSelectedFiles = $event"
      />

      <SettingsPage
        v-else-if="panel.activeTab === 'settings'"
        :panel="panel"
        :backup-columns="backupColumns"
        :save-settings="saveSettings"
        :send-bark-test="sendBarkTest"
        :open-dedicated-manager="openDedicatedManager"
        :export-backup-direct="exportBackupDirect"
        :create-backup="createBackup"
        :bytes-text="bytesText"
        :format-time="formatTime"
        :download-backup="downloadBackup"
        :restore-backup="restoreBackup"
        :delete-backup="deleteBackup"
      />
    </AppShell>

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
				  <template v-if="dedicatedInboundCreateIsVless">
					<a-select v-model:value="dedicatedInboundForm.vless_security" style="width:100%" placeholder="VLESS 安全">
					  <a-select-option v-for="opt in dedicatedVlessSecurityOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</a-select-option>
					</a-select>
					<a-input v-model:value="dedicatedInboundForm.vless_flow" placeholder="Flow，可选，如 xtls-rprx-vision" />
					<a-input v-model:value="dedicatedInboundForm.vless_type" placeholder="传输类型，如 tcp / ws / grpc / xhttp" />
					<a-input v-model:value="dedicatedInboundForm.vless_sni" placeholder="SNI，可选" />
					<a-input v-model:value="dedicatedInboundForm.vless_fingerprint" placeholder="uTLS 指纹，如 chrome" />
					<a-input v-model:value="dedicatedInboundForm.vless_host" placeholder="Host，可选；ws/xhttp/httpupgrade 常用" />
					<a-input v-model:value="dedicatedInboundForm.vless_path" placeholder="Path / ServiceName，可选" />
					<template v-if="dedicatedInboundCreateUsesTLS">
					  <a-input v-model:value="dedicatedInboundForm.vless_tls_cert_file" placeholder="TLS 证书文件路径，如 /etc/xray/cert.pem" />
					  <a-input v-model:value="dedicatedInboundForm.vless_tls_key_file" placeholder="TLS 私钥文件路径，如 /etc/xray/key.pem" />
					</template>
					<template v-if="dedicatedInboundCreateUsesReality">
					  <a-space style="width:100%">
						<a-switch :checked="dedicatedInboundForm.reality_show" @change="(v:boolean)=>dedicatedInboundForm.reality_show=v" />
						<span class="text-xs text-slate-500">REALITY 调试输出</span>
					  </a-space>
					  <a-space>
						<a-button size="small" @click="fillRealityKeyPair(dedicatedInboundForm)">自动生成密钥</a-button>
						<a-button size="small" @click="validateDedicatedInboundConfig(dedicatedInboundForm)">校验参数</a-button>
					  </a-space>
					  <a-input v-model:value="dedicatedInboundForm.reality_target" placeholder="Target，如 www.tesla.com:443" />
					  <a-input v-model:value="dedicatedInboundForm.reality_server_names" placeholder="Server Names，逗号分隔；留空默认取 SNI" />
					  <a-input v-model:value="dedicatedInboundForm.reality_private_key" placeholder="REALITY 私钥（x25519）" />
					  <a-input :value="dedicatedInboundForm.reality_public_key" readonly placeholder="公钥会由私钥自动生成" />
					  <a-input v-model:value="dedicatedInboundForm.reality_short_ids" placeholder="Short IDs，逗号分隔" />
					  <a-input v-model:value="dedicatedInboundForm.reality_spider_x" placeholder="SpiderX，可选，如 /" />
					  <a-space style="width:100%">
						<a-input-number v-model:value="dedicatedInboundForm.reality_xver" :min="0" :max="2" style="width:100%" placeholder="Xver" />
						<a-input-number v-model:value="dedicatedInboundForm.reality_max_time_diff" :min="0" style="width:100%" placeholder="Max Time Diff(ms)" />
					  </a-space>
					  <a-space style="width:100%">
						<a-input v-model:value="dedicatedInboundForm.reality_min_client_ver" placeholder="Min Client Ver，可选" />
						<a-input v-model:value="dedicatedInboundForm.reality_max_client_ver" placeholder="Max Client Ver，可选" />
					  </a-space>
					  <a-input v-model:value="dedicatedInboundForm.reality_mldsa65_seed" placeholder="mldsa65 Seed，可选" />
					  <a-input v-model:value="dedicatedInboundForm.reality_mldsa65_verify" placeholder="mldsa65 Verify，可选" />
					</template>
				  </template>
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
			  <a-table-column title="协议" key="protocol" width="180">
				<template #default="{ record }">
				  <span>{{ record.protocol }}</span>
				  <span v-if="record.protocol === 'vless'" class="text-xs text-slate-500"> / {{ record.vless_security || 'none' }}</span>
				</template>
			  </a-table-column>
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
				<a-button size="small" :loading="exportingOrderID===panel.selectedOrder.id" @click="exportOrder(panel.selectedOrder.id)">提取导出</a-button>
				<a-button v-if="panel.selectedOrder.mode === 'dedicated'" size="small" :loading="copyingLinksOrderID===panel.selectedOrder.id" @click="copyOrderLinks(panel.selectedOrder)">{{ copyLinksLabel(panel.selectedOrder) }}</a-button>
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
		        <template v-if="orderEditForm.mode !== 'forward' && orderEditForm.mode !== 'dedicated'">
		          <a-form-item label="家宽账号策略">
		            <a-radio-group v-model:value="orderEditForm.residential_credential_mode">
		              <a-radio-button value="random">保持现状/新增随机</a-radio-button>
		              <a-radio-button value="custom">指定 User:Pass</a-radio-button>
		            </a-radio-group>
		          </a-form-item>
		          <a-form-item v-if="orderEditForm.residential_credential_mode === 'custom'" label="指定家宽凭据">
		            <a-radio-group v-model:value="orderEditForm.residential_credential_strategy" class="mb-2">
		              <a-radio-button value="per_line">按顺序逐条指定</a-radio-button>
		              <a-radio-button value="shared">整单共用 1 组</a-radio-button>
		            </a-radio-group>
		            <a-textarea
		              v-model:value="orderEditForm.residential_credential_lines"
		              :rows="4"
		              :placeholder="residentialCredentialPlaceholder(orderEditForm.residential_credential_strategy, true)"
		            />
		            <div class="mt-1 text-xs text-slate-500">
		              {{ orderEditForm.residential_credential_strategy === 'shared'
		                ? '保存时会把同一组 User:Pass 复用到当前订单全部不同 IP，并校验全局用户名唯一。'
		                : '保存时会按当前订单项顺序整体改写凭据，并校验全局用户名唯一。' }}
		            </div>
		          </a-form-item>
		        </template>
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
		<template v-if="dedicatedInboundEditIsVless">
		  <a-form-item label="VLESS 安全">
			<a-select v-model:value="dedicatedInboundEditForm.vless_security" style="width:100%">
			  <a-select-option v-for="opt in dedicatedVlessSecurityOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</a-select-option>
			</a-select>
		  </a-form-item>
		  <a-form-item label="Flow"><a-input v-model:value="dedicatedInboundEditForm.vless_flow" /></a-form-item>
		  <a-form-item label="传输类型"><a-input v-model:value="dedicatedInboundEditForm.vless_type" placeholder="tcp / ws / grpc / xhttp" /></a-form-item>
		  <a-form-item label="SNI"><a-input v-model:value="dedicatedInboundEditForm.vless_sni" /></a-form-item>
		  <a-form-item label="uTLS 指纹"><a-input v-model:value="dedicatedInboundEditForm.vless_fingerprint" placeholder="chrome" /></a-form-item>
		  <a-form-item label="Host"><a-input v-model:value="dedicatedInboundEditForm.vless_host" /></a-form-item>
		  <a-form-item label="Path / ServiceName"><a-input v-model:value="dedicatedInboundEditForm.vless_path" /></a-form-item>
		  <template v-if="dedicatedInboundEditUsesTLS">
			<a-form-item label="TLS 证书文件"><a-input v-model:value="dedicatedInboundEditForm.vless_tls_cert_file" /></a-form-item>
			<a-form-item label="TLS 私钥文件"><a-input v-model:value="dedicatedInboundEditForm.vless_tls_key_file" /></a-form-item>
		  </template>
		  <template v-if="dedicatedInboundEditUsesReality">
			<a-form-item>
			  <a-switch :checked="dedicatedInboundEditForm.reality_show" @change="(v:boolean)=>dedicatedInboundEditForm.reality_show=v" />
			  <span class="ml-2 text-xs text-slate-500">REALITY 调试输出</span>
			</a-form-item>
			<a-form-item>
			  <a-space>
				<a-button size="small" @click="fillRealityKeyPair(dedicatedInboundEditForm)">自动生成密钥</a-button>
				<a-button size="small" @click="validateDedicatedInboundConfig(dedicatedInboundEditForm)">校验参数</a-button>
			  </a-space>
			</a-form-item>
			<a-form-item label="Target"><a-input v-model:value="dedicatedInboundEditForm.reality_target" /></a-form-item>
			<a-form-item label="Server Names"><a-input v-model:value="dedicatedInboundEditForm.reality_server_names" placeholder="逗号分隔；留空默认取 SNI" /></a-form-item>
			<a-form-item label="私钥"><a-input v-model:value="dedicatedInboundEditForm.reality_private_key" /></a-form-item>
			<a-form-item label="公钥"><a-input :value="dedicatedInboundEditForm.reality_public_key" readonly /></a-form-item>
			<a-form-item label="Short IDs"><a-input v-model:value="dedicatedInboundEditForm.reality_short_ids" placeholder="逗号分隔" /></a-form-item>
			<a-form-item label="SpiderX"><a-input v-model:value="dedicatedInboundEditForm.reality_spider_x" /></a-form-item>
			<a-form-item label="Xver / Max Time Diff(ms)">
			  <a-space style="width:100%">
				<a-input-number v-model:value="dedicatedInboundEditForm.reality_xver" :min="0" :max="2" style="width:100%" />
				<a-input-number v-model:value="dedicatedInboundEditForm.reality_max_time_diff" :min="0" style="width:100%" />
			  </a-space>
			</a-form-item>
			<a-form-item label="客户端版本限制">
			  <a-space style="width:100%">
				<a-input v-model:value="dedicatedInboundEditForm.reality_min_client_ver" placeholder="Min" />
				<a-input v-model:value="dedicatedInboundEditForm.reality_max_client_ver" placeholder="Max" />
			  </a-space>
			</a-form-item>
			<a-form-item label="mldsa65 Seed"><a-input v-model:value="dedicatedInboundEditForm.reality_mldsa65_seed" /></a-form-item>
			<a-form-item label="mldsa65 Verify"><a-input v-model:value="dedicatedInboundEditForm.reality_mldsa65_verify" /></a-form-item>
		  </template>
		</template>
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

	<a-modal
		v-model:open="exportDialogOpen"
		:title="exportDialogTitle"
		:confirm-loading="exportDialogSubmitting"
		ok-text="开始下载"
		cancel-text="取消"
		@ok="submitExportDialog"
		@cancel="closeExportDialog"
	>
	  <a-form layout="vertical">
		<a-form-item label="导出格式">
		  <a-radio-group v-model:value="exportDialogFormat">
			<a-radio-button value="xlsx">XLSX</a-radio-button>
			<a-radio-button value="txt">TXT</a-radio-button>
		  </a-radio-group>
		</a-form-item>
		<a-form-item v-if="exportDialogContainsResidential && exportDialogFormat === 'txt'" label="家宽 TXT 格式">
		  <a-radio-group v-model:value="exportDialogResidentialTXTLayout">
			<a-radio value="uri">socks5://user:pass@ip:port</a-radio>
			<a-radio value="colon">ip:port:user:pass</a-radio>
		  </a-radio-group>
		</a-form-item>
		<a-form-item v-if="exportDialogContainsDedicated" label="专线附带出口 Socks5">
		  <a-switch v-model:checked="exportDialogIncludeRawSocks5" />
		</a-form-item>
		<a-alert
			v-if="exportDialogFormat === 'xlsx'"
			type="info"
			show-icon
			:message="exportDialogContainsDedicated && exportDialogIncludeRawSocks5
				? '专线 XLSX 将包含订单号、协议链接、二维码、到期时间，并附带出口 Socks5 列。'
				: '专线 XLSX 将包含订单号、协议链接、二维码和到期时间。'"
		/>
		<a-alert
			v-else
			class="mt-2"
			type="info"
			show-icon
			:message="exportDialogContainsDedicated
				? '专线 TXT 现在支持 Socks5 专线按上方格式直接导出，也可以选择是否附带出口 Socks5；家宽 TXT 按上方格式输出。'
				: 'TXT 将按上方格式输出。'"
		/>
	  </a-form>
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

.logo-version {
  margin-top: 4px;
  font-size: 11px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.82);
  letter-spacing: 0;
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
  gap: 12px;
  flex-wrap: wrap;
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
	.layout-header {
		align-items: flex-start;
	}

	.title {
		font-size: 18px;
	}

	.ip-grid {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}
}
</style>
