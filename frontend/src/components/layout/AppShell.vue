<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import {
	LogoutOutlined,
	MenuOutlined,
	NotificationOutlined,
	ReloadOutlined
} from '@ant-design/icons-vue'

const props = defineProps<{
	activeTab: string
	menuItems: Array<Record<string, any>>
	releaseVersionText: string
	notice: string
	error: string
	pendingRequests: number
	runningActivityCount: number
	activeRequests: Array<Record<string, any>>
	recentActivities: Array<Record<string, any>>
	taskLogs: Array<Record<string, any>>
}>()

const emit = defineEmits<{
	(e: 'menu-click', value: { key: string }): void
	(e: 'refresh'): void
	(e: 'logout'): void
}>()

const mobileMenuOpen = ref(false)
const activityDrawerOpen = ref(false)
const mobileViewport = ref(false)

const selectedKeys = computed(() => [props.activeTab])

function syncViewport() {
	mobileViewport.value = window.innerWidth < 992
	if (!mobileViewport.value) {
		mobileMenuOpen.value = false
	}
}

function onMenuClick(info: { key: string }) {
	emit('menu-click', info)
	if (mobileViewport.value) {
		mobileMenuOpen.value = false
	}
}

onMounted(() => {
	syncViewport()
	window.addEventListener('resize', syncViewport)
})

onBeforeUnmount(() => {
	window.removeEventListener('resize', syncViewport)
})
</script>

<template>
	<a-layout class="layout-root">
		<a-layout-sider
			v-if="!mobileViewport"
			theme="dark"
			width="228"
			collapsible
		>
			<div class="logo-row">
				<div>XrayTool</div>
				<div class="logo-version">{{ releaseVersionText }}</div>
			</div>
			<a-menu
				:selected-keys="selectedKeys"
				theme="dark"
				mode="inline"
				:items="menuItems"
				@click="onMenuClick"
			/>
		</a-layout-sider>

		<a-drawer
			v-model:open="mobileMenuOpen"
			placement="left"
			width="260"
			:body-style="{ padding: '12px 0', background: '#0f172a' }"
			:header-style="{ display: 'none' }"
		>
			<div class="mobile-drawer-logo">
				<div class="logo-row">
					<div>XrayTool</div>
					<div class="logo-version">{{ releaseVersionText }}</div>
				</div>
			</div>
			<a-menu
				:selected-keys="selectedKeys"
				theme="dark"
				mode="inline"
				:items="menuItems"
				@click="onMenuClick"
			/>
		</a-drawer>

		<a-layout>
			<a-layout-header class="layout-header">
				<div class="header-main">
					<a-button
						v-if="mobileViewport"
						class="menu-trigger"
						:icon="h(MenuOutlined)"
						@click="mobileMenuOpen = true"
					/>
					<div>
						<div class="title">XrayTool Managed Panel</div>
						<div class="subtitle">Ant Design Vue 管理面板风格 · 支持详情弹窗/批量操作/状态可视化</div>
						<div class="subtitle">{{ releaseVersionText }}</div>
					</div>
				</div>
				<a-space class="header-actions" wrap>
					<a-badge :count="pendingRequests + runningActivityCount" :offset="[-2, 4]">
						<a-button :icon="h(NotificationOutlined)" @click="activityDrawerOpen = true">操作中心</a-button>
					</a-badge>
					<a-button :icon="h(ReloadOutlined)" @click="emit('refresh')">刷新</a-button>
					<a-button :icon="h(LogoutOutlined)" @click="emit('logout')">退出</a-button>
				</a-space>
			</a-layout-header>

			<a-layout-content class="layout-content">
				<a-alert v-if="notice" :message="notice" class="mb-3" type="success" show-icon />
				<a-alert v-if="error" :message="error" class="mb-3" type="error" show-icon />
				<slot />
			</a-layout-content>
		</a-layout>

		<a-drawer
			v-model:open="activityDrawerOpen"
			title="操作中心"
			placement="right"
			width="420"
		>
			<a-row :gutter="12" class="mb-3">
				<a-col :span="12">
					<a-card size="small">
						<div class="ops-stat-label">请求进行中</div>
						<div class="ops-stat-value">{{ pendingRequests }}</div>
					</a-card>
				</a-col>
				<a-col :span="12">
					<a-card size="small">
						<div class="ops-stat-label">长任务运行中</div>
						<div class="ops-stat-value">{{ runningActivityCount }}</div>
					</a-card>
				</a-col>
			</a-row>

			<a-card size="small" title="当前请求" class="mb-3">
				<div v-if="!activeRequests.length" class="empty-text">当前没有进行中的请求。</div>
				<a-space v-else direction="vertical" style="width:100%">
					<div v-for="request in activeRequests" :key="request.id" class="feed-row">
						<div class="feed-title">{{ request.method }} {{ request.url }}</div>
						<div class="feed-detail">{{ request.sinceText }}</div>
					</div>
				</a-space>
			</a-card>

			<a-card size="small" title="最近操作" class="mb-3">
				<div v-if="!recentActivities.length" class="empty-text">还没有最近操作记录。</div>
				<a-space v-else direction="vertical" style="width:100%">
					<div v-for="entry in recentActivities" :key="entry.id" class="feed-row">
						<div class="feed-title">
							<a-tag :color="entry.status === 'success' ? 'green' : entry.status === 'error' ? 'red' : entry.status === 'warning' ? 'orange' : entry.status === 'running' ? 'blue' : 'default'">
								{{ entry.status }}
							</a-tag>
							<span>{{ entry.title }}</span>
						</div>
						<div class="feed-detail">{{ entry.detail }}</div>
						<div class="feed-meta">{{ entry.updated_at }}</div>
					</div>
				</a-space>
			</a-card>

			<a-card size="small" title="系统任务日志">
				<div v-if="!taskLogs.length" class="empty-text">暂无系统任务日志。</div>
				<a-space v-else direction="vertical" style="width:100%">
					<div v-for="item in taskLogs.slice(0, 8)" :key="item.id" class="feed-row">
						<div class="feed-title">
							<a-tag :color="item.level === 'error' ? 'red' : item.level === 'warn' ? 'orange' : 'blue'">{{ item.level }}</a-tag>
							<span>{{ item.message }}</span>
						</div>
						<div class="feed-meta">{{ item.created_at }}</div>
					</div>
				</a-space>
			</a-card>
		</a-drawer>
	</a-layout>
</template>

<style scoped>
.layout-root {
	min-height: 100vh;
}

.mobile-drawer-logo {
	padding: 0 12px 12px;
}

.logo-row {
	border-radius: 10px;
	background: rgba(255, 255, 255, 0.16);
	color: #fff;
	font-weight: 800;
	text-align: center;
	padding: 10px 8px;
	letter-spacing: 0.3px;
}

.logo-version {
	margin-top: 4px;
	font-size: 11px;
	font-weight: 600;
	color: rgba(255, 255, 255, 0.82);
	letter-spacing: 0;
}

.layout-header {
	background: #fff;
	border-bottom: 1px solid rgba(148, 163, 184, 0.28);
	min-height: 66px;
	padding: 10px 16px;
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	flex-wrap: wrap;
}

.header-main {
	display: flex;
	align-items: flex-start;
	gap: 12px;
}

.header-actions {
	margin-left: auto;
}

.menu-trigger {
	margin-top: 2px;
}

.title {
	font-size: 20px;
	font-weight: 800;
	color: #0f172a;
}

.subtitle {
	color: #64748b;
	font-size: 12px;
}

.layout-content {
	padding: 14px;
}

.ops-stat-label {
	font-size: 12px;
	color: #64748b;
}

.ops-stat-value {
	font-size: 28px;
	font-weight: 800;
	line-height: 1.1;
	color: #0f172a;
}

.feed-row {
	border: 1px solid rgba(148, 163, 184, 0.18);
	border-radius: 10px;
	padding: 10px;
}

.feed-title {
	display: flex;
	align-items: center;
	gap: 8px;
	font-weight: 600;
	color: #0f172a;
}

.feed-detail {
	margin-top: 6px;
	font-size: 12px;
	color: #475569;
	word-break: break-word;
}

.feed-meta {
	margin-top: 6px;
	font-size: 11px;
	color: #94a3b8;
}

.empty-text {
	font-size: 12px;
	color: #64748b;
}

@media (max-width: 768px) {
	.layout-header {
		align-items: flex-start;
	}

	.title {
		font-size: 18px;
	}

	.header-actions {
		width: 100%;
		justify-content: flex-end;
	}
}
</style>
