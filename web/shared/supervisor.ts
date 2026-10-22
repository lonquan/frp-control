const supervisorStateLabels: Record<string, string> = {
  running: '运行中',
  starting: '启动中',
  restarting: '重启中',
  stopping: '停止中',
  unmanaged: '未托管',
  unavailable: '不可用',
  'unexpected-exit': '异常退出',
}

const dateTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  dateStyle: 'medium',
  timeStyle: 'short',
})

export function supervisorStateLabel(state?: string) {
  if (!state) return '-'
  return supervisorStateLabels[state.toLowerCase()] || state
}

export function formatDateTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) return '-'
  return dateTimeFormatter.format(date)
}
