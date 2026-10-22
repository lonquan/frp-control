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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Domains     []string  `json:"domains"`
	Provider    string    `json:"provider,omitempty"`
	Email       string    `json:"email"`
	CAURL       string    `json:"caURL,omitempty"`
	KeyType     string    `json:"keyType"`
	Status      string    `json:"status"`
	LastError   string    `json:"lastError,omitempty"`
	CertPath    string    `json:"certPath,omitempty"`
	KeyPath     string    `json:"keyPath,omitempty"`
	NotAfter    time.Time `json:"notAfter,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	LastIssueAt time.Time `json:"lastIssueAt,omitempty"`
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

var errCertificateIssuing = errors.New("证书正在签发，不能删除")

func (e *CertificateInUseError) Error() string {
	return fmt.Sprintf("证书仍被代理引用: %s", strings.Join(e.Proxies, ", "))
}

type certificateState struct {
	Certificates []*Certificate `json:"certificates"`
}

type CertificateManager struct {
	mu            sync.Mutex
	dataDir       string
	statePath     string
	helperPath    string
	records       map[string]*Certificate
	reloadCerts   func()
	references    func(id, certPath, keyPath string) []string
	lastRenewScan time.Time
}

type acmeJob struct {
	Email       string   `json:"email"`
	Domains     []string `json:"domains"`
	Provider    string   `json:"provider,omitempty"`
	CAURL       string   `json:"caURL,omitempty"`
	KeyType     string   `json:"keyType"`
	CertPath    string   `json:"certPath"`
	KeyPath     string   `json:"keyPath"`
	AccountPath string   `json:"accountPath"`
}

func NewCertificateManager(dataDir string, reloadCerts func(), references func(id, certPath, keyPath string) []string) (*CertificateManager, error) {
	dir := filepath.Join(dataDir, "acme")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	manager := &CertificateManager{
		dataDir:     dir,
		statePath:   filepath.Join(dir, "certificates.json"),
		helperPath:  os.Getenv("FRP_ACME_HELPER"),
		records:     make(map[string]*Certificate),
		reloadCerts: reloadCerts,
		references:  references,
	}
	if manager.helperPath == "" {
		manager.helperPath = "/usr/bin/frp-acme"
	}
	if err := manager.load(); err != nil {
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
	for _, cert := range state.Certificates {
		m.records[cert.ID] = cert
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
	return atomicWrite(m.statePath, b, 0o600)
}

func (m *CertificateManager) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v2/certificates/available", m.handleAvailable).Methods(http.MethodGet)
	router.HandleFunc("/api/v2/certificates", m.handleListCreate).Methods(http.MethodGet, http.MethodPost)
	router.HandleFunc("/api/v2/certificates/{id}", m.handleUpdateDelete).Methods(http.MethodPut, http.MethodDelete)
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
	var body Certificate
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	cert, issueNow, err := m.create(body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if issueNow {
		_, _ = m.issueAsync(cert.ID)
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
	var body Certificate
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	cert, err := m.update(id, body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cert)
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

func (m *CertificateManager) list() []Certificate {
	m.mu.Lock()
	defer m.mu.Unlock()
	records := make([]Certificate, 0, len(m.records))
	for _, cert := range m.records {
		records = append(records, *cert)
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
		copy := *cert
		copy.Domains = append([]string(nil), cert.Domains...)
		records = append(records, copy)
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
	copy := *cert
	copy.Domains = append([]string(nil), cert.Domains...)
	return &copy, true
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

func (m *CertificateManager) create(input Certificate) (*Certificate, bool, error) {
	if err := validateCertificate(input); err != nil {
		return nil, false, err
	}
	issueNow := input.Status == "issue"
	now := time.Now()
	id, err := randomToken()
	if err != nil {
		return nil, false, err
	}
	id = id[:12]
	cert := &Certificate{
		ID:        id,
		Name:      strings.TrimSpace(input.Name),
		Domains:   normalizeDomains(input.Domains),
		Provider:  strings.TrimSpace(input.Provider),
		Email:     strings.TrimSpace(input.Email),
		CAURL:     strings.TrimSpace(input.CAURL),
		KeyType:   strings.TrimSpace(input.KeyType),
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if cert.KeyType == "" {
		cert.KeyType = "rsa2048"
	}
	m.mu.Lock()
	m.records[id] = cert
	err = m.saveLocked()
	m.mu.Unlock()
	if err != nil {
		return nil, false, err
	}
	copy := *cert
	return &copy, issueNow, nil
}

func (m *CertificateManager) update(id string, input Certificate) (*Certificate, error) {
	if err := validateCertificate(input); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cert := m.records[id]
	if cert == nil {
		return nil, errors.New("证书不存在")
	}
	cert.Name = strings.TrimSpace(input.Name)
	cert.Domains = normalizeDomains(input.Domains)
	cert.Provider = strings.TrimSpace(input.Provider)
	cert.Email = strings.TrimSpace(input.Email)
	cert.CAURL = strings.TrimSpace(input.CAURL)
	cert.KeyType = input.KeyType
	if cert.KeyType == "" {
		cert.KeyType = "rsa2048"
	}
	cert.UpdatedAt = time.Now()
	if err := m.saveLocked(); err != nil {
		return nil, err
	}
	copy := *cert
	copy.Domains = append([]string(nil), cert.Domains...)
	return &copy, nil
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
	m.mu.Lock()
	defer m.mu.Unlock()
	cert := m.records[id]
	if cert == nil || cert.Status == "issuing" {
		if cert == nil {
			return false, errors.New("证书不存在")
		}
		return false, errors.New("该证书已有签发任务正在执行")
	}
	if strings.TrimSpace(cert.Provider) == "" {
		cert.Status = "error"
		cert.LastError = "DNS 提供商不能为空"
		cert.UpdatedAt = time.Now()
		_ = m.saveLocked()
		return false, errors.New(cert.LastError)
	}
	cert.Status = "issuing"
	cert.LastError = ""
	cert.UpdatedAt = time.Now()
	copy := *cert
	copy.Domains = append([]string(nil), cert.Domains...)
	_ = m.saveLocked()
	go m.issue(id, &copy)
	return true, nil
}

func (m *CertificateManager) issue(id string, cert *Certificate) {
	certDir := filepath.Join(m.dataDir, "certificates", id)
	if err := os.MkdirAll(certDir, 0o700); err != nil {
		m.finishIssue(id, "", "", time.Time{}, err)
		return
	}
	job := acmeJob{
		Email:       cert.Email,
		Domains:     cert.Domains,
		Provider:    cert.Provider,
		CAURL:       cert.CAURL,
		KeyType:     cert.KeyType,
		CertPath:    filepath.Join(certDir, "cert.pem"),
		KeyPath:     filepath.Join(certDir, "key.pem"),
		AccountPath: filepath.Join(m.dataDir, "account"),
	}
	jobPath := filepath.Join(certDir, "job.json")
	b, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		m.finishIssue(id, "", "", time.Time{}, err)
		return
	}
	if err := atomicWrite(jobPath, b, 0o600); err != nil {
		m.finishIssue(id, "", "", time.Time{}, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.helperPath, "--job", jobPath)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = errors.New("ACME 任务超时")
		} else {
			err = fmt.Errorf("%w: %s", err, strings.TrimSpace(output.String()))
		}
		m.finishIssue(id, "", "", time.Time{}, err)
		return
	}
	notAfter, err := certificateNotAfter(job.CertPath)
	if err != nil {
		m.finishIssue(id, "", "", time.Time{}, err)
		return
	}
	m.finishIssue(id, job.CertPath, job.KeyPath, notAfter, nil)
	if m.reloadCerts != nil {
		m.reloadCerts()
	}
}

func (m *CertificateManager) finishIssue(id, certPath, keyPath string, notAfter time.Time, issueErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cert := m.records[id]
	if cert == nil {
		return
	}
	cert.UpdatedAt = time.Now()
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

func validateCertificate(cert Certificate) error {
	if strings.TrimSpace(cert.Name) == "" {
		return errors.New("证书名称不能为空")
	}
	domains := normalizeDomains(cert.Domains)
	if len(domains) == 0 {
		return errors.New("至少需要一个域名")
	}
	if strings.TrimSpace(cert.Provider) == "" {
		return errors.New("DNS-01 必须选择 DNS 提供商")
	}
	for _, domain := range domains {
		if strings.Contains(domain, " ") || !strings.Contains(domain, ".") {
			return fmt.Errorf("域名格式无效: %s", domain)
		}
	}
	if strings.TrimSpace(cert.Email) == "" {
		return errors.New("ACME 账号邮箱不能为空")
	}
	return nil
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

func certificateNotAfter(path string) (time.Time, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	for len(b) > 0 {
		block, rest := pem.Decode(b)
		if block == nil {
			break
		}
		b = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			return cert.NotAfter, nil
		}
	}
	return time.Time{}, errors.New("无法解析签发证书")
}
