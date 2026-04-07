<script setup lang="ts">
defineProps<{
	panel: any
	importForm: Record<string, any>
	singboxSelectedFiles: Array<string>
	allSingboxSelected: boolean
	selectableSingboxFiles: Array<string>
	previewingSingboxImport: boolean
	previewingImport: boolean
	confirmingImport: boolean
	importPreviewValid: boolean
	nodeForm: Record<string, any>
	nodeColumns: Array<Record<string, any>>
	importColumns: Array<Record<string, any>>
	migrationColumns: Array<Record<string, any>>
	setImportExpiryDays: (days: number) => void
	toggleSingboxSelectAll: (checked: boolean) => void
	scanSingboxConfigs: () => void
	previewSelectedSingboxFiles: () => void
	previewImport: () => void
	previewCrossNodeMigration: () => void
	confirmImport: () => void
	createNode: () => void
	removeNode: (id: number) => void
	migrationStateColor: (state: string) => string
}>()

const emit = defineEmits<{
	(e: 'update:singboxSelectedFiles', value: string[]): void
}>()
</script>

<template>
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
					<a-space wrap>
						<a-button @click="scanSingboxConfigs">扫描宿主机配置</a-button>
						<a-button type="primary" ghost :loading="previewingSingboxImport" :disabled="singboxSelectedFiles.length === 0" @click="previewSelectedSingboxFiles">预检已选文件</a-button>
					</a-space>
					<a-checkbox :checked="allSingboxSelected" @change="(e:any)=>toggleSingboxSelectAll(Boolean(e.target?.checked))">全选可导入文件</a-checkbox>
					<div class="max-h-52 overflow-auto rounded border border-slate-200 p-2">
						<a-checkbox-group :value="singboxSelectedFiles" style="width:100%" @update:value="emit('update:singboxSelectedFiles', $event)">
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
					<a-space wrap>
						<a-button size="small" @click="setImportExpiryDays(15)">15天(预设)</a-button>
						<a-button size="small" @click="setImportExpiryDays(30)">30天</a-button>
						<a-button size="small" @click="setImportExpiryDays(2)">2天</a-button>
					</a-space>
					<a-textarea v-model:value="importForm.lines" :rows="14" placeholder="每行: ip:port:user:pass" />
					<a-space wrap>
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
							<a-button danger size="small" @click="removeNode(record.id)">删除</a-button>
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
