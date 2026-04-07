<script setup lang="ts">
import { message, Modal } from 'ant-design-vue'
import { computed, ref } from 'vue'

const props = defineProps<{
	panel: any
	backupColumns: Array<Record<string, any>>
	saveSettings: () => void
	sendBarkTest: () => void
	openDedicatedManager: () => void
	exportBackupDirect: () => void
	createBackup: () => void
	bytesText: (value: number) => string
	formatTime: (value: string) => string
	downloadBackup: (name: string) => void
	restoreBackup: (name: string) => void
	deleteBackup: (name: string) => void
}>()

const repairingAll = ref(false)
const repairingUsername = ref('')

const allConflictOrderIDs = computed(() => {
	const set = new Set<number>()
	for (const row of props.panel.residentialCredentialConflicts || []) {
		for (const orderID of row.affected_order_ids || []) {
			set.add(Number(orderID))
		}
	}
	return Array.from(set.values()).sort((a, b) => a - b)
})

async function refreshConflicts() {
	try {
		await props.panel.loadResidentialCredentialConflicts()
		message.success('冲突列表已刷新')
	} catch (err) {
		props.panel.setError(err)
	}
}

async function repairConflict(orderIDs: number[], title: string) {
	if (!orderIDs.length) {
		message.warning('没有可修复的订单')
		return
	}
	Modal.confirm({
		title,
		content: `将刷新 ${orderIDs.length} 个订单的家宽凭据，旧账号会失效。是否继续？`,
		async onOk() {
			try {
				const results = await props.panel.repairResidentialCredentialConflicts(orderIDs)
				const failed = (results || []).filter((row: any) => !row.success)
				if (failed.length === 0) {
					message.success(`修复完成，共处理 ${orderIDs.length} 个订单`)
					return
				}
				const successCount = orderIDs.length - failed.length
				props.panel.setError(`部分修复失败，成功 ${successCount}，失败 ${failed.length}`)
			} catch (err) {
				props.panel.setError(err)
			}
		}
	})
}

async function repairAllConflicts() {
	repairingAll.value = true
	try {
		await repairConflict(allConflictOrderIDs.value, '一键修复全部家宽账号冲突')
	} finally {
		repairingAll.value = false
	}
}

async function repairOneConflict(username: string, orderIDs: number[]) {
	repairingUsername.value = username
	try {
		await repairConflict(orderIDs, `修复用户名 ${username} 的冲突`)
	} finally {
		repairingUsername.value = ''
	}
}
</script>

<template>
	<a-card :bordered="false" title="系统设置" class="max-w-4xl mb-3">
		<a-row :gutter="12">
			<a-col :xs="24" :md="12"><a-form-item label="默认入口端口"><a-input v-model:value="panel.settings.default_inbound_port" /></a-form-item></a-col>
			<a-col :xs="24" :md="12">
				<a-form-item label="Bark启用">
					<a-switch :checked="panel.settings.bark_enabled === 'true'" @change="(checked:boolean)=> panel.settings.bark_enabled = checked ? 'true' : 'false'" />
				</a-form-item>
			</a-col>
			<a-col :xs="24" :md="12"><a-form-item label="Bark地址"><a-input v-model:value="panel.settings.bark_base_url" /></a-form-item></a-col>
			<a-col :xs="24" :md="12"><a-form-item label="Bark设备Key"><a-input v-model:value="panel.settings.bark_device_key" /></a-form-item></a-col>
			<a-col :xs="24" :md="12"><a-form-item label="Bark分组"><a-input v-model:value="panel.settings.bark_group" /></a-form-item></a-col>
		</a-row>
		<div class="text-right">
			<a-space wrap>
				<a-button @click="sendBarkTest">发送测试通知</a-button>
				<a-button type="primary" @click="saveSettings">保存设置</a-button>
			</a-space>
		</div>
	</a-card>

	<a-card :bordered="false" title="设置-GoSea Light Telemetry" class="max-w-4xl mb-3">
		<a-row :gutter="12">
			<a-col :xs="24" :md="12">
				<a-form-item label="启用上报">
					<a-switch :checked="panel.settings.gosealight_telemetry_enabled === 'true'" @change="(checked:boolean)=> panel.settings.gosealight_telemetry_enabled = checked ? 'true' : 'false'" />
				</a-form-item>
			</a-col>
			<a-col :xs="24" :md="12"><a-form-item label="上报间隔(秒)"><a-input v-model:value="panel.settings.gosealight_telemetry_interval_seconds" placeholder="60" /></a-form-item></a-col>
			<a-col :xs="24" :md="12"><a-form-item label="GoSea-Light 地址"><a-input v-model:value="panel.settings.gosealight_base_url" placeholder="http://127.0.0.1:3000" /></a-form-item></a-col>
			<a-col :xs="24" :md="12"><a-form-item label="节点 ID"><a-input v-model:value="panel.settings.gosealight_node_id" placeholder="provider node id" /></a-form-item></a-col>
			<a-col :xs="24" :md="12"><a-form-item label="节点用户名"><a-input v-model:value="panel.settings.gosealight_node_username" /></a-form-item></a-col>
			<a-col :xs="24" :md="12"><a-form-item label="节点密码"><a-input-password v-model:value="panel.settings.gosealight_node_password" /></a-form-item></a-col>
		</a-row>
		<a-alert type="info" show-icon message="启用后会周期性向 GoSea-Light /api/nodes/telemetry/ingest 上报本机版本、能力、连接数、速度和 24h 流量。" />
	</a-card>

	<a-card :bordered="false" title="设置-专线" class="max-w-4xl mb-3">
		<a-alert type="info" show-icon class="mb-3" message="VLESS 的 Security / TLS / REALITY 已迁移到每个 Inbound 单独配置。" />
		<a-space wrap>
			<a-button @click="openDedicatedManager">管理 Inbound / Ingress</a-button>
			<span class="text-xs text-slate-500">在 VLESS Inbound 中可分别配置 none / tls / reality、flow、SNI、ShortID、公私钥等参数</span>
		</a-space>
	</a-card>

	<a-card :bordered="false" title="设置-家宽" class="max-w-4xl mb-3">
		<a-row :gutter="12">
			<a-col :xs="24" :md="12"><a-form-item label="家宽订单名称前缀"><a-input v-model:value="panel.settings.residential_name_prefix" placeholder="家宽-Socks5" /></a-form-item></a-col>
		</a-row>
		<a-alert type="info" show-icon message="forward 模式已废弃，家宽订单统一使用 家宽-自动 / 家宽-手动。" />
	</a-card>

	<a-card :bordered="false" title="家宽账号冲突治理" class="max-w-4xl mb-3">
		<template #extra>
			<a-space wrap>
				<a-button @click="refreshConflicts">刷新</a-button>
				<a-button type="primary" :disabled="!allConflictOrderIDs.length" :loading="repairingAll" @click="repairAllConflicts">一键修复全部</a-button>
			</a-space>
		</template>
		<a-alert
			class="mb-3"
			type="warning"
			show-icon
			message="如果历史上存在跨订单重复用户名，这里会列出来。修复会刷新受影响订单的家宽凭据，旧账号会立即失效。"
		/>
		<div v-if="!panel.residentialCredentialConflicts.length" class="text-sm text-slate-500">当前没有检测到跨订单重复的家宽用户名。</div>
		<a-table
			v-else
			:data-source="panel.residentialCredentialConflicts"
			size="small"
			:row-key="(row:any) => row.username"
			:pagination="{ pageSize: 6 }"
			:scroll="{ x: 960 }"
		>
			<a-table-column title="用户名" key="username" width="180">
				<template #default="{ record }"><span class="font-mono">{{ record.username }}</span></template>
			</a-table-column>
			<a-table-column title="涉及订单" data-index="order_count" key="order_count" width="100" />
			<a-table-column title="涉及线路" data-index="item_count" key="item_count" width="100" />
			<a-table-column title="订单" key="orders" width="420">
				<template #default="{ record }">
					<div class="text-xs leading-6">
						<div v-for="member in record.members" :key="`${record.username}-${member.order_id}-${member.ip}`">
							#{{ member.order_id }} / {{ member.order_no || '-' }} / {{ member.customer_name }} / {{ member.ip }}
						</div>
					</div>
				</template>
			</a-table-column>
			<a-table-column title="动作" key="action" width="140">
				<template #default="{ record }">
					<a-button size="small" type="primary" :loading="repairingUsername === record.username" @click="repairOneConflict(record.username, record.affected_order_ids)">修复</a-button>
				</template>
			</a-table-column>
		</a-table>
	</a-card>

	<a-card :bordered="false" title="数据库备份恢复" class="max-w-4xl">
		<template #extra>
			<a-space wrap>
				<a-button type="primary" ghost @click="exportBackupDirect">一键导出到本机</a-button>
				<a-button @click="panel.loadBackups">刷新</a-button>
				<a-button type="primary" @click="createBackup">创建备份</a-button>
			</a-space>
		</template>
		<a-table
			:columns="backupColumns"
			:data-source="panel.backups"
			size="small"
			:row-key="(row:any) => row.name"
			:pagination="{ pageSize: 8 }"
		>
			<template #bodyCell="{ column, record }">
				<template v-if="column.key === 'size'">
					{{ bytesText(record.size_bytes) }}
				</template>
				<template v-else-if="column.key === 'updated_at'">
					{{ formatTime(record.updated_at) }}
				</template>
				<template v-else-if="column.key === 'action'">
					<a-space :size="4" wrap>
						<a-button size="small" @click="downloadBackup(record.name)">下载</a-button>
						<a-button size="small" @click="restoreBackup(record.name)">恢复</a-button>
						<a-button size="small" danger @click="deleteBackup(record.name)">删除</a-button>
					</a-space>
				</template>
			</template>
		</a-table>
		<div class="mt-2 text-xs text-slate-500">恢复备份后服务会自动退出并由 systemd 拉起。</div>
	</a-card>
</template>
