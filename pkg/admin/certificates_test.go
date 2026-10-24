package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestCertificateAvailabilityAndWildcardCoverage(t *testing.T) {
	manager := newCertificateTestManager(t, nil)
	cert := &Certificate{
		ID:        "wildcard",
		Name:      "wildcard certificate",
		Domains:   []string{"*.example.com"},
		Provider:  "cloudflare",
		Email:     "admin@example.com",
		KeyType:   "ec256",
		Status:    "error",
		LastError: "renewal failed",
		NotAfter:  time.Now().Add(60 * 24 * time.Hour),
	}
	cert.CertPath, cert.KeyPath = writeCertificatePair(
		t,
		filepath.Join(manager.dataDir, "certificates", cert.ID),
		[]string{"*.example.com"},
		cert.NotAfter,
	)
	manager.records[cert.ID] = cert

	available := manager.Available()
	require.Len(t, available, 1)
	require.False(t, available[0].ExpiringSoon)
	require.Equal(t, "error", available[0].RenewalStatus)

	_, err := manager.ResolveAvailable(cert.ID, []string{"api.example.com"})
	require.NoError(t, err)
	_, err = manager.ResolveAvailable(cert.ID, []string{"example.com"})
	require.ErrorContains(t, err, "不覆盖")
	_, err = manager.ResolveAvailable(cert.ID, []string{"a.b.example.com"})
	require.ErrorContains(t, err, "不覆盖")
}

func TestCertificateDeleteBlockedWhenReferenced(t *testing.T) {
	manager := newCertificateTestManager(t, func(id, certPath, keyPath string) []string {
		require.Equal(t, "cert-id", id)
		require.NotEmpty(t, certPath)
		require.NotEmpty(t, keyPath)
		return []string{"api", "web"}
	})
	manager.records["cert-id"] = &Certificate{
		ID:       "cert-id",
		Name:     "in-use",
		CertPath: "/tmp/cert.pem",
		KeyPath:  "/tmp/key.pem",
	}

	err := manager.delete("cert-id")
	var inUse *CertificateInUseError
	require.True(t, errors.As(err, &inUse))
	require.Equal(t, []string{"api", "web"}, inUse.Proxies)
	require.Contains(t, manager.records, "cert-id")
}

func TestDNSProviderRegistryIsGeneratedAndExcludesInteractiveProviders(t *testing.T) {
	registry, err := loadDNSProviderRegistry()
	require.NoError(t, err)
	require.Equal(t, "v5.5.2", registry.LegoVer)
	require.Len(t, registry.Providers, 221)
	require.Nil(t, registry.provider("manual"))
	require.Nil(t, registry.provider("exec"))
	require.NotNil(t, registry.provider("cloudflare"))
	require.NotEmpty(t, registry.provider("cloudflare").Fields)

	goMod, err := os.ReadFile(filepath.Join("..", "..", "cmd", "frp-acme", "go.mod"))
	require.NoError(t, err)
	require.Contains(t, string(goMod), "github.com/go-acme/lego/v5 v5.5.2")
}

func TestDNSProviderConfigMasksAndPreservesCredentials(t *testing.T) {
	manager := newCertificateTestManager(t, nil)
	var checked map[string]string
	manager.providerCheck = func(provider string, credentials map[string]string) error {
		require.Equal(t, "cloudflare", provider)
		checked = copyStringMap(credentials)
		return nil
	}

	view, err := manager.saveDNSProviderConfig(saveDNSProviderRequest{
		Provider: "cloudflare",
		Credentials: map[string]string{
			"CF_DNS_API_TOKEN": "super-secret-token",
		},
	})
	require.NoError(t, err)
	require.True(t, view.Configured)
	require.Equal(t, "super-secret-token", checked["CF_DNS_API_TOKEN"])

	b, err := os.ReadFile(manager.providerConfigPath)
	require.NoError(t, err)
	require.Contains(t, string(b), "super-secret-token")
	info, err := os.Stat(manager.providerConfigPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	viewJSON, err := json.Marshal(manager.dnsProviderConfigView())
	require.NoError(t, err)
	require.NotContains(t, string(viewJSON), "super-secret-token")

	_, err = manager.saveDNSProviderConfig(saveDNSProviderRequest{
		Provider:    "cloudflare",
		Credentials: map[string]string{"CF_DNS_API_TOKEN": ""},
	})
	require.NoError(t, err)
	require.Equal(t, "super-secret-token", manager.providerConfig.Credentials["CF_DNS_API_TOKEN"])

	manager.providerCheck = func(string, map[string]string) error {
		return errors.New("invalid credential combination")
	}
	_, err = manager.saveDNSProviderConfig(saveDNSProviderRequest{
		Provider: "cloudflare",
		Credentials: map[string]string{
			"CF_DNS_API_TOKEN": "replacement",
		},
	})
	require.ErrorContains(t, err, "invalid credential combination")
	require.Equal(t, "super-secret-token", manager.providerConfig.Credentials["CF_DNS_API_TOKEN"])

	require.NoError(t, manager.clearDNSProviderConfig())
	require.False(t, manager.dnsProviderConfigView().Configured)

	t.Setenv("CF_DNS_API_TOKEN", "legacy-token")
	t.Setenv("ALICLOUD_ACCESS_KEY", "legacy-aliyun-key")
	env := manager.helperEnvironment(map[string]string{"CF_DNS_API_TOKEN": "configured-token"})
	require.Equal(t, 1, countEnvironmentName(env, "CF_DNS_API_TOKEN"))
	require.Contains(t, env, "CF_DNS_API_TOKEN=configured-token")
	require.Equal(t, 0, countEnvironmentName(env, "ALICLOUD_ACCESS_KEY"))
}

func TestCreateCertificateStartsIssueAndIsImmutable(t *testing.T) {
	manager := newCertificateTestManager(t, nil)
	configureTestDNSProvider(t, manager)
	certPEM, keyPEM := generateCertificatePair(t, []string{"example.com"}, time.Now().Add(90*24*time.Hour))
	started := make(chan struct{})
	release := make(chan struct{})
	manager.acmeRunner = func(_ context.Context, job acmeJob) error {
		close(started)
		<-release
		return writeCertificatePairBytes(job.CertPath, job.KeyPath, certPEM, keyPEM)
	}

	cert, err := manager.createAndIssue(certificateInput{
		Name:    "example",
		Domains: []string{"example.com"},
		Email:   "admin@example.com",
		KeyType: "ec256",
	})
	require.NoError(t, err)
	require.Equal(t, "issuing", cert.Status)
	require.Equal(t, "cloudflare", cert.Provider)

	<-started
	_, err = manager.createAndIssue(certificateInput{
		Name:    "example",
		Domains: []string{"example.com"},
		Email:   "admin@example.com",
		KeyType: "ec256",
	})
	require.ErrorIs(t, err, errCertificateNameExists)

	ok, err := manager.issueAsync(cert.ID)
	require.False(t, ok)
	require.ErrorContains(t, err, "已有签发任务")

	router := mux.NewRouter()
	manager.RegisterRoutes(router)
	request := httptest.NewRequest(http.MethodPut, "/api/v2/certificates/"+cert.ID, strings.NewReader("{}"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "不可修改")

	close(release)
	require.Eventually(t, func() bool {
		got, found := manager.get(cert.ID)
		return found && got.Status == "valid" && !hasTemporaryACMEJobs(manager)
	}, 3*time.Second, 20*time.Millisecond)
	got, found := manager.get(cert.ID)
	require.True(t, found)
	require.True(t, got.Downloadable)
	require.Equal(t, cert.ID, got.ID)
}

func TestRenewSingleOnlyTargetsOneCertificate(t *testing.T) {
	manager := newCertificateTestManager(t, nil)
	configureTestDNSProvider(t, manager)
	certPEM, keyPEM := generateCertificatePair(t, []string{"example.com"}, time.Now().Add(90*24*time.Hour))
	manager.acmeRunner = func(_ context.Context, job acmeJob) error {
		return writeCertificatePairBytes(job.CertPath, job.KeyPath, certPEM, keyPEM)
	}
	manager.records["one"] = testCertificate("one", "valid", time.Now().Add(60*24*time.Hour))
	manager.records["two"] = testCertificate("two", "valid", time.Now().Add(60*24*time.Hour))

	ok, err := manager.issueAsync("one")
	require.True(t, ok)
	require.NoError(t, err)
	require.Equal(t, "issuing", certificateStatus(t, manager, "one"))
	require.Equal(t, "valid", certificateStatus(t, manager, "two"))

	require.Eventually(t, func() bool {
		return certificateStatus(t, manager, "one") == "valid"
	}, 3*time.Second, 20*time.Millisecond)
	require.Equal(t, "valid", certificateStatus(t, manager, "two"))
}

func TestRenewAllUsesSerialQueueAndCoversEveryNonIssuingStatus(t *testing.T) {
	manager := newCertificateTestManager(t, nil)
	configureTestDNSProvider(t, manager)
	certPEM, keyPEM := generateCertificatePair(t, []string{"example.com"}, time.Now().Add(90*24*time.Hour))
	var active atomic.Int32
	var maximum atomic.Int32
	manager.acmeRunner = func(_ context.Context, job acmeJob) error {
		current := active.Add(1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		err := writeCertificatePairBytes(job.CertPath, job.KeyPath, certPEM, keyPEM)
		active.Add(-1)
		return err
	}

	now := time.Now()
	manager.records["pending"] = testCertificate("pending", "pending", time.Time{})
	manager.records["valid"] = testCertificate("valid", "valid", now.Add(60*24*time.Hour))
	manager.records["expired"] = testCertificate("expired", "valid", now.Add(-24*time.Hour))
	manager.records["error"] = testCertificate("error", "error", now.Add(30*24*time.Hour))
	manager.records["issuing"] = testCertificate("issuing", "issuing", time.Time{})

	result, err := manager.renewAll()
	require.NoError(t, err)
	require.Equal(t, 4, result.Accepted)
	require.Equal(t, 1, result.Skipped)
	require.Eventually(t, func() bool {
		for _, id := range []string{"pending", "valid", "expired", "error"} {
			if certificateStatus(t, manager, id) != "valid" {
				return false
			}
		}
		return !hasTemporaryACMEJobs(manager)
	}, 5*time.Second, 20*time.Millisecond)
	require.Equal(t, int32(1), maximum.Load())
	require.Equal(t, "issuing", certificateStatus(t, manager, "issuing"))
}

func TestExpiredAndInterruptedStatusViews(t *testing.T) {
	dataDir := t.TempDir()
	acmeDir := filepath.Join(dataDir, "acme")
	require.NoError(t, os.MkdirAll(acmeDir, 0o700))
	state := certificateState{Certificates: []*Certificate{
		{
			ID:        "expired",
			Name:      "expired",
			Domains:   []string{"example.com"},
			Email:     "admin@example.com",
			Status:    "valid",
			NotAfter:  time.Now().Add(-time.Hour),
			CreatedAt: time.Now().Add(-time.Hour),
		},
		{
			ID:        "interrupted",
			Name:      "interrupted",
			Domains:   []string{"example.org"},
			Email:     "admin@example.com",
			Status:    "issuing",
			CreatedAt: time.Now().Add(-time.Hour),
		},
	}}
	b, err := json.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(acmeDir, "certificates.json"), b, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(acmeDir, ".provider-check-stale.json"), []byte("{}"), 0o600))
	staleJobDir := filepath.Join(acmeDir, "certificates", "expired")
	require.NoError(t, os.MkdirAll(staleJobDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(staleJobDir, ".job-stale.json"), []byte("{}"), 0o600))

	manager, err := NewCertificateManager(dataDir, nil, nil)
	require.NoError(t, err)
	expired, ok := manager.get("expired")
	require.True(t, ok)
	require.Equal(t, "expired", expired.Status)
	interrupted, ok := manager.get("interrupted")
	require.True(t, ok)
	require.Equal(t, "error", interrupted.Status)
	require.Contains(t, interrupted.LastError, "中断")
	_, err = os.Stat(filepath.Join(acmeDir, ".provider-check-stale.json"))
	require.True(t, errors.Is(err, os.ErrNotExist))
	_, err = os.Stat(filepath.Join(staleJobDir, ".job-stale.json"))
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestCertificateDownloadZIPAndPathProtection(t *testing.T) {
	manager := newCertificateTestManager(t, nil)
	cert := testCertificate("download", "valid", time.Now().Add(-time.Hour))
	cert.CertPath, cert.KeyPath = writeCertificatePair(
		t,
		filepath.Join(manager.dataDir, "certificates", cert.ID),
		[]string{"example.com"},
		cert.NotAfter,
	)
	manager.records[cert.ID] = cert

	router := mux.NewRouter()
	manager.RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v2/certificates/download/download", nil),
	)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/zip", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "frp-cert-download.zip")
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))

	archive, err := zip.NewReader(bytes.NewReader(recorder.Body.Bytes()), int64(recorder.Body.Len()))
	require.NoError(t, err)
	require.Len(t, archive.File, 2)
	names := []string{archive.File[0].Name, archive.File[1].Name}
	require.ElementsMatch(t, []string{"cert.pem", "key.pem"}, names)
	for _, entry := range archive.File {
		if entry.Name == "key.pem" {
			require.Equal(t, os.FileMode(0o600), entry.Mode().Perm())
		}
		reader, err := entry.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		require.NotEmpty(t, content)
	}

	_, err = os.Stat(filepath.Join(manager.dataDir, "admin", "audit.log"))
	require.True(t, errors.Is(err, os.ErrNotExist))

	cert.CertPath = "/tmp/outside-cert.pem"
	recorder = httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v2/certificates/download/download", nil),
	)
	require.Equal(t, http.StatusConflict, recorder.Code)

	cert.CertPath = filepath.Join(manager.dataDir, "certificates", cert.ID, "cert.pem")
	require.NoError(t, os.Remove(cert.CertPath))
	require.NoError(t, os.Symlink(cert.KeyPath, cert.CertPath))
	recorder = httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v2/certificates/download/download", nil),
	)
	require.Equal(t, http.StatusConflict, recorder.Code)
}

func newCertificateTestManager(
	t *testing.T,
	references func(id, certPath, keyPath string) []string,
) *CertificateManager {
	t.Helper()
	manager, err := NewCertificateManager(t.TempDir(), nil, references)
	require.NoError(t, err)
	manager.providerCheck = func(string, map[string]string) error { return nil }
	return manager
}

func configureTestDNSProvider(t *testing.T, manager *CertificateManager) {
	t.Helper()
	_, err := manager.saveDNSProviderConfig(saveDNSProviderRequest{
		Provider: "cloudflare",
		Credentials: map[string]string{
			"CF_DNS_API_TOKEN": "test-token",
		},
	})
	require.NoError(t, err)
}

func testCertificate(id, status string, notAfter time.Time) *Certificate {
	return &Certificate{
		ID:        id,
		Name:      id,
		Domains:   []string{"example.com"},
		Email:     "admin@example.com",
		KeyType:   "ec256",
		Status:    status,
		NotAfter:  notAfter,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func writeCertificatePair(
	t *testing.T,
	dir string,
	dnsNames []string,
	notAfter time.Time,
) (string, string) {
	t.Helper()
	certPEM, keyPEM := generateCertificatePair(t, dnsNames, notAfter)
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	require.NoError(t, writeCertificatePairBytes(certPath, keyPath, certPEM, keyPEM))
	return certPath, keyPath
}

func writeCertificatePairBytes(certPath, keyPath string, certPEM, keyPEM []byte) error {
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return err
	}
	return os.WriteFile(keyPath, keyPEM, 0o600)
}

func generateCertificatePair(
	t *testing.T,
	dnsNames []string,
	notAfter time.Time,
) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	require.NoError(t, err)
	notBefore := time.Now().Add(-time.Hour)
	if notAfter.Before(notBefore) {
		notBefore = notAfter.Add(-time.Hour)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: dnsNames[0]},
		DNSNames:     dnsNames,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func hasTemporaryACMEJobs(manager *CertificateManager) bool {
	entries, err := os.ReadDir(filepath.Join(manager.dataDir, "certificates"))
	if err != nil {
		return true
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(manager.dataDir, "certificates", entry.Name()))
		if err != nil {
			return true
		}
		for _, file := range files {
			if strings.HasPrefix(file.Name(), ".job-") {
				return true
			}
		}
	}
	return false
}

func certificateStatus(t *testing.T, manager *CertificateManager, id string) string {
	t.Helper()
	cert, ok := manager.get(id)
	require.True(t, ok)
	return cert.Status
}

func countEnvironmentName(env []string, name string) int {
	count := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, name+"=") {
			count++
		}
	}
	return count
}
