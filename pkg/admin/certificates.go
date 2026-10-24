// Copyright 2026 The frp Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type Certificate struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Domains      []string  `json:"domains"`
	Provider     string    `json:"provider,omitempty"`
	Email        string    `json:"email"`
	CAURL        string    `json:"caURL,omitempty"`
	KeyType      string    `json:"keyType"`
	Status       string    `json:"status"`
	LastError    string    `json:"lastError,omitempty"`
	CertPath     string    `json:"certPath,omitempty"`
	KeyPath      string    `json:"keyPath,omitempty"`
	NotAfter     time.Time `json:"notAfter,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	LastIssueAt  time.Time `json:"lastIssueAt,omitempty"`
	Downloadable bool      `json:"downloadable"`
}

type certificateInput struct {
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
	Email   string   `json:"email"`
	CAURL   string   `json:"caURL,omitempty"`
	KeyType string   `json:"keyType"`
}

type RenewAllResult struct {
	Accepted int `json:"accepted"`
	Skipped  int `json:"skipped"`
}

type AvailableCertificate struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Domains       []string  `json:"domains"`
	NotAfter      time.Time `json:"notAfter"`
	DaysRemaining int       `json:"daysRemaining"`
	RenewalStatus string    `json:"renewalStatus"`
	LastError     string    `json:"lastError,omitempty"`
	ExpiringSoon  bool      `json:"expiringSoon"`
}

type CertificateMaterial struct {
	AvailableCertificate
	CertPath string
	KeyPath  string
	leaf     *x509.Certificate
}

type CertificateInUseError struct {
	Proxies []string
}

var (
	errCertificateIssuing       = errors.New("证书正在签发，不能删除")
	errCertificateImmutable     = errors.New("证书创建后不可修改，请删除后重新创建")
	errCertificateNameExists    = errors.New("证书名称已存在")
	errDNSProviderNotConfigured = errors.New("请先在 DNS 设置中配置 provider 和凭据")
)

func (e *CertificateInUseError) Error() string {
	return fmt.Sprintf("证书仍被代理引用: %s", strings.Join(e.Proxies, ", "))
}

type certificateState struct {
	Certificates []*Certificate `json:"certificates"`
}

type CertificateManager struct {
	mu                 sync.Mutex
	providerMu         sync.Mutex
	dataDir            string
	statePath          string
	providerConfigPath string
	helperPath         string
	records            map[string]*Certificate
	reloadCerts        func()
	references         func(id, certPath, keyPath string) []string
	lastRenewScan      time.Time
	providerRegistry   *dnsProviderRegistry
	providerConfig     dnsProviderConfigState
	providerCheck      func(provider string, credentials map[string]string) error
	acmeRunner         func(context.Context, acmeJob) error
	issueSem           chan struct{}
}

type acmeJob struct {
	ID          string            `json:"-"`
	Email       string            `json:"email"`
	Domains     []string          `json:"domains"`
	Provider    string            `json:"provider,omitempty"`
	CAURL       string            `json:"caURL,omitempty"`
	KeyType     string            `json:"keyType"`
	CertPath    string            `json:"certPath"`
	KeyPath     string            `json:"keyPath"`
	AccountPath string            `json:"accountPath"`
	ProviderEnv map[string]string `json:"providerEnv,omitempty"`
}

func NewCertificateManager(dataDir string, reloadCerts func(), references func(id, certPath, keyPath string) []string) (*CertificateManager, error) {
	dir := filepath.Join(dataDir, "acme")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	registry, err := loadDNSProviderRegistry()
	if err != nil {
		return nil, err
	}
	manager := &CertificateManager{
		dataDir:            dir,
		statePath:          filepath.Join(dir, "certificates.json"),
		providerConfigPath: filepath.Join(dir, "dns-provider.json"),
		helperPath:         os.Getenv("FRP_ACME_HELPER"),
		records:            make(map[string]*Certificate),
		reloadCerts:        reloadCerts,
		references:         references,
		providerRegistry:   registry,
		issueSem:           make(chan struct{}, 1),
	}
	if manager.helperPath == "" {
		manager.helperPath = "/usr/bin/frp-acme"
	}
	if err := manager.loadDNSProviderConfig(); err != nil {
		return nil, err
	}
	if err := manager.load(); err != nil {
		return nil, err
	}
	if err := cleanupACMETemporaryFiles(dir); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *CertificateManager) StartRenewal(ctx context.Context) {
	go func() {
		m.renewDue()
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.renewDue()
			}
		}
	}()
}

func (m *CertificateManager) load() error {
	b, err := os.ReadFile(m.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var state certificateState
	if err := json.Unmarshal(b, &state); err != nil {
		return err
	}
	changed := false
	for _, cert := range state.Certificates {
		if cert == nil || cert.ID == "" {
			continue
		}
		cert.Downloadable = false
		if cert.Status == "issuing" {
			cert.Status = "error"
			cert.LastError = "上次签发被中断"
			cert.UpdatedAt = time.Now()
			changed = true
		}
		m.records[cert.ID] = cert
	}
	if changed {
		return m.saveLocked()
	}
	return nil
}

func (m *CertificateManager) saveLocked() error {
	state := certificateState{Certificates: make([]*Certificate, 0, len(m.records))}
	for _, cert := range m.records {
		state.Certificates = append(state.Certificates, cert)
	}
	sort.Slice(state.Certificates, func(i, j int) bool {
		return state.Certificates[i].CreatedAt.Before(state.Certificates[j].CreatedAt)
	})
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(m.statePath, b)
}

func (m *CertificateManager) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v2/certificates/available", m.handleAvailable).Methods(http.MethodGet)
	m.RegisterDNSProviderRoutes(router)
	router.HandleFunc("/api/v2/certificates", m.handleListCreate).Methods(http.MethodGet, http.MethodPost)
	router.HandleFunc("/api/v2/certificates/renew-all", m.handleRenewAll).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/certificates/{id}", m.handleUpdateDelete).Methods(http.MethodPut, http.MethodDelete)
	router.HandleFunc("/api/v2/certificates/{id}/download", m.handleDownload).Methods(http.MethodGet)
	router.HandleFunc("/api/v2/certificates/{id}/issue", m.handleIssue).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/certificates/{id}/renew", m.handleIssue).Methods(http.MethodPost)
}

func (m *CertificateManager) handleAvailable(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, m.Available())
}

func (m *CertificateManager) handleListCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		records := m.list()
		writeJSON(w, http.StatusOK, records)
		return
	}
	var body certificateInput
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	cert, err := m.createAndIssue(body)
	if err != nil {
		if errors.Is(err, errDNSProviderNotConfigured) || errors.Is(err, errCertificateNameExists) {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, cert)
}

func (m *CertificateManager) handleUpdateDelete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if r.Method == http.MethodDelete {
		if err := m.delete(id); err != nil {
			var inUse *CertificateInUseError
			if errors.As(err, &inUse) || errors.Is(err, errCertificateIssuing) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusConflict, errCertificateImmutable.Error())
}

func (m *CertificateManager) handleIssue(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	if _, ok := m.get(id); !ok {
		writeError(w, http.StatusNotFound, "证书不存在")
		return
	}
	if ok, err := m.issueAsync(id); !ok {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
}

func (m *CertificateManager) handleRenewAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	result, err := m.renewAll()
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (m *CertificateManager) list() []Certificate {
	m.mu.Lock()
	defer m.mu.Unlock()
	records := make([]Certificate, 0, len(m.records))
	for _, cert := range m.records {
		records = append(records, m.certificateViewLocked(cert))
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return records
}

func (m *CertificateManager) Available() []AvailableCertificate {
	m.mu.Lock()
	records := make([]Certificate, 0, len(m.records))
	for _, cert := range m.records {
		snapshot := *cert
		snapshot.Domains = append([]string(nil), cert.Domains...)
		records = append(records, snapshot)
	}
	m.mu.Unlock()

	available := make([]AvailableCertificate, 0, len(records))
	for i := range records {
		material, err := resolveCertificateMaterial(&records[i])
		if err != nil {
			continue
		}
		available = append(available, material.AvailableCertificate)
	}
	sort.Slice(available, func(i, j int) bool {
		if available[i].Name == available[j].Name {
			return available[i].NotAfter.Before(available[j].NotAfter)
		}
		return available[i].Name < available[j].Name
	})
	return available
}

// ResolveAvailable validates a managed certificate for proxy use. When domains
// are provided, every explicit hostname must be covered by the certificate SAN.
func (m *CertificateManager) ResolveAvailable(id string, domains []string) (*CertificateMaterial, error) {
	cert, ok := m.get(id)
	if !ok {
		return nil, errors.New("证书不存在")
	}
	material, err := resolveCertificateMaterial(cert)
	if err != nil {
		return nil, err
	}
	for _, domain := range normalizeDomains(domains) {
		if err := material.leaf.VerifyHostname(domain); err != nil {
			return nil, fmt.Errorf("证书 %s 不覆盖域名 %s", cert.Name, domain)
		}
	}
	return material, nil
}

func (m *CertificateManager) get(id string) (*Certificate, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cert, ok := m.records[id]
	if !ok {
		return nil, false
	}
	snapshot := *cert
	snapshot.Domains = append([]string(nil), cert.Domains...)
	m.decorateCertificateLocked(&snapshot)
	return &snapshot, true
}

func resolveCertificateMaterial(cert *Certificate) (*CertificateMaterial, error) {
	if cert == nil {
		return nil, errors.New("证书不存在")
	}
	if cert.CertPath == "" || cert.KeyPath == "" {
		return nil, errors.New("证书文件尚未生成")
	}
	pair, err := tls.LoadX509KeyPair(cert.CertPath, cert.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("加载证书失败: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return nil, errors.New("证书链为空")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("解析证书失败: %w", err)
	}
	remaining := time.Until(leaf.NotAfter)
	if time.Now().Before(leaf.NotBefore) {
		return nil, errors.New("证书尚未生效")
	}
	if remaining <= 0 {
		return nil, errors.New("证书已过期")
	}
	days := int(remaining / (24 * time.Hour))
	return &CertificateMaterial{
		AvailableCertificate: AvailableCertificate{
			ID:            cert.ID,
			Name:          cert.Name,
			Domains:       append([]string(nil), leaf.DNSNames...),
			NotAfter:      leaf.NotAfter,
			DaysRemaining: days,
			RenewalStatus: cert.Status,
			LastError:     cert.LastError,
			ExpiringSoon:  remaining < 30*24*time.Hour,
		},
		CertPath: cert.CertPath,
		KeyPath:  cert.KeyPath,
		leaf:     leaf,
	}, nil
}

func (m *CertificateManager) createAndIssue(input certificateInput) (*Certificate, error) {
	if err := validateCertificateInput(input); err != nil {
		return nil, err
	}
	provider, err := m.dnsProviderSnapshot()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	id, err := randomToken()
	if err != nil {
		return nil, err
	}
	id = id[:12]
	cert := &Certificate{
		ID:        id,
		Name:      strings.TrimSpace(input.Name),
		Domains:   normalizeDomains(input.Domains),
		Provider:  provider.Provider,
		Email:     strings.TrimSpace(input.Email),
		CAURL:     strings.TrimSpace(input.CAURL),
		KeyType:   strings.TrimSpace(input.KeyType),
		Status:    "issuing",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if cert.KeyType == "" {
		cert.KeyType = "rsa2048"
	}
	job := m.newACMEJob(cert, provider)
	m.mu.Lock()
	for _, existing := range m.records {
		if strings.EqualFold(existing.Name, cert.Name) {
			m.mu.Unlock()
			return nil, errCertificateNameExists
		}
	}
	m.records[id] = cert
	err = m.saveLocked()
	if err != nil {
		delete(m.records, id)
	}
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	go m.issue(job)
	view := m.certificateView(cert)
	return &view, nil
}

func (m *CertificateManager) delete(id string) error {
	m.mu.Lock()
	cert := m.records[id]
	if cert == nil {
		m.mu.Unlock()
		return errors.New("证书不存在")
	}
	if cert.Status == "issuing" {
		m.mu.Unlock()
		return errCertificateIssuing
	}
	certPath, keyPath := cert.CertPath, cert.KeyPath
	m.mu.Unlock()

	if m.references != nil {
		proxies := m.references(id, certPath, keyPath)
		if len(proxies) > 0 {
			sort.Strings(proxies)
			return &CertificateInUseError{Proxies: proxies}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.records[id]; !ok {
		return errors.New("证书不存在")
	}
	delete(m.records, id)
	_ = os.RemoveAll(filepath.Join(m.dataDir, "certificates", id))
	return m.saveLocked()
}

func (m *CertificateManager) issueAsync(id string) (bool, error) {
	provider, err := m.dnsProviderSnapshot()
	if err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cert := m.records[id]
	if cert == nil {
		return false, errors.New("证书不存在")
	}
	if cert.Status == "issuing" {
		return false, errors.New("该证书已有签发任务正在执行")
	}
	job := m.newACMEJob(cert, provider)
	previousStatus := cert.Status
	previousError := cert.LastError
	previousProvider := cert.Provider
	previousUpdatedAt := cert.UpdatedAt
	cert.Status = "issuing"
	cert.LastError = ""
	cert.UpdatedAt = time.Now()
	cert.Provider = provider.Provider
	if err := m.saveLocked(); err != nil {
		cert.Status = previousStatus
		cert.LastError = previousError
		cert.Provider = previousProvider
		cert.UpdatedAt = previousUpdatedAt
		return false, err
	}
	go m.issue(job)
	return true, nil
}

func (m *CertificateManager) newACMEJob(cert *Certificate, provider dnsProviderConfigState) acmeJob {
	certDir := filepath.Join(m.dataDir, "certificates", cert.ID)
	return acmeJob{
		ID:          cert.ID,
		Email:       cert.Email,
		Domains:     append([]string(nil), cert.Domains...),
		Provider:    provider.Provider,
		ProviderEnv: copyStringMap(provider.Credentials),
		CAURL:       cert.CAURL,
		KeyType:     cert.KeyType,
		CertPath:    filepath.Join(certDir, "cert.pem"),
		KeyPath:     filepath.Join(certDir, "key.pem"),
		AccountPath: filepath.Join(m.dataDir, "account"),
	}
}

func (m *CertificateManager) issue(job acmeJob) {
	m.issueSem <- struct{}{}
	defer func() { <-m.issueSem }()

	certDir := filepath.Join(m.dataDir, "certificates", job.ID)
	if err := os.MkdirAll(certDir, 0o700); err != nil {
		m.finishIssue(job.ID, "", "", time.Time{}, err)
		return
	}
	b, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		m.finishIssue(job.ID, "", "", time.Time{}, err)
		return
	}
	jobFile, err := os.CreateTemp(certDir, ".job-*.json")
	if err != nil {
		m.finishIssue(job.ID, "", "", time.Time{}, err)
		return
	}
	jobPath := jobFile.Name()
	defer os.Remove(jobPath)
	if err := jobFile.Chmod(0o600); err != nil {
		jobFile.Close()
		m.finishIssue(job.ID, "", "", time.Time{}, err)
		return
	}
	if _, err := jobFile.Write(b); err != nil {
		jobFile.Close()
		m.finishIssue(job.ID, "", "", time.Time{}, err)
		return
	}
	if err := jobFile.Close(); err != nil {
		m.finishIssue(job.ID, "", "", time.Time{}, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	if m.acmeRunner != nil {
		err = m.acmeRunner(ctx, job)
	} else {
		err = m.runACMEHelper(ctx, jobPath, job.ProviderEnv)
	}
	if err != nil {
		m.finishIssue(job.ID, "", "", time.Time{}, err)
		return
	}
	notAfter, err := certificateNotAfter(job.CertPath, job.KeyPath)
	if err != nil {
		m.finishIssue(job.ID, "", "", time.Time{}, err)
		return
	}
	m.finishIssue(job.ID, job.CertPath, job.KeyPath, notAfter, nil)
	if m.reloadCerts != nil {
		m.reloadCerts()
	}
}

func (m *CertificateManager) runACMEHelper(ctx context.Context, jobPath string, providerEnv map[string]string) error {
	cmd := exec.CommandContext(ctx, m.helperPath, "--job", jobPath)
	cmd.Env = m.helperEnvironment(providerEnv)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.New("ACME 任务超时")
	}
	if message := strings.TrimSpace(output.String()); message != "" {
		return fmt.Errorf("%w: %s", err, message)
	}
	return err
}

func (m *CertificateManager) finishIssue(id, certPath, keyPath string, notAfter time.Time, issueErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cert := m.records[id]
	if cert == nil {
		return
	}
	cert.UpdatedAt = time.Now()
	cert.Downloadable = false
	if issueErr != nil {
		cert.Status = "error"
		cert.LastError = issueErr.Error()
	} else {
		cert.Status = "valid"
		cert.LastError = ""
		cert.CertPath = certPath
		cert.KeyPath = keyPath
		cert.NotAfter = notAfter
		cert.LastIssueAt = time.Now()
	}
	_ = m.saveLocked()
}

func (m *CertificateManager) renewAll() (RenewAllResult, error) {
	provider, err := m.dnsProviderSnapshot()
	if err != nil {
		return RenewAllResult{}, err
	}
	var jobs []acmeJob
	result := RenewAllResult{}
	previous := make(map[string]*Certificate)
	m.mu.Lock()
	for id, cert := range m.records {
		if cert.Status == "issuing" {
			result.Skipped++
			continue
		}
		snapshot := *cert
		snapshot.Domains = append([]string(nil), cert.Domains...)
		previous[id] = &snapshot
		job := m.newACMEJob(cert, provider)
		cert.Status = "issuing"
		cert.LastError = ""
		cert.Provider = provider.Provider
		cert.UpdatedAt = time.Now()
		jobs = append(jobs, job)
		result.Accepted++
	}
	if err := m.saveLocked(); err != nil {
		maps.Copy(m.records, previous)
		m.mu.Unlock()
		return RenewAllResult{}, err
	}
	m.mu.Unlock()
	for _, job := range jobs {
		go m.issue(job)
	}
	return result, nil
}

func (m *CertificateManager) renewDue() {
	m.mu.Lock()
	if time.Since(m.lastRenewScan) < time.Hour {
		m.mu.Unlock()
		return
	}
	m.lastRenewScan = time.Now()
	ids := make([]string, 0)
	for id, cert := range m.records {
		if cert.Status == "valid" && time.Until(cert.NotAfter) < 30*24*time.Hour {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		_, _ = m.issueAsync(id)
	}
}

func validateCertificateInput(input certificateInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("证书名称不能为空")
	}
	domains := normalizeDomains(input.Domains)
	if len(domains) == 0 {
		return errors.New("至少需要一个域名")
	}
	for _, domain := range domains {
		if strings.Contains(domain, " ") || !strings.Contains(domain, ".") {
			return fmt.Errorf("域名格式无效: %s", domain)
		}
	}
	if strings.TrimSpace(input.Email) == "" {
		return errors.New("ACME 账号邮箱不能为空")
	}
	return nil
}

func (m *CertificateManager) certificateView(cert *Certificate) Certificate {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.certificateViewLocked(cert)
}

func (m *CertificateManager) certificateViewLocked(cert *Certificate) Certificate {
	view := *cert
	view.Domains = append([]string(nil), cert.Domains...)
	view.Status = effectiveCertificateStatus(&view)
	_, err := m.secureCertificateFiles(&view)
	view.Downloadable = err == nil
	return view
}

func (m *CertificateManager) decorateCertificateLocked(cert *Certificate) {
	cert.Status = effectiveCertificateStatus(cert)
	_, err := m.secureCertificateFiles(cert)
	cert.Downloadable = err == nil
}

func effectiveCertificateStatus(cert *Certificate) string {
	if cert.Status == "issuing" || cert.Status == "pending" {
		return cert.Status
	}
	if !cert.NotAfter.IsZero() && !time.Now().Before(cert.NotAfter) {
		return "expired"
	}
	return cert.Status
}

func cleanupACMETemporaryFiles(root string) error {
	paths := []string{root}
	certificatesDir := filepath.Join(root, "certificates")
	entries, err := os.ReadDir(certificatesDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			paths = append(paths, filepath.Join(certificatesDir, entry.Name()))
		}
	}
	for _, dir := range paths {
		files, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, file := range files {
			name := file.Name()
			if file.IsDir() || (!strings.HasPrefix(name, ".job-") && !strings.HasPrefix(name, ".provider-check-")) {
				continue
			}
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func (m *CertificateManager) secureCertificateFiles(cert *Certificate) ([]string, error) {
	if cert == nil || cert.ID == "" || cert.CertPath == "" || cert.KeyPath == "" {
		return nil, errors.New("证书文件尚未生成")
	}
	if filepath.Base(cert.ID) != cert.ID || !filepath.IsLocal(cert.ID) {
		return nil, errors.New("证书 ID 无效")
	}
	certDir := filepath.Join(m.dataDir, "certificates", cert.ID)
	certPath := filepath.Join(certDir, "cert.pem")
	keyPath := filepath.Join(certDir, "key.pem")
	if !pathWithin(m.dataDir, certDir) {
		return nil, errors.New("证书目录路径无效")
	}
	dirInfo, err := os.Lstat(certDir)
	if err != nil {
		return nil, err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return nil, errors.New("证书目录不是普通目录")
	}
	if filepath.Clean(cert.CertPath) != certPath || filepath.Clean(cert.KeyPath) != keyPath {
		return nil, errors.New("证书文件路径无效")
	}
	for _, path := range []string{certPath, keyPath} {
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, errors.New("证书文件不是普通文件")
		}
	}
	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		return nil, fmt.Errorf("证书和私钥不匹配: %w", err)
	}
	return []string{certPath, keyPath}, nil
}

func (m *CertificateManager) handleDownload(w http.ResponseWriter, r *http.Request) {
	cert, ok := m.get(mux.Vars(r)["id"])
	if !ok {
		writeError(w, http.StatusNotFound, "证书不存在")
		return
	}
	paths, err := m.secureCertificateFiles(cert)
	if err != nil {
		writeError(w, http.StatusConflict, "证书或私钥不可下载: "+err.Error())
		return
	}
	certPEM, err := os.ReadFile(paths[0])
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	keyPEM, err := os.ReadFile(paths[1])
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	for _, file := range []struct {
		name string
		mode os.FileMode
		data []byte
	}{
		{name: "cert.pem", mode: 0o644, data: certPEM},
		{name: "key.pem", mode: 0o600, data: keyPEM},
	} {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		header.SetMode(file.mode)
		entry, err := zipWriter.CreateHeader(header)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := entry.Write(file.data); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := zipWriter.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=frp-cert-%s.zip", cert.ID))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive.Bytes())
}

func normalizeDomains(domains []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		result = append(result, domain)
	}
	return result
}

func certificateNotAfter(certPath, keyPath string) (time.Time, error) {
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return time.Time{}, err
	}
	if len(pair.Certificate) == 0 {
		return time.Time{}, errors.New("证书链为空")
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return time.Time{}, err
	}
	return cert.NotAfter, nil
}
