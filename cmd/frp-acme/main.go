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

package main

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/go-acme/lego/v5/certificate"
	"github.com/go-acme/lego/v5/lego"
	"github.com/go-acme/lego/v5/providers/dns"
	"github.com/go-acme/lego/v5/registration"
)

const defaultCA = "https://acme-v02.api.letsencrypt.org/directory"

type job struct {
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

type providerCheck struct {
	Provider    string            `json:"provider"`
	ProviderEnv map[string]string `json:"providerEnv"`
}

type account struct {
	Email        string
	registration *acme.ExtendedAccount
	key          crypto.Signer
}

func (a *account) GetEmail() string {
	return a.Email
}

func (a *account) GetRegistration() *acme.ExtendedAccount {
	return a.registration
}

func (a *account) GetPrivateKey() crypto.Signer {
	return a.key
}

func main() {
	jobPath := flag.String("job", "", "path to ACME job JSON")
	checkPath := flag.String("validate-provider", "", "path to DNS provider configuration JSON")
	flag.Parse()
	if (*jobPath == "") == (*checkPath == "") {
		log.Fatal("exactly one of --job or --validate-provider is required")
	}
	if *checkPath != "" {
		if err := validateProvider(*checkPath); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := run(*jobPath); err != nil {
		log.Fatal(err)
	}
}

func run(jobPath string) error {
	b, err := os.ReadFile(jobPath)
	if err != nil {
		return err
	}
	var request job
	if err := json.Unmarshal(b, &request); err != nil {
		return err
	}
	if err := validateJob(request); err != nil {
		return err
	}
	applyProviderEnv(request.ProviderEnv)
	if err := os.MkdirAll(request.AccountPath, 0o700); err != nil {
		return err
	}
	key, err := loadOrCreateAccountKey(filepath.Join(request.AccountPath, "account.key"))
	if err != nil {
		return err
	}
	user := &account{Email: request.Email, key: key}
	config := lego.NewConfig(user)
	config.CADirURL = strings.TrimSpace(request.CAURL)
	if config.CADirURL == "" {
		config.CADirURL = defaultCA
	}
	client, err := lego.NewClient(config)
	if err != nil {
		return err
	}
	provider, err := dns.NewDNSChallengeProviderByName(strings.TrimSpace(request.Provider))
	if err != nil {
		return fmt.Errorf("create DNS provider %q: %w", request.Provider, err)
	}
	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	registrationResource, err := client.Registration.Register(ctx, registration.RegisterOptions{
		TermsOfServiceAgreed: true,
	})
	if err != nil {
		return fmt.Errorf("register ACME account: %w", err)
	}
	user.registration = registrationResource

	certificates, err := client.Certificate.Obtain(ctx, certificate.ObtainRequest{
		Domains: request.Domains,
		Bundle:  true,
		KeyType: parseKeyType(request.KeyType),
	})
	if err != nil {
		return fmt.Errorf("obtain certificate: %w", err)
	}
	if err := writeCertificatePair(
		request.CertPath,
		request.KeyPath,
		certificates.Certificate,
		certificates.PrivateKey,
	); err != nil {
		return err
	}
	return nil
}

func validateProvider(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var request providerCheck
	if err := json.Unmarshal(b, &request); err != nil {
		return err
	}
	if strings.TrimSpace(request.Provider) == "" {
		return errors.New("DNS provider is required")
	}
	applyProviderEnv(request.ProviderEnv)
	if _, err := dns.NewDNSChallengeProviderByName(strings.TrimSpace(request.Provider)); err != nil {
		return fmt.Errorf("create DNS provider %q: %w", request.Provider, err)
	}
	return nil
}

func applyProviderEnv(values map[string]string) {
	for name, value := range values {
		_ = os.Setenv(name, value)
	}
}

func validateJob(request job) error {
	if len(request.Domains) == 0 {
		return errors.New("no domains supplied")
	}
	if strings.TrimSpace(request.Provider) == "" {
		return errors.New("DNS provider is required")
	}
	if strings.TrimSpace(request.CertPath) == "" || strings.TrimSpace(request.KeyPath) == "" {
		return errors.New("certificate and key paths are required")
	}
	if request.AccountPath == "" {
		return errors.New("account path is required")
	}
	return nil
}

func loadOrCreateAccountKey(path string) (*ecdsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(b)
		if block == nil {
			return nil, errors.New("invalid account key")
		}
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := writeFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func parseKeyType(value string) certcrypto.KeyType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "rsa4096":
		return certcrypto.RSA4096
	case "ec256":
		return certcrypto.EC256
	case "ec384":
		return certcrypto.EC384
	default:
		return certcrypto.RSA2048
	}
}

func writeFile(path string, content []byte, mode os.FileMode) error {
	tmpName, err := stageFile(path, content, mode)
	if err != nil {
		return err
	}
	defer os.Remove(tmpName)
	return os.Rename(tmpName, path)
}

func writeCertificatePair(certPath, keyPath string, certContent, keyContent []byte) error {
	oldKey, err := os.ReadFile(keyPath)
	oldKeyExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	certTmp, err := stageFile(certPath, certContent, 0o644)
	if err != nil {
		return err
	}
	defer os.Remove(certTmp)
	keyTmp, err := stageFile(keyPath, keyContent, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(keyTmp)
	if err := os.Rename(keyTmp, keyPath); err != nil {
		return err
	}
	if err := os.Rename(certTmp, certPath); err != nil {
		if oldKeyExists {
			if restoreErr := writeFile(keyPath, oldKey, 0o600); restoreErr != nil {
				return fmt.Errorf("replace certificate: %w; restore private key: %v", err, restoreErr)
			}
		} else if removeErr := os.Remove(keyPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("replace certificate: %w; remove new private key: %v", err, removeErr)
		}
		return err
	}
	return nil
}

func stageFile(path string, content []byte, mode os.FileMode) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
