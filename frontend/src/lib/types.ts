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

export interface Order {
  id: number
  customer_id: number
  group_id?: number
  parent_order_id?: number
  is_group_head?: boolean
  sequence_no?: number
  dedicated_entry_id?: number
  dedicated_entry?: DedicatedEntry
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
  online_clients: number
  realtime_bps: number
  traffic_1h: number
  traffic_24h: number
  traffic_7d: number
  updated_at: string
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
