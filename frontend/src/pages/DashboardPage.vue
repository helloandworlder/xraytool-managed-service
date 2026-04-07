<script setup lang="ts">
const props = defineProps<{
	healthCards: Array<Record<string, any>>
	oversellCustomerID: number
	panel: any
	oversellColumns: Array<Record<string, any>>
	logFilters: Record<string, any>
	changeOversellView: (customerID: number) => void
	bpsText: (value: number) => string
	bytesText: (value: number) => string
	formatTime: (value: string) => string
	applyLogFilter: () => void
	resetLogFilter: () => void
}>()

const runtimeCustomerColumns = [
	{ title: '客户', key: 'customer', width: 180 },
	{ title: '在线数', dataIndex: 'online_clients', key: 'online_clients', width: 90 },
	{ title: '家宽/专线在线', key: 'mode_online', width: 140 },
	{ title: '实时速度', key: 'speed', width: 130 },
	{ title: '1h流量', key: 't1h', width: 110 },
	{ title: '24h流量', key: 't24h', width: 110 },
	{ title: '7d流量', key: 't7d', width: 110 },
	{ title: '累计流量', key: 'ttotal', width: 120 },
	{ title: '更新时间', key: 'updated_at', width: 170 }
]

const runtimeGroupColumns = [
	{ title: '订单组', key: 'group', width: 220 },
	{ title: '客户', key: 'customer', width: 180 },
	{ title: '模式', key: 'mode', width: 90 },
	{ title: '订单数', dataIndex: 'order_count', key: 'order_count', width: 90 },
	{ title: '线路数', dataIndex: 'active_items', key: 'active_items', width: 90 },
	{ title: '在线数', dataIndex: 'online_clients', key: 'online_clients', width: 90 },
	{ title: '实时速度', key: 'speed', width: 130 },
	{ title: '24h流量', key: 't24h', width: 110 },
	{ title: '累计流量', key: 'ttotal', width: 120 },
	{ title: '更新时间', key: 'updated_at', width: 170 }
]

const runtimeOrderColumns = [
	{ title: '订单', key: 'order', width: 260 },
	{ title: '客户', key: 'customer', width: 180 },
	{ title: '组头', key: 'group', width: 180 },
	{ title: '模式', key: 'mode', width: 90 },
	{ title: '数量', dataIndex: 'quantity', key: 'quantity', width: 80 },
	{ title: '在线数', dataIndex: 'online_clients', key: 'online_clients', width: 90 },
	{ title: '实时速度', key: 'speed', width: 130 },
	{ title: '24h流量', key: 't24h', width: 110 },
	{ title: '累计流量', key: 'ttotal', width: 120 },
	{ title: '更新时间', key: 'updated_at', width: 170 }
]
</script>

<template>
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
					<div
						v-for="row in panel.oversell"
						:key="row.ip"
						class="ip-cell"
						:style="{ backgroundColor: row.oversold_count > 0 ? 'rgba(239,68,68,0.16)' : row.total_active_count > 0 ? 'rgba(34,197,94,0.16)' : 'rgba(148,163,184,0.08)' }"
						:title="`${row.ip} 总占用:${row.total_active_count}`"
					>
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

			<a-card :bordered="false" title="Socks5 运行时状态" class="mb-3">
				<template #extra>
					<a-space>
						<span class="text-xs text-slate-500">Top 30 客户 / 订单组 / 订单</span>
						<a-button size="small" @click="panel.loadRuntimeStats">刷新</a-button>
					</a-space>
				</template>
				<div class="runtime-summary">
					<div class="runtime-chip">
						<div class="runtime-chip-label">客户</div>
						<div class="runtime-chip-value">{{ panel.runtimeOverview.customers.length }}</div>
					</div>
					<div class="runtime-chip">
						<div class="runtime-chip-label">订单组</div>
						<div class="runtime-chip-value">{{ panel.runtimeOverview.groups.length }}</div>
					</div>
					<div class="runtime-chip">
						<div class="runtime-chip-label">订单</div>
						<div class="runtime-chip-value">{{ panel.runtimeOverview.orders.length }}</div>
					</div>
					<div class="runtime-chip runtime-chip-wide">
						<div class="runtime-chip-label">快照时间</div>
						<div class="runtime-chip-value runtime-chip-time">{{ panel.runtimeOverview.updated_at ? formatTime(panel.runtimeOverview.updated_at) : '-' }}</div>
					</div>
				</div>
				<a-alert
					v-for="warning in (panel.runtimeOverview.warnings || [])"
					:key="warning"
					class="mb-3"
					type="warning"
					show-icon
					:message="warning"
				/>
				<a-tabs size="small" class="runtime-tabs">
					<a-tab-pane key="customers" tab="客户 Top 30">
						<a-table :columns="runtimeCustomerColumns" :data-source="panel.runtimeOverview.customers" size="small" :row-key="(row:any)=>row.customer_id" :pagination="{ pageSize: 8 }" :scroll="{ x: 1120 }">
							<template #bodyCell="{ column, record }">
								<template v-if="column.key === 'customer'">
									<div class="font-semibold">{{ record.customer_name }}{{ record.customer_code ? ` (${record.customer_code})` : '' }}</div>
									<div class="text-xs text-slate-500">线路 {{ record.home_items || 0 }} / 专线 {{ record.dedicated_items || 0 }}</div>
								</template>
								<template v-else-if="column.key === 'mode_online'">
									<div>家宽 {{ record.home_online_clients || 0 }}</div>
									<div class="text-xs text-slate-500">专线 {{ record.dedicated_online_clients || 0 }}</div>
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
								<template v-else-if="column.key === 'ttotal'">
									{{ bytesText(Number(record.traffic_total || 0)) }}
								</template>
								<template v-else-if="column.key === 'updated_at'">
									{{ formatTime(record.updated_at) }}
								</template>
							</template>
						</a-table>
					</a-tab-pane>
					<a-tab-pane key="groups" tab="订单组">
						<a-table :columns="runtimeGroupColumns" :data-source="panel.runtimeOverview.groups" size="small" :row-key="(row:any)=>row.group_id" :pagination="{ pageSize: 8 }" :scroll="{ x: 1180 }">
							<template #bodyCell="{ column, record }">
								<template v-if="column.key === 'group'">
									<div class="font-semibold">#{{ record.group_id }} {{ record.group_order_no || '' }}</div>
									<div class="text-xs text-slate-500">{{ record.group_name }}</div>
								</template>
								<template v-else-if="column.key === 'customer'">
									{{ record.customer_name }}{{ record.customer_code ? ` (${record.customer_code})` : '' }}
								</template>
								<template v-else-if="column.key === 'mode'">
									<a-tag :color="record.mode === 'dedicated' ? 'blue' : 'green'">{{ record.mode === 'dedicated' ? '专线' : '家宽' }}</a-tag>
								</template>
								<template v-else-if="column.key === 'speed'">
									{{ bpsText(Number(record.realtime_bps || 0)) }}
								</template>
								<template v-else-if="column.key === 't24h'">
									{{ bytesText(Number(record.traffic_24h || 0)) }}
								</template>
								<template v-else-if="column.key === 'ttotal'">
									{{ bytesText(Number(record.traffic_total || 0)) }}
								</template>
								<template v-else-if="column.key === 'updated_at'">
									{{ formatTime(record.updated_at) }}
								</template>
							</template>
						</a-table>
					</a-tab-pane>
					<a-tab-pane key="orders" tab="订单">
						<a-table :columns="runtimeOrderColumns" :data-source="panel.runtimeOverview.orders" size="small" :row-key="(row:any)=>row.order_id" :pagination="{ pageSize: 8 }" :scroll="{ x: 1380 }">
							<template #bodyCell="{ column, record }">
								<template v-if="column.key === 'order'">
									<div class="font-semibold">#{{ record.order_id }} {{ record.order_no || '' }}</div>
									<div class="text-xs text-slate-500">{{ record.order_name }}</div>
								</template>
								<template v-else-if="column.key === 'customer'">
									{{ record.customer_name }}{{ record.customer_code ? ` (${record.customer_code})` : '' }}
								</template>
								<template v-else-if="column.key === 'group'">
									{{ record.group_order_no || `#${record.group_id}` }}
								</template>
								<template v-else-if="column.key === 'mode'">
									<a-tag :color="record.mode === 'dedicated' ? 'blue' : 'green'">{{ record.mode === 'dedicated' ? '专线' : '家宽' }}</a-tag>
								</template>
								<template v-else-if="column.key === 'speed'">
									{{ bpsText(Number(record.realtime_bps || 0)) }}
								</template>
								<template v-else-if="column.key === 't24h'">
									{{ bytesText(Number(record.traffic_24h || 0)) }}
								</template>
								<template v-else-if="column.key === 'ttotal'">
									{{ bytesText(Number(record.traffic_total || 0)) }}
								</template>
								<template v-else-if="column.key === 'updated_at'">
									{{ formatTime(record.updated_at) }}
								</template>
							</template>
						</a-table>
					</a-tab-pane>
				</a-tabs>
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
					<a-button size="small" @click="applyLogFilter">筛选</a-button>
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

<style scoped>
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

.runtime-summary {
	display: grid;
	grid-template-columns: repeat(4, minmax(0, 1fr));
	gap: 8px;
	margin-bottom: 12px;
}

.runtime-chip {
	border: 1px solid rgba(148, 163, 184, 0.2);
	border-radius: 12px;
	padding: 10px 12px;
	background: rgba(248, 250, 252, 0.8);
}

.runtime-chip-wide {
	grid-column: span 1;
}

.runtime-chip-label {
	font-size: 12px;
	color: #64748b;
}

.runtime-chip-value {
	margin-top: 4px;
	font-size: 20px;
	font-weight: 700;
}

.runtime-chip-time {
	font-size: 13px;
	font-weight: 600;
}

.runtime-tabs :deep(.ant-tabs-content) {
	min-height: 18rem;
}

@media (max-width: 768px) {
	.ip-grid {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	.runtime-summary {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}
}
</style>
