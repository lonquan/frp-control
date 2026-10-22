package admin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCertificateAvailabilityAndWildcardCoverage(t *testing.T) {
	manager, err := NewCertificateManager(t.TempDir(), nil, nil)
	require.NoError(t, err)
	cert := &Certificate{
		ID:        "wildcard",
		Name:      "wildcard certificate",
		Domains:   []string{"*.example.com"},
		Provider:  "cloudflare",
		Email:     "admin@example.com",
		KeyType:   "ec256",
		Status:    "error",
		LastError: "renewal failed",
	}
	cert.CertPath, cert.KeyPath = writeCertificatePair(t, t.TempDir(), []string{"*.example.com"}, time.Now().Add(60*24*time.Hour))
	manager.records[cert.ID] = cert

	available := manager.Available()
	require.Len(t, available, 1)
	require.True(t, available[0].ExpiringSoon == false)
	require.Equal(t, "error", available[0].RenewalStatus)

	_, err = manager.ResolveAvailable(cert.ID, []string{"api.example.com"})
	require.NoError(t, err)
	_, err = manager.ResolveAvailable(cert.ID, []string{"example.com"})
	require.ErrorContains(t, err, "不覆盖")
	_, err = manager.ResolveAvailable(cert.ID, []string{"a.b.example.com"})
	require.ErrorContains(t, err, "不覆盖")
}

func TestCertificateDeleteBlockedWhenReferenced(t *testing.T) {
	manager, err := NewCertificateManager(t.TempDir(), nil, func(id, certPath, keyPath string) []string {
		require.Equal(t, "cert-id", id)
		require.NotEmpty(t, certPath)
		require.NotEmpty(t, keyPath)
		return []string{"api", "web"}
	})
	require.NoError(t, err)
	manager.records["cert-id"] = &Certificate{
		ID:       "cert-id",
		Name:     "in-use",
		CertPath: "/tmp/cert.pem",
		KeyPath:  "/tmp/key.pem",
	}

	err = manager.delete("cert-id")
	var inUse *CertificateInUseError
	require.True(t, errors.As(err, &inUse))
	require.Equal(t, []string{"api", "web"}, inUse.Proxies)
	require.Contains(t, manager.records, "cert-id")
}

func TestCertificateRequiresDNSProvider(t *testing.T) {
	manager, err := NewCertificateManager(t.TempDir(), nil, nil)
	require.NoError(t, err)
	_, _, err = manager.create(Certificate{
		Name:    "missing provider",
		Domains: []string{"example.com"},
		Email:   "admin@example.com",
	})
	require.ErrorContains(t, err, "DNS 提供商")
	require.Empty(t, manager.records)
}

func writeCertificatePair(t *testing.T, dir string, dnsNames []string, notAfter time.Time) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: dnsNames[0]},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certPath, keyPath
}
