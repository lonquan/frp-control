export interface CertificateProviderOption {
  value: string
  label: string
  envHint?: string
}

export const customCertificateProviderValue = '__custom_provider__'

export const commonCertificateProviders: CertificateProviderOption[] = [
  {
    value: 'cloudflare',
    label: 'Cloudflare',
    envHint: 'CF_DNS_API_TOKEN',
  },
  {
    value: 'alidns',
    label: '阿里云 DNS',
    envHint: 'ALICLOUD_ACCESS_KEY 和 ALICLOUD_SECRET_KEY',
  },
  {
    value: 'tencentcloud',
    label: '腾讯云 DNSPod',
    envHint: 'TENCENTCLOUD_SECRET_ID 和 TENCENTCLOUD_SECRET_KEY',
  },
  { value: 'huaweicloud', label: '华为云 DNS' },
  { value: 'route53', label: 'AWS Route 53' },
  { value: 'gcloud', label: 'Google Cloud DNS' },
  { value: 'azuredns', label: 'Azure DNS' },
  { value: 'digitalocean', label: 'DigitalOcean' },
  { value: 'namecheap', label: 'Namecheap' },
  { value: 'namesilo', label: 'NameSilo' },
  { value: 'porkbun', label: 'Porkbun' },
]

export function isCommonCertificateProvider(provider: string): boolean {
  return commonCertificateProviders.some((option) => option.value === provider)
}

export function certificateProviderSelection(provider: string): string {
  return isCommonCertificateProvider(provider)
    ? provider
    : customCertificateProviderValue
}

export function certificateProviderLabel(provider?: string): string {
  if (!provider) return '-'
  return (
    commonCertificateProviders.find((option) => option.value === provider)
      ?.label || provider
  )
}
