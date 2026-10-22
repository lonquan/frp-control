import {
  IconApps,
  IconDashboard,
  IconFile,
  IconLink,
  IconSafe,
  IconSettings,
} from '@arco-design/web-vue/es/icon'

export const clientNavItems = [
  { path: '/', label: '总览', icon: IconDashboard },
  { path: '/proxies', label: '代理', icon: IconLink },
  { path: '/visitors', label: '访问者', icon: IconDashboard },
  { path: '/captures', label: '流量捕获', icon: IconFile },
  { path: '/config', label: '配置', icon: IconApps },
  { path: '/certificates', label: '证书', icon: IconSafe },
  { path: '/settings', label: '设置', icon: IconSettings },
]
