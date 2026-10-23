export interface Envelope<T> {
  code: number
  msg: string
  data: T
}

export class APIError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

let csrfToken = ''
let onUnauthorized: (() => void) | null = null

export function setCSRFToken(token: string) {
  csrfToken = token
}

export function setUnauthorizedHandler(handler: () => void) {
  onUnauthorized = handler
}

async function parseError(response: Response): Promise<APIError> {
  const payload = (await response.json().catch(() => null)) as
    | Partial<Envelope<unknown>>
    | null
  return new APIError(
    response.status,
    payload?.msg || response.statusText || `HTTP ${response.status}`,
  )
}

export async function adminRequest<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  if (csrfToken && options.method && !['GET', 'HEAD'].includes(options.method)) {
    headers.set('X-CSRF-Token', csrfToken)
  }
  const response = await fetch(path, {
    ...options,
    credentials: 'include',
    headers,
  })
  if (response.status === 401) {
    csrfToken = ''
    onUnauthorized?.()
  }
  if (!response.ok) {
    throw await parseError(response)
  }
  if (response.status === 204) {
    return undefined as T
  }
  const payload = (await response.json()) as Envelope<T>
  if (typeof payload.code === 'number' && payload.code >= 400) {
    throw new APIError(payload.code, payload.msg)
  }
  return payload.data
}

export async function adminBlob(path: string, options: RequestInit = {}) {
  const headers = new Headers(options.headers)
  if (csrfToken && options.method && !['GET', 'HEAD'].includes(options.method)) {
    headers.set('X-CSRF-Token', csrfToken)
  }
  const response = await fetch(path, {
    ...options,
    credentials: 'include',
    headers,
  })
  if (!response.ok) {
    throw await parseError(response)
  }
  return response.blob()
}

export async function legacyRequest<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  if (csrfToken && options.method && !['GET', 'HEAD'].includes(options.method)) {
    headers.set('X-CSRF-Token', csrfToken)
  }
  const response = await fetch(path, {
    ...options,
    credentials: 'include',
    headers,
  })
  if (response.status === 401) {
    csrfToken = ''
    onUnauthorized?.()
  }
  if (!response.ok) {
    throw await parseError(response)
  }
  if (response.status === 204) {
    return undefined as T
  }
  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    return (await response.json()) as T
  }
  return (await response.text()) as T
}

export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

export const adminAPI = {
  login: (username: string, password: string) =>
    adminRequest<SessionInfo>('/api/v2/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  session: () => adminRequest<SessionInfo>('/api/v2/auth/session'),
  logout: () => adminRequest<void>('/api/v2/auth/logout', { method: 'POST' }),
  changePassword: (currentPassword: string, newPassword: string) =>
    adminRequest<{ message: string }>('/api/v2/auth/password', {
      method: 'POST',
      body: JSON.stringify({ currentPassword, newPassword }),
    }),
  configTree: () => adminRequest<ConfigTree>('/api/v2/config/tree'),
  configFile: (path: string) =>
    adminRequest<ConfigDocument>(
      `/api/v2/config/file?path=${encodeURIComponent(path)}`,
    ),
  saveConfigFile: (
    path: string,
    content: string,
    revision: string,
  ) =>
    adminRequest<ConfigDocument>(
      `/api/v2/config/file?path=${encodeURIComponent(path)}`,
      {
        method: 'PUT',
        body: JSON.stringify({ content, revision }),
      },
    ),
  createConfigFile: (path: string, content = '', include = true) =>
    adminRequest<ConfigDocument>('/api/v2/config/file/create', {
      method: 'POST',
      body: JSON.stringify({ path, content, include }),
    }),
  renameConfigFile: (
    oldPath: string,
    newPath: string,
    updateIncludes = true,
  ) =>
    adminRequest<ConfigDocument>('/api/v2/config/file/rename', {
      method: 'POST',
      body: JSON.stringify({ oldPath, newPath, updateIncludes }),
    }),
  deleteConfigFile: (path: string) =>
    adminRequest<void>(
      `/api/v2/config/file?path=${encodeURIComponent(path)}`,
      { method: 'DELETE' },
    ),
  validateConfig: (path: string, content: string) =>
    adminRequest<ConfigValidation>('/api/v2/config/validate', {
      method: 'POST',
      body: JSON.stringify({ path, content }),
    }),
  configHistory: (path: string) =>
    adminRequest<ConfigHistoryEntry[]>(
      `/api/v2/config/history?path=${encodeURIComponent(path)}`,
    ),
  restoreConfigHistory: (path: string, name: string) =>
    adminRequest<ConfigDocument>('/api/v2/config/history', {
      method: 'POST',
      body: JSON.stringify({ path, name }),
    }),
  restart: () =>
    adminRequest<{ message: string }>('/api/v2/system/restart', {
      method: 'POST',
    }),
  supervisor: () =>
    adminRequest<SupervisorStatus>('/api/v2/system/supervisor'),
  metrics: () =>
    adminRequest<Record<string, unknown>>('/api/v2/observability/summary'),
  runtime: () =>
    adminRequest<Record<string, unknown>>(
      '/api/v2/observability/runtime',
    ),
  diagnosticSettings: () =>
    adminRequest<{ pprofEnabled: boolean }>(
      '/api/v2/diagnostics/settings',
    ),
  saveDiagnosticSettings: (pprofEnabled: boolean) =>
    adminRequest<{ pprofEnabled: boolean }>(
      '/api/v2/diagnostics/settings',
      {
        method: 'PUT',
        body: JSON.stringify({ pprofEnabled }),
      },
    ),
  downloadGoroutine: () =>
    adminBlob('/api/v2/diagnostics/pprof/goroutine', { method: 'POST' }),
  downloadHeap: () =>
    adminBlob('/api/v2/diagnostics/pprof/heap', { method: 'POST' }),
  downloadCPU: (seconds = 15) =>
    adminBlob(
      `/api/v2/diagnostics/pprof/cpu?seconds=${encodeURIComponent(String(seconds))}`,
      { method: 'POST' },
    ),
  certificates: () => adminRequest<CertificateRecord[]>('/api/v2/certificates'),
  availableCertificates: () =>
    adminRequest<AvailableCertificate[]>('/api/v2/certificates/available'),
  dnsProviders: () =>
    adminRequest<DNSProviderDefinition[]>('/api/v2/certificates/dns-providers'),
  dnsProviderConfig: () =>
    adminRequest<DNSProviderConfig>('/api/v2/certificates/dns-provider'),
  saveDNSProviderConfig: (
    provider: string,
    credentials: Record<string, string>,
  ) =>
    adminRequest<DNSProviderConfig>('/api/v2/certificates/dns-provider', {
      method: 'PUT',
      body: JSON.stringify({ provider, credentials }),
    }),
  clearDNSProviderConfig: () =>
    adminRequest<void>('/api/v2/certificates/dns-provider', {
      method: 'DELETE',
    }),
  createCertificate: (certificate: CertificateInput) =>
    adminRequest<CertificateRecord>('/api/v2/certificates', {
      method: 'POST',
      body: JSON.stringify(certificate),
    }),
  deleteCertificate: (id: string) =>
    adminRequest<void>(`/api/v2/certificates/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),
  issueCertificate: (id: string) =>
    adminRequest<{ accepted: boolean }>(
      `/api/v2/certificates/${encodeURIComponent(id)}/issue`,
      { method: 'POST', body: '{}' },
    ),
  renewCertificate: (id: string) =>
    adminRequest<{ accepted: boolean }>(
      `/api/v2/certificates/${encodeURIComponent(id)}/renew`,
      { method: 'POST', body: '{}' },
    ),
  renewAllCertificates: () =>
    adminRequest<RenewAllResult>('/api/v2/certificates/renew-all', {
      method: 'POST',
      body: '{}',
    }),
  downloadCertificate: (id: string) =>
    adminBlob(`/api/v2/certificates/${encodeURIComponent(id)}/download`),
  captureSettings: () =>
    adminRequest<CaptureSettings>('/api/v2/captures/settings'),
  saveCaptureSettings: (settings: CaptureSettings) =>
    adminRequest<CaptureSettings>('/api/v2/captures/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),
  captures: (query: Record<string, string> = {}) => {
    const params = new URLSearchParams(query)
    return adminRequest<CaptureListItem[]>(`/api/v2/captures?${params}`)
  },
  clearCaptures: () =>
    adminRequest<void>('/api/v2/captures', {
      method: 'DELETE',
    }),
  capture: (id: string) =>
    adminRequest<CaptureRecord>(
      `/api/v2/captures/${encodeURIComponent(id)}`,
    ),
  deleteCapture: (id: string) =>
    adminRequest<void>(`/api/v2/captures/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),
  auditCaptureReveal: (id: string) =>
    adminRequest<{ recorded: boolean }>(
      `/api/v2/captures/${encodeURIComponent(id)}/reveal`,
      { method: 'POST', body: '{}' },
    ),
  replayCaptures: (ids: string[], concurrency = 1, intervalMs = 0) =>
    adminRequest<CaptureReplayResponse[]>('/api/v2/captures/replay', {
      method: 'POST',
      body: JSON.stringify({ ids, concurrency, intervalMs }),
    }),
}

export interface SessionInfo {
  username: string
  role: string
  csrfToken: string
  expiresAt: string
}

export interface ConfigFileInfo {
  path: string
  name: string
  size: number
  modTime: string
  main: boolean
  editable: boolean
  directory: boolean
}

export interface ConfigTree {
  root: string
  main: string
  files: ConfigFileInfo[]
}

export interface ConfigDocument {
  path: string
  content: string
  revision: string
  main: boolean
  format: string
}

export interface ConfigValidation {
  valid: boolean
  revision?: string
  errors?: string[]
  warnings?: string[]
}

export interface ConfigHistoryEntry {
  name: string
  path: string
  revision: string
  createdAt: string
}

export interface SupervisorStatus {
  state: string
  pid: number
  startedAt: string
  restartedAt?: string
  lastError?: string
  rollback?: boolean
}

export interface CertificateRecord {
  id: string
  name: string
  domains: string[]
  provider?: string
  email: string
  caURL?: string
  keyType: string
  status: string
  lastError?: string
  certPath?: string
  keyPath?: string
  notAfter?: string
  createdAt: string
  updatedAt: string
  lastIssueAt?: string
  downloadable: boolean
}

export interface AvailableCertificate {
  id: string
  name: string
  domains: string[]
  notAfter: string
  daysRemaining: number
  renewalStatus: string
  lastError?: string
  expiringSoon: boolean
}

export type CertificateInput = Pick<
  CertificateRecord,
  'name' | 'domains' | 'email' | 'caURL' | 'keyType'
>

export interface DNSProviderField {
  name: string
  label: string
}

export interface DNSProviderDefinition {
  code: string
  name: string
  url?: string
  fields?: DNSProviderField[]
}

export interface DNSProviderCredentialView extends DNSProviderField {
  configured: boolean
}

export interface DNSProviderConfig {
  configured: boolean
  provider?: string
  providerName?: string
  credentials: DNSProviderCredentialView[]
  updatedAt?: string
}

export interface RenewAllResult {
  accepted: number
  skipped: number
}

export interface CaptureSettings {
  enabled: boolean
  proxies: string[]
  excludePatterns: string[]
  ttlSeconds: number
  maxEntryBytes: number
  maxTotalBytes: number
}

export interface CaptureListItem {
  id: string
  proxy: string
  protocol: string
  method: string
  host: string
  path: string
  sourceAddr?: string
  sourcePort?: number
  responseStatus: number
  startedAt: string
  durationMs: number
  truncated: boolean
  webSocket: boolean
  size: number
}

export interface WebSocketFrame {
  direction: string
  opcode: number
  payload?: string
  at: string
}

export interface CaptureRecord extends CaptureListItem {
  scheme: string
  query: string
  requestHeaders?: Record<string, string[]>
  requestBody?: string
  responseHeaders?: Record<string, string[]>
  responseBody?: string
  webSocketFrames?: WebSocketFrame[]
  target: string
  error?: string
  replayResults?: Array<{
    startedAt: string
    status: number
    durationMs: number
    responseBody?: string
    error?: string
  }>
}

export interface CaptureReplayResponse {
  id: string
  result: {
    startedAt: string
    status: number
    durationMs: number
    responseBody?: string
    error?: string
  }
}
