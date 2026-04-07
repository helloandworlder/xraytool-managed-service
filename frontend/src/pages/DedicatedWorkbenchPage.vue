<script setup lang="ts">
defineProps<{
	panel: any
	dedicatedSearchKeyword: string
	dedicatedCustomerID: number
	dedicatedStatusFilter: string
	dedicatedPagination: Record<string, any>
	dedicatedGroupHeads: Array<Record<string, any>>
	activeDedicatedHeadID: number
	activeDedicatedHead: Record<string, any> | null
	activeDedicatedChildren: Array<Record<string, any>>
	dedicatedCheckRunning: boolean
	dedicatedCheckResults: Record<number, Record<string, any>>
	loadDedicatedView: (page?: number, pageSize?: number) => void
	selectDedicatedHead: (orderID: number) => void
	openOrderEdit: (record: any) => void
	openGroupSocksModal: (orderID: number) => void
	openGroupCredModal: (orderID: number) => void
	openGroupGeoModal: (orderID: number) => void
	openGroupRenewModal: (orderID: number) => void
	exportOrder: (orderID: number) => void
	copyOrderLinks: (record: any) => void
	copyLinksLabel: (record: any) => string
	runDedicatedGroupProtocolCheck: (orderID?: number) => void
	formatTime: (value: string) => string
	expiresHint: (value: string) => string
}>()

const emit = defineEmits<{
	(e: 'update:dedicatedSearchKeyword', value: string): void
	(e: 'update:dedicatedCustomerID', value: number): void
	(e: 'update:dedicatedStatusFilter', value: string): void
}>()
</script>

<template>
	<a-card :bordered="false" title="专线工作台" class="mb-3">
		<template #extra>
			<a-space wrap>
				<a-input :value="dedicatedSearchKeyword" style="width:220px" placeholder="搜索组头/子单/域名/订单号" allow-clear @update:value="emit('update:dedicatedSearchKeyword', $event)" />
				<a-select :value="dedicatedCustomerID" style="width:180px" placeholder="客户筛选" @update:value="emit('update:dedicatedCustomerID', Number($event || 0))">
					<a-select-option :value="0">全部客户</a-select-option>
					<a-select-option v-for="c in panel.customers" :key="c.id" :value="c.id">{{ c.name }}</a-select-option>
				</a-select>
				<a-select :value="dedicatedStatusFilter" style="width:140px" @update:value="emit('update:dedicatedStatusFilter', String($event || 'all'))">
					<a-select-option value="all">全部状态</a-select-option>
					<a-select-option value="active">active</a-select-option>
					<a-select-option value="expired">expired</a-select-option>
					<a-select-option value="disabled">disabled</a-select-option>
				</a-select>
				<a-button :loading="dedicatedCheckRunning" :disabled="!activeDedicatedHead" @click="runDedicatedGroupProtocolCheck(activeDedicatedHeadID)">XrayCore 探测</a-button>
			</a-space>
		</template>
		<a-alert type="info" show-icon message="这里专门处理专线组头、子单、批量改出口 Socks5、改凭据、国家地区、部分续费，以及真实 XrayCore 协议探测。" />
	</a-card>

	<a-row :gutter="12">
		<a-col :xs="24" :xl="9">
			<a-card :bordered="false" title="专线组列表">
				<div v-if="!dedicatedGroupHeads.length" class="text-sm text-slate-500">当前筛选下没有专线组头订单。</div>
				<a-space v-else direction="vertical" style="width:100%">
					<button
						v-for="head in dedicatedGroupHeads"
						:key="head.id"
						type="button"
						class="group-card"
						:class="{ active: Number(activeDedicatedHeadID) === Number(head.id) }"
						@click="selectDedicatedHead(Number(head.id))"
					>
						<div class="group-card-top">
							<div class="group-card-title">#{{ head.id }} / {{ head.name || '未命名组头' }}</div>
							<a-tag :color="head.status === 'active' ? 'green' : head.status === 'disabled' ? 'red' : 'orange'">{{ head.status }}</a-tag>
						</div>
						<div class="group-card-meta">
							<span>{{ head.customer?.name || head.customer_id }}</span>
							<span>{{ head.dedicated_protocol || 'mixed' }}</span>
							<span>{{ head.dedicated_ingress?.domain || head.dedicated_entry?.domain || '-' }}</span>
						</div>
						<div class="group-card-meta">
							<span>{{ expiresHint(head.expires_at) }}</span>
							<span>{{ formatTime(head.expires_at) }}</span>
						</div>
					</button>
				</a-space>
				<div class="mt-3 flex justify-end">
					<a-pagination
						:current="dedicatedPagination.current"
						:page-size="dedicatedPagination.pageSize"
						:total="panel.orderList.total"
						:show-size-changer="false"
						@change="(page:number, pageSize:number) => { dedicatedPagination.current = Number(page || 1); dedicatedPagination.pageSize = Number(pageSize || 12); void loadDedicatedView(dedicatedPagination.current, dedicatedPagination.pageSize) }"
					/>
				</div>
			</a-card>
		</a-col>

		<a-col :xs="24" :xl="15">
			<a-card :bordered="false" :title="activeDedicatedHead ? `组内子订单 · #${activeDedicatedHead.id}` : '组内子订单'">
				<template #extra>
					<a-space wrap v-if="activeDedicatedHead">
						<a-button @click="openOrderEdit(activeDedicatedHead)">编辑组头</a-button>
						<a-button type="primary" @click="openGroupSocksModal(activeDedicatedHead.id)">批量改 Socks5</a-button>
						<a-button @click="openGroupCredModal(activeDedicatedHead.id)">批量改凭据</a-button>
						<a-button @click="openGroupGeoModal(activeDedicatedHead.id)">国家地区</a-button>
						<a-button @click="openGroupRenewModal(activeDedicatedHead.id)">部分续费</a-button>
						<a-button :loading="dedicatedCheckRunning" @click="runDedicatedGroupProtocolCheck(activeDedicatedHead.id)">协议探测</a-button>
						<a-button @click="exportOrder(activeDedicatedHead.id)">导出</a-button>
						<a-button @click="copyOrderLinks(activeDedicatedHead)">{{ copyLinksLabel(activeDedicatedHead) }}</a-button>
					</a-space>
				</template>

				<div v-if="!activeDedicatedHead" class="text-sm text-slate-500">左侧选择一个专线组头后，这里会显示组内子订单和探测结果。</div>
				<div v-else>
					<a-descriptions size="small" :column="2" bordered class="mb-3">
						<a-descriptions-item label="客户">{{ activeDedicatedHead.customer?.name || activeDedicatedHead.customer_id }}</a-descriptions-item>
						<a-descriptions-item label="协议">{{ activeDedicatedHead.dedicated_protocol || 'mixed' }}</a-descriptions-item>
						<a-descriptions-item label="入口">{{ activeDedicatedHead.dedicated_ingress?.domain || activeDedicatedHead.dedicated_entry?.domain || '-' }}</a-descriptions-item>
						<a-descriptions-item label="到期">{{ formatTime(activeDedicatedHead.expires_at) }}</a-descriptions-item>
					</a-descriptions>

					<a-table :data-source="activeDedicatedChildren" size="small" :pagination="false" :row-key="(row:any) => row.id" :scroll="{ x: 1100 }">
						<a-table-column title="子单" key="order" width="220">
							<template #default="{ record }">
								<div class="text-xs">
									<div class="font-medium">#{{ record.id }} / {{ record.name || '-' }}</div>
									<div class="text-slate-500">序号 {{ record.sequence_no || '-' }} / {{ record.order_no || `OD-${record.id}` }}</div>
								</div>
							</template>
						</a-table-column>
						<a-table-column title="入口" key="entry" width="240">
							<template #default="{ record }">
								<span class="font-mono text-xs">{{ record.dedicated_ingress?.domain || record.dedicated_entry?.domain || '-' }}:{{ record.dedicated_ingress?.ingress_port || record.port }}</span>
							</template>
						</a-table-column>
						<a-table-column title="凭据" key="credential" width="220">
							<template #default="{ record }">
								<span class="font-mono text-xs">{{ record.items?.[0]?.username || '-' }}:{{ record.items?.[0]?.password || '-' }}</span>
							</template>
						</a-table-column>
						<a-table-column title="到期" key="expires" width="180">
							<template #default="{ record }">
								<div>{{ expiresHint(record.expires_at) }}</div>
								<div class="text-xs text-slate-500">{{ formatTime(record.expires_at) }}</div>
							</template>
						</a-table-column>
						<a-table-column title="XrayCore 探测" key="probe" width="260">
							<template #default="{ record }">
								<div v-if="dedicatedCheckResults[record.id]" class="text-xs">
									<a-tag :color="dedicatedCheckResults[record.id].ok ? 'green' : 'red'">{{ dedicatedCheckResults[record.id].ok ? '可用' : '失败' }}</a-tag>
									<span v-if="dedicatedCheckResults[record.id].exitIp" class="font-mono">{{ dedicatedCheckResults[record.id].exitIp }}</span>
									<div class="text-slate-500">{{ dedicatedCheckResults[record.id].countryCode || '--' }} {{ dedicatedCheckResults[record.id].region || '' }}</div>
									<div v-if="dedicatedCheckResults[record.id].message" class="text-rose-600">{{ dedicatedCheckResults[record.id].message }}</div>
								</div>
								<span v-else class="text-xs text-slate-400">尚未探测</span>
							</template>
						</a-table-column>
						<a-table-column title="动作" key="action" width="180" fixed="right">
							<template #default="{ record }">
								<a-space :size="4" wrap>
									<a-button size="small" @click="openOrderEdit(record)">编辑</a-button>
									<a-button size="small" @click="copyOrderLinks(record)">{{ copyLinksLabel(record) }}</a-button>
								</a-space>
							</template>
						</a-table-column>
					</a-table>
				</div>
			</a-card>
		</a-col>
	</a-row>
</template>

<style scoped>
.group-card {
	width: 100%;
	border: 1px solid rgba(148, 163, 184, 0.22);
	border-radius: 14px;
	padding: 12px;
	text-align: left;
	background: #fff;
	transition: all 0.16s ease;
}

.group-card:hover,
.group-card.active {
	border-color: #2563eb;
	background: rgba(37, 99, 235, 0.05);
}

.group-card-top {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
	margin-bottom: 6px;
}

.group-card-title {
	font-weight: 700;
	color: #0f172a;
}

.group-card-meta {
	display: flex;
	flex-wrap: wrap;
	gap: 10px;
	font-size: 12px;
	color: #64748b;
}
</style>
