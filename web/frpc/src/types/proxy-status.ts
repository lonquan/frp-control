export interface ProxyStatus {
  name: string
  type: string
  status: string
  err: string
  local_addr: string
  plugin: string
  remote_addr: string
  certificate_id?: string
  source?: 'store' | 'config'
  [key: string]: any
}

export type StatusResponse = Record<string, ProxyStatus[]>

export interface ProxyConfigSummary {
  name: string
  type: string
  source: 'store' | 'config'
  enabled: boolean
  local_addr?: string
  remote_addr?: string
  plugin?: string
  certificate_id?: string
}

export interface VisitorStatus {
  name: string
  type: string
  status: string
  err: string
  bind_addr: string
  bind_port: number
  server_user?: string
  server_name?: string
  plugin?: string
}

export interface VisitorConfigSummary {
  name: string
  type: string
  source: 'store' | 'config'
  enabled: boolean
  secret_configured: boolean
  server_user?: string
  server_name?: string
  bind_addr?: string
  bind_port: number
  plugin?: string
  use_encryption: boolean
  use_compression: boolean
  xtcp_protocol?: string
  keep_tunnel_open?: boolean
  fallback_to?: string
}
