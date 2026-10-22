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
