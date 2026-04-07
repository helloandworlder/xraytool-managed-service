<script setup lang="ts">
import { DeleteOutlined, EditOutlined } from '@ant-design/icons-vue'
import { h } from 'vue'

defineProps<{
	customerForm: Record<string, any>
	panel: any
	customerColumns: Array<Record<string, any>>
	createCustomer: () => void
	openCustomerEdit: (record: any) => void
	deleteCustomer: (id: number) => void
}>()
</script>

<template>
	<a-row :gutter="12">
		<a-col :xs="24" :lg="8">
			<a-card :bordered="false" title="创建客户">
				<a-form layout="vertical">
					<a-form-item label="客户名"><a-input v-model:value="customerForm.name" /></a-form-item>
					<a-form-item label="客户代号"><a-input v-model:value="customerForm.code" placeholder="如 liunian" /></a-form-item>
					<a-form-item label="联系方式"><a-input v-model:value="customerForm.contact" /></a-form-item>
					<a-form-item label="备注"><a-textarea v-model:value="customerForm.notes" :rows="4" /></a-form-item>
					<a-button type="primary" block @click="createCustomer">创建客户</a-button>
				</a-form>
			</a-card>
		</a-col>
		<a-col :xs="24" :lg="16">
			<a-card :bordered="false" title="客户列表">
				<a-table
					:columns="customerColumns"
					:data-source="panel.customers"
					:pagination="{ pageSize: 10 }"
					:row-key="(row:any) => row.id"
					size="small"
				>
					<template #bodyCell="{ column, record }">
						<template v-if="column.dataIndex === 'status'">
							<a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status }}</a-tag>
						</template>
						<template v-else-if="column.key === 'action'">
							<a-space :size="2">
								<a-tooltip title="编辑客户">
									<a-button size="small" :icon="h(EditOutlined)" aria-label="编辑客户" @click="openCustomerEdit(record)" />
								</a-tooltip>
								<a-tooltip title="删除客户">
									<a-button size="small" danger :icon="h(DeleteOutlined)" aria-label="删除客户" @click="deleteCustomer(record.id)" />
								</a-tooltip>
							</a-space>
						</template>
					</template>
				</a-table>
			</a-card>
		</a-col>
	</a-row>
</template>
