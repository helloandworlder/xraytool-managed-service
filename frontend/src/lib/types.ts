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
  managed: boolean
  status: string
  resources?: Array<{
    inbound_tag: string
    outbound_tag: string
    rule_tag: string
  }>
}

export interface Order {
  id: number
  customer_id: number
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
  ip: string
  port: number
  username: string
  password: string
  is_local_ip: boolean
  port_occupied: boolean
  error?: string
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
