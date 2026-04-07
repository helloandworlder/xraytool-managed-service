export interface Customer {
  id: number
  name: string
  code: string
  contact: string
  notes: string
  status: string
}

export interface HostIP {
  id: number
  ip: string
  is_public: boolean
  is_local: boolean
  enabled: boolean
}

export interface OrderItem {
  id: number
  order_id: number
  host_ip_id?: number
  ip: string
  port: number
  username: string
  password: string
  vmess_uuid?: string
  outbound_type?: string
  socks_outbound_id?: number
  forward_address?: string
  forward_port?: number
  forward_username?: string
  forward_password?: string
  managed: boolean
  status: string
  resources?: Array<{
    inbound_tag: string
    outbound_tag: string
    rule_tag: string
  }>
}

export interface DedicatedEntry {
  id: number
  name: string
  domain: string
  mixed_port: number
  vmess_port: number
  vless_port: number
  shadowsocks_port: number
  priority: number
  features: string
  enabled: boolean
  notes: string
}

export interface DedicatedInbound {
  id: number
  name: string
  protocol: string
  listen_port: number
  priority: number
  enabled: boolean
  notes: string
  vless_security?: string
  vless_flow?: string
  vless_type?: string
  vless_sni?: string
  vless_host?: string
  vless_path?: string
  vless_fingerprint?: string
  vless_tls_cert_file?: string
  vless_tls_key_file?: string
  reality_show?: boolean
  reality_target?: string
  reality_server_names?: string
  reality_private_key?: string
  reality_public_key?: string
  reality_short_ids?: string
  reality_spider_x?: string
  reality_xver?: number
  reality_max_time_diff?: number
  reality_min_client_ver?: string
  reality_max_client_ver?: string
  reality_mldsa65_seed?: string
  reality_mldsa65_verify?: string
}

export interface DedicatedIngress {
  id: number
  dedicated_inbound_id: number
  dedicated_inbound?: DedicatedInbound
  name: string
  domain: string
  ingress_port: number
  country_code: string
  region: string
  priority: number
  enabled: boolean
  notes: string
}

export interface Order {
  id: number
  order_no?: string
  customer_id: number
  group_id?: number
  parent_order_id?: number
  is_group_head?: boolean
  sequence_no?: number
  dedicated_entry_id?: number
  dedicated_inbound_id?: number
  dedicated_ingress_id?: number
  dedicated_protocol?: string
  dedicated_entry?: DedicatedEntry
  dedicated_inbound?: DedicatedInbound
  dedicated_ingress?: DedicatedIngress
  customer?: Customer
  name: string
  mode: string
  status: string
  quantity: number
  port: number
  starts_at: string
  expires_at: string
  items: OrderItem[]
}

export interface OrderListStats {
  total: number
  active: number
  expired: number
  disabled: number
}

export interface OrderListResponse {
  rows: Order[]
  page: number
  page_size: number
  total: number
  stats: OrderListStats
}

export interface OrderListQuery {
  page?: number
  page_size?: number
  keyword?: string
  mode?: string
  status?: string
  customer_id?: number
}

export interface OversellRow {
  ip: string
  count: number
  total_active_count: number
  customer_active_count: number
  unique_customer_count: number
  oversold_count: number
  oversell_rate: number
  enabled: boolean
  is_public: boolean
  is_local: boolean
}

export interface AllocationPreview {
  pool_size: number
  used_by_customer: number
  available: number
}

export interface CustomerRuntimeStat {
  customer_id: number
  customer_name: string
  customer_code: string
  home_items: number
  dedicated_items: number
  home_online_clients: number
  dedicated_online_clients: number
  online_clients: number
  realtime_bps: number
  traffic_1h: number
  traffic_24h: number
  traffic_7d: number
  traffic_total: number
  updated_at: string
}

export interface OrderGroupRuntimeStat {
  group_id: number
  group_order_no: string
  group_name: string
  customer_id: number
  customer_name: string
  customer_code: string
  mode: string
  order_count: number
  active_items: number
  online_clients: number
  realtime_bps: number
  traffic_1h: number
  traffic_24h: number
  traffic_7d: number
  traffic_total: number
  updated_at: string
}

export interface OrderRuntimeStat {
  order_id: number
  order_no: string
  order_name: string
  group_id: number
  group_order_no: string
  customer_id: number
  customer_name: string
  customer_code: string
  mode: string
  quantity: number
  active_items: number
  online_clients: number
  realtime_bps: number
  traffic_1h: number
  traffic_24h: number
  traffic_7d: number
  traffic_total: number
  updated_at: string
}

export interface RuntimeOverviewStat {
  customers: CustomerRuntimeStat[]
  groups: OrderGroupRuntimeStat[]
  orders: OrderRuntimeStat[]
  warnings?: string[]
  updated_at: string
}

export interface ResidentialCredentialConflictMember {
  order_id: number
  order_no: string
  order_name: string
  customer_id: number
  customer_name: string
  customer_code: string
  mode: string
  ip: string
  expires_at: string
}

export interface ResidentialCredentialConflict {
  username: string
  order_count: number
  item_count: number
  affected_order_ids: number[]
  members: ResidentialCredentialConflictMember[]
}

export interface ImportPreviewRow {
  raw: string
  source_file?: string
  ip: string
  port: number
  username: string
  password: string
  is_local_ip: boolean
  port_occupied: boolean
  error?: string
}

export interface SingboxScanFile {
  path: string
  entry_count: number
  selectable: boolean
  error?: string
}

export interface SingboxScanResult {
  files: SingboxScanFile[]
  total_files: number
  total_entries: number
}

export interface XrayNode {
  id: number
  name: string
  base_url: string
  username: string
  password: string
  enabled: boolean
  is_local: boolean
}

export interface SocksMigrationPreviewRow {
  raw: string
  ip: string
  port: number
  username: string
  password: string
  node_id?: number
  node_name?: string
  state: 'ready' | 'blocked' | 'unmatched' | 'ambiguous' | 'invalid' | string
  reason?: string
}

export interface SocksMigrationNodeSummary {
  node_id?: number
  node_name: string
  is_local: boolean
  reachable: boolean
  assigned_count: number
  ready_count: number
  blocked: boolean
  port_conflicts?: number[]
  error?: string
  action_hint?: string
  highlight_color: 'red' | 'green' | string
}

export interface SocksMigrationPreviewResult {
  rows: SocksMigrationPreviewRow[]
  nodes: SocksMigrationNodeSummary[]
  ready_rows: number
  blocked_rows: number
  unmatched_rows: number
  ambiguous_rows: number
  invalid_rows: number
  blocked_node_count: number
  reachable_node_size: number
}

export interface ForwardOutbound {
  id: number
  name: string
  address: string
  port: number
  username: string
  password: string
  route_user: string
  exit_ip: string
  country_code: string
  enabled: boolean
  probe_status: string
  probe_error: string
  last_probed_at?: string
}

export interface TaskLog {
  id: number
  level: string
  message: string
  detail: string
  created_at: string
}

export interface BackupInfo {
  name: string
  size_bytes: number
  updated_at: string
}

export interface VersionInfo {
  version: string
  commit: string
  build_time: string
  protocolVersion: string
  capabilities: string[]
}

export interface DedicatedProtocolCheckResult {
  ok: boolean
  connectivityOk?: boolean
  exitIp?: string
  countryCode?: string
  region?: string
  message?: string
  error?: string
  errorCode?: string
  checkedAt?: string
}

export interface ActivityEntry {
  id: string
  title: string
  detail: string
  status: 'running' | 'success' | 'error' | 'warning' | 'info'
  created_at: string
  updated_at: string
  source?: string
}
