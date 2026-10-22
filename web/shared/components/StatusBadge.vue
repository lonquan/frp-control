<template>
  <span class="status-badge" :class="state">
    <span class="status-dot" aria-hidden="true" />
    <span>{{ label || fallbackLabel }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
  label?: string
}>()

const state = computed(() => {
  const status = props.status.toLowerCase()
  if (['running', 'online', 'active', 'ok', 'enabled', 'valid'].includes(status)) {
    return 'success'
  }
  if (
    ['starting', 'restarting', 'stopping', 'new', 'wait start', 'pending', 'checking', 'issuing'].includes(
      status,
    )
  ) {
    return 'warning'
  }
  if (
    [
      'error',
      'failed',
      'offline',
      'closed',
      'start error',
      'check failed',
      'unavailable',
      'unexpected-exit',
      'expired',
    ].includes(status)
  ) {
    return 'danger'
  }
  return 'neutral'
})

const fallbackLabel = computed(() => {
  const labels: Record<string, string> = {
    running: '运行中',
    online: '在线',
    offline: '离线',
    closed: '已关闭',
    new: '新建',
    'wait start': '等待启动',
    'start error': '启动失败',
    'check failed': '检查失败',
    active: '可用',
    valid: '有效',
    expired: '已过期',
    issuing: '签发中',
    pending: '待签发',
    error: '失败',
    ok: '正常',
    checking: '检查中',
    starting: '启动中',
    restarting: '重启中',
    stopping: '停止中',
    unmanaged: '未托管',
    unavailable: '不可用',
    'unexpected-exit': '异常退出',
  }
  return labels[props.status.toLowerCase()] || props.status
})
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  min-height: 26px;
  align-items: center;
  gap: 7px;
  border: 1px solid transparent;
  border-radius: 999px;
  padding: 2px 10px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.success {
  border-color: rgb(var(--success-3));
  background: rgb(var(--success-1));
  color: rgb(var(--success-7));
}

.warning {
  border-color: rgb(var(--warning-3));
  background: rgb(var(--warning-1));
  color: rgb(var(--warning-7));
}

.danger {
  border-color: rgb(var(--danger-3));
  background: rgb(var(--danger-1));
  color: rgb(var(--danger-7));
}

.neutral {
  border-color: var(--frp-border);
  background: var(--frp-surface-muted);
  color: var(--frp-muted-strong);
}
</style>
