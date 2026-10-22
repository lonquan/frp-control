import { describe, expect, it } from 'vitest'
import {
  certificateProviderLabel,
  certificateProviderSelection,
  commonCertificateProviders,
  customCertificateProviderValue,
  isCommonCertificateProvider,
} from '../src/types/certificate-provider'

describe('certificate provider options', () => {
  it('lists the supported common providers', () => {
    expect(commonCertificateProviders.map((option) => option.value)).toEqual(
      expect.arrayContaining([
        'cloudflare',
        'alidns',
        'tencentcloud',
        'huaweicloud',
        'route53',
      ]),
    )
  })

  it('uses the common provider selection for known values', () => {
    expect(isCommonCertificateProvider('cloudflare')).toBe(true)
    expect(certificateProviderSelection('alidns')).toBe('alidns')
    expect(certificateProviderLabel('tencentcloud')).toBe('腾讯云 DNSPod')
  })

  it('preserves unknown lego providers through the custom selection', () => {
    expect(isCommonCertificateProvider('customdns')).toBe(false)
    expect(certificateProviderSelection('customdns')).toBe(
      customCertificateProviderValue,
    )
    expect(certificateProviderLabel('customdns')).toBe('customdns')
  })
})
