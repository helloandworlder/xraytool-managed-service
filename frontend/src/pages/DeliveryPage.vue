<script setup lang="ts">
defineProps<{
	orderForm: Record<string, any>
	panel: any
	dedicatedProtocolOptions: Array<Record<string, any>>
	manualHostIPOptions: Array<Record<string, any>>
	filteredDedicatedInboundsForCreate: Array<Record<string, any>>
	filteredDedicatedIngressesForCreate: Array<Record<string, any>>
	dedicatedProbeRunning: boolean
	dedicatedProbeMeta: Record<string, any>
	dedicatedProbeRows: Array<Record<string, any>>
	creatingOrder: boolean
	deliverySearchKeyword: string
	deliveryCustomerID: number
	deliveryMode: string
	deliveryRows: Array<Record<string, any>>
	deliveryPagination: Record<string, any>
	exportingOrderID: number | null
	copyingLinksOrderID: number | null
	formatTime: (value: string) => string
	exportOrder: (orderID: number) => void
	setQuickExpiry: (days: number, target: string) => void
	dedicatedLinesCount: (lines: string) => number
	residentialCredentialLinesCount: (lines: string) => number
	residentialCredentialPlaceholder: (strategy: string, isEdit?: boolean) => string
	downloadDedicatedCreateSample: () => void
	probeDedicatedCreateLines: () => void
	createOrder: () => void
	copyOrderLines: (record: any) => void
	copyOrderLinks: (record: any) => void
	copyLinksLabel: (record: any) => string
	resetOrderCredentials: (orderID: number) => void
	removeOrder: (orderID: number) => void
	loadDeliveryView: (page?: number, pageSize?: number) => void
}>()

const emit = defineEmits<{
	(e: 'update:deliverySearchKeyword', value: string): void
	(e: 'update:deliveryCustomerID', value: number): void
	(e: 'update:deliveryMode', value: string): void
}>()
</script>

<template>
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
				<a-space wrap>
					<a-button size="small" @click="setQuickExpiry(7, 'create')">7天</a-button>
					<a-button size="small" @click="setQuickExpiry(15, 'create')">15天</a-button>
					<a-button size="small" @click="setQuickExpiry(30, 'create')">30天</a-button>
					<a-button size="small" @click="setQuickExpiry(90, 'create')">90天</a-button>
				</a-space>
			</a-col>
		</a-row>
		<div v-if="panel.allocationPreview" class="mt-2">
			<a-alert type="info" show-icon :message="`可分配IP: ${panel.allocationPreview.available} / 池总量: ${panel.allocationPreview.pool_size} / 已被该客户占用: ${panel.allocationPreview.used_by_customer}`" />
		</div>
		<div v-if="orderForm.mode === 'manual'" class="mt-2">
			<a-select v-model:value="orderForm.manual_ip_ids" mode="multiple" style="width: 100%" placeholder="选择手动IP">
				<a-select-option v-for="ip in manualHostIPOptions" :key="ip.id" :value="ip.id">{{ ip.ip }}</a-select-option>
			</a-select>
		</div>
		<div v-if="orderForm.mode !== 'dedicated'" class="mt-2">
			<a-form-item label="家宽账号策略" class="mb-2">
				<a-radio-group v-model:value="orderForm.residential_credential_mode">
					<a-radio-button value="random">随机 User:Pass</a-radio-button>
					<a-radio-button value="custom">指定 User:Pass</a-radio-button>
				</a-radio-group>
			</a-form-item>
			<template v-if="orderForm.residential_credential_mode === 'custom'">
				<a-form-item label="指定方式" class="mb-2">
					<a-radio-group v-model:value="orderForm.residential_credential_strategy">
						<a-radio-button value="per_line">逐条指定</a-radio-button>
						<a-radio-button value="shared">整单共用 1 组</a-radio-button>
					</a-radio-group>
				</a-form-item>
				<a-textarea
					v-model:value="orderForm.residential_credential_lines"
					:rows="4"
					:placeholder="residentialCredentialPlaceholder(orderForm.residential_credential_strategy)"
				/>
			</template>
			<a-alert
				class="mt-2"
				type="info"
				show-icon
				:message="orderForm.residential_credential_mode === 'custom'
					? (orderForm.residential_credential_strategy === 'shared'
						? `已填写 ${residentialCredentialLinesCount(orderForm.residential_credential_lines)} 行；整单共用 1 组 User:Pass，并校验全局用户名唯一`
						: `已填写 ${residentialCredentialLinesCount(orderForm.residential_credential_lines)} 行；提交时会校验与数量一致，并校验全局用户名唯一`)
					: '随机模式会自动生成全局唯一的家宽用户名，避免运行时统计与路由冲突'"
			/>
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

	<a-card :bordered="false" title="发货控制台">
		<template #extra>
			<a-space wrap>
				<a-input :value="deliverySearchKeyword" style="width:220px" placeholder="搜索: 订单号/客户/域名/IP/账号" allow-clear @update:value="emit('update:deliverySearchKeyword', $event)" />
				<a-select :value="deliveryCustomerID" style="width:180px" placeholder="客户筛选" @update:value="emit('update:deliveryCustomerID', Number($event || 0))">
					<a-select-option :value="0">全部客户</a-select-option>
					<a-select-option v-for="c in panel.customers" :key="c.id" :value="c.id">{{ c.name }}</a-select-option>
				</a-select>
				<a-select :value="deliveryMode" style="width:140px" @update:value="emit('update:deliveryMode', String($event || 'all'))">
					<a-select-option value="all">全部类型</a-select-option>
					<a-select-option value="home">家宽</a-select-option>
					<a-select-option value="dedicated">专线</a-select-option>
				</a-select>
			</a-space>
		</template>
		<a-alert class="mb-3" type="info" show-icon message="订单管理与发货已分离：本页用于导出、复制发货内容、刷新家宽凭据。" />
		<a-table :data-source="deliveryRows" :row-key="(row:any)=>row.id" size="small" :pagination="false">
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
						<a-button v-if="record.mode === 'dedicated'" size="small" :loading="copyingLinksOrderID===record.id" @click="copyOrderLinks(record)">{{ copyLinksLabel(record) }}</a-button>
						<a-button v-if="record.mode === 'auto' || record.mode === 'manual' || record.mode === 'import'" size="small" @click="resetOrderCredentials(record.id)">刷新凭据</a-button>
						<a-button size="small" danger @click="removeOrder(record.id)">删除</a-button>
					</a-space>
				</template>
			</a-table-column>
		</a-table>
		<div class="mt-3 flex justify-end">
			<a-pagination
				:current="deliveryPagination.current"
				:page-size="deliveryPagination.pageSize"
				:total="panel.orderList.total"
				:show-size-changer="false"
				@change="(page:number, pageSize:number) => { deliveryPagination.current = Number(page || 1); deliveryPagination.pageSize = Number(pageSize || 12); void loadDeliveryView(deliveryPagination.current, deliveryPagination.pageSize) }"
			/>
		</div>
	</a-card>
</template>
