package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
