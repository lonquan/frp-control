package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-acme/lego/v5/challenge"
	"github.com/stretchr/testify/require"
)

type testDNSProvider struct{}

func (p *testDNSProvider) Present(context.Context, string, string, string) error {
	return nil
}

func (p *testDNSProvider) CleanUp(context.Context, string, string, string) error {
	return nil
}

type testTimeoutDNSProvider struct {
	*testDNSProvider
	timeout  time.Duration
	interval time.Duration
}

func (p *testTimeoutDNSProvider) Timeout() (time.Duration, time.Duration) {
	return p.timeout, p.interval
}

type testSequentialDNSProvider struct {
	*testDNSProvider
	sequentialInterval time.Duration
}

func (p *testSequentialDNSProvider) Sequential() time.Duration {
	return p.sequentialInterval
}

func TestWrapDNSProviderTimeout(t *testing.T) {
	tests := []struct {
		name             string
		provider         challenge.Provider
		expectedTimeout  time.Duration
		expectedInterval time.Duration
	}{
		{
			name:             "provider without timeout",
			provider:         &testDNSProvider{},
			expectedTimeout:  minimumDNSPropagationTimeout,
			expectedInterval: minimumDNSPropagationInterval,
		},
		{
			name: "short provider timeout",
			provider: &testTimeoutDNSProvider{
				testDNSProvider: &testDNSProvider{},
				timeout:         60 * time.Second,
				interval:        5 * time.Second,
			},
			expectedTimeout:  minimumDNSPropagationTimeout,
			expectedInterval: 5 * time.Second,
		},
		{
			name: "short provider polling interval",
			provider: &testTimeoutDNSProvider{
				testDNSProvider: &testDNSProvider{},
				timeout:         60 * time.Second,
				interval:        time.Second,
			},
			expectedTimeout:  minimumDNSPropagationTimeout,
			expectedInterval: minimumDNSPropagationInterval,
		},
		{
			name: "long provider timeout",
			provider: &testTimeoutDNSProvider{
				testDNSProvider: &testDNSProvider{},
				timeout:         3 * time.Minute,
				interval:        7 * time.Second,
			},
			expectedTimeout:  3 * time.Minute,
			expectedInterval: 7 * time.Second,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := wrapDNSProviderTimeout(test.provider)
			timedProvider, ok := provider.(challenge.ProviderTimeout)
			require.True(t, ok)
			timeout, interval := timedProvider.Timeout()
			require.Equal(t, test.expectedTimeout, timeout)
			require.Equal(t, test.expectedInterval, interval)
		})
	}
}

func TestWrapDNSProviderTimeoutPreservesSequential(t *testing.T) {
	provider := wrapDNSProviderTimeout(&testSequentialDNSProvider{
		testDNSProvider:    &testDNSProvider{},
		sequentialInterval: 9 * time.Second,
	})

	sequentialProvider, ok := provider.(sequentialDNSProvider)
	require.True(t, ok)
	require.Equal(t, 9*time.Second, sequentialProvider.Sequential())
}

func TestWrapDNSProviderTimeoutDoesNotAddSequential(t *testing.T) {
	provider := wrapDNSProviderTimeout(&testDNSProvider{})
	_, ok := provider.(sequentialDNSProvider)
	require.False(t, ok)
}

func TestValidateJobRequiresDNSProvider(t *testing.T) {
	err := validateJob(job{
		Domains:     []string{"example.com"},
		CertPath:    "/tmp/cert.pem",
		KeyPath:     "/tmp/key.pem",
		AccountPath: "/tmp/account",
	})
	require.ErrorContains(t, err, "DNS provider is required")
}

func TestValidateJobAcceptsWildcardDNSJob(t *testing.T) {
	require.NoError(t, validateJob(job{
		Domains:     []string{"example.com", "*.example.com"},
		Provider:    "cloudflare",
		CertPath:    "/tmp/cert.pem",
		KeyPath:     "/tmp/key.pem",
		AccountPath: "/tmp/account",
	}))
}

func TestValidateProviderRejectsUnknownProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.json")
	b, err := json.Marshal(providerCheck{Provider: "not-a-provider"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0o600))

	err = validateProvider(path)
	require.ErrorContains(t, err, "not-a-provider")
}

func TestValidateProviderAcceptsConfiguredProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.json")
	b, err := json.Marshal(providerCheck{
		Provider: "cloudflare",
		ProviderEnv: map[string]string{
			"CF_DNS_API_TOKEN": "test-token",
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0o600))

	require.NoError(t, validateProvider(path))
}

func TestWriteCertificatePairRollsBackPrivateKeyOnFailure(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	require.NoError(t, os.Mkdir(certPath, 0o700))
	require.NoError(t, os.WriteFile(keyPath, []byte("old-key"), 0o600))

	err := writeCertificatePair(certPath, keyPath, []byte("new-cert"), []byte("new-key"))
	require.Error(t, err)
	key, readErr := os.ReadFile(keyPath)
	require.NoError(t, readErr)
	require.Equal(t, "old-key", string(key))
}
