<script setup lang="ts">
defineProps<{
	panel: any
	orderSearchKeyword: string
	orderCustomerID: number
	orderModeFilter: string
	orderStatusFilter: string
	testSamplePercent: number
	exportCount: number
	batchMoreDays: number
	batchRenewExpiresAt: string
	filteredOrderRows: Array<Record<string, any>>
	rowSelection: Record<string, any>
	orderPagination: Record<string, any>
	exportingOrderID: number | null
	copyingLinksOrderID: number | null
	testingOrderID: number | null
	testResult: Record<string, string> | null
	batchTestResult: Array<Record<string, any>> | null
	ordersColumns: Array<Record<string, any>>
	loadOrdersView: (page?: number, pageSize?: number) => void
	openOrderDetail: (record: any) => void
	openOrderEditSmart: (record: any) => void
	exportOrder: (orderID: number) => void
	copyOrderLinks: (record: any) => void
	copyLinksLabel: (record: any) => string
	testOrder: (orderID: number) => void
	renewOrder: (orderID: number) => void
	streamTestOrder: (orderID: number) => void
	resetOrderCredentials: (orderID: number) => void
	splitOrderHead: (orderID: number) => void
	openGroupEditor: (orderID: number) => void
	openGroupGeoModal: (orderID: number) => void
	openGroupSocksModal: (orderID: number) => void
	openGroupCredModal: (orderID: number) => void
	openGroupRenewModal: (orderID: number) => void
	removeOrder: (orderID: number) => void
	activateOrder: (orderID: number) => void
	deactivateOrder: (orderID: number) => void
	statusColor: (value: string) => string
	modeColor: (value: string) => string
	modeLabel: (value: string) => string
	dedicatedSummary: (order: any) => string
	forwardSummary: (order: any) => string
	expiresHint: (value: string) => string
	formatTime: (value: string) => string
	doBatchRenew: () => void
	doBatchResync: () => void
	doBatchTest: () => void
	doBatchExport: () => void
	doBatchActivate: () => void
	doBatchDeactivate: () => void
}>()

const emit = defineEmits<{
	(e: 'update:orderSearchKeyword', value: string): void
	(e: 'update:orderCustomerID', value: number): void
	(e: 'update:orderModeFilter', value: string): void
	(e: 'update:orderStatusFilter', value: string): void
	(e: 'update:testSamplePercent', value: number): void
	(e: 'update:exportCount', value: number): void
	(e: 'update:batchMoreDays', value: number): void
	(e: 'update:batchRenewExpiresAt', value: string): void
}>()
</script>

<template>
	<a-card :bordered="false" title="订单列表">
		<template #extra>
			<a-space wrap>
				<a-input :value="orderSearchKeyword" size="small" style="width: 220px" placeholder="搜索: 订单号/客户/域名/IP/账号" allow-clear @update:value="emit('update:orderSearchKeyword', $event)" />
				<a-select :value="orderCustomerID" size="small" style="width: 180px" placeholder="客户筛选" @update:value="emit('update:orderCustomerID', Number($event || 0))">
					<a-select-option :value="0">全部客户</a-select-option>
					<a-select-option v-for="c in panel.customers" :key="c.id" :value="c.id">{{ c.name }}</a-select-option>
				</a-select>
				<a-select :value="orderModeFilter" size="small" style="width: 120px" @update:value="emit('update:orderModeFilter', String($event || 'all'))">
					<a-select-option value="all">全部模式</a-select-option>
					<a-select-option value="home">家宽</a-select-option>
					<a-select-option value="dedicated">专线</a-select-option>
				</a-select>
				<a-select :value="orderStatusFilter" size="small" style="width: 120px" @update:value="emit('update:orderStatusFilter', String($event || 'all'))">
					<a-select-option value="all">全部状态</a-select-option>
					<a-select-option value="active">active</a-select-option>
					<a-select-option value="expired">expired</a-select-option>
					<a-select-option value="disabled">disabled</a-select-option>
				</a-select>
				<span class="text-xs text-slate-500">已选 {{ panel.orderSelection.length }} 个</span>
				<a-select :value="testSamplePercent" size="small" style="width: 110px" @update:value="emit('update:testSamplePercent', Number($event || 100))">
					<a-select-option :value="100">测活100%</a-select-option>
					<a-select-option :value="10">测活10%</a-select-option>
					<a-select-option :value="5">测活5%</a-select-option>
				</a-select>
				<a-input-number :value="exportCount" :min="0" size="small" :placeholder="'导出条数(0=全部)'" @update:value="emit('update:exportCount', Number($event || 0))" />
				<a-input-number :value="batchMoreDays" :min="1" :max="365" size="small" @update:value="emit('update:batchMoreDays', Number($event || 30))" />
				<a-button size="small" @click="emit('update:batchMoreDays', 30)">30天</a-button>
				<a-button size="small" @click="emit('update:batchMoreDays', 60)">60天</a-button>
				<a-button size="small" @click="emit('update:batchMoreDays', 90)">90天</a-button>
				<a-date-picker :value="batchRenewExpiresAt" size="small" show-time value-format="YYYY-MM-DDTHH:mm:ss" placeholder="续期到期时间(可选)" @update:value="emit('update:batchRenewExpiresAt', String($event || ''))" />
				<a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchRenew">批量续期</a-button>
				<a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchResync">批量重同步</a-button>
				<a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchTest">批量测活</a-button>
				<a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchExport">批量导出</a-button>
				<a-button size="small" :disabled="panel.orderSelection.length===0" @click="doBatchActivate">批量启用</a-button>
				<a-button size="small" danger :disabled="panel.orderSelection.length===0" @click="doBatchDeactivate">批量停用</a-button>
			</a-space>
		</template>

		<a-table
			:columns="ordersColumns"
			:data-source="filteredOrderRows"
			:row-selection="rowSelection"
			:scroll="{ x: 1300 }"
			:pagination="false"
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
						<a-button v-if="record.mode === 'dedicated'" size="small" :loading="copyingLinksOrderID===record.id" @click="copyOrderLinks(record)">{{ copyLinksLabel(record) }}</a-button>
						<a-button size="small" :disabled="record.mode === 'dedicated'" :loading="testingOrderID===record.id" @click="testOrder(record.id)">测活</a-button>
						<a-dropdown>
							<a-button size="small">更多</a-button>
							<template #overlay>
								<a-menu>
									<a-menu-item @click="renewOrder(record.id)">续期</a-menu-item>
									<a-menu-item :disabled="record.mode === 'dedicated'" @click="streamTestOrder(record.id)">流式测活</a-menu-item>
									<a-menu-item v-if="record.mode === 'auto' || record.mode === 'manual' || record.mode === 'import'" @click="resetOrderCredentials(record.id)">刷新家宽凭据</a-menu-item>
									<a-menu-item v-if="!record.parent_order_id && record.quantity > 1" @click="splitOrderHead(record.id)">拆分为子订单</a-menu-item>
									<a-menu-item v-if="record.is_group_head" @click="openGroupEditor(record.id)">组编辑工作台</a-menu-item>
									<a-menu-item v-if="record.is_group_head" @click="openGroupGeoModal(record.id)">批量设置国家地区</a-menu-item>
									<a-menu-item v-if="record.is_group_head" @click="openGroupSocksModal(record.id)">组内顺序改 Socks5</a-menu-item>
									<a-menu-item v-if="record.is_group_head" @click="openGroupCredModal(record.id)">组内批量改凭据</a-menu-item>
									<a-menu-item v-if="record.is_group_head" @click="openGroupRenewModal(record.id)">组内部分续期</a-menu-item>
									<a-menu-divider />
									<a-menu-item danger @click="removeOrder(record.id)">删除订单</a-menu-item>
									<a-menu-item v-if="record.status === 'disabled'" @click="activateOrder(record.id)">启用订单</a-menu-item>
									<a-menu-item v-else danger @click="deactivateOrder(record.id)">停用订单</a-menu-item>
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
		<div class="mt-3 flex justify-end">
			<a-pagination
				:current="orderPagination.current"
				:page-size="orderPagination.pageSize"
				:total="panel.orderList.total"
				:show-size-changer="false"
				@change="(page:number, pageSize:number) => { orderPagination.current = Number(page || 1); orderPagination.pageSize = Number(pageSize || 12); void loadOrdersView(orderPagination.current, orderPagination.pageSize) }"
			/>
		</div>
	</a-card>
</template>
