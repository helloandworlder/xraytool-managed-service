import { useMemo, useState } from 'react'
import {
  ApiOutlined,
  CloudServerOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  ImportOutlined,
  LogoutOutlined,
  NodeIndexOutlined,
  ReloadOutlined,
  SettingOutlined,
  TeamOutlined
} from '@ant-design/icons'
import {
  ModalForm,
  ProCard,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable
} from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntApp, Button, Card, Descriptions, Form, Input, Layout, Menu, Modal, Popconfirm, Space, Statistic, Tag, Typography } from 'antd'
import { http, normalizeApiError, setAuthToken } from './lib/http'
import type { Customer, DedicatedInbound, DedicatedIngress, HostIP, ImportPreviewRow, Order, OrderListResponse, RuntimeSyncTask } from './lib/types'

type PageKey = 'dashboard' | 'orders' | 'delivery' | 'dedicated' | 'import' | 'customers' | 'ips' | 'nodes' | 'settings'
type XrayNode = { id: number; name: string; base_url: string; username: string; enabled: boolean; is_local: boolean }
type SettingsMap = Record<string, string>

const pageItems = [
  { key: 'dashboard', icon: <DashboardOutlined />, label: '总览' },
  { key: 'orders', icon: <DatabaseOutlined />, label: '订单' },
  { key: 'delivery', icon: <ApiOutlined />, label: '发货' },
  { key: 'dedicated', icon: <CloudServerOutlined />, label: '专线' },
  { key: 'import', icon: <ImportOutlined />, label: '导入' },
  { key: 'customers', icon: <TeamOutlined />, label: '客户' },
  { key: 'ips', icon: <CloudServerOutlined />, label: 'IP' },
  { key: 'nodes', icon: <NodeIndexOutlined />, label: '节点' },
  { key: 'settings', icon: <SettingOutlined />, label: '设置' }
]

function statusTag(status: string) {
  const color = status === 'active' || status === 'success' ? 'green' : status === 'failed' || status === 'expired' ? 'red' : status === 'running' ? 'blue' : 'default'
  return <Tag color={color}>{status || '-'}</Tag>
}

function formatTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function orderProxyLines(order?: Partial<Order> | null) {
  return (order?.items || [])
    .map((item) => {
      if (!item.ip || !item.port || !item.username || !item.password) return ''
      return `${item.ip}:${item.port}:${item.username}:${item.password}`
    })
    .filter(Boolean)
}

function proxyLinesFromPayload(payload: unknown) {
  const data = payload as { order?: Order; items?: Order['items'] }
  if (data.order) return orderProxyLines(data.order)
  if (Array.isArray(data.items)) return orderProxyLines({ items: data.items })
  return []
}

function downloadTextFile(fileName: string, text: string) {
  const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.click()
  URL.revokeObjectURL(url)
}

async function loadOrders(params: Record<string, unknown>) {
  const query = new URLSearchParams()
  query.set('page', String(params.current || 1))
  query.set('page_size', String(params.pageSize || 12))
  if (params.keyword) query.set('keyword', String(params.keyword))
  if (params.status && params.status !== 'all') query.set('status', String(params.status))
  if (params.mode && params.mode !== 'all') query.set('mode', String(params.mode))
  const res = await http.get<OrderListResponse>(`/api/orders?${query.toString()}`)
  return {
    data: res.data.rows || [],
    success: true,
    total: res.data.total || 0
  }
}

export default function App() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const [token, setToken] = useState(localStorage.getItem('xtool_token') || '')
  const [username, setUsername] = useState(localStorage.getItem('xtool_user') || '')
  const [page, setPage] = useState<PageKey>('dashboard')

  useMemo(() => setAuthToken(token), [token])

  const logout = () => {
    localStorage.removeItem('xtool_token')
    localStorage.removeItem('xtool_user')
    setAuthToken('')
    setToken('')
    setUsername('')
  }

  if (!token) {
    return <LoginView onLogin={(nextToken, nextUser) => {
      localStorage.setItem('xtool_token', nextToken)
      localStorage.setItem('xtool_user', nextUser)
      setAuthToken(nextToken)
      setToken(nextToken)
      setUsername(nextUser)
    }} />
  }

  return (
    <Layout className="app-layout">
      <Layout.Sider width={232} className="app-sider">
        <div className="brand">
          <div className="brand-title">XrayTool</div>
          <div className="brand-subtitle">节点运营面板</div>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[page]}
          items={pageItems}
          onClick={(item) => setPage(item.key as PageKey)}
        />
      </Layout.Sider>
      <Layout>
        <Layout.Header className="app-header">
          <Space direction="vertical" size={0}>
            <Typography.Text strong>{pageItems.find((item) => item.key === page)?.label}</Typography.Text>
            <Typography.Text type="secondary">{username}</Typography.Text>
          </Space>
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => queryClient.invalidateQueries()}>刷新</Button>
            <Button icon={<LogoutOutlined />} onClick={logout}>退出</Button>
          </Space>
        </Layout.Header>
        <Layout.Content className="app-content">
          {page === 'dashboard' && <DashboardPage />}
          {page === 'orders' && <OrdersPage />}
          {page === 'delivery' && <DeliveryPage />}
          {page === 'dedicated' && <DedicatedPage />}
          {page === 'import' && <ImportPage />}
          {page === 'customers' && <CustomersPage />}
          {page === 'ips' && <HostIPsPage />}
          {page === 'nodes' && <NodesPage />}
          {page === 'settings' && <SettingsPage />}
        </Layout.Content>
      </Layout>
    </Layout>
  )
}

function LoginView({ onLogin }: { onLogin: (token: string, username: string) => void }) {
  const { message } = AntApp.useApp()
  const [loading, setLoading] = useState(false)
  return (
    <div className="login-page">
      <Card className="login-card" title="XrayTool">
        <Form layout="vertical" onFinish={async (values) => {
          setLoading(true)
          try {
            const res = await http.post('/api/auth/login', values)
            onLogin(res.data.token, res.data.username)
          } catch (error) {
            message.error(normalizeApiError(error))
          } finally {
            setLoading(false)
          }
        }} initialValues={{ username: 'admin' }}>
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}>
            <Input autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Button block type="primary" htmlType="submit" loading={loading}>登录</Button>
        </Form>
      </Card>
    </div>
  )
}

function DashboardPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const runtime = useQuery({ queryKey: ['runtime-overview'], queryFn: async () => (await http.get('/api/runtime/overview?limit=20')).data })
  const syncTasks = useQuery({ queryKey: ['runtime-sync-tasks'], queryFn: async () => (await http.get<RuntimeSyncTask[]>('/api/runtime/sync-tasks?limit=20')).data })
  const customers = useQuery({ queryKey: ['customers'], queryFn: async () => (await http.get<Customer[]>('/api/customers')).data })
  const hostIPs = useQuery({ queryKey: ['host-ips'], queryFn: async () => (await http.get<HostIP[]>('/api/host-ips')).data })
  const orders = useQuery({ queryKey: ['orders-summary'], queryFn: async () => (await http.get<OrderListResponse>('/api/orders?page=1&page_size=1')).data })
  const failedTasks = (syncTasks.data || []).filter((task) => task.status === 'failed')
  const pendingTasks = (syncTasks.data || []).filter((task) => task.status === 'pending' || task.status === 'running')
  const retrySync = useMutation({
    mutationFn: (id: number) => http.post(`/api/runtime/sync-tasks/${id}/retry`, {}),
    onSuccess: async () => {
      message.success('同步任务已重新提交')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['runtime-sync-tasks'] }),
        queryClient.invalidateQueries({ queryKey: ['runtime-overview'] })
      ])
    },
    onError: (error) => message.error(normalizeApiError(error))
  })
  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <ProCard.Group gutter={16}>
        <ProCard loading={customers.isLoading}><Statistic title="客户" value={customers.data?.length || 0} /></ProCard>
        <ProCard loading={hostIPs.isLoading}><Statistic title="可用 IP" value={(hostIPs.data || []).filter((ip) => ip.enabled && ip.is_public).length} /></ProCard>
        <ProCard loading={orders.isLoading}><Statistic title="订单" value={orders.data?.stats?.total || orders.data?.total || 0} /></ProCard>
        <ProCard loading={syncTasks.isLoading}><Statistic title="同步中" value={pendingTasks.length} valueStyle={{ color: pendingTasks.length ? '#1677ff' : undefined }} /></ProCard>
        <ProCard loading={syncTasks.isLoading}><Statistic title="同步失败" value={failedTasks.length} valueStyle={{ color: failedTasks.length ? '#dc2626' : undefined }} /></ProCard>
      </ProCard.Group>
      <ProCard title="运行态" loading={runtime.isLoading}>
        <Descriptions column={4} size="small">
          <Descriptions.Item label="客户 Top">{runtime.data?.customers?.length || 0}</Descriptions.Item>
          <Descriptions.Item label="订单组">{runtime.data?.groups?.length || 0}</Descriptions.Item>
          <Descriptions.Item label="订单">{runtime.data?.orders?.length || 0}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{formatTime(runtime.data?.updated_at)}</Descriptions.Item>
        </Descriptions>
      </ProCard>
      <ProTable<RuntimeSyncTask>
        rowKey="id"
        headerTitle="运行态同步任务"
        search={false}
        options={{ density: false, fullScreen: false, setting: false }}
        pagination={{ pageSize: 8 }}
        loading={syncTasks.isLoading}
        dataSource={syncTasks.data || []}
        toolBarRender={() => [
          <Button key="refresh" icon={<ReloadOutlined />} onClick={() => void syncTasks.refetch()}>刷新</Button>
        ]}
        columns={[
          { title: '状态', dataIndex: 'status', render: (_, row) => statusTag(row.status) },
          { title: '原因', dataIndex: 'reason' },
          { title: '订单', dataIndex: 'order_id', render: (_, row) => row.order_id || '-' },
          { title: '尝试', dataIndex: 'attempts' },
          { title: '请求时间', dataIndex: 'requested_at', render: (_, row) => formatTime(row.requested_at) },
          { title: '错误', dataIndex: 'error', ellipsis: true },
          {
            title: '操作',
            valueType: 'option',
            render: (_, row) => row.status === 'failed'
              ? <Button size="small" loading={retrySync.isPending} onClick={() => retrySync.mutate(row.id)}>重试</Button>
              : null
          }
        ]}
      />
    </Space>
  )
}

function OrdersPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const syncTasks = useQuery({
    queryKey: ['runtime-sync-tasks', 'orders'],
    queryFn: async () => (await http.get<RuntimeSyncTask[]>('/api/runtime/sync-tasks?limit=200')).data
  })
  const latestSyncTaskByOrder = useMemo(() => {
    const rows = new Map<number, RuntimeSyncTask>()
    for (const task of syncTasks.data || []) {
      if (!task.order_id) continue
      const current = rows.get(task.order_id)
      const taskTime = new Date(task.requested_at || task.updated_at).getTime()
      const currentTime = current ? new Date(current.requested_at || current.updated_at).getTime() : 0
      if (!current || taskTime >= currentTime) rows.set(task.order_id, task)
    }
    return rows
  }, [syncTasks.data])
  const action = useMutation({
    mutationFn: async ({ id, op }: { id: number; op: 'renew' | 'activate' | 'deactivate' }) => {
      if (op === 'renew') return http.post(`/api/orders/${id}/renew`, { more_days: 30 })
      if (op === 'activate') return http.post(`/api/orders/${id}/activate`, {})
      return http.post(`/api/orders/${id}/deactivate`, {})
    },
    onSuccess: async () => {
      message.success('操作已提交')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['runtime-sync-tasks'] }),
        queryClient.invalidateQueries()
      ])
    },
    onError: (error) => message.error(normalizeApiError(error))
  })
  const retrySync = useMutation({
    mutationFn: (id: number) => http.post(`/api/runtime/sync-tasks/${id}/retry`, {}),
    onSuccess: async () => {
      message.success('同步任务已重新提交')
      await queryClient.invalidateQueries({ queryKey: ['runtime-sync-tasks'] })
    },
    onError: (error) => message.error(normalizeApiError(error))
  })
  return (
    <ProTable<Order>
      rowKey="id"
      request={loadOrders}
      search={{ labelWidth: 80 }}
      pagination={{ pageSize: 12 }}
      expandable={{
        rowExpandable: (row) => orderProxyLines(row).length > 0,
        expandedRowRender: (row) => {
          const lines = orderProxyLines(row)
          const text = lines.join('\n')
          return (
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space>
                <Button size="small" type="primary" onClick={async () => {
                  await navigator.clipboard.writeText(text)
                  message.success('已复制')
                }}>复制本单代理</Button>
                <Button size="small" onClick={() => downloadTextFile(`xraytool-order-${row.order_no || row.id}.txt`, text)}>下载 TXT</Button>
              </Space>
              <pre className="proxy-output">{text}</pre>
            </Space>
          )
        }
      }}
      columns={[
        { title: '关键词', dataIndex: 'keyword', hideInTable: true },
        { title: '状态', dataIndex: 'status', hideInTable: true, valueType: 'select', valueEnum: { all: '全部', active: 'active', expired: 'expired', disabled: 'disabled' } },
        { title: '模式', dataIndex: 'mode', hideInTable: true, valueType: 'select', valueEnum: { all: '全部', home: '家宽', dedicated: '专线' } },
        { title: '订单', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><strong>{row.name}</strong><Typography.Text type="secondary">{row.order_no || `#${row.id}`}</Typography.Text></Space> },
        { title: '客户', dataIndex: ['customer', 'name'], render: (_, row) => row.customer?.name || row.customer_id },
        { title: '模式', dataIndex: 'mode', render: (_, row) => <Tag color={row.mode === 'dedicated' ? 'blue' : 'green'}>{row.mode}</Tag> },
        { title: '状态', dataIndex: 'status', render: (_, row) => statusTag(row.status) },
        {
          title: '同步',
          width: 150,
          render: (_, row) => {
            const task = latestSyncTaskByOrder.get(row.id)
            if (!task) return <Tag>未记录</Tag>
            return (
              <Space direction="vertical" size={2}>
                {statusTag(task.status)}
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {task.attempts} 次 · {formatTime(task.requested_at)}
                </Typography.Text>
              </Space>
            )
          }
        },
        { title: '数量', dataIndex: 'quantity' },
        { title: '到期时间', dataIndex: 'expires_at', render: (_, row) => formatTime(row.expires_at) },
        {
          title: '操作',
          valueType: 'option',
          render: (_, row) => {
            const task = latestSyncTaskByOrder.get(row.id)
            return [
              <Button key="renew" size="small" onClick={() => action.mutate({ id: row.id, op: 'renew' })}>续期30天</Button>,
              row.status === 'disabled'
                ? <Button key="activate" size="small" onClick={() => action.mutate({ id: row.id, op: 'activate' })}>启用</Button>
                : <Button key="deactivate" size="small" danger onClick={() => action.mutate({ id: row.id, op: 'deactivate' })}>停用</Button>,
              task?.status === 'failed'
                ? <Button key="retry-sync" size="small" loading={retrySync.isPending} onClick={() => retrySync.mutate(task.id)}>重试同步</Button>
                : null
            ]
          }
        }
      ]}
    />
  )
}

function DeliveryPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const [createdLines, setCreatedLines] = useState<string[]>([])
  const customers = useQuery({ queryKey: ['customers'], queryFn: async () => (await http.get<Customer[]>('/api/customers')).data })
  const create = useMutation({
    mutationFn: (values: Record<string, unknown>) => http.post('/api/orders', values),
    onSuccess: (res) => {
      message.success('订单已提交，后台同步中')
      setCreatedLines(proxyLinesFromPayload(res.data))
      queryClient.invalidateQueries()
    },
    onError: (error) => message.error(normalizeApiError(error))
  })
  const createdText = createdLines.join('\n')
  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Modal
        title="本次代理"
        open={createdLines.length > 0}
        onCancel={() => setCreatedLines([])}
        footer={[
          <Button key="copy" type="primary" onClick={async () => {
            await navigator.clipboard.writeText(createdText)
            message.success('已复制')
          }}>复制全部</Button>,
          <Button key="download" onClick={() => downloadTextFile(`xraytool-order-${Date.now()}.txt`, createdText)}>下载 TXT</Button>,
          <Button key="close" onClick={() => setCreatedLines([])}>关闭</Button>
        ]}
      >
        <pre className="proxy-output">{createdText}</pre>
      </Modal>
      <ProCard title="创建订单">
        <ModalForm<Record<string, unknown>>
          title="新建订单"
          trigger={<Button type="primary">新建订单</Button>}
          initialValues={{ mode: 'auto', quantity: 1, duration_day: 30, port: 23457, dedicated_protocol: 'mixed' }}
          modalProps={{ destroyOnHidden: true }}
          onFinish={async (values) => {
            await create.mutateAsync(values)
            return true
          }}
        >
          <ProFormSelect name="customer_id" label="客户" rules={[{ required: true }]} options={(customers.data || []).map((item) => ({ label: item.name, value: item.id }))} />
          <ProFormText name="name" label="订单名" />
          <ProFormSelect name="mode" label="类型" options={[
            { label: '家宽自动', value: 'auto' },
            { label: '家宽手动', value: 'manual' },
            { label: '专线分发', value: 'dedicated' }
          ]} />
          <ProFormDigit name="quantity" label="数量" min={1} />
          <ProFormDigit name="duration_day" label="有效天数" min={1} />
          <ProFormDigit name="port" label="端口" min={1} max={65535} />
          <ProFormSelect name="dedicated_protocol" label="专线协议" options={[
            { label: 'mixed', value: 'mixed' },
            { label: 'vmess', value: 'vmess' },
            { label: 'vless', value: 'vless' },
            { label: 'shadowsocks', value: 'shadowsocks' }
          ]} />
          <ProFormTextArea name="dedicated_egress_lines" label="专线出口" fieldProps={{ rows: 5, placeholder: 'ip:port:user:pass' }} />
        </ModalForm>
      </ProCard>
      <ProTable<Order>
        rowKey="id"
        request={loadOrders}
        search={{ labelWidth: 80 }}
        pagination={{ pageSize: 12 }}
        columns={[
          { title: '关键词', dataIndex: 'keyword', hideInTable: true },
          { title: '订单', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><strong>{row.name}</strong><Typography.Text type="secondary">{row.order_no || `#${row.id}`}</Typography.Text></Space> },
          { title: '客户', render: (_, row) => row.customer?.name || row.customer_id },
          { title: '类型', dataIndex: 'mode', render: (_, row) => <Tag color={row.mode === 'dedicated' ? 'blue' : 'green'}>{row.mode}</Tag> },
          { title: '状态', dataIndex: 'status', render: (_, row) => statusTag(row.status) },
          { title: '到期时间', render: (_, row) => formatTime(row.expires_at) },
          {
            title: '发货',
            valueType: 'option',
            render: (_, row) => [
              <Button key="export" size="small" onClick={() => window.open(`/api/orders/${row.id}/export`, '_blank')}>导出</Button>,
              row.mode === 'dedicated' ? <Button key="copy" size="small" onClick={async () => {
                const res = await http.get(`/api/orders/${row.id}/copy-links`, { responseType: 'text' })
                await navigator.clipboard.writeText(String(res.data || ''))
                message.success('已复制')
              }}>复制链接</Button> : null
            ]
          }
        ]}
      />
    </Space>
  )
}

function DedicatedPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const inbounds = useQuery({ queryKey: ['dedicated-inbounds'], queryFn: async () => (await http.get<DedicatedInbound[]>('/api/orders/dedicated-inbounds')).data })
  const ingresses = useQuery({ queryKey: ['dedicated-ingresses'], queryFn: async () => (await http.get<DedicatedIngress[]>('/api/orders/dedicated-ingresses')).data })
  const saveInbound = useMutation({
    mutationFn: (values: Partial<DedicatedInbound>) => values.id ? http.put(`/api/orders/dedicated-inbounds/${values.id}`, values) : http.post('/api/orders/dedicated-inbounds', values),
    onSuccess: () => {
      message.success('Inbound 已保存')
      queryClient.invalidateQueries({ queryKey: ['dedicated-inbounds'] })
    },
    onError: (error) => message.error(normalizeApiError(error))
  })
  const saveIngress = useMutation({
    mutationFn: (values: Partial<DedicatedIngress>) => values.id ? http.put(`/api/orders/dedicated-ingresses/${values.id}`, values) : http.post('/api/orders/dedicated-ingresses', values),
    onSuccess: () => {
      message.success('Ingress 已保存')
      queryClient.invalidateQueries({ queryKey: ['dedicated-ingresses'] })
    },
    onError: (error) => message.error(normalizeApiError(error))
  })
  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <ProTable<DedicatedInbound>
        rowKey="id"
        headerTitle="专线 Inbound"
        search={false}
        options={false}
        loading={inbounds.isLoading}
        dataSource={inbounds.data || []}
        toolBarRender={() => [<DedicatedInboundForm key="create" onSubmit={(values) => saveInbound.mutateAsync(values)} />]}
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '协议', dataIndex: 'protocol', render: (_, row) => <Tag color="blue">{row.protocol}</Tag> },
          { title: '监听端口', dataIndex: 'listen_port' },
          { title: '优先级', dataIndex: 'priority' },
          { title: '启用', dataIndex: 'enabled', render: (_, row) => row.enabled ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag> },
          { title: '操作', valueType: 'option', render: (_, row) => <DedicatedInboundForm row={row} onSubmit={(values) => saveInbound.mutateAsync({ ...values, id: row.id })} /> }
        ]}
      />
      <ProTable<DedicatedIngress>
        rowKey="id"
        headerTitle="专线 Ingress"
        search={false}
        options={false}
        loading={ingresses.isLoading}
        dataSource={ingresses.data || []}
        toolBarRender={() => [<DedicatedIngressForm key="create" inbounds={inbounds.data || []} onSubmit={(values) => saveIngress.mutateAsync(values)} />]}
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '域名', dataIndex: 'domain' },
          { title: '入口端口', dataIndex: 'ingress_port' },
          { title: '国家/地区', render: (_, row) => `${row.country_code || '-'} ${row.region || ''}` },
          { title: '优先级', dataIndex: 'priority' },
          { title: '启用', dataIndex: 'enabled', render: (_, row) => row.enabled ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag> },
          { title: '操作', valueType: 'option', render: (_, row) => <DedicatedIngressForm row={row} inbounds={inbounds.data || []} onSubmit={(values) => saveIngress.mutateAsync({ ...values, id: row.id })} /> }
        ]}
      />
      <ProTable<Order>
        rowKey="id"
        headerTitle="专线订单"
        request={(params) => loadOrders({ ...params, mode: 'dedicated' })}
        search={{ labelWidth: 80 }}
        pagination={{ pageSize: 12 }}
        columns={[
          { title: '关键词', dataIndex: 'keyword', hideInTable: true },
          { title: '订单', dataIndex: 'name' },
          { title: '客户', render: (_, row) => row.customer?.name || row.customer_id },
          { title: '协议', dataIndex: 'dedicated_protocol', render: (_, row) => row.dedicated_protocol || '-' },
          { title: '状态', dataIndex: 'status', render: (_, row) => statusTag(row.status) },
          { title: '到期', render: (_, row) => formatTime(row.expires_at) },
          { title: '操作', valueType: 'option', render: (_, row) => <Button size="small" onClick={() => window.open(`/api/orders/${row.id}/export`, '_blank')}>导出</Button> }
        ]}
      />
    </Space>
  )
}

function DedicatedInboundForm({ row, onSubmit }: { row?: DedicatedInbound; onSubmit: (values: Partial<DedicatedInbound>) => Promise<unknown> }) {
  return (
    <ModalForm<Partial<DedicatedInbound>>
      title={row ? '编辑 Inbound' : '新建 Inbound'}
      trigger={<Button type={row ? 'default' : 'primary'} size={row ? 'small' : 'middle'}>{row ? '编辑' : '新建 Inbound'}</Button>}
      initialValues={row || { protocol: 'mixed', listen_port: 1080, priority: 100, enabled: true }}
      modalProps={{ destroyOnHidden: true }}
      onFinish={async (values) => { await onSubmit(values); return true }}
    >
      <ProFormText name="name" label="名称" />
      <ProFormSelect name="protocol" label="协议" options={[
        { label: 'mixed', value: 'mixed' },
        { label: 'vmess', value: 'vmess' },
        { label: 'vless', value: 'vless' },
        { label: 'shadowsocks', value: 'shadowsocks' }
      ]} />
      <ProFormDigit name="listen_port" label="监听端口" min={1} max={65535} />
      <ProFormDigit name="priority" label="优先级" min={1} />
      <ProFormSwitch name="enabled" label="启用" />
      <ProFormTextArea name="notes" label="备注" />
    </ModalForm>
  )
}

function DedicatedIngressForm({ row, inbounds, onSubmit }: { row?: DedicatedIngress; inbounds: DedicatedInbound[]; onSubmit: (values: Partial<DedicatedIngress>) => Promise<unknown> }) {
  return (
    <ModalForm<Partial<DedicatedIngress>>
      title={row ? '编辑 Ingress' : '新建 Ingress'}
      trigger={<Button type={row ? 'default' : 'primary'} size={row ? 'small' : 'middle'}>{row ? '编辑' : '新建 Ingress'}</Button>}
      initialValues={row || { priority: 100, enabled: true }}
      modalProps={{ destroyOnHidden: true }}
      onFinish={async (values) => { await onSubmit(values); return true }}
    >
      <ProFormSelect name="dedicated_inbound_id" label="Inbound" options={inbounds.map((item) => ({ label: `${item.name || item.protocol} :${item.listen_port}`, value: item.id }))} rules={[{ required: true }]} />
      <ProFormText name="name" label="名称" />
      <ProFormText name="domain" label="域名" rules={[{ required: true }]} />
      <ProFormDigit name="ingress_port" label="入口端口" min={1} max={65535} />
      <ProFormText name="country_code" label="国家" />
      <ProFormText name="region" label="地区" />
      <ProFormDigit name="priority" label="优先级" min={1} />
      <ProFormSwitch name="enabled" label="启用" />
      <ProFormTextArea name="notes" label="备注" />
    </ModalForm>
  )
}

function ImportPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const [lines, setLines] = useState('')
  const [preview, setPreview] = useState<ImportPreviewRow[]>([])
  const [confirmedLines, setConfirmedLines] = useState<string[]>([])
  const [customerId, setCustomerId] = useState(0)
  const [orderName, setOrderName] = useState('')
  const customers = useQuery({ queryKey: ['customers'], queryFn: async () => (await http.get<Customer[]>('/api/customers')).data })
  const valid = preview.length > 0 && preview.every((row) => !row.error)
  const confirmedText = confirmedLines.join('\n')
  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Modal
        title="导入结果"
        open={confirmedLines.length > 0}
        onCancel={() => setConfirmedLines([])}
        footer={[
          <Button key="copy" type="primary" onClick={async () => {
            await navigator.clipboard.writeText(confirmedText)
            message.success('已复制')
          }}>复制全部</Button>,
          <Button key="download" onClick={() => downloadTextFile(`xraytool-import-${Date.now()}.txt`, confirmedText)}>下载 TXT</Button>,
          <Button key="close" onClick={() => setConfirmedLines([])}>关闭</Button>
        ]}
      >
        <pre className="proxy-output">{confirmedText}</pre>
      </Modal>
      <ProCard title="批量导入 Socks5">
        <Space direction="vertical" style={{ width: '100%' }}>
          <ProFormSelect
            fieldProps={{ value: customerId, onChange: (value) => setCustomerId(Number(value || 0)) }}
            options={[{ label: '未分配客户', value: 0 }, ...(customers.data || []).map((item) => ({ label: item.name, value: item.id }))]}
          />
          <Input value={orderName} onChange={(event) => setOrderName(event.target.value)} placeholder="订单名" />
          <Input.TextArea rows={8} value={lines} onChange={(event) => setLines(event.target.value)} placeholder="ip:port:user:pass" />
          <Space>
            <Button onClick={async () => {
              const res = await http.post('/api/orders/import/preview', { lines })
              setPreview(res.data || [])
            }}>预检</Button>
            <Button type="primary" disabled={!valid} onClick={async () => {
              const res = await http.post('/api/orders/import/confirm', {
                customer_id: customerId,
                order_name: orderName,
                rows: preview
              })
              message.success('导入已提交，后台同步中')
              setConfirmedLines(proxyLinesFromPayload(res.data))
              setPreview([])
              setLines('')
              queryClient.invalidateQueries()
            }}>确认导入</Button>
          </Space>
        </Space>
      </ProCard>
      <ProTable<ImportPreviewRow>
        rowKey={(row, index) => `${index}-${row.raw}`}
        search={false}
        options={false}
        dataSource={preview}
        columns={[
          { title: '状态', render: (_, row) => row.error ? <Tag color="red">{row.error}</Tag> : <Tag color="green">ok</Tag> },
          { title: 'IP', dataIndex: 'ip' },
          { title: '端口', dataIndex: 'port' },
          { title: '账号', dataIndex: 'username' },
          { title: '原始行', dataIndex: 'raw', ellipsis: true }
        ]}
      />
    </Space>
  )
}

function CustomersPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const customers = useQuery({ queryKey: ['customers'], queryFn: async () => (await http.get<Customer[]>('/api/customers')).data })
  const save = useMutation({
    mutationFn: (payload: Partial<Customer>) => payload.id ? http.put(`/api/customers/${payload.id}`, payload) : http.post('/api/customers', payload),
    onSuccess: () => {
      message.success('客户已保存')
      queryClient.invalidateQueries({ queryKey: ['customers'] })
    },
    onError: (error) => message.error(normalizeApiError(error))
  })
  const remove = useMutation({
    mutationFn: (id: number) => http.delete(`/api/customers/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['customers'] }),
    onError: (error) => message.error(normalizeApiError(error))
  })
  return (
    <ProTable<Customer>
      rowKey="id"
      search={false}
      options={false}
      loading={customers.isLoading}
      dataSource={customers.data || []}
      toolBarRender={() => [<CustomerForm key="create" onSubmit={(values) => save.mutateAsync(values)} />]}
      columns={[
        { title: '名称', dataIndex: 'name' },
        { title: '编码', dataIndex: 'code' },
        { title: '联系方式', dataIndex: 'contact' },
        { title: '状态', dataIndex: 'status', render: (_, row) => statusTag(row.status) },
        { title: '备注', dataIndex: 'notes', ellipsis: true },
        {
          title: '操作',
          valueType: 'option',
          render: (_, row) => [
            <CustomerForm key="edit" row={row} onSubmit={(values) => save.mutateAsync({ ...values, id: row.id })} />,
            <Popconfirm key="delete" title="确认删除客户？" onConfirm={() => remove.mutate(row.id)}>
              <Button size="small" danger>删除</Button>
            </Popconfirm>
          ]
        }
      ]}
    />
  )
}

function CustomerForm({ row, onSubmit }: { row?: Customer; onSubmit: (values: Partial<Customer>) => Promise<unknown> }) {
  return (
    <ModalForm<Partial<Customer>>
      title={row ? '编辑客户' : '新建客户'}
      trigger={<Button type={row ? 'default' : 'primary'} size={row ? 'small' : 'middle'}>{row ? '编辑' : '新建客户'}</Button>}
      initialValues={row || { status: 'active' }}
      modalProps={{ destroyOnHidden: true }}
      onFinish={async (values) => { await onSubmit(values); return true }}
    >
      <ProFormText name="name" label="名称" rules={[{ required: true }]} />
      <ProFormText name="code" label="编码" />
      <ProFormText name="contact" label="联系方式" />
      <ProFormSelect name="status" label="状态" options={[{ label: 'active', value: 'active' }, { label: 'disabled', value: 'disabled' }]} />
      <ProFormTextArea name="notes" label="备注" />
    </ModalForm>
  )
}

function HostIPsPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const hostIPs = useQuery({ queryKey: ['host-ips'], queryFn: async () => (await http.get<HostIP[]>('/api/host-ips')).data })
  return (
    <ProTable<HostIP>
      rowKey="id"
      search={false}
      options={false}
      loading={hostIPs.isLoading}
      dataSource={hostIPs.data || []}
      toolBarRender={() => [
        <Button key="scan" icon={<ReloadOutlined />} onClick={async () => {
          await http.post('/api/host-ips/scan', {})
          message.success('扫描完成')
          queryClient.invalidateQueries({ queryKey: ['host-ips'] })
        }}>扫描 IP</Button>
      ]}
      columns={[
        { title: 'IP', dataIndex: 'ip' },
        { title: '公网', dataIndex: 'is_public', render: (_, row) => row.is_public ? <Tag color="green">是</Tag> : <Tag>否</Tag> },
        { title: '本机', dataIndex: 'is_local', render: (_, row) => row.is_local ? <Tag color="blue">是</Tag> : <Tag>否</Tag> },
        { title: '启用', dataIndex: 'enabled', render: (_, row) => row.enabled ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag> },
        {
          title: '操作',
          valueType: 'option',
          render: (_, row) => <Button size="small" onClick={async () => {
            await http.post(`/api/host-ips/${row.id}/toggle`, { enabled: !row.enabled })
            queryClient.invalidateQueries({ queryKey: ['host-ips'] })
          }}>{row.enabled ? '停用' : '启用'}</Button>
        }
      ]}
    />
  )
}

function NodesPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const nodes = useQuery({ queryKey: ['nodes'], queryFn: async () => (await http.get<XrayNode[]>('/api/nodes')).data })
  const save = useMutation({
    mutationFn: (payload: Partial<XrayNode> & { password?: string }) => payload.id ? http.put(`/api/nodes/${payload.id}`, payload) : http.post('/api/nodes', payload),
    onSuccess: () => {
      message.success('节点已保存')
      queryClient.invalidateQueries({ queryKey: ['nodes'] })
    },
    onError: (error) => message.error(normalizeApiError(error))
  })
  return (
    <ProTable<XrayNode>
      rowKey="id"
      search={false}
      options={false}
      loading={nodes.isLoading}
      dataSource={nodes.data || []}
      toolBarRender={() => [<NodeForm key="create" onSubmit={(values) => save.mutateAsync(values)} />]}
      columns={[
        { title: '名称', dataIndex: 'name' },
        { title: '地址', dataIndex: 'base_url', ellipsis: true },
        { title: '账号', dataIndex: 'username' },
        { title: '启用', dataIndex: 'enabled', render: (_, row) => row.enabled ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag> },
        { title: '本机', dataIndex: 'is_local', render: (_, row) => row.is_local ? <Tag color="blue">是</Tag> : <Tag>否</Tag> },
        { title: '操作', valueType: 'option', render: (_, row) => <NodeForm row={row} onSubmit={(values) => save.mutateAsync({ ...values, id: row.id })} /> }
      ]}
    />
  )
}

function NodeForm({ row, onSubmit }: { row?: XrayNode; onSubmit: (values: Partial<XrayNode> & { password?: string }) => Promise<unknown> }) {
  return (
    <ModalForm<Partial<XrayNode> & { password?: string }>
      title={row ? '编辑节点' : '新建节点'}
      trigger={<Button type={row ? 'default' : 'primary'} size={row ? 'small' : 'middle'}>{row ? '编辑' : '新建节点'}</Button>}
      initialValues={row || { enabled: true, is_local: false }}
      modalProps={{ destroyOnHidden: true }}
      onFinish={async (values) => { await onSubmit(values); return true }}
    >
      <ProFormText name="name" label="名称" rules={[{ required: true }]} />
      <ProFormText name="base_url" label="基础地址" rules={[{ required: true }]} />
      <ProFormText name="username" label="账号" rules={[{ required: true }]} />
      <ProFormText.Password name="password" label={row ? '新密码' : '密码'} rules={row ? [] : [{ required: true }]} />
      <ProFormSwitch name="enabled" label="启用" />
      <ProFormSwitch name="is_local" label="本机" />
    </ModalForm>
  )
}

function SettingsPage() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const settings = useQuery({ queryKey: ['settings'], queryFn: async () => (await http.get<SettingsMap>('/api/settings')).data })
  return (
    <ProCard title="设置" loading={settings.isLoading}>
      <ModalForm<SettingsMap>
        title="编辑 GoSea 上报"
        trigger={<Button type="primary">编辑设置</Button>}
        initialValues={settings.data}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          await http.put('/api/settings', values)
          message.success('设置已保存')
          queryClient.invalidateQueries({ queryKey: ['settings'] })
          return true
        }}
      >
        <ProFormSwitch name="gosealight_telemetry_enabled" label="启用 GoSea 上报" />
        <ProFormText name="gosealight_base_url" label="GoSea 地址" />
        <ProFormText name="gosealight_node_id" label="节点 ID" />
        <ProFormText name="gosealight_node_username" label="节点账号" />
        <ProFormText.Password name="gosealight_node_password" label="节点密码" />
        <ProFormDigit name="gosealight_telemetry_interval_seconds" label="上报间隔秒" min={10} />
      </ModalForm>
      <Descriptions className="settings-descriptions" column={1} bordered size="small">
        {Object.entries(settings.data || {}).map(([key, value]) => (
          <Descriptions.Item key={key} label={key}>{String(value || '-')}</Descriptions.Item>
        ))}
      </Descriptions>
    </ProCard>
  )
}
