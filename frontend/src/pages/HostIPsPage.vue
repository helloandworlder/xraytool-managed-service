<script setup lang="ts">
defineProps<{
	probeForm: Record<string, any>
	probeResult: string
	panel: any
	hostColumns: Array<Record<string, any>>
	probePort: () => void
}>()
</script>

<template>
	<a-row :gutter="12">
		<a-col :xs="24" :lg="8">
			<a-card :bordered="false" title="IP扫描与端口检测" class="mb-3">
				<a-space direction="vertical" style="width: 100%">
					<a-button type="primary" block @click="panel.scanHostIPs">扫描本机IP</a-button>
					<a-divider style="margin: 8px 0" />
					<a-input v-model:value="probeForm.ip" addon-before="IP" />
					<a-input-number v-model:value="probeForm.port" addon-before="Port" style="width: 100%" />
					<a-button block @click="probePort">检测端口占用</a-button>
					<a-alert v-if="probeResult" :message="probeResult" type="info" show-icon />
				</a-space>
			</a-card>
		</a-col>
		<a-col :xs="24" :lg="16">
			<a-card :bordered="false" title="宿主机IP池">
				<a-table
					:columns="hostColumns"
					:data-source="panel.hostIPs"
					:row-key="(row:any) => row.id"
					size="small"
					:pagination="{ pageSize: 12 }"
				>
					<template #bodyCell="{ column, record }">
						<template v-if="column.key === 'ip'">
							<span class="font-mono">{{ record.ip }}</span>
						</template>
						<template v-else-if="column.key === 'is_public'">
							{{ record.is_public ? '是' : '否' }}
						</template>
						<template v-else-if="column.key === 'enabled'">
							<a-tag :color="record.enabled ? 'green' : 'red'">{{ record.enabled ? '启用' : '禁用' }}</a-tag>
						</template>
						<template v-else-if="column.key === 'action'">
							<a-switch :checked="record.enabled" @change="(checked:boolean) => panel.toggleHostIP(record.id, checked)" />
						</template>
					</template>
				</a-table>
			</a-card>
		</a-col>
	</a-row>
</template>
